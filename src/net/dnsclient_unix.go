// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd solaris

// DNS client: see RFC 1035.
// Has to be linked into package net for Dial.

// TODO(rsc):
//	Could potentially handle many outstanding lookups faster.
//	Could have a small cache.
//	Random UDP source port (net.Dial should do that for us).
//	Random request IDs.

package net

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"os"
	"sync"
	"time"

	"golang_org/x/net/dns/dnsmessage"
)

func newRequest(q dnsmessage.Question) (id uint16, udpReq, tcpReq []byte, err error) {
	id = uint16(rand.Int()) ^ uint16(time.Now().UnixNano())
	b := dnsmessage.NewBuilder(make([]byte, 2, 514), dnsmessage.Header{ID: id, RecursionDesired: true})
	b.EnableCompression()
	if err := b.StartQuestions(); err != nil {
		return 0, nil, nil, err
	}
	if err := b.Question(q); err != nil {
		return 0, nil, nil, err
	}
	tcpReq, err = b.Finish()
	udpReq = tcpReq[2:]
	l := len(tcpReq) - 2
	tcpReq[0] = byte(l >> 8)
	tcpReq[1] = byte(l)
	return id, udpReq, tcpReq, err
}

func checkResponse(reqID uint16, reqQues dnsmessage.Question, respHdr dnsmessage.Header, respQues dnsmessage.Question) bool {
	if !respHdr.Response {
		return false
	}
	if reqID != respHdr.ID {
		return false
	}
	if reqQues.Type != respQues.Type || reqQues.Class != respQues.Class || !equalASCIIName(reqQues.Name, respQues.Name) {
		return false
	}
	return true
}

func dnsPacketRoundTrip(c Conn, id uint16, query dnsmessage.Question, b []byte) (dnsmessage.Parser, dnsmessage.Header, error) {
	if _, err := c.Write(b); err != nil {
		return dnsmessage.Parser{}, dnsmessage.Header{}, err
	}

	b = make([]byte, 512) // see RFC 1035
	for {
		n, err := c.Read(b)
		if err != nil {
			return dnsmessage.Parser{}, dnsmessage.Header{}, err
		}
		var p dnsmessage.Parser
		// Ignore invalid responses as they may be malicious
		// forgery attempts. Instead continue waiting until
		// timeout. See golang.org/issue/13281.
		h, err := p.Start(b[:n])
		if err != nil {
			continue
		}
		q, err := p.Question()
		if err != nil || !checkResponse(id, query, h, q) {
			continue
		}
		return p, h, nil
	}
}

func dnsStreamRoundTrip(c Conn, id uint16, query dnsmessage.Question, b []byte) (dnsmessage.Parser, dnsmessage.Header, error) {
	if _, err := c.Write(b); err != nil {
		return dnsmessage.Parser{}, dnsmessage.Header{}, err
	}

	b = make([]byte, 1280) // 1280 is a reasonable initial size for IP over Ethernet, see RFC 4035
	if _, err := io.ReadFull(c, b[:2]); err != nil {
		return dnsmessage.Parser{}, dnsmessage.Header{}, err
	}
	l := int(b[0])<<8 | int(b[1])
	if l > len(b) {
		b = make([]byte, l)
	}
	n, err := io.ReadFull(c, b[:l])
	if err != nil {
		return dnsmessage.Parser{}, dnsmessage.Header{}, err
	}
	var p dnsmessage.Parser
	h, err := p.Start(b[:n])
	if err != nil {
		return dnsmessage.Parser{}, dnsmessage.Header{}, errors.New("cannot unmarshal DNS message")
	}
	q, err := p.Question()
	if err != nil {
		return dnsmessage.Parser{}, dnsmessage.Header{}, errors.New("cannot unmarshal DNS message")
	}
	if !checkResponse(id, query, h, q) {
		return dnsmessage.Parser{}, dnsmessage.Header{}, errors.New("invalid DNS response")
	}
	return p, h, nil
}

// exchange sends a query on the connection and hopes for a response.
func (r *Resolver) exchange(ctx context.Context, server string, q dnsmessage.Question, timeout time.Duration) (dnsmessage.Parser, dnsmessage.Header, error) {
	q.Class = dnsmessage.ClassINET
	id, udpReq, tcpReq, err := newRequest(q)
	if err != nil {
		return dnsmessage.Parser{}, dnsmessage.Header{}, errors.New("cannot marshal DNS message")
	}
	for _, network := range []string{"udp", "tcp"} {
		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(timeout))
		defer cancel()

		c, err := r.dial(ctx, network, server)
		if err != nil {
			return dnsmessage.Parser{}, dnsmessage.Header{}, err
		}
		if d, ok := ctx.Deadline(); ok && !d.IsZero() {
			c.SetDeadline(d)
		}
		var p dnsmessage.Parser
		var h dnsmessage.Header
		if network == "tcp" {
			p, h, err = dnsStreamRoundTrip(c, id, q, tcpReq)
		} else {
			p, h, err = dnsPacketRoundTrip(c, id, q, udpReq)
		}
		c.Close()
		if err != nil {
			return dnsmessage.Parser{}, dnsmessage.Header{}, mapErr(err)
		}
		if err := p.SkipQuestion(); err != dnsmessage.ErrSectionDone {
			return dnsmessage.Parser{}, dnsmessage.Header{}, errors.New("invalid DNS response")
		}
		if h.Truncated { // see RFC 5966
			continue
		}
		return p, h, nil
	}
	return dnsmessage.Parser{}, dnsmessage.Header{}, errors.New("no answer from DNS server")
}

func checkHeaders(p *dnsmessage.Parser, h dnsmessage.Header, name, server string) error {
	_, err := p.AnswerHeader()
	if err != nil && err != dnsmessage.ErrSectionDone {
		return &DNSError{
			Err:    "cannot unmarshal DNS message",
			Name:   name,
			Server: server,
		}
	}

	// libresolv continues to the next server when it receives
	// an invalid referral response. See golang.org/issue/15434.
	if h.RCode == dnsmessage.RCodeSuccess && !h.Authoritative && !h.RecursionAvailable && err == dnsmessage.ErrSectionDone {
		return &DNSError{Err: "lame referral", Name: name, Server: server}
	}

	// If answer errored for rcodes dnsRcodeSuccess or dnsRcodeNameError,
	// it means the response in msg was not useful and trying another
	// server probably won't help. Return now in those cases.
	// TODO: indicate this in a more obvious way, such as a field on DNSError?
	if h.RCode == dnsmessage.RCodeNameError {
		return &DNSError{Err: errNoSuchHost.Error(), Name: name, Server: server}
	}
	if h.RCode != dnsmessage.RCodeSuccess {
		// None of the error codes make sense
		// for the query we sent. If we didn't get
		// a name error and we didn't get success,
		// the server is behaving incorrectly or
		// having temporary trouble.
		err := &DNSError{Err: "server misbehaving", Name: name, Server: server}
		if h.RCode == dnsmessage.RCodeServerFailure {
			err.IsTemporary = true
		}
		return err
	}

	return nil
}

func skipToAnswer(p *dnsmessage.Parser, qtype dnsmessage.Type, name, server string) error {
	for {
		h, err := p.AnswerHeader()
		if err == dnsmessage.ErrSectionDone {
			return &DNSError{
				Err:    errNoSuchHost.Error(),
				Name:   name,
				Server: server,
			}
		}
		if err != nil {
			return &DNSError{
				Err:    "cannot unmarshal DNS message",
				Name:   name,
				Server: server,
			}
		}
		if h.Type == qtype {
			return nil
		}
		if err := p.SkipAnswer(); err != nil {
			return &DNSError{
				Err:    "cannot unmarshal DNS message",
				Name:   name,
				Server: server,
			}
		}
	}
}

// Do a lookup for a single name, which must be rooted
// (otherwise answer will not find the answers).
func (r *Resolver) tryOneName(ctx context.Context, cfg *dnsConfig, name string, qtype dnsmessage.Type) (dnsmessage.Parser, string, error) {
	var lastErr error
	serverOffset := cfg.serverOffset()
	sLen := uint32(len(cfg.servers))

	n, err := dnsmessage.NewName(name)
	if err != nil {
		return dnsmessage.Parser{}, "", errors.New("cannot marshal DNS message")
	}
	q := dnsmessage.Question{
		Name:  n,
		Type:  qtype,
		Class: dnsmessage.ClassINET,
	}

	for i := 0; i < cfg.attempts; i++ {
		for j := uint32(0); j < sLen; j++ {
			server := cfg.servers[(serverOffset+j)%sLen]

			p, h, err := r.exchange(ctx, server, q, cfg.timeout)
			if err != nil {
				lastErr = &DNSError{
					Err:    err.Error(),
					Name:   name,
					Server: server,
				}
				if nerr, ok := err.(Error); ok && nerr.Timeout() {
					lastErr.(*DNSError).IsTimeout = true
				}
				// Set IsTemporary for socket-level errors. Note that this flag
				// may also be used to indicate a SERVFAIL response.
				if _, ok := err.(*OpError); ok {
					lastErr.(*DNSError).IsTemporary = true
				}
				continue
			}

			lastErr = checkHeaders(&p, h, name, server)
			if lastErr != nil {
				continue
			}

			lastErr = skipToAnswer(&p, qtype, name, server)
			if lastErr == nil {
				return p, server, nil
			}
		}
	}
	return dnsmessage.Parser{}, "", lastErr
}

// A resolverConfig represents a DNS stub resolver configuration.
type resolverConfig struct {
	initOnce sync.Once // guards init of resolverConfig

	// ch is used as a semaphore that only allows one lookup at a
	// time to recheck resolv.conf.
	ch          chan struct{} // guards lastChecked and modTime
	lastChecked time.Time     // last time resolv.conf was checked

	mu        sync.RWMutex // protects dnsConfig
	dnsConfig *dnsConfig   // parsed resolv.conf structure used in lookups
}

var resolvConf resolverConfig

// init initializes conf and is only called via conf.initOnce.
func (conf *resolverConfig) init() {
	// Set dnsConfig and lastChecked so we don't parse
	// resolv.conf twice the first time.
	conf.dnsConfig = systemConf().resolv
	if conf.dnsConfig == nil {
		conf.dnsConfig = dnsReadConfig("/etc/resolv.conf")
	}
	conf.lastChecked = time.Now()

	// Prepare ch so that only one update of resolverConfig may
	// run at once.
	conf.ch = make(chan struct{}, 1)
}

// tryUpdate tries to update conf with the named resolv.conf file.
// The name variable only exists for testing. It is otherwise always
// "/etc/resolv.conf".
func (conf *resolverConfig) tryUpdate(name string) {
	conf.initOnce.Do(conf.init)

	// Ensure only one update at a time checks resolv.conf.
	if !conf.tryAcquireSema() {
		return
	}
	defer conf.releaseSema()

	now := time.Now()
	if conf.lastChecked.After(now.Add(-5 * time.Second)) {
		return
	}
	conf.lastChecked = now

	var mtime time.Time
	if fi, err := os.Stat(name); err == nil {
		mtime = fi.ModTime()
	}
	if mtime.Equal(conf.dnsConfig.mtime) {
		return
	}

	dnsConf := dnsReadConfig(name)
	conf.mu.Lock()
	conf.dnsConfig = dnsConf
	conf.mu.Unlock()
}

func (conf *resolverConfig) tryAcquireSema() bool {
	select {
	case conf.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

func (conf *resolverConfig) releaseSema() {
	<-conf.ch
}

func (r *Resolver) lookup(ctx context.Context, name string, qtype dnsmessage.Type) (dnsmessage.Parser, string, error) {
	if !isDomainName(name) {
		// We used to use "invalid domain name" as the error,
		// but that is a detail of the specific lookup mechanism.
		// Other lookups might allow broader name syntax
		// (for example Multicast DNS allows UTF-8; see RFC 6762).
		// For consistency with libc resolvers, report no such host.
		return dnsmessage.Parser{}, "", &DNSError{Err: errNoSuchHost.Error(), Name: name}
	}
	resolvConf.tryUpdate("/etc/resolv.conf")
	resolvConf.mu.RLock()
	conf := resolvConf.dnsConfig
	resolvConf.mu.RUnlock()
	var (
		p      dnsmessage.Parser
		server string
		err    error
	)
	for _, fqdn := range conf.nameList(name) {
		p, server, err = r.tryOneName(ctx, conf, fqdn, qtype)
		if err == nil {
			break
		}
		if nerr, ok := err.(Error); ok && nerr.Temporary() && r.StrictErrors {
			// If we hit a temporary error with StrictErrors enabled,
			// stop immediately instead of trying more names.
			break
		}
	}
	if err == nil {
		return p, server, nil
	}
	if err, ok := err.(*DNSError); ok {
		// Show original name passed to lookup, not suffixed one.
		// In general we might have tried many suffixes; showing
		// just one is misleading. See also golang.org/issue/6324.
		err.Name = name
	}
	return dnsmessage.Parser{}, "", err
}

// avoidDNS reports whether this is a hostname for which we should not
// use DNS. Currently this includes only .onion, per RFC 7686. See
// golang.org/issue/13705. Does not cover .local names (RFC 6762),
// see golang.org/issue/16739.
func avoidDNS(name string) bool {
	if name == "" {
		return true
	}
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	return stringsHasSuffixFold(name, ".onion")
}

// nameList returns a list of names for sequential DNS queries.
func (conf *dnsConfig) nameList(name string) []string {
	if avoidDNS(name) {
		return nil
	}

	// Check name length (see isDomainName).
	l := len(name)
	rooted := l > 0 && name[l-1] == '.'
	if l > 254 || l == 254 && rooted {
		return nil
	}

	// If name is rooted (trailing dot), try only that name.
	if rooted {
		return []string{name}
	}

	hasNdots := count(name, '.') >= conf.ndots
	name += "."
	l++

	// Build list of search choices.
	names := make([]string, 0, 1+len(conf.search))
	// If name has enough dots, try unsuffixed first.
	if hasNdots {
		names = append(names, name)
	}
	// Try suffixes that are not too long (see isDomainName).
	for _, suffix := range conf.search {
		if l+len(suffix) <= 254 {
			names = append(names, name+suffix)
		}
	}
	// Try unsuffixed, if not tried first above.
	if !hasNdots {
		names = append(names, name)
	}
	return names
}

// hostLookupOrder specifies the order of LookupHost lookup strategies.
// It is basically a simplified representation of nsswitch.conf.
// "files" means /etc/hosts.
type hostLookupOrder int

const (
	// hostLookupCgo means defer to cgo.
	hostLookupCgo      hostLookupOrder = iota
	hostLookupFilesDNS                 // files first
	hostLookupDNSFiles                 // dns first
	hostLookupFiles                    // only files
	hostLookupDNS                      // only DNS
)

var lookupOrderName = map[hostLookupOrder]string{
	hostLookupCgo:      "cgo",
	hostLookupFilesDNS: "files,dns",
	hostLookupDNSFiles: "dns,files",
	hostLookupFiles:    "files",
	hostLookupDNS:      "dns",
}

func (o hostLookupOrder) String() string {
	if s, ok := lookupOrderName[o]; ok {
		return s
	}
	return "hostLookupOrder=" + itoa(int(o)) + "??"
}

// goLookupHost is the native Go implementation of LookupHost.
// Used only if cgoLookupHost refuses to handle the request
// (that is, only if cgoLookupHost is the stub in cgo_stub.go).
// Normally we let cgo use the C library resolver instead of
// depending on our lookup code, so that Go and C get the same
// answers.
func (r *Resolver) goLookupHost(ctx context.Context, name string) (addrs []string, err error) {
	return r.goLookupHostOrder(ctx, name, hostLookupFilesDNS)
}

func (r *Resolver) goLookupHostOrder(ctx context.Context, name string, order hostLookupOrder) (addrs []string, err error) {
	if order == hostLookupFilesDNS || order == hostLookupFiles {
		// Use entries from /etc/hosts if they match.
		addrs = lookupStaticHost(name)
		if len(addrs) > 0 || order == hostLookupFiles {
			return
		}
	}
	ips, _, err := r.goLookupIPCNAMEOrder(ctx, name, order)
	if err != nil {
		return
	}
	addrs = make([]string, 0, len(ips))
	for _, ip := range ips {
		addrs = append(addrs, ip.String())
	}
	return
}

// lookup entries from /etc/hosts
func goLookupIPFiles(name string) (addrs []IPAddr) {
	for _, haddr := range lookupStaticHost(name) {
		haddr, zone := splitHostZone(haddr)
		if ip := ParseIP(haddr); ip != nil {
			addr := IPAddr{IP: ip, Zone: zone}
			addrs = append(addrs, addr)
		}
	}
	sortByRFC6724(addrs)
	return
}

// goLookupIP is the native Go implementation of LookupIP.
// The libc versions are in cgo_*.go.
func (r *Resolver) goLookupIP(ctx context.Context, host string) (addrs []IPAddr, err error) {
	order := systemConf().hostLookupOrder(host)
	addrs, _, err = r.goLookupIPCNAMEOrder(ctx, host, order)
	return
}

func (r *Resolver) goLookupIPCNAMEOrder(ctx context.Context, name string, order hostLookupOrder) (addrs []IPAddr, cname dnsmessage.Name, err error) {
	if order == hostLookupFilesDNS || order == hostLookupFiles {
		addrs = goLookupIPFiles(name)
		if len(addrs) > 0 || order == hostLookupFiles {
			return addrs, dnsmessage.Name{}, nil
		}
	}
	if !isDomainName(name) {
		// See comment in func lookup above about use of errNoSuchHost.
		return nil, dnsmessage.Name{}, &DNSError{Err: errNoSuchHost.Error(), Name: name}
	}
	resolvConf.tryUpdate("/etc/resolv.conf")
	resolvConf.mu.RLock()
	conf := resolvConf.dnsConfig
	resolvConf.mu.RUnlock()
	type racer struct {
		p      dnsmessage.Parser
		server string
		error
	}
	lane := make(chan racer, 1)
	qtypes := [...]dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA}
	var lastErr error
	for _, fqdn := range conf.nameList(name) {
		for _, qtype := range qtypes {
			dnsWaitGroup.Add(1)
			go func(qtype dnsmessage.Type) {
				p, server, err := r.tryOneName(ctx, conf, fqdn, qtype)
				lane <- racer{p, server, err}
				dnsWaitGroup.Done()
			}(qtype)
		}
		hitStrictError := false
		for range qtypes {
			racer := <-lane
			if racer.error != nil {
				if nerr, ok := racer.error.(Error); ok && nerr.Temporary() && r.StrictErrors {
					// This error will abort the nameList loop.
					hitStrictError = true
					lastErr = racer.error
				} else if lastErr == nil || fqdn == name+"." {
					// Prefer error for original name.
					lastErr = racer.error
				}
				continue
			}

			// Presotto says it's okay to assume that servers listed in
			///etc/resolv.conf are recursive resolvers.
			//
			// We asked for recursion, so it should have included all the
			// answers we need in this one packet.
			//
			// Further, RFC 1035 section 4.3.1 says that "the recursive
			// response to a query will be... The answer to the query,
			// possibly preface by one or more CNAME RRs that specify
			// aliases encountered on the way to an answer."
			//
			// Therefore, we should be able to assume that we can ignore
			// CNAMEs and that the A and AAAA records we requested are
			// for the canonical name.

		loop:
			for {
				h, err := racer.p.AnswerHeader()
				if err != nil && err != dnsmessage.ErrSectionDone {
					lastErr = &DNSError{
						Err:    "cannot marshal DNS message",
						Name:   name,
						Server: racer.server,
					}
				}
				if err != nil {
					break
				}
				switch h.Type {
				case dnsmessage.TypeA:
					a, err := racer.p.AResource()
					if err != nil {
						lastErr = &DNSError{
							Err:    "cannot marshal DNS message",
							Name:   name,
							Server: racer.server,
						}
						break loop
					}
					addrs = append(addrs, IPAddr{IP: IP(a.A[:])})

				case dnsmessage.TypeAAAA:
					aaaa, err := racer.p.AAAAResource()
					if err != nil {
						lastErr = &DNSError{
							Err:    "cannot marshal DNS message",
							Name:   name,
							Server: racer.server,
						}
						break loop
					}
					addrs = append(addrs, IPAddr{IP: IP(aaaa.AAAA[:])})

				default:
					if err := racer.p.SkipAnswer(); err != nil {
						lastErr = &DNSError{
							Err:    "cannot marshal DNS message",
							Name:   name,
							Server: racer.server,
						}
						break loop
					}
					continue
				}
				if cname.Length == 0 && h.Name.Length != 0 {
					cname = h.Name
				}
			}
		}
		if hitStrictError {
			// If either family hit an error with StrictErrors enabled,
			// discard all addresses. This ensures that network flakiness
			// cannot turn a dualstack hostname IPv4/IPv6-only.
			addrs = nil
			break
		}
		if len(addrs) > 0 {
			break
		}
	}
	if lastErr, ok := lastErr.(*DNSError); ok {
		// Show original name passed to lookup, not suffixed one.
		// In general we might have tried many suffixes; showing
		// just one is misleading. See also golang.org/issue/6324.
		lastErr.Name = name
	}
	sortByRFC6724(addrs)
	if len(addrs) == 0 {
		if order == hostLookupDNSFiles {
			addrs = goLookupIPFiles(name)
		}
		if len(addrs) == 0 && lastErr != nil {
			return nil, dnsmessage.Name{}, lastErr
		}
	}
	return addrs, cname, nil
}

// goLookupCNAME is the native Go (non-cgo) implementation of LookupCNAME.
func (r *Resolver) goLookupCNAME(ctx context.Context, host string) (string, error) {
	order := systemConf().hostLookupOrder(host)
	_, cname, err := r.goLookupIPCNAMEOrder(ctx, host, order)
	return cname.String(), err
}

// goLookupPTR is the native Go implementation of LookupAddr.
// Used only if cgoLookupPTR refuses to handle the request (that is,
// only if cgoLookupPTR is the stub in cgo_stub.go).
// Normally we let cgo use the C library resolver instead of depending
// on our lookup code, so that Go and C get the same answers.
func (r *Resolver) goLookupPTR(ctx context.Context, addr string) ([]string, error) {
	names := lookupStaticAddr(addr)
	if len(names) > 0 {
		return names, nil
	}
	arpa, err := reverseaddr(addr)
	if err != nil {
		return nil, err
	}
	p, server, err := r.lookup(ctx, arpa, dnsmessage.TypePTR)
	if err != nil {
		return nil, err
	}
	var ptrs []string
	for {
		h, err := p.AnswerHeader()
		if err == dnsmessage.ErrSectionDone {
			break
		}
		if err != nil {
			return nil, &DNSError{
				Err:    "cannot marshal DNS message",
				Name:   addr,
				Server: server,
			}
		}
		if h.Type != dnsmessage.TypePTR {
			continue
		}
		ptr, err := p.PTRResource()
		if err != nil {
			return nil, &DNSError{
				Err:    "cannot marshal DNS message",
				Name:   addr,
				Server: server,
			}
		}
		ptrs = append(ptrs, ptr.PTR.String())

	}
	return ptrs, nil
}

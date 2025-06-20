// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa_test

import (
	"bytes"
	"flag"
	"fmt"
	"internal/testenv"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("u", false, "update test reference files")
var verbose = flag.Bool("v", false, "print debugger interactions (very verbose)")
var dryrun = flag.Bool("n", false, "just print the command line and first debugging bits")
var useDelve = flag.Bool("d", false, "use Delve (dlv) instead of gdb, use dlv reverence files")
var force = flag.Bool("f", false, "force run under not linux-amd64; also do not use tempdir")

var repeats = flag.Bool("r", false, "detect repeats in debug steps and don't ignore them")
var inlines = flag.Bool("i", false, "do inlining for gdb (makes testing flaky till inlining info is correct)")

var hexRe = regexp.MustCompile("0x[a-zA-Z0-9]+")
var numRe = regexp.MustCompile("-?[0-9]+")
var stringRe = regexp.MustCompile("\"([^\\\"]|(\\.))*\"")
var leadingDollarNumberRe = regexp.MustCompile("^[$][0-9]+")
var optOutGdbRe = regexp.MustCompile("[<]optimized out[>]")
var numberColonRe = regexp.MustCompile("^ *[0-9]+:")

var gdb = "gdb"      // Might be "ggdb" on Darwin, because gdb no longer part of XCode
var debugger = "gdb" // For naming files, etc.

var gogcflags = os.Getenv("GO_GCFLAGS")

// optimizedLibs usually means "not running in a noopt test builder".
var optimizedLibs = (!strings.Contains(gogcflags, "-N") && !strings.Contains(gogcflags, "-l"))

// TestNexting go-builds a file, then uses a debugger (default gdb, optionally delve)
// to next through the generated executable, recording each line landed at, and
// then compares those lines with reference file(s).
// Flag -u updates the reference file(s).
// Flag -d changes the debugger to delve (and uses delve-specific reference files)
// Flag -v is ever-so-slightly verbose.
// Flag -n is for dry-run, and prints the shell and first debug commands.
//
// Because this test (combined with existing compiler deficiencies) is flaky,
// for gdb-based testing by default inlining is disabled
// (otherwise output depends on library internals)
// and for both gdb and dlv by default repeated lines in the next stream are ignored
// (because this appears to be timing-dependent in gdb, and the cleanest fix is in code common to gdb and dlv).
//
// Also by default, any source code outside of .../testdata/ is not mentioned
// in the debugging histories.  This deals both with inlined library code once
// the compiler is generating clean inline records, and also deals with
// runtime code between return from main and process exit.  This is hidden
// so that those files (in the runtime/library) can change without affecting
// this test.
//
// These choices can be reversed with -i (inlining on) and -r (repeats detected) which
// will also cause their own failures against the expected outputs.  Note that if the compiler
// and debugger were behaving properly, the inlined code and repeated lines would not appear,
// so the expected output is closer to what we hope to see, though it also encodes all our
// current bugs.
//
// The file being tested may contain comments of the form
////DBG-TAG=(v1,v2,v3)
// where DBG = {gdb,dlv} and TAG={dbg,opt}
// each variable may optionally be followed by a / and one or more of S,A,N,O
// to indicate normalization of Strings, (hex) addresses, and numbers.
// "O" is an explicit indication that we expect it to be optimized out.
// For example:
/*
	if len(os.Args) > 1 { //gdb-dbg=(hist/A,cannedInput/A) //dlv-dbg=(hist/A,cannedInput/A)
*/
// TODO: not implemented for Delve yet, but this is the plan
//
// After a compiler change that causes a difference in the debug behavior, check
// to see if it is sensible or not, and if it is, update the reference files with
// go test debug_test.go -args -u
// (for Delve)
// go test debug_test.go -args -u -d

func TestNexting(t *testing.T) {
	skipReasons := "" // Many possible skip reasons, list all that apply
	if testing.Short() {
		skipReasons = "not run in short mode; "
	}
	testenv.MustHaveGoBuild(t)

	if !*useDelve && !*force && !(runtime.GOOS == "linux" && runtime.GOARCH == "amd64") {
		// Running gdb on OSX/darwin is very flaky.
		// Sometimes it is called ggdb, depending on how it is installed.
		// It also sometimes requires an admin password typed into a dialog box.
		// Various architectures tend to differ slightly sometimes, and keeping them
		// all in sync is a pain for people who don't have them all at hand,
		// so limit testing to amd64 (for now)
		skipReasons += "not run unless linux-amd64 or -d (delve) or -f (force); "
	}

	if *useDelve {
		debugger = "dlv"
		_, err := exec.LookPath("dlv")
		if err != nil {
			skipReasons += "not run because dlv (requested by -d option) not on path; "
		}
	} else {
		_, err := exec.LookPath(gdb)
		if err != nil {
			if runtime.GOOS != "darwin" {
				skipReasons += "not run because gdb not on path; "
			} else {
				// On Darwin, MacPorts installs gdb as "ggdb".
				_, err = exec.LookPath("ggdb")
				if err != nil {
					skipReasons += "not run because gdb (and also ggdb) not on path; "
				} else {
					gdb = "ggdb"
				}
			}
		}
	}

	if skipReasons != "" {
		t.Skip(skipReasons[:len(skipReasons)-2])
	}

	optFlags := "-dwarflocationlists"
	if !*useDelve && !*inlines {
		// For gdb (default), disable inlining so that a compiler test does not depend on library code.
		// TODO: This may not be necessary with 1.10 and later.
		optFlags += " -l"
	}

	subTest(t, debugger+"-dbg", "hist", "-N -l")
	subTest(t, debugger+"-dbg-race", "i22600", "-N -l", "-race")
	subTest(t, debugger+"-dbg-22558", "i22558", "-N -l")
	optSubTest(t, debugger+"-opt", "hist", optFlags)
}

// subTest creates a subtest that compiles basename.go with the specified gcflags and additional compiler arguments,
// then runs the debugger on the resulting binary, with any comment-specified actions matching tag triggered.
func subTest(t *testing.T, tag string, basename string, gcflags string, moreargs ...string) {
	t.Run(tag, func(t *testing.T) {
		testNexting(t, basename, tag, gcflags, moreargs...)
	})
}

// optSubTest is the same as subTest except that it skips the test if the runtime and libraries
// were not compiled with optimization turned on.  (The skip may not be necessary with Go 1.10 and later)
func optSubTest(t *testing.T, tag string, basename string, gcflags string, moreargs ...string) {
	// If optimized test is run with unoptimized libraries (compiled with -N -l), it is very likely to fail.
	// This occurs in the noopt builders (for example).
	t.Run(tag, func(t *testing.T) {
		if *force || optimizedLibs {
			testNexting(t, basename, tag, gcflags, moreargs...)
		} else {
			t.Skip("skipping for unoptimized stdlib/runtime")
		}
	})
}

func testNexting(t *testing.T, base, tag, gcflags string, moreArgs ...string) {
	// (1) In testdata, build sample.go into test-sample.<tag>
	// (2) Run debugger gathering a history
	// (3) Read expected history from testdata/sample.<tag>.nexts
	// optionally, write out testdata/sample.<tag>.nexts

	testbase := filepath.Join("testdata", base) + "." + tag
	tmpbase := filepath.Join("testdata", "test-"+base+"."+tag)

	// Use a temporary directory unless -f is specified
	if !*force {
		tmpdir, err := ioutil.TempDir("", "debug_test")
		if err != nil {
			panic(fmt.Sprintf("Problem creating TempDir, error %v\n", err))
		}
		tmpbase = filepath.Join(tmpdir, "test-"+base+"."+tag)
		if *verbose {
			fmt.Printf("Tempdir is %s\n", tmpdir)
		}
		defer os.RemoveAll(tmpdir)
	}
	exe := tmpbase

	runGoArgs := []string{"build", "-o", exe, "-gcflags=all=" + gcflags}
	runGoArgs = append(runGoArgs, moreArgs...)
	runGoArgs = append(runGoArgs, filepath.Join("testdata", base+".go"))

	runGo(t, "", runGoArgs...)

	nextlog := testbase + ".nexts"
	tmplog := tmpbase + ".nexts"
	var dbg dbgr
	if *useDelve {
		dbg = newDelve(tag, exe)
	} else {
		dbg = newGdb(tag, exe)
	}
	h1 := runDbgr(dbg, 1000)
	if *dryrun {
		fmt.Printf("# Tag for above is %s\n", dbg.tag())
		return
	}
	if *update {
		h1.write(nextlog)
	} else {
		h0 := &nextHist{}
		h0.read(nextlog)
		if !h0.equals(h1) {
			// Be very noisy about exactly what's wrong to simplify debugging.
			h1.write(tmplog)
			cmd := exec.Command("diff", "-u", nextlog, tmplog)
			line := asCommandLine("", cmd)
			bytes, err := cmd.CombinedOutput()
			if err != nil && len(bytes) == 0 {
				t.Fatalf("step/next histories differ, diff command %s failed with error=%v", line, err)
			}
			t.Fatalf("step/next histories differ, diff=\n%s", string(bytes))
		}
	}
}

type dbgr interface {
	start()
	stepnext(s string) bool // step or next, possible with parameter, gets line etc.  returns true for success, false for unsure response
	quit()
	hist() *nextHist
	tag() string
}

func runDbgr(dbg dbgr, maxNext int) *nextHist {
	dbg.start()
	if *dryrun {
		return nil
	}
	for i := 0; i < maxNext; i++ {
		if !dbg.stepnext("n") {
			break
		}
	}
	h := dbg.hist()
	return h
}

func runGo(t *testing.T, dir string, args ...string) string {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(testenv.GoToolPath(t), args...)
	cmd.Dir = dir
	if *dryrun {
		fmt.Printf("%s\n", asCommandLine("", cmd))
		return ""
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("error running cmd (%s): %v\nstdout:\n%sstderr:\n%s\n", asCommandLine("", cmd), err, stdout.String(), stderr.String())
	}

	if s := stderr.String(); s != "" {
		t.Fatalf("Stderr = %s\nWant empty", s)
	}

	return stdout.String()
}

// tstring provides two strings, o (stdout) and e (stderr)
type tstring struct {
	o string
	e string
}

func (t tstring) String() string {
	return t.o + t.e
}

type pos struct {
	line uint16
	file uint8 // Artifact of plans to implement differencing instead of calling out to diff.
}

type nextHist struct {
	f2i   map[string]uint8
	fs    []string
	ps    []pos
	texts []string
	vars  [][]string
}

func (h *nextHist) write(filename string) {
	file, err := os.Create(filename)
	if err != nil {
		panic(fmt.Sprintf("Problem opening %s, error %v\n", filename, err))
	}
	defer file.Close()
	var lastfile uint8
	for i, x := range h.texts {
		p := h.ps[i]
		if lastfile != p.file {
			fmt.Fprintf(file, "  %s\n", h.fs[p.file-1])
			lastfile = p.file
		}
		fmt.Fprintf(file, "%d:%s\n", p.line, x)
		// TODO, normalize between gdb and dlv into a common, comparable format.
		for _, y := range h.vars[i] {
			y = strings.TrimSpace(y)
			fmt.Fprintf(file, "%s\n", y)
		}
	}
	file.Close()
}

func (h *nextHist) read(filename string) {
	h.f2i = make(map[string]uint8)
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("Problem reading %s, error %v\n", filename, err))
	}
	var lastfile string
	lines := strings.Split(string(bytes), "\n")
	for i, l := range lines {
		if len(l) > 0 && l[0] != '#' {
			if l[0] == ' ' {
				// file -- first two characters expected to be "  "
				lastfile = strings.TrimSpace(l)
			} else if numberColonRe.MatchString(l) {
				// line number -- <number>:<line>
				colonPos := strings.Index(l, ":")
				if colonPos == -1 {
					panic(fmt.Sprintf("Line %d (%s) in file %s expected to contain '<number>:' but does not.\n", i+1, l, filename))
				}
				h.add(lastfile, l[0:colonPos], l[colonPos+1:])
			} else {
				h.addVar(l)
			}
		}
	}
}

// add appends file (name), line (number) and text (string) to the history,
// provided that the file+line combo does not repeat the previous position,
// and provided that the file is within the testdata directory.  The return
// value indicates whether the append occurred.
func (h *nextHist) add(file, line, text string) bool {
	// Only record source code in testdata unless the inlines flag is set
	if !*inlines && !strings.Contains(file, "/testdata/") {
		return false
	}
	fi := h.f2i[file]
	if fi == 0 {
		h.fs = append(h.fs, file)
		fi = uint8(len(h.fs))
		h.f2i[file] = fi
	}

	line = strings.TrimSpace(line)
	var li int
	var err error
	if line != "" {
		li, err = strconv.Atoi(line)
		if err != nil {
			panic(fmt.Sprintf("Non-numeric line: %s, error %v\n", line, err))
		}
	}
	l := len(h.ps)
	p := pos{line: uint16(li), file: fi}

	if l == 0 || *repeats || h.ps[l-1] != p {
		h.ps = append(h.ps, p)
		h.texts = append(h.texts, text)
		h.vars = append(h.vars, []string{})
		return true
	}
	return false
}

func (h *nextHist) addVar(text string) {
	l := len(h.texts)
	h.vars[l-1] = append(h.vars[l-1], text)
}

func invertMapSU8(hf2i map[string]uint8) map[uint8]string {
	hi2f := make(map[uint8]string)
	for hs, i := range hf2i {
		hi2f[i] = hs
	}
	return hi2f
}

func (h *nextHist) equals(k *nextHist) bool {
	if len(h.f2i) != len(k.f2i) {
		return false
	}
	if len(h.ps) != len(k.ps) {
		return false
	}
	hi2f := invertMapSU8(h.f2i)
	ki2f := invertMapSU8(k.f2i)

	for i, hs := range hi2f {
		if hs != ki2f[i] {
			return false
		}
	}

	for i, x := range h.ps {
		if k.ps[i] != x {
			return false
		}
	}

	for i, hv := range h.vars {
		kv := k.vars[i]
		if len(hv) != len(kv) {
			return false
		}
		for j, hvt := range hv {
			if hvt != kv[j] {
				return false
			}
		}
	}

	return true
}

// canonFileName strips everything before "/src/" from a filename.
// This makes file names portable across different machines,
// home directories, and temporary directories.
func canonFileName(f string) string {
	i := strings.Index(f, "/src/")
	if i != -1 {
		f = f[i+1:]
	}
	return f
}

/* Delve */

type delveState struct {
	cmd  *exec.Cmd
	tagg string
	*ioState
	atLineRe         *regexp.Regexp // "\n =>"
	funcFileLinePCre *regexp.Regexp // "^> ([^ ]+) ([^:]+):([0-9]+) .*[(]PC: (0x[a-z0-9]+)"
	line             string
	file             string
	function         string
}

func newDelve(tag, executable string, args ...string) dbgr {
	cmd := exec.Command("dlv", "exec", executable)
	cmd.Env = replaceEnv(cmd.Env, "TERM", "dumb")
	if len(args) > 0 {
		cmd.Args = append(cmd.Args, "--")
		cmd.Args = append(cmd.Args, args...)
	}
	s := &delveState{tagg: tag, cmd: cmd}
	// HAHA Delve has control characters embedded to change the color of the => and the line number
	// that would be '(\\x1b\\[[0-9;]+m)?' OR TERM=dumb
	s.atLineRe = regexp.MustCompile("\n=>[[:space:]]+[0-9]+:(.*)")
	s.funcFileLinePCre = regexp.MustCompile("> ([^ ]+) ([^:]+):([0-9]+) .*[(]PC: (0x[a-z0-9]+)[)]\n")
	s.ioState = newIoState(s.cmd)
	return s
}

func (s *delveState) tag() string {
	return s.tagg
}

func (s *delveState) stepnext(ss string) bool {
	x := s.ioState.writeReadExpect(ss+"\n", "[(]dlv[)] ")
	excerpts := s.atLineRe.FindStringSubmatch(x.o)
	locations := s.funcFileLinePCre.FindStringSubmatch(x.o)
	excerpt := ""
	if len(excerpts) > 1 {
		excerpt = excerpts[1]
	}
	if len(locations) > 0 {
		fn := canonFileName(locations[2])
		if *verbose {
			if s.file != fn {
				fmt.Printf("%s\n", locations[2]) // don't canonocalize verbose logging
			}
			fmt.Printf("  %s\n", locations[3])
		}
		s.line = locations[3]
		s.file = fn
		s.function = locations[1]
		s.ioState.history.add(s.file, s.line, excerpt)
		// TODO: here is where variable processing will be added.  See gdbState.stepnext as a guide.
		// Adding this may require some amount of normalization so that logs are comparable.
		return true
	}
	if *verbose {
		fmt.Printf("DID NOT MATCH EXPECTED NEXT OUTPUT\nO='%s'\nE='%s'\n", x.o, x.e)
	}
	return false
}

func (s *delveState) start() {
	if *dryrun {
		fmt.Printf("%s\n", asCommandLine("", s.cmd))
		fmt.Printf("b main.test\n")
		fmt.Printf("c\n")
		return
	}
	err := s.cmd.Start()
	if err != nil {
		line := asCommandLine("", s.cmd)
		panic(fmt.Sprintf("There was an error [start] running '%s', %v\n", line, err))
	}
	s.ioState.readExpecting(-1, 5000, "Type 'help' for list of commands.")
	expect("Breakpoint [0-9]+ set at ", s.ioState.writeReadExpect("b main.test\n", "[(]dlv[)] "))
	s.stepnext("c")
}

func (s *delveState) quit() {
	expect("", s.ioState.writeRead("q\n"))
}

/* Gdb */

type gdbState struct {
	cmd  *exec.Cmd
	tagg string
	args []string
	*ioState
	atLineRe         *regexp.Regexp
	funcFileLinePCre *regexp.Regexp
	line             string
	file             string
	function         string
}

func newGdb(tag, executable string, args ...string) dbgr {
	// Turn off shell, necessary for Darwin apparently
	cmd := exec.Command(gdb, "-ex", "set startup-with-shell off", executable)
	cmd.Env = replaceEnv(cmd.Env, "TERM", "dumb")
	s := &gdbState{tagg: tag, cmd: cmd, args: args}
	s.atLineRe = regexp.MustCompile("(^|\n)([0-9]+)(.*)")
	s.funcFileLinePCre = regexp.MustCompile(
		"([^ ]+) [(][^)]*[)][ \\t\\n]+at ([^:]+):([0-9]+)")
	// runtime.main () at /Users/drchase/GoogleDrive/work/go/src/runtime/proc.go:201
	//                                    function              file    line
	// Thread 2 hit Breakpoint 1, main.main () at /Users/drchase/GoogleDrive/work/debug/hist.go:18
	s.ioState = newIoState(s.cmd)
	return s
}

func (s *gdbState) tag() string {
	return s.tagg
}

func (s *gdbState) start() {
	run := "run"
	for _, a := range s.args {
		run += " " + a // Can't quote args for gdb, it will pass them through including the quotes
	}
	if *dryrun {
		fmt.Printf("%s\n", asCommandLine("", s.cmd))
		fmt.Printf("tbreak main.test\n")
		fmt.Printf("%s\n", run)
		return
	}
	err := s.cmd.Start()
	if err != nil {
		line := asCommandLine("", s.cmd)
		panic(fmt.Sprintf("There was an error [start] running '%s', %v\n", line, err))
	}
	s.ioState.readExpecting(-1, -1, "[(]gdb[)] ")
	x := s.ioState.writeReadExpect("b main.test\n", "[(]gdb[)] ")
	expect("Breakpoint [0-9]+ at", x)
	s.stepnext(run)
}

func (s *gdbState) stepnext(ss string) bool {
	x := s.ioState.writeReadExpect(ss+"\n", "[(]gdb[)] ")
	excerpts := s.atLineRe.FindStringSubmatch(x.o)
	locations := s.funcFileLinePCre.FindStringSubmatch(x.o)
	excerpt := ""
	addedLine := false
	if len(excerpts) == 0 && len(locations) == 0 {
		if *verbose {
			fmt.Printf("DID NOT MATCH %s", x.o)
		}
		return false
	}
	if len(excerpts) > 0 {
		excerpt = excerpts[3]
	}
	if len(locations) > 0 {
		fn := canonFileName(locations[2])
		if *verbose {
			if s.file != fn {
				fmt.Printf("%s\n", locations[2])
			}
			fmt.Printf("  %s\n", locations[3])
		}
		s.line = locations[3]
		s.file = fn
		s.function = locations[1]
		addedLine = s.ioState.history.add(s.file, s.line, excerpt)
	}
	if len(excerpts) > 0 {
		if *verbose {
			fmt.Printf("  %s\n", excerpts[2])
		}
		s.line = excerpts[2]
		addedLine = s.ioState.history.add(s.file, s.line, excerpt)
	}

	if !addedLine {
		// True if this was a repeat line
		return true
	}
	// Look for //gdb-<tag>=(v1,v2,v3) and print v1, v2, v3
	vars := varsToPrint(excerpt, "//"+s.tag()+"=(")
	for _, v := range vars {
		response := printVariableAndNormalize(v, func(v string) string {
			return s.ioState.writeReadExpect("p "+v+"\n", "[(]gdb[)] ").String()
		})
		s.ioState.history.addVar(response)
	}
	return true
}

// printVariableAndNormalize extracts any slash-indicated normalizing requests from the variable
// name, then uses printer to get the value of the variable from the debugger, and then
// normalizes and returns the response.
func printVariableAndNormalize(v string, printer func(v string) string) string {
	slashIndex := strings.Index(v, "/")
	substitutions := ""
	if slashIndex != -1 {
		substitutions = v[slashIndex:]
		v = v[:slashIndex]
	}
	response := printer(v)
	// expect something like "$1 = ..."
	dollar := strings.Index(response, "$")
	cr := strings.Index(response, "\n")

	if dollar == -1 { // some not entirely expected response, whine and carry on.
		if cr == -1 {
			response = strings.TrimSpace(response) // discards trailing newline
			response = strings.Replace(response, "\n", "<BR>", -1)
			return "$ Malformed response " + response
		}
		response = strings.TrimSpace(response[:cr])
		return "$ " + response
	}
	if cr == -1 {
		cr = len(response)
	}
	// Convert the leading $<number> into the variable name to enhance readability
	// and reduce scope of diffs if an earlier print-variable is added.
	response = strings.TrimSpace(response[dollar:cr])
	response = leadingDollarNumberRe.ReplaceAllString(response, v)

	// Normalize value as requested.
	if strings.Contains(substitutions, "A") {
		response = hexRe.ReplaceAllString(response, "<A>")
	}
	if strings.Contains(substitutions, "N") {
		response = numRe.ReplaceAllString(response, "<N>")
	}
	if strings.Contains(substitutions, "S") {
		response = stringRe.ReplaceAllString(response, "<S>")
	}
	if strings.Contains(substitutions, "O") {
		response = optOutGdbRe.ReplaceAllString(response, "<Optimized out, as expected>")
	}
	return response
}

// varsToPrint takes a source code line, and extracts the comma-separated variable names
// found between lookfor and the next ")".
// For example, if line includes "... //gdb-foo=(v1,v2,v3)" and
// lookfor="//gdb-foo=(", then varsToPrint returns ["v1", "v2", "v3"]
func varsToPrint(line, lookfor string) []string {
	var vars []string
	if strings.Contains(line, lookfor) {
		x := line[strings.Index(line, lookfor)+len(lookfor):]
		end := strings.Index(x, ")")
		if end == -1 {
			panic(fmt.Sprintf("Saw variable list begin %s in %s but no closing ')'", lookfor, line))
		}
		vars = strings.Split(x[:end], ",")
		for i, y := range vars {
			vars[i] = strings.TrimSpace(y)
		}
	}
	return vars
}

func (s *gdbState) quit() {
	response := s.ioState.writeRead("q\n")
	if strings.Contains(response.o, "Quit anyway? (y or n)") {
		s.ioState.writeRead("Y\n")
	}
}

type ioState struct {
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	stdin   io.WriteCloser
	outChan chan string
	errChan chan string
	last    tstring // Output of previous step
	history *nextHist
}

func newIoState(cmd *exec.Cmd) *ioState {
	var err error
	s := &ioState{}
	s.history = &nextHist{}
	s.history.f2i = make(map[string]uint8)
	s.stdout, err = cmd.StdoutPipe()
	line := asCommandLine("", cmd)
	if err != nil {
		panic(fmt.Sprintf("There was an error [stdoutpipe] running '%s', %v\n", line, err))
	}
	s.stderr, err = cmd.StderrPipe()
	if err != nil {
		panic(fmt.Sprintf("There was an error [stdouterr] running '%s', %v\n", line, err))
	}
	s.stdin, err = cmd.StdinPipe()
	if err != nil {
		panic(fmt.Sprintf("There was an error [stdinpipe] running '%s', %v\n", line, err))
	}

	s.outChan = make(chan string, 1)
	s.errChan = make(chan string, 1)
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := s.stdout.Read(buffer)
			if n > 0 {
				s.outChan <- string(buffer[0:n])
			}
			if err == io.EOF || n == 0 {
				break
			}
			if err != nil {
				fmt.Printf("Saw an error forwarding stdout")
				break
			}
		}
		close(s.outChan)
		s.stdout.Close()
	}()

	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := s.stderr.Read(buffer)
			if n > 0 {
				s.errChan <- string(buffer[0:n])
			}
			if err == io.EOF || n == 0 {
				break
			}
			if err != nil {
				fmt.Printf("Saw an error forwarding stderr")
				break
			}
		}
		close(s.errChan)
		s.stderr.Close()
	}()
	return s
}

func (s *ioState) hist() *nextHist {
	return s.history
}

// writeRead writes ss, then reads stdout and stderr, waiting 500ms to
// be sure all the output has appeared.
func (s *ioState) writeRead(ss string) tstring {
	if *verbose {
		fmt.Printf("=> %s", ss)
	}
	_, err := io.WriteString(s.stdin, ss)
	if err != nil {
		panic(fmt.Sprintf("There was an error writing '%s', %v\n", ss, err))
	}
	return s.readExpecting(-1, 500, "")
}

// writeReadExpect writes ss, then reads stdout and stderr until something
// that matches expectRE appears.  expectRE should not be ""
func (s *ioState) writeReadExpect(ss, expectRE string) tstring {
	if *verbose {
		fmt.Printf("=> %s", ss)
	}
	if expectRE == "" {
		panic("expectRE should not be empty; use .* instead")
	}
	_, err := io.WriteString(s.stdin, ss)
	if err != nil {
		panic(fmt.Sprintf("There was an error writing '%s', %v\n", ss, err))
	}
	return s.readExpecting(-1, -1, expectRE)
}

func (s *ioState) readExpecting(millis, interlineTimeout int, expectedRE string) tstring {
	timeout := time.Millisecond * time.Duration(millis)
	interline := time.Millisecond * time.Duration(interlineTimeout)
	s.last = tstring{}
	var re *regexp.Regexp
	if expectedRE != "" {
		re = regexp.MustCompile(expectedRE)
	}
loop:
	for {
		var timer <-chan time.Time
		if timeout > 0 {
			timer = time.After(timeout)
		}
		select {
		case x, ok := <-s.outChan:
			if !ok {
				s.outChan = nil
			}
			s.last.o += x
		case x, ok := <-s.errChan:
			if !ok {
				s.errChan = nil
			}
			s.last.e += x
		case <-timer:
			break loop
		}
		if re != nil {
			if re.MatchString(s.last.o) {
				break
			}
			if re.MatchString(s.last.e) {
				break
			}
		}
		timeout = interline
	}
	if *verbose {
		fmt.Printf("<= %s%s", s.last.o, s.last.e)
	}
	return s.last
}

// replaceEnv returns a new environment derived from env
// by removing any existing definition of ev and adding ev=evv.
func replaceEnv(env []string, ev string, evv string) []string {
	evplus := ev + "="
	var found bool
	for i, v := range env {
		if strings.HasPrefix(v, evplus) {
			found = true
			env[i] = evplus + evv
		}
	}
	if !found {
		env = append(env, evplus+evv)
	}
	return env
}

// asCommandLine renders cmd as something that could be copy-and-pasted into a command line
// If cwd is not empty and different from the command's directory, prepend an approprirate "cd"
func asCommandLine(cwd string, cmd *exec.Cmd) string {
	s := "("
	if cmd.Dir != "" && cmd.Dir != cwd {
		s += "cd" + escape(cmd.Dir) + ";"
	}
	for _, e := range cmd.Env {
		if !strings.HasPrefix(e, "PATH=") &&
			!strings.HasPrefix(e, "HOME=") &&
			!strings.HasPrefix(e, "USER=") &&
			!strings.HasPrefix(e, "SHELL=") {
			s += escape(e)
		}
	}
	for _, a := range cmd.Args {
		s += escape(a)
	}
	s += " )"
	return s
}

// escape inserts escapes appropriate for use in a shell command line
func escape(s string) string {
	s = strings.Replace(s, "\\", "\\\\", -1)
	s = strings.Replace(s, "'", "\\'", -1)
	// Conservative guess at characters that will force quoting
	if strings.ContainsAny(s, "\\ ;#*&$~?!|[]()<>{}`") {
		s = " '" + s + "'"
	} else {
		s = " " + s
	}
	return s
}

func expect(want string, got tstring) {
	if want != "" {
		match, err := regexp.MatchString(want, got.o)
		if err != nil {
			panic(fmt.Sprintf("Error for regexp %s, %v\n", want, err))
		}
		if match {
			return
		}
		match, err = regexp.MatchString(want, got.e)
		if match {
			return
		}
		fmt.Printf("EXPECTED '%s'\n GOT O='%s'\nAND E='%s'\n", want, got.o, got.e)
	}
}

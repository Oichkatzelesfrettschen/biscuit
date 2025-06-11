// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/// Package errors implements functions to manipulate errors.
package errors

/// New returns an error that formats as the given text.
///
/// @param text descriptive text for the resulting error
/// @return error instance wrapping the provided text
func New(text string) error {
	return &errorString{text}
}

/// errorString is a trivial implementation of the error interface.
/// It stores the provided error message as a string.
type errorString struct {
	s string
}

/// Error implements the error interface and returns the underlying
/// text message.
///
/// @return the original error text
func (e *errorString) Error() string {
	return e.s
}

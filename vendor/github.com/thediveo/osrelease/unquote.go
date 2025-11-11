// Copyright 2021 Harald Albrecht.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy
// of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package osrelease

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Unquote unquotes and unescapes a variable assignment value as necessary. It
// follows the "specification" in
// https://www.freedesktop.org/software/systemd/man/os-release.html.
func Unquote(in string) (string, error) {
	if in == "" {
		return "", nil
	}
	// As per the aforementioned specification, the value of the variable
	// assignment can be enclosed in single or double quotes. Or rather, the
	// value must be enclosed for certain values.
	quote := in[0]
	if quote != '"' && quote != '\'' {
		// It's an unqouted assignment value which we simply return verbatim.
		return in, nil
	}
	// Okay, so the assignment value is quotes. But, is the easy route
	// available, as no escapes have been used?
	if !strings.Contains(in, "\\") {
		// Make sure there is the correct quote at the end of the assignment
		// value and then return everything between the single or double quotes.
		if in[len(in)-1] != quote {
			return "", errors.New("malformed assignment value: missing final quote")
		}
		return in[1 : len(in)-1], nil
	}
	// Nope, we now need to work on the escapes, and we need to do so one after
	// the other. The rationale is that separate consecutive string replacement
	// operations can yield rather unexpected results, especially when placing
	// escaping backslashes and then immediately escaping something else...
	out := make([]byte, 0, len(in))
	in = in[1:] // skip leading quote, it's a cheap slice operation
	for len(in) > 0 && in[0] != quote {
		r, mb, tail, err := unescapeChar(in)
		if err != nil {
			return "", err
		}
		if !mb {
			// ...our unquoteChar correctly sets multibyte in any case
			out = append(out, byte(r))
		} else {
			var utf8bytes [utf8.UTFMax]byte
			n := utf8.EncodeRune(utf8bytes[:], r)
			out = append(out, utf8bytes[:n]...) // ...convoluted expression :(
		}
		in = tail
	}
	// Ensure that we've reached THE END correctly...
	if len(in) != 1 || in[0] != quote {
		return "", errors.New("malformed assignment value: missing terminating quote")
	}
	return string(out), nil
}

// unescapeChar decodes the first character or byte in the string, unescaping as
// necessary per
// https://www.freedesktop.org/software/systemd/man/os-release.html. It is kind
// of a very poor kin to strconv.UnquoteChar. It only allows escaping backslash,
// single and double quotes (regardless of quoting context!), dollar, and
// finally backtick. No other escaping.
func unescapeChar(s string) (r rune, multibyte bool, tail string, err error) {
	switch c := s[0]; {
	case c >= utf8.RuneSelf:
		r, size := utf8.DecodeRuneInString(s)
		return r, true, s[size:], nil
	case c != '\\':
		return rune(c), false, s[1:], nil
	}
	// A lone backslash is an error.
	if len(s) < 2 {
		return 0, false, "", errors.New("malformed assignment value: single backslash")
	}
	tail = s[2:]
	switch c := s[1]; c {
	case '\\', '"', '\'', '$', '`':
		return rune(c), false, tail, nil // ...always RuneSelf
	}
	return 0, false, "", errors.New("malformed assignment value: invalid escape")
}

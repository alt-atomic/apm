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
	"bufio"
	"io"
	"os"
	"strings"
)

// New returns os-release variables with their assignment values. It looks for
// /etc/os-release and falls back to /usr/lib/os-release if the former does not
// exist. Returns nil if neither file exists.
func New() map[string]string {
	return new("/")
}

// new returns os-release variables, looking into the standard locations
// etc/os-release and usr/lib/os-release inside the specified base path. This
// function allows unit testing.
func new(base string) map[string]string {
	if osrel := NewFromName(base + "/etc/os-release"); osrel != nil {
		return osrel
	}
	return NewFromName(base + "/usr/lib/os-release")
}

// NewFromName returns the variable assignments from the file (in os-release
// format) with the specified path. It returns nil if the file does not exist.
// It returns an empty variables map if the file is empty or only contains empty
// line and comments.
func NewFromName(name string) map[string]string {
	vars, _ := NewFromNameErr(name)
	return vars
}

// NewFromNameErr returns the variable assignments from the file (in os-release
// format) with the specified path or an error if the file doesn't exists or
// cannot be read.
func NewFromNameErr(name string) (map[string]string, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return assignmentsFromReader(f)
}

// assignmentsFromReader reads variable value assignments from the specified
// reader. It returns nil in case the reader failed (not EOF).
func assignmentsFromReader(r io.Reader) (map[string]string, error) {
	variables := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		variable, value := assignment(scanner.Text())
		if variable == "" {
			continue
		}
		variables[variable] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return variables, nil
}

// assignment returns the variable name and its (unquoted and unescaped) value,
// if any. If the line is empty, a comment line, or invalid, then assignment
// returns a zero variable name.
func assignment(line string) (variable, value string) {
	// blank lines (but without any whitespace) are acceptable.
	if line == "" {
		return "", ""
	}
	// comment lines need to start with a "#" sign, no whitespace before
	// allowed, as the specification says that a line must begin with "#".
	if line[0] == '#' {
		return "", ""
	}
	// We now try to break the line into a variable name and its assignment
	// value. Please note that we accept an empty assignment value, as the
	// specification has nothing to say about this situation. But then, this
	// specification comes from the systemd project where writing concise
	// specifications doesn't seem to be exactly priority.
	fields := strings.SplitN(line, "=", 2)
	if len(fields) != 2 {
		return "", ""
	}
	value, err := Unquote(fields[1])
	if err != nil {
		return "", ""
	}
	return fields[0], value
}

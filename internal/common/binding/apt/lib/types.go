// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package lib

/*
// cgo-timestamp: 1756744254
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"
)

// Public Go types and error codes

type PackageState int

const (
	AptPkgStateInstalled = 1
)

// APT error codes (must match apt_wrapper.h)
const (
	AptErrorPackageNotFound   = 21
	AptErrorInvalidParameters = 91
)

type AptError struct {
	Code    int
	Message string
}

// func (e *AptError) Error() string { return fmt.Sprintf("APT Error %d: %s", e.Code, e.Message) }
func (e *AptError) Error() string { return e.Message }

func CustomError(code int, message string) *AptError {
	return &AptError{Code: code, Message: message}
}

// ErrorFromResult converts C.AptResult to Go error and frees message
func ErrorFromResult(res C.AptResult) *AptError {
	code := int(res.code)
	var msg string
	if res.message != nil {
		msg = C.GoString(res.message)
		C.free(unsafe.Pointer(res.message))
	}
	if msg == "" {
		msg = C.GoString(C.apt_error_string(res.code))
	}
	return &AptError{Code: code, Message: msg}
}

type PackageInfo struct {
	Name             string
	Version          string
	Description      string
	ShortDescription string
	Section          string
	Architecture     string
	Maintainer       string
	Homepage         string
	Priority         string
	MD5Hash          string
	Blake2bHash      string
	SourcePackage    string
	Changelog        string
	Filename         string
	Depends          string
	Provides         string
	Conflicts        string
	Obsoletes        string
	Recommends       string
	Suggests         string
	State            PackageState
	AutoInstalled    bool
	Essential        bool
	InstalledSize    uint64
	DownloadSize     uint64
	PackageID        uint32
	Aliases          []string
}

// PackageChanges represents the changes that would occur during package ops
type PackageChanges struct {
	ExtraInstalled       []string `json:"extraInstalled"`
	UpgradedPackages     []string `json:"upgradedPackages"`
	NewInstalledPackages []string `json:"newInstalledPackages"`
	RemovedPackages      []string `json:"removedPackages"`

	UpgradedCount     int `json:"upgradedCount"`
	NewInstalledCount int `json:"newInstalledCount"`
	RemovedCount      int `json:"removedCount"`
	NotUpgradedCount  int `json:"-"`

	DownloadSize uint64 `json:"downloadSize"`
	InstallSize  uint64 `json:"installSize"`
}

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
#include "apt.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"
)

// Public Go types and error codes

type PackageState int

// Package states (must match AptPackageState in apt_common.h)
const (
	//	PackageStateNotInstalled    PackageState = 0
	PackageStateInstalled PackageState = 1
	//	PackageStateConfigFiles     PackageState = 2
	//	PackageStateUnpacked        PackageState = 3
	//	PackageStateHalfConfigured  PackageState = 4
	//	PackageStateHalfInstalled   PackageState = 5
	//	PackageStateTriggersAwaited PackageState = 6
	//	PackageStateTriggersPending PackageState = 7
)

// APT error codes (must match apt_error.h)
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
	Files            []string
}

// EssentialPackage represents an essential/important package that will be removed
type EssentialPackage struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// PackageChanges represents the changes that would occur during package ops
type PackageChanges struct {
	ExtraInstalled       []string `json:"extraInstalled"`
	UpgradedPackages     []string `json:"upgradedPackages"`
	NewInstalledPackages []string `json:"newInstalledPackages"`
	RemovedPackages      []string `json:"removedPackages"`
	KeptBackPackages     []string `json:"keptBackPackages"`

	UpgradedCount     int `json:"upgradedCount"`
	NewInstalledCount int `json:"newInstalledCount"`
	RemovedCount      int `json:"removedCount"`
	KeptBackCount     int `json:"keptBackCount"`
	NotUpgradedCount  int `json:"notUpgradedCount"`

	DownloadSize uint64 `json:"downloadSize"`
	InstallSize  int64  `json:"installSize"`

	EssentialPackages []EssentialPackage `json:"essentialPackages"`
}

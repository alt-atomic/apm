package lib

/*
// cgo-timestamp: 1754823911
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
	PackageStateNotInstalled PackageState = iota
	PackageStateInstalled
	PackageStateConfigFiles
	PackageStateUnpacked
	PackageStateHalfConfigured
	PackageStateHalfInstalled
	PackageStateTriggersAwaited
	PackageStateTriggerssPending
)

// APT error codes (must match apt_wrapper.h)
const (
	APT_SUCCESS = 0

	APT_ERROR_INIT_FAILED        = 1
	APT_ERROR_CONFIG_FAILED      = 2
	APT_ERROR_SYSTEM_INIT_FAILED = 3

	APT_ERROR_CACHE_OPEN_FAILED    = 11
	APT_ERROR_CACHE_REFRESH_FAILED = 12
	APT_ERROR_CACHE_UPDATE_FAILED  = 13
	APT_ERROR_CACHE_CORRUPTED      = 14

	APT_ERROR_PACKAGE_NOT_FOUND                  = 21
	APT_ERROR_PACKAGE_NOT_INSTALLED              = 22
	APT_ERROR_PACKAGE_ALREADY_INSTALLED          = 23
	APT_ERROR_PACKAGE_VIRTUAL_MULTIPLE_PROVIDERS = 24
	APT_ERROR_PACKAGE_VIRTUAL_NO_PROVIDERS       = 25
	APT_ERROR_PACKAGE_ESSENTIAL                  = 26
	APT_ERROR_PACKAGE_INFO_UNAVAILABLE           = 27

	APT_ERROR_DEPENDENCY_BROKEN       = 41
	APT_ERROR_DEPENDENCY_UNRESOLVABLE = 42
	APT_ERROR_DEPENDENCY_CONFLICTS    = 43
	APT_ERROR_UNMET_DEPENDENCIES      = 44

	APT_ERROR_OPERATION_COMPLETED  = 51
	APT_ERROR_OPERATION_FAILED     = 52
	APT_ERROR_OPERATION_INCOMPLETE = 53
	APT_ERROR_INSTALL_FAILED       = 54
	APT_ERROR_REMOVE_FAILED        = 55
	APT_ERROR_UPGRADE_FAILED       = 56
	APT_ERROR_DOWNLOAD_FAILED      = 57
	APT_ERROR_ARCHIVE_FAILED       = 58
	APT_ERROR_SUBPROCESS_ERROR     = 59

	APT_ERROR_LOCK_FAILED       = 71
	APT_ERROR_PERMISSION_DENIED = 72
	APT_ERROR_LOCK_TIMEOUT      = 73

	APT_ERROR_OUT_OF_MEMORY = 81
	APT_ERROR_DISK_SPACE    = 82
	APT_ERROR_NETWORK       = 83
	APT_ERROR_IO_ERROR      = 84
	APT_ERROR_PIPE_FAILED   = 85

	APT_ERROR_INVALID_PARAMETERS   = 91
	APT_ERROR_INVALID_PACKAGE_NAME = 92
	APT_ERROR_INVALID_REGEX        = 93

	APT_ERROR_UNKNOWN = 999
)

type AptError struct {
	Code    int
	Message string
}

//func (e *AptError) Error() string { return fmt.Sprintf("APT Error %d: %s", e.Code, e.Message) }
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
	NotUpgradedCount  int `json:"notUpgradedCount"`

	DownloadSize uint64 `json:"downloadSize"`
	InstallSize  uint64 `json:"installSize"`
}

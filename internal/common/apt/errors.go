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

package apt

import (
	"apm/lib"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	ErrBrokenPackages = iota + 1
	ErrPermissionDenied
	ErrInternalBrokenPackages
	ErrRemoveDisabled
	ErrLockDownloadDir
	ErrYWithoutForceYes
	ErrNotEnoughSpace
	ErrPackageFileOutOfSync
	ErrTrivialOnly
	ErrOperationCancelled
	ErrMissingBuilddepPackage
	ErrSourcePackageNotFound
	ErrBuilddepInfoFailed
	ErrBuilddepBrokenPackages
	ErrVirtualNoProviders
	ErrVirtualMultipleProviders
	ErrNoPackagesFound
	ErrPackageNotInstalled
	ErrReleaseNotFound
	ErrVersionNotFound
	ErrSourcesListReadFailed
	ErrSourcesListMissing
	ErrExcessiveArguments
	ErrResolverBroken
	ErrOrderingFailed
	ErrDownloadFailed
	ErrFetchArchivesFailed
	ErrFixMissingUnsupported
	ErrCorrectMissingFailed
	ErrAbortingInstall
	ErrParseNameFailed
	ErrWriteStdoutFailed
	ErrMaxArgumentsExceeded
	ErrDependencyUnsatisfied
	ErrRegexCompilationError
	ErrNoInstallationCandidate
	ErrInternalAllUpgrade
	ErrRequestedAutoremoveFailed
	ErrPackageNotFound
	ErrDependencyUnsatisfied2
	ErrFailedDependencyTooNew
	ErrFailedDependency
	ErrGiveOnePattern
	ErrNoHelpForThat
	ErrChangesToBeMade
	ErrFailedToFetchArchives
	ErrFailedToFetch
	ErrFailedToFetchSomeIndex
	ErrUpgradeDisabled
	ErrUnmetDependencies
	ErrMissingFetchSourcePackage
	ErrChildProcessFailed
	ErrMissingChangelogPackage
	ErrProcessBuildDependencies
	ErrVirtualNoProvidersShort
	ErrVirtualMultipleProvidersShort
	ErrRpmDatabaseLock
	ErrPackageIsAlreadyNewest
	ErrConflictsViolated
	// ErrAptInitConfigFailed bindings-specific (APT wrapper messages)
	ErrAptInitConfigFailed
	ErrInvalidSystemPointer
	ErrAptInitSystemFailed
	ErrInvalidArgsCacheOpen
	ErrSystemNotInitialized
	ErrAptLockFailed
	ErrCacheOpenFailed
	ErrCheckDepsFailed
	ErrGetDepCacheFailed
	ErrCacheReopenFailed
	ErrCheckDepsAfterRefreshFailed
	ErrGetDepCacheAfterRefreshFailed
	ErrGetPackageIndexesFailed
	ErrDownloadPackageListsFailed
	ErrRebuildCachesFailed
	ErrInvalidCacheForPM
	ErrCreatePackageManagerFailed
	ErrInvalidArgsMarkInstall
	ErrInvalidArgsMarkRemove
	ErrResolverRemoveDepsFailed
	ErrInvalidArgsMarkKeep
	ErrInvalidArgsMarkAuto
	ErrInvalidPMInstance
	ErrCannotInstallWithBrokenDeps
	ErrGetPackageArchivesFailed
	ErrDownloadPackagesFailed
	ErrPMOperationFailed
	ErrPMOperationIncomplete
	ErrPMUnknownResult
	ErrUpdatePackageMarksFailed
	ErrInvalidCacheForDistUpgrade
	ErrDistUpgradeFailed
	ErrCreatePMForDistUpgradeFailed
	ErrGetArchivesForDistUpgradeFailed
	ErrDownloadPackagesForDist
	ErrUpdateMarksAfterDistUpgradeFailed
	ErrInvalidParametersForSearch
	ErrCreatePackageRecordsParserFailed
	ErrAllocSearchResultsFailed
	ErrUnknownExceptionInSearch
	ErrInvalidParametersForGetInfo
	ErrInvalidParametersForSimulation
	ErrCacheFileNotAvailable
	ErrInvalidParametersForMultiSimulation
	ErrVirtualNoInstallableProviders
	ErrVirtualMultipleProvidersNeedSelect
	ErrPackageIsNotInstalled
	ErrVirtualNoInstalledProviders
	ErrVirtualMultipleInstalledProviders
	ErrDistUpgradeSimulationFailed
	ErrMultiInstallSimulationFailed
	ErrMultiRemoveSimulationFailed
	ErrCombinedSimulationFailed
	ErrVirtualNameMultipleProvidersExact
)

// MatchedError представляет найденную ошибку с извлечёнными параметрами.
type MatchedError struct {
	Entry  ErrorEntry
	Params []string
}

// ErrorEntry описывает шаблон ошибки.
type ErrorEntry struct {
	Code              int
	Pattern           string
	TranslatedPattern func() string
	Params            int
}

var errorPatterns = []ErrorEntry{
	{ErrBrokenPackages, "Broken packages", func() string {
		return lib.T_("Broken packages")
	}, 0},
	{ErrPermissionDenied, "You have no permissions for that", func() string {
		return lib.T_("You have no permissions for that")
	}, 0},
	{ErrInternalBrokenPackages, "Internal Error, InstallPackages was called with broken packages!", func() string {
		return lib.T_("Internal Error, InstallPackages was called with broken packages!")
	}, 0},
	{ErrRemoveDisabled, "Packages need to be removed but Remove is disabled.", func() string {
		return lib.T_("Packages need to be removed but Remove is disabled.")
	}, 0},
	{ErrLockDownloadDir, "Unable to lock the download directory", func() string {
		return lib.T_("Unable to lock the download directory")
	}, 0},
	{ErrYWithoutForceYes, "There are problems and -y was used without --force-yes", func() string {
		return lib.T_("There are problems and -y was used without --force-yes")
	}, 0},
	{ErrNotEnoughSpace, "You don't have enough free space in %s", func() string {
		return lib.T_("You don't have enough free space in %s")
	}, 1},
	{ErrPackageFileOutOfSync, "Package file %s is out of sync", func() string {
		return lib.T_("Package file %s is out of sync")
	}, 1},
	{ErrTrivialOnly, "Trivial Only specified but this is not a trivial operation", func() string {
		return lib.T_("Trivial Only specified but this is not a trivial operation")
	}, 0},
	{ErrOperationCancelled, "Operation cancelled", func() string {
		return lib.T_("Operation cancelled")
	}, 0},
	{ErrMissingBuilddepPackage, "Must specify at least one package to check builddeps for", func() string {
		return lib.T_("Must specify at least one package to check builddeps for")
	}, 0},
	{ErrSourcePackageNotFound, "Unable to find a source package for %s", func() string {
		return lib.T_("Unable to find a source package for %s")
	}, 1},
	{ErrBuilddepInfoFailed, "Unable to get build-dependency information for %s", func() string {
		return lib.T_("Unable to get build-dependency information for %s")
	}, 1},
	{ErrBuilddepBrokenPackages, "Some broken packages were found while trying to process build-dependencies for %s.", func() string {
		return lib.T_("Some broken packages were found while trying to process build-dependencies for %s.")
	}, 1},
	{ErrVirtualNoProviders, "Package %s is a virtual package with no good providers", func() string {
		return lib.T_("Package %s is a virtual package with no good providers")
	}, 1},
	{ErrVirtualMultipleProviders, "Package %s is a virtual package with multiple good providers", func() string {
		return lib.T_("Package %s is a virtual package with multiple good providers")
	}, 1},
	{ErrNoPackagesFound, "No packages found", func() string {
		return lib.T_("No packages found")
	}, 0},
	{ErrPackageNotInstalled, "Package %s is not installed, so not removed", func() string {
		return lib.T_("Package %s is not installed, so not removed")
	}, 1},
	{ErrReleaseNotFound, "Release %s'%s' for '%s' was not found", func() string {
		return lib.T_("Release %s'%s' for '%s' was not found")
	}, 3},
	{ErrVersionNotFound, "Version %s'%s' for '%s' was not found", func() string {
		return lib.T_("Version %s'%s' for '%s' was not found")
	}, 3},
	{ErrSourcesListReadFailed, "Sources list %s could not be read", func() string {
		return lib.T_("Sources list %s could not be read")
	}, 1},
	{ErrSourcesListMissing, "Sources list %s doesn't exist", func() string {
		return lib.T_("Sources list %s doesn't exist")
	}, 1},
	{ErrExcessiveArguments, "Excessive arguments", func() string {
		return lib.T_("Excessive arguments")
	}, 0},
	{ErrResolverBroken, "Internal Error, problem resolver broke stuff", func() string {
		return lib.T_("Internal Error, problem resolver broke stuff")
	}, 0},
	{ErrOrderingFailed, "Internal Error, Ordering didn't finish", func() string {
		return lib.T_("Internal Error, Ordering didn't finish")
	}, 0},
	{ErrDownloadFailed, "Some files failed to download", func() string {
		return lib.T_("Some files failed to download")
	}, 0},
	{ErrFetchArchivesFailed, "Unable to fetch some archives, maybe run apt-get update or try with --fix-missing?", func() string {
		return lib.T_("Unable to fetch some archives, maybe run apt-get update or try with --fix-missing?")
	}, 0},
	{ErrFixMissingUnsupported, "--fix-missing and media swapping is not currently supported", func() string {
		return lib.T_("--fix-missing and media swapping is not currently supported")
	}, 0},
	{ErrCorrectMissingFailed, "Unable to correct missing packages", func() string {
		return lib.T_("Unable to correct missing packages")
	}, 0},
	{ErrAbortingInstall, "Aborting Install", func() string {
		return lib.T_("Aborting Install")
	}, 0},
	{ErrRpmDatabaseLock, "Could not open RPM database", func() string {
		return lib.T_("Could not open RPM database")
	}, 0},
	{ErrParseNameFailed, "Couldn't parse name '%s'", func() string {
		return lib.T_("Couldn't parse name '%s'")
	}, 1},
	{ErrWriteStdoutFailed, "Write to stdout failed", func() string {
		return lib.T_("Write to stdout failed")
	}, 0},
	{ErrMaxArgumentsExceeded, "Exceeded maximum number of command arguments", func() string {
		return lib.T_("Exceeded maximum number of command arguments")
	}, 0},
	{ErrDependencyUnsatisfied, "Package %s dependency for %s cannot be satisfied because the package %s cannot be found", func() string {
		return lib.T_("Package %s dependency for %s cannot be satisfied because the package %s cannot be found")
	}, 3},
	{ErrRegexCompilationError, "Regex compilation error - %s", func() string {
		return lib.T_("Regex compilation error - %s")
	}, 1},
	{ErrNoInstallationCandidate, "Package %s has no installation candidate", func() string {
		return lib.T_("Package %s has no installation candidate")
	}, 1},
	{ErrInternalAllUpgrade, "Internal Error, AllUpgrade broke stuff", func() string {
		return lib.T_("Internal Error, AllUpgrade broke stuff")
	}, 0},
	{ErrRequestedAutoremoveFailed, "Requested autoremove failed.", func() string {
		return lib.T_("Requested autoremove failed.")
	}, 0},
	{ErrPackageNotFound, "Couldn't find package %s", func() string {
		return lib.T_("Couldn't find package %s")
	}, 1},
	{ErrDependencyUnsatisfied2, "%s dependency for %s cannot be satisfied", func() string {
		return lib.T_("%s dependency for %s cannot be satisfied")
	}, 2},
	{ErrFailedDependencyTooNew, "Failed to satisfy %s dependency for %s: Installed package %s is too new", func() string {
		return lib.T_("Failed to satisfy %s dependency for %s: Installed package %s is too new")
	}, 3},
	{ErrFailedDependency, "Failed to satisfy %s dependency for %s: %s", func() string {
		return lib.T_("Failed to satisfy %s dependency for %s: %s")
	}, 3},
	{ErrGiveOnePattern, "You must give exactly one pattern", func() string {
		return lib.T_("You must give exactly one pattern")
	}, 0},
	{ErrNoHelpForThat, "No help for that", func() string { return lib.T_("No help for that") }, 0},
	{ErrPackageIsAlreadyNewest, "%s is already the newest version.", func() string {
		return lib.T_("%s is already the newest version.")
	}, 1},
	{ErrSourcesListReadFailed, "The list of sources could not be read.", func() string {
		return lib.T_("The list of sources could not be read.")
	}, 0},
	{ErrChangesToBeMade, "There are changes to be made", func() string {
		return lib.T_("There are changes to be made")
	}, 0},
	{ErrFailedToFetchArchives, "Failed to fetch some archives.", func() string {
		return lib.T_("Failed to fetch some archives.")
	}, 0},
	{ErrFailedToFetch, "Failed to fetch %s  %s", func() string {
		return lib.T_("Failed to fetch %s  %s")
	}, 2},
	{ErrFailedToFetchSomeIndex, "Some index files failed to download. They have been ignored, or old ones used instead.", func() string {
		return lib.T_("Some index files failed to download. They have been ignored, or old ones used instead.")
	}, 0},
	{ErrUpgradeDisabled, "'apt-get upgrade' is disabled because it can leave system in a broken state.", func() string {
		return lib.T_("'apt-get upgrade' is disabled because it can leave system in a broken state.")
	}, 0},
	{ErrUnmetDependencies, "Unmet dependencies. Try 'apt-get --fix-broken install' with no packages (or specify a solution).", func() string {
		return lib.T_("Unmet dependencies. Try 'apt-get --fix-broken install' with no packages (or specify a solution).")
	}, 0},
	{ErrMissingFetchSourcePackage, "Must specify at least one package to fetch source for", func() string {
		return lib.T_("Must specify at least one package to fetch source for")
	}, 0},
	{ErrConflictsViolated, "Fatal, conflicts violated %s", func() string {
		return lib.T_("Fatal: conflicts violated %s")
	}, 1},
	{ErrChildProcessFailed, "Child process failed", func() string {
		return lib.T_("Child process failed")
	}, 0},
	{ErrMissingChangelogPackage, "Must specify at least one package to get changelog for", func() string {
		return lib.T_("Must specify at least one package to get changelog for")
	}, 0},
	{ErrProcessBuildDependencies, "Failed to process build dependencies", func() string {
		return lib.T_("Failed to process build dependencies")
	}, 0},
	{ErrVirtualNoProvidersShort, "Package %s is a virtual package with no ", func() string {
		return lib.T_("Package %s is a virtual package with no ")
	}, 1},
	{ErrVirtualMultipleProvidersShort, "Package %s is a virtual package with multiple ", func() string {
		return lib.T_("Package %s is a virtual package with multiple ")
	}, 1},
	// Bindings specific patterns (from C++ bindings messages)
	{ErrAptInitConfigFailed, "Failed to initialize APT configuration", func() string { return lib.T_("Failed to initialize APT configuration") }, 0},
	{ErrInvalidSystemPointer, "Invalid system pointer", func() string { return lib.T_("Invalid system pointer") }, 0},
	{ErrAptInitSystemFailed, "Failed to initialize APT system", func() string { return lib.T_("Failed to initialize APT system") }, 0},
	{ErrInvalidArgsCacheOpen, "Invalid arguments for cache_open", func() string { return lib.T_("Invalid arguments for cache_open") }, 0},
	{ErrSystemNotInitialized, "System not properly initialized", func() string { return lib.T_("System not properly initialized") }, 0},
	{ErrAptLockFailed, "Unable to acquire APT system lock - another process may be using APT", func() string { return lib.T_("Unable to acquire APT system lock - another process may be using APT") }, 0},
	{ErrCacheOpenFailed, "Failed to open APT cache", func() string { return lib.T_("Failed to open APT cache") }, 0},
	{ErrCheckDepsFailed, "Failed to check dependencies", func() string { return lib.T_("Failed to check dependencies") }, 0},
	{ErrGetDepCacheFailed, "Failed to get dependency cache", func() string { return lib.T_("Failed to get dependency cache") }, 0},
	{ErrCacheReopenFailed, "Failed to reopen cache after refresh", func() string { return lib.T_("Failed to reopen cache after refresh") }, 0},
	{ErrCheckDepsAfterRefreshFailed, "Failed to check dependencies after refresh", func() string { return lib.T_("Failed to check dependencies after refresh") }, 0},
	{ErrGetDepCacheAfterRefreshFailed, "Failed to get dependency cache after refresh", func() string { return lib.T_("Failed to get dependency cache after refresh") }, 0},
	{ErrGetPackageIndexesFailed, "Failed to get package indexes", func() string { return lib.T_("Failed to get package indexes") }, 0},
	{ErrDownloadPackageListsFailed, "Failed to download package lists", func() string { return lib.T_("Failed to download package lists") }, 0},
	{ErrRebuildCachesFailed, "Failed to rebuild caches", func() string { return lib.T_("Failed to rebuild caches") }, 0},
	{ErrInvalidCacheForPM, "Invalid cache or output pointer for pm create", func() string { return lib.T_("Invalid cache or output pointer for pm create") }, 0},
	{ErrCreatePackageManagerFailed, "Failed to create package manager", func() string { return lib.T_("Failed to create package manager") }, 0},
	{ErrInvalidArgsMarkInstall, "Invalid arguments for mark_install", func() string { return lib.T_("Invalid arguments for mark_install") }, 0},
	{ErrInvalidArgsMarkRemove, "Invalid arguments for mark_remove", func() string { return lib.T_("Invalid arguments for mark_remove") }, 0},
	{ErrResolverRemoveDepsFailed, "Problem resolver failed to handle package removal dependencies", func() string { return lib.T_("Problem resolver failed to handle package removal dependencies") }, 0},
	{ErrInvalidArgsMarkKeep, "Invalid arguments for mark_keep", func() string { return lib.T_("Invalid arguments for mark_keep") }, 0},
	{ErrInvalidArgsMarkAuto, "Invalid arguments for mark_auto", func() string { return lib.T_("Invalid arguments for mark_auto") }, 0},
	{ErrInvalidPMInstance, "Invalid package manager instance", func() string { return lib.T_("Invalid package manager instance") }, 0},
	{ErrCannotInstallWithBrokenDeps, "Cannot install packages with broken dependencies", func() string { return lib.T_("Cannot install packages with broken dependencies") }, 0},
	{ErrGetPackageArchivesFailed, "Failed to get package archives", func() string { return lib.T_("Failed to get package archives") }, 0},
	{ErrDownloadPackagesFailed, "Failed to download packages", func() string { return lib.T_("Failed to download packages") }, 0},
	{ErrPMOperationFailed, "Package manager operation failed", func() string { return lib.T_("Package manager operation failed") }, 0},
	{ErrPMOperationIncomplete, "Package manager operation incomplete", func() string { return lib.T_("Package manager operation incomplete") }, 0},
	{ErrPMUnknownResult, "Unknown package manager result", func() string { return lib.T_("Unknown package manager result") }, 0},
	{ErrUpdatePackageMarksFailed, "Failed to update package marks", func() string { return lib.T_("Failed to update package marks") }, 0},
	{ErrInvalidCacheForDistUpgrade, "Invalid cache for dist upgrade", func() string { return lib.T_("Invalid cache for dist upgrade") }, 0},
	{ErrDistUpgradeFailed, "Distribution upgrade failed", func() string { return lib.T_("Distribution upgrade failed") }, 0},
	{ErrCreatePMForDistUpgradeFailed, "Failed to create package manager for dist upgrade", func() string { return lib.T_("Failed to create package manager for dist upgrade") }, 0},
	{ErrGetArchivesForDistUpgradeFailed, "Failed to get package archives for dist upgrade", func() string { return lib.T_("Failed to get package archives for dist upgrade") }, 0},
	{ErrDownloadPackagesForDist, "Failed to download packages for dist upgrade", func() string { return lib.T_("Failed to download packages for dist upgrade") }, 0},
	{ErrUpdateMarksAfterDistUpgradeFailed, "Failed to update package marks after dist upgrade", func() string { return lib.T_("Failed to update package marks after dist upgrade") }, 0},
	{ErrInvalidParametersForSearch, "Invalid parameters for search", func() string { return lib.T_("Invalid parameters for search") }, 0},
	{ErrRegexCompilationError, "Failed to compile regex pattern", func() string { return lib.T_("Failed to compile regex pattern") }, 0},
	{ErrCreatePackageRecordsParserFailed, "Failed to create package records parser", func() string { return lib.T_("Failed to create package records parser") }, 0},
	{ErrAllocSearchResultsFailed, "Failed to allocate memory for search results", func() string { return lib.T_("Failed to allocate memory for search results") }, 0},
	{ErrUnknownExceptionInSearch, "Unknown exception in search", func() string { return lib.T_("Unknown exception in search") }, 0},
	{ErrInvalidParametersForGetInfo, "Invalid parameters for get_package_info", func() string { return lib.T_("Invalid parameters for get_package_info") }, 0},
	{ErrInvalidParametersForSimulation, "Invalid parameters for simulation", func() string { return lib.T_("Invalid parameters for simulation") }, 0},
	{ErrCacheFileNotAvailable, "Cache file not available", func() string { return lib.T_("Cache file not available") }, 0},
	{ErrInvalidParametersForMultiSimulation, "Invalid parameters for multi-package simulation", func() string { return lib.T_("Invalid parameters for multi-package simulation") }, 0},
	{ErrVirtualNoInstallableProviders, "Virtual package %s has no installable providers", func() string { return lib.T_("Virtual package %s has no installable providers") }, 1},
	{ErrVirtualMultipleProvidersNeedSelect, "Virtual package %s has multiple providers. Please select specific package.", func() string {
		return lib.T_("Virtual package %s has multiple providers. Please select specific package.")
	}, 1},
	{ErrPackageNotFound, "Package not found: %s", func() string { return lib.T_("Package not found: %s") }, 1},
	{ErrPackageIsNotInstalled, "Package is not installed: %s", func() string { return lib.T_("Package is not installed: %s") }, 1},
	{ErrVirtualNoInstalledProviders, "Package %s has no installed providers", func() string { return lib.T_("Package %s has no installed providers") }, 1},
	{ErrVirtualMultipleInstalledProviders, "Virtual package %s has multiple installed providers: %s. Please remove specific package.", func() string {
		return lib.T_("Virtual package %s has multiple installed providers: %s. Please remove specific package.")
	}, 2},
	{ErrDistUpgradeSimulationFailed, "Dist upgrade simulation failed: %s", func() string { return lib.T_("Dist upgrade simulation failed: %s") }, 1},
	{ErrMultiInstallSimulationFailed, "Multi-package install simulation failed: %s", func() string { return lib.T_("Multi-package install simulation failed: %s") }, 1},
	{ErrMultiRemoveSimulationFailed, "Multi-package remove simulation failed: %s", func() string { return lib.T_("Multi-package remove simulation failed: %s") }, 1},
	{ErrCombinedSimulationFailed, "Combined simulation failed: %s", func() string { return lib.T_("Combined simulation failed: %s") }, 1},
	{ErrVirtualNameMultipleProvidersExact, "Virtual name '%s' has multiple providers; specify exact package name", func() string { return lib.T_("Virtual name '%s' has multiple providers; specify exact package name") }, 1},
}

// ErrorLinesAnalyseAll проверяет все строки и возвращает срез найденных ошибок.
func ErrorLinesAnalyseAll(lines []string) []*MatchedError {
	var errorsFound []*MatchedError
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		cleanLine := strings.ReplaceAll(trimmed, "E: ", "")
		if matchedErr := CheckError(cleanLine); matchedErr != nil {
			errorsFound = append(errorsFound, matchedErr)
		}
	}
	return errorsFound
}

// ErrorLinesAnalise возвращает любую ошибку
func ErrorLinesAnalise(lines []string) *MatchedError {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		cleanLine := strings.ReplaceAll(trimmed, "E: ", "")

		if matchedErr := CheckError(cleanLine); matchedErr != nil {
			return matchedErr
		}
	}

	return nil
}

// CheckError ищет ошибку в тексте requestError с учетом шаблонов и возвращает найденную ошибку с параметрами.
func CheckError(requestError string) *MatchedError {
	for _, entry := range errorPatterns {
		regexPattern := patternToRegex(entry.Pattern)
		re, err := regexp.Compile(regexPattern)
		if err != nil {
			continue
		}
		matches := re.FindStringSubmatch(requestError)
		if len(matches) > 0 {
			var params []string
			if len(matches) > 1 {
				params = matches[1:]
			}
			return &MatchedError{
				Entry:  entry,
				Params: params,
			}
		}
	}
	return nil
}

// Error возвращает переведённое сообщение об ошибке с подстановкой найденных параметров.
func (e *MatchedError) Error() string {
	var template = e.Entry.TranslatedPattern()

	if e.Entry.Params > 0 && len(e.Params) >= e.Entry.Params {
		return fmt.Sprintf(template, toInterfaceSlice(e.Params[:e.Entry.Params])...)
	}
	return template
}

func (e *MatchedError) IsCritical() bool {
	switch e.Entry.Code {
	case ErrPackageNotInstalled:
		return false
	case ErrPackageIsAlreadyNewest:
		return false
	default:
		return true
	}
}

func (e *MatchedError) NeedUpdate() bool {
	switch e.Entry.Code {
	case ErrFailedToFetchArchives:
		return true
	case ErrFailedToFetch:
		return true
	case ErrFailedToFetchSomeIndex:
		return true
	default:
		return false
	}
}

func FindCriticalError(errorList []error) error {
	for _, err := range errorList {
		var matchedErr *MatchedError
		if err != nil && !errors.As(err, &matchedErr) {
			return err
		}

		if err != nil && matchedErr.IsCritical() {
			return matchedErr
		}
	}

	return nil
}

func patternToRegex(pattern string) string {
	parts := strings.Split(pattern, "%s")
	for i, part := range parts {
		parts[i] = regexp.QuoteMeta(part)
	}
	return "^" + strings.Join(parts, "(.+)") + "$"
}

func toInterfaceSlice(strings []string) []interface{} {
	result := make([]interface{}, len(strings))
	for i, s := range strings {
		result[i] = s
	}
	return result
}

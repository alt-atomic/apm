package apt

import (
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
)

// MatchedError представляет найденную ошибку с извлечёнными параметрами.
type MatchedError struct {
	Entry  ErrorEntry
	Params []string
}

// ErrorEntry описывает шаблон ошибки.
type ErrorEntry struct {
	Code    int
	Pattern string
	Params  int
}

var errorPatterns = []ErrorEntry{
	{ErrBrokenPackages, "Broken packages", 0},
	{ErrPermissionDenied, "You have no permissions for that", 0},
	{ErrInternalBrokenPackages, "Internal Error, InstallPackages was called with broken packages!", 0},
	{ErrRemoveDisabled, "Packages need to be removed but Remove is disabled.", 0},
	{ErrLockDownloadDir, "Unable to lock the download directory", 0},
	{ErrYWithoutForceYes, "There are problems and -y was used without --force-yes", 0},
	{ErrNotEnoughSpace, "You don't have enough free space in %s", 1},
	{ErrPackageFileOutOfSync, "Package file %s is out of sync", 1},
	{ErrTrivialOnly, "Trivial Only specified but this is not a trivial operation", 0},
	{ErrOperationCancelled, "Operation cancelled", 0},
	{ErrMissingBuilddepPackage, "Must specify at least one package to check builddeps for", 0},
	{ErrSourcePackageNotFound, "Unable to find a source package for %s", 1},
	{ErrBuilddepInfoFailed, "Unable to get build-dependency information for %s", 1},
	{ErrBuilddepBrokenPackages, "Some broken packages were found while trying to process build-dependencies for %s.\\n\"\n\t\t\t\t\"You might want to run `apt-get --fix-broken install' to correct these.", 1},
	{ErrVirtualNoProviders, "Package %s is a virtual package with no good providers", 1},
	{ErrVirtualMultipleProviders, "Package %s is a virtual package with multiple good providers", 1},
	{ErrNoPackagesFound, "No packages found", 0},
	{ErrPackageNotInstalled, "Package %s is not installed, so not removed", 1},
	{ErrReleaseNotFound, "Release %s'%s' for '%s' was not found", 3},
	{ErrVersionNotFound, "Version %s'%s' for '%s' was not found", 3},
	{ErrSourcesListReadFailed, "Sources list %s could not be read", 1},
	{ErrSourcesListMissing, "Sources list %s doesn't exist", 1},
	{ErrExcessiveArguments, "Excessive arguments", 0},
	{ErrResolverBroken, "Internal Error, problem resolver broke stuff", 0},
	{ErrOrderingFailed, "Internal Error, Ordering didn't finish", 0},
	{ErrDownloadFailed, "Some files failed to download", 0},
	{ErrFetchArchivesFailed, "Unable to fetch some archives, maybe run apt-get update or try with --fix-missing?", 0},
	{ErrFixMissingUnsupported, "--fix-missing and media swapping is not currently supported", 0},
	{ErrCorrectMissingFailed, "Unable to correct missing packages", 0},
	{ErrAbortingInstall, "Aborting Install", 0},
	{ErrRpmDatabaseLock, "Could not open RPM database", 0},
	{ErrParseNameFailed, "Couldn't parse name '%s'", 1},
	{ErrWriteStdoutFailed, "Write to stdout failed", 0},
	{ErrMaxArgumentsExceeded, "Exceeded maximum number of command arguments", 0},
	{ErrDependencyUnsatisfied, "Package %s dependency for %s cannot be satisfied because the package %s cannot be found", 3},
	{ErrRegexCompilationError, "Regex compilation error - %s", 1},
	{ErrNoInstallationCandidate, "Package %s has no installation candidate", 1},
	{ErrInternalAllUpgrade, "Internal Error, AllUpgrade broke stuff", 0},
	{ErrRequestedAutoremoveFailed, "Requested autoremove failed.", 0},
	{ErrPackageNotFound, "Couldn't find package %s", 1},
	{ErrDependencyUnsatisfied2, "%s dependency for %s cannot be satisfied", 2},
	{ErrFailedDependencyTooNew, "Failed to satisfy %s dependency for %s: Installed package %s is too new", 3},
	{ErrFailedDependency, "Failed to satisfy %s dependency for %s: %s", 3},
	{ErrGiveOnePattern, "You must give exactly one pattern", 0},
	{ErrNoHelpForThat, "No help for that", 0},
	{ErrSourcesListReadFailed, "The list of sources could not be read.", 0},
	{ErrChangesToBeMade, "There are changes to be made", 0},
	{ErrFailedToFetchArchives, "Failed to fetch some archives.", 0},
	{ErrFailedToFetch, "Failed to fetch %s  %s", 2},
	{ErrFailedToFetchSomeIndex, "Some index files failed to download. They have been ignored, or old ones used instead.", 0},
	{ErrUpgradeDisabled, "'apt-get upgrade' is disabled because it can leave system in a broken state.\\n\"\n                             \"It is advised to use 'apt-get dist-upgrade' instead.\\n\"\n                             \"If you still want to use 'apt-get upgrade' despite of this warning,\\n\"\n                             \"use '--enable-upgrade' option or enable 'APT::Get::EnableUpgrade' configuration setting\"", 0},
	{ErrUnmetDependencies, "Unmet dependencies. Try 'apt-get --fix-broken install' with no packages (or specify a solution).", 0},
	{ErrMissingFetchSourcePackage, "Must specify at least one package to fetch source for", 0},
	{ErrChildProcessFailed, "Child process failed", 0},
	{ErrMissingChangelogPackage, "Must specify at least one package to get changelog for", 0},
	{ErrProcessBuildDependencies, "Failed to process build dependencies", 0},
	{ErrVirtualNoProvidersShort, "Package %s is a virtual package with no ", 1},
	{ErrVirtualMultipleProvidersShort, "Package %s is a virtual package with multiple ", 1},
}

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
	translations := map[int]string{
		ErrBrokenPackages:      "Сломанные пакеты",
		ErrPermissionDenied:    "У вас нет прав для выполнения этой операции",
		ErrNotEnoughSpace:      "У вас недостаточно свободно места в %s",
		ErrPackageNotInstalled: "Пакет %s не установлен, и не может быть удалён",
		// можно добавить и другие переводы по необходимости
	}
	template, ok := translations[e.Entry.Code]
	if !ok {
		template = e.Entry.Pattern
	}
	if e.Entry.Params > 0 && len(e.Params) >= e.Entry.Params {
		return fmt.Sprintf(template, toInterfaceSlice(e.Params[:e.Entry.Params])...)
	}
	return template
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

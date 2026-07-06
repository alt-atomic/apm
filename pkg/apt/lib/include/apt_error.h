#ifndef APT_ERROR_H
#define APT_ERROR_H
#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

// Error codes for APT operations, grouped by subsystem.
typedef enum {
    APT_SUCCESS = 0,

    APT_ERROR_INIT_FAILED = 1,

    APT_ERROR_CACHE_OPEN_FAILED = 11,
    APT_ERROR_CACHE_REFRESH_FAILED = 12,
    APT_ERROR_CACHE_UPDATE_FAILED = 13,

    APT_ERROR_PACKAGE_NOT_FOUND = 21,
    APT_ERROR_PACKAGES_ALREADY_INSTALLED = 22,

    APT_ERROR_DEPENDENCY_BROKEN = 41,

    APT_ERROR_OPERATION_FAILED = 52,
    APT_ERROR_OPERATION_INCOMPLETE = 53,
    APT_ERROR_INSTALL_FAILED = 54,
    APT_ERROR_DOWNLOAD_FAILED = 57,

    APT_ERROR_LOCK_FAILED = 71,

    APT_ERROR_INVALID_PARAMETERS = 91,

    APT_ERROR_UNKNOWN = 999
} AptErrorCode;

// Result of an APT operation. Caller must free `message` on failure.
typedef struct {
    AptErrorCode code;
    char *message;
} AptResult;

/* Init */
#define APT_MSG_INIT_CONFIG_FAILED      "Failed to initialize APT configuration"
#define APT_MSG_INIT_SYSTEM_FAILED      "Failed to initialize APT system"
#define APT_MSG_INIT_SYSTEM_INVALID_PTR "Invalid system pointer"
#define APT_MSG_INIT_PM_FAILED          "Failed to create package manager"

/* Cache */
#define APT_MSG_CACHE_INVALID_ARGS      "Invalid arguments for cache_open"
#define APT_MSG_CACHE_SYSTEM_NOT_INIT   "System not properly initialized"
#define APT_MSG_CACHE_DEP_FAILED        "Failed to get dependency cache"
#define APT_MSG_CACHE_NOT_AVAILABLE     "Cache not available"
#define APT_MSG_CACHE_FILE_NOT_AVAILABLE "Cache file not available"

/* Cache refresh */
#define APT_MSG_CACHE_REFRESH_DEP_FAILED "Failed to get dependency cache after refresh"

/* Cache update */
#define APT_MSG_CACHE_INVALIDATE_FAILED "Failed to invalidate releases"
#define APT_MSG_CACHE_RELEASE_FAILED    "Failed to get release files"
#define APT_MSG_CACHE_INDEX_FAILED      "Failed to get package indexes"
#define APT_MSG_CACHE_DOWNLOAD_FAILED   "Repository update failed: Unable to download release files"

/* Lock */
#define APT_MSG_LOCK_LIST_DIR           "Unable to lock the list directory"
#define APT_MSG_LOCK_DOWNLOAD_DIR       "Unable to lock the download directory"

/* Dependencies */
#define APT_MSG_BROKEN_DEPS             "Broken dependencies"
#define APT_MSG_BROKEN_CACHE            "Cache has broken packages"
#define APT_MSG_AUTOREMOVE_CALC_FAILED  "Failed to calculate autoremove packages"
#define APT_MSG_AUTOREMOVE_RESOLVE_FAILED "Failed to resolve dependencies during autoremove"

/* Install / download */
#define APT_MSG_DOWNLOAD_FAILED         "Failed to download packages"
#define APT_MSG_MARKS_UPDATE_FAILED     "Failed to update package marks"

/* Transaction */
#define APT_MSG_TX_ALLOC_FAILED         "Failed to allocate transaction"

/* Parameters validation */
#define APT_MSG_INVALID_PARAMS          "Invalid parameters"
#define APT_MSG_INVALID_CACHE           "Invalid cache"
#define APT_MSG_NULL_TRANSACTION        "Null transaction"
#define APT_MSG_NO_PACKAGE_NAMES        "No package names"

/* Package (single) */
#define APT_MSG_PACKAGE_INVALID_PARAMS  "Invalid parameters for package_get"

/* Packages (search) */
#define APT_MSG_PACKAGES_INVALID_PARAMS "Invalid parameters for packages_search"
#define APT_MSG_PACKAGES_REGEX_FAILED   "Failed to compile regex pattern"
#define APT_MSG_PACKAGES_RECORDS_FAILED "Failed to create package records parser"
#define APT_MSG_PACKAGES_ALLOC_FAILED   "Failed to allocate memory for search results"
#define APT_MSG_PACKAGES_UNKNOWN        "Unknown exception in packages_search"

#ifdef __cplusplus
}
#endif

#endif

#ifndef APT_WRAPPER_H
#define APT_WRAPPER_H
#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

// Forward declarations
typedef struct AptSystem AptSystem;
typedef struct AptCache AptCache;
typedef struct AptPackageManager AptPackageManager;

typedef struct AptVersion AptVersion;

// Error handling - based on real APT error patterns
typedef enum {
    // Success
    APT_SUCCESS = 0,
    
    // Initialization errors (1-10)
    APT_ERROR_INIT_FAILED = 1,
    APT_ERROR_CONFIG_FAILED = 2,
    APT_ERROR_SYSTEM_INIT_FAILED = 3,
    
    // Cache errors (11-20)
    APT_ERROR_CACHE_OPEN_FAILED = 11,
    APT_ERROR_CACHE_REFRESH_FAILED = 12,
    APT_ERROR_CACHE_UPDATE_FAILED = 13,
    APT_ERROR_CACHE_CORRUPTED = 14,
    
    // Package errors (21-40)
    APT_ERROR_PACKAGE_NOT_FOUND = 21,
    APT_ERROR_PACKAGE_NOT_INSTALLED = 22,
    APT_ERROR_PACKAGE_ALREADY_INSTALLED = 23,
    APT_ERROR_PACKAGE_VIRTUAL_MULTIPLE_PROVIDERS = 24,
    APT_ERROR_PACKAGE_VIRTUAL_NO_PROVIDERS = 25,
    APT_ERROR_PACKAGE_ESSENTIAL = 26,
    APT_ERROR_PACKAGE_INFO_UNAVAILABLE = 27,
    
    // Dependency errors (41-50) - from APT CheckDeps/BrokenCount
    APT_ERROR_DEPENDENCY_BROKEN = 41,
    APT_ERROR_DEPENDENCY_UNRESOLVABLE = 42,
    APT_ERROR_DEPENDENCY_CONFLICTS = 43,
    APT_ERROR_UNMET_DEPENDENCIES = 44,
    
    // Package Manager Operation errors (51-70) - from pkgPackageManager::OrderResult
    APT_ERROR_OPERATION_COMPLETED = 51,     // OrderResult::Completed
    APT_ERROR_OPERATION_FAILED = 52,        // OrderResult::Failed
    APT_ERROR_OPERATION_INCOMPLETE = 53,    // OrderResult::Incomplete
    APT_ERROR_INSTALL_FAILED = 54,          // Installation specific failure
    APT_ERROR_REMOVE_FAILED = 55,           // Removal specific failure
    APT_ERROR_UPGRADE_FAILED = 56,          // Upgrade specific failure
    APT_ERROR_DOWNLOAD_FAILED = 57,         // pkgAcquire::Failed
    APT_ERROR_ARCHIVE_FAILED = 58,          // Archive operation failed
    APT_ERROR_SUBPROCESS_ERROR = 59,        // Subprocess error (rpm/dpkg)
    
    // Lock and permission errors (71-80)
    APT_ERROR_LOCK_FAILED = 71,
    APT_ERROR_PERMISSION_DENIED = 72,
    APT_ERROR_LOCK_TIMEOUT = 73,
    
    // Resource errors (81-90)
    APT_ERROR_OUT_OF_MEMORY = 81,
    APT_ERROR_DISK_SPACE = 82,
    APT_ERROR_NETWORK = 83,
    APT_ERROR_IO_ERROR = 84,              // File I/O errors
    APT_ERROR_PIPE_FAILED = 85,           // IPC pipe creation failed
    
    // Validation errors (91-99)
    APT_ERROR_INVALID_PARAMETERS = 91,
    APT_ERROR_INVALID_PACKAGE_NAME = 92,
    APT_ERROR_INVALID_REGEX = 93,
    
    // Generic/Unknown errors (999)
    APT_ERROR_UNKNOWN = 999
} AptErrorCode;

// Callback types
typedef enum {
    APT_CALLBACK_UNKNOWN = 0,
    APT_CALLBACK_INST_PROGRESS = 1,
    APT_CALLBACK_INST_START = 2,
    APT_CALLBACK_INST_STOP = 3,
    APT_CALLBACK_REMOVE_PROGRESS = 4,
    APT_CALLBACK_REMOVE_START = 5,
    APT_CALLBACK_REMOVE_STOP = 6,
    APT_CALLBACK_ERROR = 7,

    APT_CALLBACK_TRANS_PROGRESS = 8,
    APT_CALLBACK_TRANS_START = 9,
    APT_CALLBACK_TRANS_STOP = 10,
    APT_CALLBACK_ELEM_PROGRESS = 11,

    APT_CALLBACK_DOWNLOAD_START = 20,
    APT_CALLBACK_DOWNLOAD_PROGRESS = 21,
    APT_CALLBACK_DOWNLOAD_STOP = 22
} AptCallbackType;

typedef void (*AptProgressCallback)(const char* package_name,
                                   AptCallbackType callback_type,
                                   uint64_t current,
                                   uint64_t total,
                                   void* user_data);

// Optional log callback to route error/info messages instead of stderr
typedef void (*AptLogCallback)(const char* message, void* user_data);

// Package states
typedef enum {
    APT_PKG_STATE_NOT_INSTALLED = 0,
    APT_PKG_STATE_INSTALLED = 1,
    APT_PKG_STATE_CONFIG_FILES = 2,
    APT_PKG_STATE_UNPACKED = 3,
    APT_PKG_STATE_HALF_CONFIGURED = 4,
    APT_PKG_STATE_HALF_INSTALLED = 5,
    APT_PKG_STATE_TRIGGERS_AWAITED = 6,
    APT_PKG_STATE_TRIGGERS_PENDING = 7
} AptPackageState;

// Package info structure
typedef struct {
    char* name;
    char* version;
    char* description;
    char* short_description;
    char* section;
    char* architecture;
    char* maintainer;
    char* homepage;
    char* priority;
    char* md5_hash;
    char* blake2b_hash;
    char* source_package;
    char* changelog;
    char* filename;
    char* depends;
    char* provides;
    char* conflicts;
    char* obsoletes;
    char* recommends;
    char* suggests;
    AptPackageState state;
    bool auto_installed;
    bool essential;
    uint64_t installed_size;
    uint64_t download_size;
    uint32_t package_id;
} AptPackageInfo;

// Structured result for operations: code + optional message (malloc'ed; caller must free)
typedef struct {
    AptErrorCode code;
    char* message;
} AptResult;

// System initialization
AptResult apt_init_config();
AptResult apt_init_system(AptSystem** system);
void apt_cleanup_system(AptSystem* system);

// Cache management
AptResult apt_cache_open(AptSystem* system, AptCache** cache, bool with_lock);
void apt_cache_close(AptCache* cache);
AptResult apt_cache_refresh(AptCache* cache);
AptResult apt_cache_update(AptCache* cache);
AptResult apt_cache_dist_upgrade(AptCache* cache);

// Package manager
AptResult apt_package_manager_create(AptCache* cache, AptPackageManager** pm);
void apt_package_manager_destroy(AptPackageManager* pm);

// Package operations
AptResult apt_mark_install(AptCache* cache, const char* package_name, bool auto_install);
AptResult apt_mark_remove(AptCache* cache, const char* package_name, bool purge);
AptResult apt_mark_keep(AptCache* cache, const char* package_name);
AptResult apt_mark_auto(AptCache* cache, const char* package_name, bool auto_flag);

// Package execution
AptResult apt_install_packages(AptPackageManager* pm,
                               AptProgressCallback callback,
                               void* user_data);

// Full dist-upgrade execution with progress callbacks (mark + download + install)
AptResult apt_dist_upgrade_with_progress(AptCache* cache,
                                         AptProgressCallback callback,
                                         void* user_data);

// Register a default/global progress callback that will be used if the
// per-call callback is NULL. Useful for language bindings.
void apt_register_progress_callback(AptProgressCallback callback, void* user_data);

// Register a log callback to receive error/info messages instead of writing to stderr
void apt_set_log_callback(AptLogCallback callback, void* user_data);

// Helpers to enable Go-exported callbacks without exposing them as C symbols in Go code
void apt_use_go_progress_callback(void* user_data);
void apt_enable_go_log_callback(void* user_data);

// Enable/disable capturing of std::cout/std::cerr into the registered log callback
void apt_capture_stdio(int enable);

// Package information (cleaned up - removed unsafe iterator-based functions)
AptResult apt_get_package_info(AptCache* cache, const char* package_name, AptPackageInfo* info);
void apt_free_package_info(AptPackageInfo* info);

// Package searching
typedef struct {
    AptPackageInfo* packages;
    size_t count;
} AptPackageList;

// Package changes structure for simulation
typedef struct {
    char** extra_installed;        // Additional packages that will be installed
    char** upgraded_packages;      // Packages that will be upgraded
    char** new_installed_packages; // New packages that will be installed
    char** removed_packages;       // Packages that will be removed

    size_t extra_installed_count;
    size_t upgraded_count;
    size_t new_installed_count;
    size_t removed_count;
    size_t not_upgraded_count;

    uint64_t download_size;        // Size in bytes to download
    uint64_t install_size;         // Size in bytes after installation
} AptPackageChanges;

AptResult apt_search_packages(AptCache* cache, const char* pattern, AptPackageList* result);
void apt_free_package_list(AptPackageList* list);

// Simulation functions (support multiple packages)
AptResult apt_simulate_install(AptCache* cache, const char** package_names, size_t count, AptPackageChanges* changes);
AptResult apt_simulate_remove(AptCache* cache, const char** package_names, size_t count, AptPackageChanges* changes);
AptResult apt_simulate_dist_upgrade(AptCache* cache, AptPackageChanges* changes);

void apt_free_package_changes(AptPackageChanges* changes);

// Utility functions
const char* apt_error_string(AptErrorCode error);
bool apt_has_broken_packages(AptCache* cache);
uint32_t apt_get_broken_count(AptCache* cache);

// Debug/testing functions
bool apt_test_findpkg(AptCache* cache, const char* package_name);

// Configuration
AptErrorCode apt_set_config(const char* key, const char* value);
const char* apt_get_config(const char* key, const char* default_value);

// Force cleanup functions
void apt_force_unlock();

#ifdef __cplusplus
}
#endif

#endif

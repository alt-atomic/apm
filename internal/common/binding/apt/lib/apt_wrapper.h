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

// Error handling - trimmed to used subset
typedef enum {
    // Success
    APT_SUCCESS = 0,

    // Initialization errors (1-10)
    APT_ERROR_INIT_FAILED = 1,

    // Cache errors (11-20)
    APT_ERROR_CACHE_OPEN_FAILED = 11,
    APT_ERROR_CACHE_REFRESH_FAILED = 12,
    APT_ERROR_CACHE_UPDATE_FAILED = 13,

    // Package errors (21-40)
    APT_ERROR_PACKAGE_NOT_FOUND = 21,
    APT_ERROR_PACKAGES_ALREADY_INSTALLED = 22,

    // Dependency errors (41-50)
    APT_ERROR_DEPENDENCY_BROKEN = 41,

    // Package Manager Operation errors (51-70)
    APT_ERROR_OPERATION_FAILED = 52,
    APT_ERROR_OPERATION_INCOMPLETE = 53,
    APT_ERROR_INSTALL_FAILED = 54,
    APT_ERROR_DOWNLOAD_FAILED = 57,

    // Lock errors (71-80)
    APT_ERROR_LOCK_FAILED = 71,

    // Validation errors (91-99)
    APT_ERROR_INVALID_PARAMETERS = 91,

    // Generic/Unknown errors (999)
    APT_ERROR_UNKNOWN = 999
} AptErrorCode;

// Callback types
typedef enum {
    APT_CALLBACK_UNKNOWN = 0,
    APT_CALLBACK_INST_START = 2,
    APT_CALLBACK_INST_STOP = 3,
    APT_CALLBACK_INST_PROGRESS = 1,
    APT_CALLBACK_DOWNLOAD_START = 20,
    APT_CALLBACK_DOWNLOAD_PROGRESS = 21,
    APT_CALLBACK_DOWNLOAD_STOP = 22,
    APT_CALLBACK_DOWNLOAD_COMPLETE = 23
} AptCallbackType;

typedef void (*AptProgressCallback)(const char *package_name,
                                    AptCallbackType callback_type,
                                    uint64_t current,
                                    uint64_t total,
                                    void *user_data);

// Optional log callback to route error/info messages instead of stderr
typedef void (*AptLogCallback)(const char *message, void *user_data);

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
    char *name;
    char *version;
    char *description;
    char *short_description;
    char *section;
    char *architecture;
    char *maintainer;
    char *homepage;
    char *priority;
    char *md5_hash;
    char *blake2b_hash;
    char *source_package;
    char *changelog;
    char *filename;
    char *depends;
    char *provides;
    char *conflicts;
    char *obsoletes;
    char *recommends;
    char *suggests;
    AptPackageState state;
    bool auto_installed;
    bool essential;
    uint64_t installed_size;
    uint64_t download_size;
    uint32_t package_id;
    // Aliases for this package (e.g., i586-<name>, i586-<name>.32bit)
    char **aliases;
    size_t alias_count;
} AptPackageInfo;

// Structured result for operations: code + optional message (malloc'ed; caller must free)
typedef struct {
    AptErrorCode code;
    char *message;
} AptResult;

// System initialization
AptResult apt_init_config();

AptResult apt_init_system(AptSystem **system);

void apt_cleanup_system(AptSystem *system);

// Cache management
AptResult apt_cache_open(AptSystem *system, AptCache **cache, bool with_lock);

void apt_cache_close(AptCache *cache);

AptResult apt_cache_refresh(AptCache *cache);

AptResult apt_cache_update(AptCache *cache);

AptResult apt_cache_dist_upgrade(AptCache *cache);

// File installation support - preprocess arguments to detect and handle RPM files
AptResult apt_preprocess_install_arguments(const char **install_names, size_t install_count);

// Package manager
AptResult apt_package_manager_create(AptCache *cache, AptPackageManager **pm);

void apt_package_manager_destroy(AptPackageManager *pm);

// Package operations
AptResult apt_mark_install(AptCache *cache, const char *package_name);

AptResult apt_mark_remove(AptCache *cache, const char *package_name, bool purge, bool remove_depends);

// Package execution
AptResult apt_install_packages(AptPackageManager *pm,
                               AptProgressCallback callback,
                               void *user_data);

// Full dist-upgrade execution with progress callbacks (mark + download + install)
AptResult apt_dist_upgrade_with_progress(AptCache *cache,
                                         AptProgressCallback callback,
                                         void *user_data);

// Register a default/global progress callback that will be used if the
// per-call callback is NULL. Useful for language bindings.
void apt_register_progress_callback(AptProgressCallback callback, void *user_data);

// Register a log callback to receive error/info messages instead of writing to stderr
void apt_set_log_callback(AptLogCallback callback, void *user_data);

// Helpers to enable Go-exported callbacks without exposing them as C symbols in Go code
void apt_use_go_progress_callback(void *user_data);

void apt_enable_go_log_callback(void *user_data);

// Enable/disable capturing of std::cout/std::cerr into the registered log callback
void apt_capture_stdio(int enable);

// Package information (cleaned up - removed unsafe iterator-based functions)
AptResult apt_get_package_info(AptCache *cache, const char *package_name, AptPackageInfo *info);

void apt_free_package_info(AptPackageInfo *info);

// Package searching
typedef struct {
    AptPackageInfo *packages;
    size_t count;
} AptPackageList;

// Package changes structure for simulation
typedef struct {
    char **extra_installed; // Additional packages that will be installed
    char **upgraded_packages; // Packages that will be upgraded
    char **new_installed_packages; // New packages that will be installed
    char **removed_packages; // Packages that will be removed

    size_t extra_installed_count;
    size_t upgraded_count;
    size_t new_installed_count;
    size_t removed_count;
    size_t not_upgraded_count;

    uint64_t download_size; // Size in bytes to download
    uint64_t install_size; // Size in bytes after installation
} AptPackageChanges;

AptResult apt_search_packages(AptCache *cache, const char *pattern, AptPackageList *result);

void apt_free_package_list(AptPackageList *list);

// Simulation functions (support multiple packages)
AptResult apt_simulate_install(AptCache *cache, const char **package_names, size_t count, AptPackageChanges *changes);

AptResult apt_simulate_remove(AptCache *cache, const char **package_names, size_t count, bool purge,
                              bool remove_depends, AptPackageChanges *changes);

AptResult apt_simulate_dist_upgrade(AptCache *cache, AptPackageChanges *changes);

AptResult apt_simulate_autoremove(AptCache *cache, AptPackageChanges *changes);

// Reinstall simulation
AptResult apt_simulate_reinstall(AptCache *cache, const char **package_names, size_t count, AptPackageChanges *changes);

// Apply reinstall changes to cache
AptResult apt_apply_reinstall(AptCache *cache, const char **package_names, size_t count);

// Combined simulation: install and remove in a single transaction
AptResult apt_simulate_change(AptCache *cache,
                              const char **install_names, size_t install_count,
                              const char **remove_names, size_t remove_count,
                              bool purge,
                              bool remove_depends,
                              AptPackageChanges *changes);

// Apply changes to cache (like simulate but with apply=true)
// Use this before creating PackageManager to ensure simulation matches reality
AptResult apt_apply_changes(AptCache *cache,
                            const char **install_names, size_t install_count,
                            const char **remove_names, size_t remove_count,
                            bool purge,
                            bool remove_depends);

void apt_free_package_changes(AptPackageChanges *changes);

// Utility functions
const char *apt_error_string(AptErrorCode error);

bool apt_has_broken_packages(AptCache *cache);

uint32_t apt_get_broken_count(AptCache *cache);

// Debug/testing functions
bool apt_test_findpkg(AptCache *cache, const char *package_name);

// Configuration
AptErrorCode apt_set_config(const char *key, const char *value);

const char *apt_get_config(const char *key, const char *default_value);

// Force cleanup functions
void apt_force_unlock();

// Lock status information
typedef struct {
    bool is_locked;
    bool can_acquire;
    int lock_pid;
    char *lock_holder;
    char *lock_file_path;
    char *error_message;
} AptLockStatus;

// Check if APT locks
AptLockStatus apt_check_lock_status();

// Free lock status structure
void apt_free_lock_status(AptLockStatus *status);

#ifdef __cplusplus
}
#endif

#endif

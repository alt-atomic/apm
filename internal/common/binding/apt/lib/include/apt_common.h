#ifndef APT_COMMON_H
#define APT_COMMON_H
#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stddef.h>
#include <stdbool.h>

#include "apt_error.h"

// Progress event types for download and install phases.
typedef enum {
    APT_CALLBACK_UNKNOWN = 0,
    APT_CALLBACK_INST_START = 2,
    APT_CALLBACK_INST_STOP = 3,
    APT_CALLBACK_INST_PROGRESS = 1,
    APT_CALLBACK_DOWNLOAD_START = 20,
    APT_CALLBACK_DOWNLOAD_PROGRESS = 21,
    APT_CALLBACK_DOWNLOAD_STOP = 22,
    APT_CALLBACK_DOWNLOAD_COMPLETE = 23,
    APT_CALLBACK_DOWNLOAD_ITEM_PROGRESS = 24
} AptCallbackType;

// Reports progress during package download/install.
typedef void (*AptProgressCallback)(const char *package_name,
                                    AptCallbackType callback_type,
                                    uint64_t current,
                                    uint64_t total,
                                    uint64_t speed_bps,
                                    uintptr_t user_data);

// Receives log messages from APT operations.
typedef void (*AptLogCallback)(const char *message, uintptr_t user_data);

// An essential package that would be removed by a transaction.
typedef struct {
    char *name;
    char *reason;
} AptEssentialPackage;

// Installation state of a package in the dpkg/rpm database.
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

// Describes all changes a planned transaction would perform.
// Free with apt_free_package_changes().
typedef struct {
    char **extra_installed;
    char **upgraded_packages;
    char **new_installed_packages;
    char **removed_packages;
    char **kept_back_packages;

    size_t extra_installed_count;
    size_t upgraded_count;
    size_t new_installed_count;
    size_t removed_count;
    size_t kept_back_count;
    size_t not_upgraded_count;

    uint64_t download_size;
    int64_t install_size;

    AptEssentialPackage *essential_packages;
    size_t essential_packages_count;
} AptPackageChanges;

// Frees all heap-allocated fields within `changes`.
void apt_free_package_changes(AptPackageChanges *changes);

// Returns name and reason for the essential package at `index`.
inline void apt_get_essential_package(const AptPackageChanges *changes, const size_t index,
                                      const char **out_name, const char **out_reason) {
    *out_name = changes->essential_packages[index].name;
    *out_reason = changes->essential_packages[index].reason;
}

// Opaque handles to internal APT state.
typedef struct AptSystem AptSystem;
typedef struct AptCache AptCache;
typedef struct AptTransaction AptTransaction;
typedef struct AptPackageManager AptPackageManager;

#ifdef __cplusplus
}
#endif

#endif

#include "apt_internal.h"

// Include APT library headers
#include <apt-pkg/init.h>
#include <apt-pkg/cachefile.h>
#include <apt-pkg/packagemanager.h>
#include <apt-pkg/depcache.h>
#include <apt-pkg/pkgcache.h>
#include <apt-pkg/configuration.h>
#include <apt-pkg/error.h>
#include <apt-pkg/pkgsystem.h>
#include <apt-pkg/acquire.h>
#include <apt-pkg/sourcelist.h>
#include <apt-pkg/pkgrecords.h>
#include <apt-pkg/algorithms.h>
#include <apt-pkg/fileutl.h>

#include <memory>
#include <string>
#include <vector>
#include <cstring>
#include <iostream>
#include <sstream>
#include <algorithm>
#include <cctype>
#include <regex.h>
#include <cstdlib>
#include <set>
#include <unistd.h>
#include <cstring>
#include <vector>

// Error handling
AptErrorCode last_error = APT_SUCCESS;
std::string last_error_message;

// Optional logging callback (bridge to Go/Rust callers)
AptLogCallback g_log_callback = nullptr;
void* g_log_user_data = nullptr;

void emit_log(const std::string& msg) {
    if (g_log_callback) {
        g_log_callback(msg.c_str(), g_log_user_data);
    } else {
        std::cerr << msg << std::endl;
    }
}

void set_error(AptErrorCode code, const std::string& message) {
    last_error = code;
    last_error_message = message;
    if (!message.empty()) {
        emit_log(std::string("APT Error: ") + message);
    }
}

bool check_apt_errors() {
    if (_error->PendingError()) {
        std::string error_msg;
        std::string all_errors;

        while (_error->PopMessage(error_msg)) {
            if (!all_errors.empty()) {
                all_errors += "; ";
            }
            all_errors += error_msg;
        }

        AptErrorCode error_code = APT_ERROR_UNKNOWN;
        if (all_errors.find("lock") != std::string::npos &&
            (all_errors.find("Resource temporarily unavailable") != std::string::npos ||
             all_errors.find("another process using it") != std::string::npos ||
             all_errors.find("EAGAIN") != std::string::npos ||
             all_errors.find("EACCES") != std::string::npos)) {
            error_code = APT_ERROR_LOCK_FAILED;
        }

        set_error(error_code, all_errors);
        return false;
    }
    return true;
}

// System initialization
AptErrorCode apt_init_config() {
    try {
        if (!pkgInitConfig(*_config)) {
            set_error(APT_ERROR_INIT_FAILED, "Failed to initialize APT configuration");
            return APT_ERROR_INIT_FAILED;
        }
        return check_apt_errors() ? APT_SUCCESS : last_error;
    } catch (const std::exception& e) {
        set_error(APT_ERROR_INIT_FAILED, std::string("Exception: ") + e.what());
        return APT_ERROR_INIT_FAILED;
    }
}

AptErrorCode apt_init_system(AptSystem** system) {
    if (!system) return APT_ERROR_INIT_FAILED;

    try {
        if (!pkgInitSystem(*_config, _system)) {
            set_error(APT_ERROR_INIT_FAILED, "Failed to initialize APT system");
            return APT_ERROR_INIT_FAILED;
        }

        // Create wrapper that points to global system
        *system = new AptSystem();
        (*system)->system = _system;

        return check_apt_errors() ? APT_SUCCESS : last_error;
    } catch (const std::exception& e) {
        if (*system) {
            delete *system;
            *system = nullptr;
        }
        set_error(APT_ERROR_INIT_FAILED, std::string("Exception: ") + e.what());
        return APT_ERROR_INIT_FAILED;
    }
}

void apt_cleanup_system(AptSystem* system) {
    if (system) {
        if (system->system) {
            system->system->UnLock(true);
        }
        delete system;
    }
}

// Cache management
AptErrorCode apt_cache_open(AptSystem* system, AptCache** cache, bool with_lock) {
    if (!system || !cache) return APT_ERROR_INIT_FAILED;

    if (!system->system) {
        set_error(APT_ERROR_INIT_FAILED, "System not properly initialized");
        return APT_ERROR_INIT_FAILED;
    }

    // Check if system can be locked before attempting to open cache
    if (with_lock) {
        if (!system->system->Lock()) {
            if (_error->PendingError()) {
                std::string error_msg;
                std::string all_errors;
                while (_error->PopMessage(error_msg)) {
                    if (!all_errors.empty()) {
                        all_errors += "; ";
                    }
                    all_errors += error_msg;
                }
                set_error(APT_ERROR_LOCK_FAILED, all_errors);
            } else {
                set_error(APT_ERROR_LOCK_FAILED, "Unable to acquire APT system lock - another process may be using APT");
            }
            return APT_ERROR_LOCK_FAILED;
        }
        system->system->UnLock(true);
    }

    try {
        *cache = new AptCache();

        static std::ostringstream nullstream;
        (*cache)->cache_file = std::make_unique<CacheFile>(nullstream, with_lock);

        if (!(*cache)->cache_file->Open()) {
            delete *cache;
            *cache = nullptr;
            set_error(APT_ERROR_CACHE_OPEN_FAILED, "Failed to open APT cache");
            return APT_ERROR_CACHE_OPEN_FAILED;
        }

        if (!(*cache)->cache_file->CheckDeps()) {
            delete *cache;
            *cache = nullptr;
            set_error(APT_ERROR_CACHE_OPEN_FAILED, "Failed to check dependencies");
            return APT_ERROR_CACHE_OPEN_FAILED;
        }

        (*cache)->dep_cache = (*cache)->cache_file->operator->();
        if (!(*cache)->dep_cache) {
            delete *cache;
            *cache = nullptr;
            set_error(APT_ERROR_CACHE_OPEN_FAILED, "Failed to get dependency cache");
            return APT_ERROR_CACHE_OPEN_FAILED;
        }

        return check_apt_errors() ? APT_SUCCESS : last_error;
    } catch (const std::exception& e) {
        if (*cache) {
            delete *cache;
            *cache = nullptr;
        }
        set_error(APT_ERROR_CACHE_OPEN_FAILED, std::string("Exception: ") + e.what());
        return APT_ERROR_CACHE_OPEN_FAILED;
    }
}

void apt_cache_close(AptCache* cache) {
    if (cache) {
        if (cache->cache_file) {
            cache->cache_file.reset();
        }
        delete cache;
    }
}

AptErrorCode apt_cache_refresh(AptCache* cache) {
    if (!cache) {
        return APT_ERROR_CACHE_OPEN_FAILED;
    }

    try {
        cache->cache_file.reset();

        static std::ostringstream nullstream;
        cache->cache_file = std::make_unique<CacheFile>(nullstream, true);

        if (!cache->cache_file->Open()) {
            set_error(APT_ERROR_CACHE_REFRESH_FAILED, "Failed to reopen cache after refresh");
            return APT_ERROR_CACHE_REFRESH_FAILED;
        }

        if (!cache->cache_file->CheckDeps()) {
            set_error(APT_ERROR_DEPENDENCY_BROKEN, "Failed to check dependencies after refresh");
            return APT_ERROR_DEPENDENCY_BROKEN;
        }

        cache->dep_cache = cache->cache_file->operator->();
        if (!cache->dep_cache) {
            set_error(APT_ERROR_CACHE_REFRESH_FAILED, "Failed to get dependency cache after refresh");
            return APT_ERROR_CACHE_REFRESH_FAILED;
        }

        return check_apt_errors() ? APT_SUCCESS : last_error;
    } catch (const std::exception& e) {
        set_error(APT_ERROR_CACHE_REFRESH_FAILED, std::string("Exception during cache refresh: ") + e.what());
        return APT_ERROR_CACHE_REFRESH_FAILED;
    }
}

AptErrorCode apt_cache_update(AptCache* cache) {
    if (!cache || !cache->cache_file) return APT_ERROR_CACHE_OPEN_FAILED;

    try {
        ProgressStatus status;
        pkgAcquire acquire(&status);
        pkgSourceList source_list;

        if (!source_list.ReadMainList()) {
            set_error(APT_ERROR_CACHE_UPDATE_FAILED, "Failed to read sources.list");
            return APT_ERROR_CACHE_UPDATE_FAILED;
        }

        if (!source_list.GetIndexes(&acquire)) {
            set_error(APT_ERROR_CACHE_UPDATE_FAILED, "Failed to get package indexes");
            return APT_ERROR_CACHE_UPDATE_FAILED;
        }

        auto fetch_result = acquire.Run();
        if (fetch_result != pkgAcquire::Continue) {
            set_error(APT_ERROR_DOWNLOAD_FAILED, "Failed to download package lists");
            return APT_ERROR_DOWNLOAD_FAILED;
        }

        if (!cache->cache_file->BuildCaches()) {
            set_error(APT_ERROR_CACHE_UPDATE_FAILED, "Failed to rebuild caches");
            return APT_ERROR_CACHE_UPDATE_FAILED;
        }

        return check_apt_errors() ? APT_SUCCESS : last_error;
    } catch (const std::exception& e) {
        set_error(APT_ERROR_UNKNOWN, std::string("Exception: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

// Package manager
AptErrorCode apt_package_manager_create(AptCache* cache, AptPackageManager** pm) {
    if (!cache || !cache->dep_cache || !pm) return APT_ERROR_CACHE_OPEN_FAILED;

    try {
        *pm = new AptPackageManager(cache);

        (*pm)->pm.reset(_system->CreatePM(cache->dep_cache));

        if (!(*pm)->pm) {
            delete *pm;
            *pm = nullptr;
            set_error(APT_ERROR_INIT_FAILED, "Failed to create package manager");
            return APT_ERROR_INIT_FAILED;
        }

        return APT_SUCCESS;
    } catch (const std::exception& e) {
        if (*pm) {
            delete *pm;
            *pm = nullptr;
        }
        set_error(APT_ERROR_INIT_FAILED, std::string("Exception: ") + e.what());
        return APT_ERROR_INIT_FAILED;
    }
}

void apt_package_manager_destroy(AptPackageManager* pm) {
    delete pm;
}

AptErrorCode apt_mark_install(AptCache* cache, const char* package_name, bool auto_install) {
    if (!cache || !cache->dep_cache || !package_name) {
        return APT_ERROR_CACHE_OPEN_FAILED;
    }

    try {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(package_name);
        if (pkg.end()) {
            std::vector<pkgCache::PkgIterator> candidate_providers;
            for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); it.end() == false; ++it) {
                pkgCache::VerIterator cand = (*cache->dep_cache)[it].CandidateVerIter(*cache->dep_cache);
                if (cand.end()) continue;
                for (pkgCache::PrvIterator prv = cand.ProvidesList(); prv.end() == false; ++prv) {
                    if (strcmp(prv.Name(), package_name) == 0) {
                        candidate_providers.push_back(it);
                        break;
                    }
                }
            }
            if (candidate_providers.empty()) {
                set_error(APT_ERROR_PACKAGE_NOT_FOUND, std::string("Package not found: ") + package_name);
                return APT_ERROR_PACKAGE_NOT_FOUND;
            }
            if (candidate_providers.size() > 1) {
                set_error(APT_ERROR_DEPENDENCY_BROKEN, std::string("Virtual name '") + package_name + "' has multiple providers; specify exact package name");
                return APT_ERROR_DEPENDENCY_BROKEN;
            }
            pkg = candidate_providers.front();
        }

        pkgProblemResolver Fix(cache->dep_cache);

        if ((*cache->dep_cache)[pkg].CandidateVer == 0 && pkg->ProvidesList != 0) {
            std::vector<pkgCache::Package *> GoodSolutions;

            for (pkgCache::PrvIterator Prv = pkg.ProvidesList(); !Prv.end(); ++Prv) {
                pkgCache::PkgIterator ProvPkg = Prv.OwnerPkg();

                if (ProvPkg.CurrentVer() == Prv.OwnerVer()) {
                    GoodSolutions.push_back(ProvPkg);
                    continue;
                }

                pkgCache::VerIterator CandVer = (*cache->dep_cache)[ProvPkg].CandidateVerIter(*cache->dep_cache);
                if (!CandVer.end() && CandVer == Prv.OwnerVer()) {
                    GoodSolutions.push_back(ProvPkg);
                }
            }

            if (GoodSolutions.empty()) {
                set_error(APT_ERROR_PACKAGE_NOT_FOUND,
                    std::string("Virtual package ") + package_name + " has no installable providers");
                return APT_ERROR_PACKAGE_NOT_FOUND;
            }

            if (GoodSolutions.size() > 1) {
                set_error(APT_ERROR_DEPENDENCY_BROKEN,
                    std::string("Virtual package ") + package_name + " has multiple providers. Please select specific package.");
                return APT_ERROR_DEPENDENCY_BROKEN;
            }

            pkg = pkgCache::PkgIterator(*cache->dep_cache, GoodSolutions[0]);
        }

        // Check if already installed
        pkgDepCache::StateCache &State = (*cache->dep_cache)[pkg];
        if (pkg->CurrentVer != 0 && State.Install() == false) {
            cache->dep_cache->MarkKeep(pkg);
            return APT_SUCCESS;
        }

        cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, false);

        if (State.InstBroken() == true && auto_install == true) {
            cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::DontChange, true);
        }

        if (cache->dep_cache->BrokenCount() > 0) {
            if (!Fix.Resolve(true)) {
                set_error(APT_ERROR_DEPENDENCY_BROKEN,
                    std::string("Unable to resolve dependencies for package: ") + package_name);
                return APT_ERROR_DEPENDENCY_BROKEN;
            }
        }

        return check_apt_errors() ? APT_SUCCESS : last_error;
    } catch (const std::exception& e) {
        set_error(APT_ERROR_UNKNOWN, std::string("Exception: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

AptErrorCode apt_mark_remove(AptCache* cache, const char* package_name, bool purge) {
    if (!cache || !cache->dep_cache || !package_name) {
        return APT_ERROR_CACHE_OPEN_FAILED;
    }

    try {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(package_name);
        if (pkg.end()) {
            set_error(APT_ERROR_PACKAGE_NOT_FOUND, std::string("Package not found: ") + package_name);
            return APT_ERROR_PACKAGE_NOT_FOUND;
        }

        if ((*cache->dep_cache)[pkg].CandidateVer == 0 && pkg->ProvidesList != 0) {
            std::vector<pkgCache::Package *> InstalledProviders;
            std::string providersList;

            for (pkgCache::PrvIterator Prv = pkg.ProvidesList(); !Prv.end(); ++Prv) {
                pkgCache::PkgIterator ProvPkg = Prv.OwnerPkg();
                if (ProvPkg.CurrentVer() == Prv.OwnerVer()) {
                    InstalledProviders.push_back(ProvPkg);
                    if (!providersList.empty()) providersList += ", ";
                    providersList += ProvPkg.Name();
                }
            }

            if (InstalledProviders.empty()) {
                set_error(APT_ERROR_PACKAGE_NOT_FOUND,
                    std::string("Virtual package ") + package_name + " has no installed providers");
                return APT_ERROR_PACKAGE_NOT_FOUND;
            }

            if (InstalledProviders.size() > 1) {
                set_error(APT_ERROR_DEPENDENCY_BROKEN,
                    std::string("Virtual package ") + package_name +
                    " has multiple installed providers: " + providersList +
                    ". Please remove specific package.");
                return APT_ERROR_DEPENDENCY_BROKEN;
            }

            pkg = pkgCache::PkgIterator(*cache->dep_cache, InstalledProviders[0]);
        }

        if (pkg->CurrentVer == 0) {
            set_error(APT_ERROR_PACKAGE_NOT_FOUND,
                std::string("Package ") + package_name + " is not installed");
            return APT_ERROR_PACKAGE_NOT_FOUND;
        }

        cache->dep_cache->MarkDelete(pkg, purge);

        pkgProblemResolver Fix(cache->dep_cache);

        Fix.Remove(pkg);

        if (!Fix.Resolve()) {
            set_error(APT_ERROR_DEPENDENCY_BROKEN, "Problem resolver failed to handle package removal dependencies");
            return APT_ERROR_DEPENDENCY_BROKEN;
        }

        return check_apt_errors() ? APT_SUCCESS : last_error;
    } catch (const std::exception& e) {
        set_error(APT_ERROR_UNKNOWN, std::string("Exception: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

AptErrorCode apt_mark_keep(AptCache* cache, const char* package_name) {
    if (!cache || !cache->dep_cache || !package_name) {
        return APT_ERROR_CACHE_OPEN_FAILED;
    }

    try {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(package_name);
        if (pkg.end()) {
            set_error(APT_ERROR_PACKAGE_NOT_FOUND, std::string("Package not found: ") + package_name);
            return APT_ERROR_PACKAGE_NOT_FOUND;
        }

        cache->dep_cache->MarkKeep(pkg);

        return check_apt_errors() ? APT_SUCCESS : last_error;
    } catch (const std::exception& e) {
        set_error(APT_ERROR_UNKNOWN, std::string("Exception: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

AptErrorCode apt_mark_auto(AptCache* cache, const char* package_name, bool auto_flag) {
    if (!cache || !cache->dep_cache || !package_name) {
        return APT_ERROR_CACHE_OPEN_FAILED;
    }

    try {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(package_name);
        if (pkg.end()) {
            set_error(APT_ERROR_PACKAGE_NOT_FOUND, std::string("Package not found: ") + package_name);
            return APT_ERROR_PACKAGE_NOT_FOUND;
        }

        cache->dep_cache->MarkAuto(pkg, auto_flag ? pkgDepCache::AutoMarkFlag::Auto : pkgDepCache::AutoMarkFlag::Manual);

        return check_apt_errors() ? APT_SUCCESS : last_error;
    } catch (const std::exception& e) {
        set_error(APT_ERROR_UNKNOWN, std::string("Exception: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

// Progress callback implementation
AptProgressCallback global_callback = nullptr;
void* global_user_data = nullptr;

// Simple progress callback that handles package manager operations
class SimpleProgressCallback {
public:
    static void InstallProgress(const char* package, int current, int total) {
        if (global_callback && package) {
            global_callback(package, APT_CALLBACK_INST_PROGRESS, current, total, global_user_data);
        }
    }

    static void InstallStart(const char* package) {
        if (global_callback && package) {
            global_callback(package, APT_CALLBACK_INST_START, 0, 0, global_user_data);
        }
    }

    static void InstallStop(const char* package) {
        if (global_callback && package) {
            global_callback(package, APT_CALLBACK_INST_STOP, 0, 0, global_user_data);
        }
    }
};

// ProgressStatus implementations (after global_callback is defined)
bool ProgressStatus::Pulse(pkgAcquire *Owner) {
    if (global_callback != nullptr && has_active_item_ && TotalBytes > 0 && CurrentBytes > 0) {
        global_callback(active_name_.c_str(), APT_CALLBACK_DOWNLOAD_PROGRESS,
                        static_cast<uint64_t>(CurrentBytes),
                        static_cast<uint64_t>(TotalBytes),
                        global_user_data);
    }
    return pkgAcquireStatus::Pulse(Owner);
}

void ProgressStatus::Fetch(pkgAcquire::ItemDesc &Itm) {
    const char *name = Itm.ShortDesc.empty() ? Itm.URI.c_str() : Itm.ShortDesc.c_str();
    active_name_.assign(name ? name : "");
    has_active_item_ = true;
    if (global_callback != nullptr) {
        global_callback(active_name_.c_str(), APT_CALLBACK_DOWNLOAD_START, 0, 0, global_user_data);
    }
    pkgAcquireStatus::Fetch(Itm);
}

void ProgressStatus::Done(pkgAcquire::ItemDesc &Itm) {
    if (global_callback != nullptr) {
        global_callback(active_name_.c_str(), APT_CALLBACK_DOWNLOAD_STOP, 0, 0, global_user_data);
    }
    has_active_item_ = false;
    active_name_.clear();
    pkgAcquireStatus::Done(Itm);
}

void ProgressStatus::Fail(pkgAcquire::ItemDesc &Itm) {
    if (global_callback != nullptr) {
        global_callback(active_name_.c_str(), APT_CALLBACK_ERROR, 0, 0, global_user_data);
    }
    has_active_item_ = false;
    active_name_.clear();
    pkgAcquireStatus::Fail(Itm);
}


void apt_free_package_info(AptPackageInfo* info) {
    if (!info) return;

    free(info->name);
    free(info->version);
    free(info->description);
    free(info->short_description);
    free(info->section);
    free(info->architecture);
    free(info->maintainer);
    free(info->homepage);
    free(info->priority);
    free(info->md5_hash);
    free(info->blake2b_hash);
    free(info->source_package);
    free(info->changelog);
    free(info->filename);
    free(info->depends);
    free(info->provides);
    free(info->conflicts);
    free(info->obsoletes);
    free(info->recommends);
    free(info->suggests);

    memset(info, 0, sizeof(AptPackageInfo));
}

void apt_free_package_list(AptPackageList* list) {
    if (!list || !list->packages) return;

    for (size_t i = 0; i < list->count; ++i) {
        apt_free_package_info(&list->packages[i]);
    }

    free(list->packages);
    list->packages = nullptr;
    list->count = 0;
}

// Utility functions
const char* apt_error_string(AptErrorCode error) {
    switch (error) {
        case APT_SUCCESS: return "Success";

        case APT_ERROR_INIT_FAILED: return "Initialization failed";
        case APT_ERROR_CONFIG_FAILED: return "Configuration failed";
        case APT_ERROR_SYSTEM_INIT_FAILED: return "System initialization failed";

        case APT_ERROR_CACHE_OPEN_FAILED: return "Cache open failed";
        case APT_ERROR_CACHE_REFRESH_FAILED: return "Cache refresh failed";
        case APT_ERROR_CACHE_UPDATE_FAILED: return "Cache update failed";
        case APT_ERROR_CACHE_CORRUPTED: return "Cache is corrupted";

        case APT_ERROR_PACKAGE_NOT_FOUND: return "Package not found";
        case APT_ERROR_PACKAGE_NOT_INSTALLED: return "Package is not installed";
        case APT_ERROR_PACKAGE_ALREADY_INSTALLED: return "Package is already installed";
        case APT_ERROR_PACKAGE_VIRTUAL_MULTIPLE_PROVIDERS: return "Virtual package has multiple providers";
        case APT_ERROR_PACKAGE_VIRTUAL_NO_PROVIDERS: return "Virtual package has no providers";
        case APT_ERROR_PACKAGE_ESSENTIAL: return "Cannot modify essential package";
        case APT_ERROR_PACKAGE_INFO_UNAVAILABLE: return "Package information unavailable";

        case APT_ERROR_DEPENDENCY_BROKEN: return "Broken dependencies";
        case APT_ERROR_DEPENDENCY_UNRESOLVABLE: return "Unresolvable dependencies";
        case APT_ERROR_DEPENDENCY_CONFLICTS: return "Dependency conflicts";
        case APT_ERROR_UNMET_DEPENDENCIES: return "Unmet dependencies";

        case APT_ERROR_OPERATION_COMPLETED: return "Operation completed successfully";
        case APT_ERROR_OPERATION_FAILED: return "Package manager operation failed";
        case APT_ERROR_OPERATION_INCOMPLETE: return "Package manager operation incomplete";
        case APT_ERROR_INSTALL_FAILED: return "Installation failed";
        case APT_ERROR_REMOVE_FAILED: return "Removal failed";
        case APT_ERROR_UPGRADE_FAILED: return "Upgrade failed";
        case APT_ERROR_DOWNLOAD_FAILED: return "Download failed";
        case APT_ERROR_ARCHIVE_FAILED: return "Archive operation failed";
        case APT_ERROR_SUBPROCESS_ERROR: return "Subprocess error";

        case APT_ERROR_LOCK_FAILED: return "Unable to acquire lock - another APT process is running";
        case APT_ERROR_PERMISSION_DENIED: return "Permission denied - root privileges required";
        case APT_ERROR_LOCK_TIMEOUT: return "Lock acquisition timeout";

        case APT_ERROR_OUT_OF_MEMORY: return "Out of memory";
        case APT_ERROR_DISK_SPACE: return "Insufficient disk space";
        case APT_ERROR_NETWORK: return "Network error";
        case APT_ERROR_IO_ERROR: return "File I/O error";
        case APT_ERROR_PIPE_FAILED: return "IPC pipe creation failed";

        case APT_ERROR_INVALID_PARAMETERS: return "Invalid parameters";
        case APT_ERROR_INVALID_PACKAGE_NAME: return "Invalid package name";
        case APT_ERROR_INVALID_REGEX: return "Invalid regular expression";

        case APT_ERROR_UNKNOWN:
        default: return "Unknown error";
    }
}

bool apt_has_broken_packages(AptCache* cache) {
    if (!cache || !cache->dep_cache) return false;
    return cache->dep_cache->BrokenCount() > 0;
}

uint32_t apt_get_broken_count(AptCache* cache) {
    if (!cache || !cache->dep_cache) return 0;
    return cache->dep_cache->BrokenCount();
}

// Debug function to test FindPkg logic like in original apt-get
bool apt_test_findpkg(AptCache* cache, const char* package_name) {
    if (!cache || !cache->dep_cache || !package_name) return false;

    try {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(package_name);

        emit_log(std::string("=== FindPkg Test for '") + package_name + "' ===");
        emit_log(std::string("pkg.end(): ") + (pkg.end() ? "true" : "false"));

        if (!pkg.end()) {
            emit_log("Package found in cache!");
            emit_log(std::string("Name: ") + pkg.Name());
            emit_log(std::string("ID: ") + std::to_string(pkg->ID));

            pkgDepCache::StateCache& state = (*cache->dep_cache)[pkg];
            pkgCache::VerIterator candidate = state.CandidateVerIter(*cache->dep_cache);
            emit_log(std::string("CandidateVer.end(): ") + (candidate.end() ? "true" : "false"));

            emit_log(std::string("ProvidesList: ") + (pkg->ProvidesList == 0 ? "empty" : "has provides"));

            if (pkg->ProvidesList != 0) {
                emit_log("This is a virtual package! Providers:");
                for (pkgCache::PrvIterator prv = pkg.ProvidesList(); !prv.end(); ++prv) {
                    pkgCache::PkgIterator provider = prv.OwnerPkg();
                    std::string line = std::string("  - ") + provider.Name();
                    if (!provider.CurrentVer().end()) {
                        line += std::string(" (INSTALLED: ") + provider.CurrentVer().VerStr() + ")";
                    }
                    pkgCache::VerIterator candVer = (*cache->dep_cache)[provider].CandidateVerIter(*cache->dep_cache);
                    if (!candVer.end()) {
                        line += std::string(" (CANDIDATE: ") + candVer.VerStr() + ")";
                    }
                    emit_log(line);
                }
            }

            return true;
        } else {
            emit_log("Package NOT found in cache.");
            return false;
        }

    } catch (const std::exception& e) {
        emit_log(std::string("Exception: ") + e.what());
        return false;
    }
}

extern "C" void apt_set_log_callback(AptLogCallback callback, void* user_data) {
    g_log_callback = callback;
    g_log_user_data = user_data;
}

// Global/default progress callback registration
extern "C" void apt_register_progress_callback(AptProgressCallback callback, void* user_data) {
    global_callback = callback;
    global_user_data = user_data;
}

extern "C" void goAptProgressCallback(const char *package_name,
                                       int callback_type,
                                       uint64_t current,
                                       uint64_t total,
                                       void *user_data);

extern "C" void goAptLogCallback(const char *message, void *user_data);


extern "C" void apt_use_go_progress_callback(void* user_data) {
    global_callback = (AptProgressCallback)goAptProgressCallback;
    global_user_data = user_data;
}

extern "C" void apt_enable_go_log_callback(void* user_data) {
    g_log_callback = (AptLogCallback)goAptLogCallback;
    g_log_user_data = user_data;
}

// Configuration
AptErrorCode apt_set_config(const char* key, const char* value) {
    if (!key || !value) return APT_ERROR_INIT_FAILED;

    try {
        _config->Set(key, value);
        return APT_SUCCESS;
    } catch (const std::exception& e) {
        set_error(APT_ERROR_UNKNOWN, std::string("Exception: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

const char* apt_get_config(const char* key, const char* default_value) {
    if (!key) return default_value;

    try {
        static std::string result;
        result = _config->Find(key, default_value ? default_value : "");
        return result.c_str();
    } catch (const std::exception& e) {
        return default_value;
    }
}

// Force unlock function to clean up hanging locks
void apt_force_unlock() {
    try {
        pkgSystem* system = _system;
        if (system) {
            system->UnLock(true);
        }
    } catch (...) {
        // Ignore any exceptions during force unlock
    }
}

void apt_free_package_changes(AptPackageChanges* changes) {
    if (!changes) return;

    if (changes->extra_installed) {
        for (size_t i = 0; i < changes->extra_installed_count; i++) {
            free(changes->extra_installed[i]);
        }
        free(changes->extra_installed);
    }

    if (changes->upgraded_packages) {
        for (size_t i = 0; i < changes->upgraded_count; i++) {
            free(changes->upgraded_packages[i]);
        }
        free(changes->upgraded_packages);
    }

    if (changes->new_installed_packages) {
        for (size_t i = 0; i < changes->new_installed_count; i++) {
            free(changes->new_installed_packages[i]);
        }
        free(changes->new_installed_packages);
    }

    if (changes->removed_packages) {
        for (size_t i = 0; i < changes->removed_count; i++) {
            free(changes->removed_packages[i]);
        }
        free(changes->removed_packages);
    }

    memset(changes, 0, sizeof(AptPackageChanges));
}

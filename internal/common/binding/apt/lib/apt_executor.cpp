#include "executor.h"

#include <apt-pkg/error.h>
#include <apt-pkg/pkgrecords.h>
#include <apt-pkg/sourcelist.h>

// RAII guard that sets APT::Get::ReInstall if any package has the ReInstall flag.
class ReInstallConfigGuard {
    bool was_already_set_ = false;
    bool was_set_by_us_ = false;

public:
    ReInstallConfigGuard(const ReInstallConfigGuard &) = delete;
    ReInstallConfigGuard &operator=(const ReInstallConfigGuard &) = delete;
    ReInstallConfigGuard() = default;

    bool setIfNeeded(pkgDepCache *cache) {
        was_already_set_ = _config->FindB("APT::Get::ReInstall", false);

        if (was_already_set_) {
            return true;
        }

        for (pkgCache::PkgIterator it = cache->PkgBegin(); !it.end(); ++it) {
            if (const pkgDepCache::StateCache &st = (*cache)[it]; (st.iFlags & pkgDepCache::ReInstall) != 0) {
                _config->Set("APT::Get::ReInstall", true);
                was_set_by_us_ = true;
                return true;
            }
        }

        return false;
    }

    ~ReInstallConfigGuard() {
        if (was_set_by_us_ && !was_already_set_) {
            _config->Set("APT::Get::ReInstall", false);
        }
    }
};

// Downloads packages and runs the package manager install phase.
AptResult execute_transaction(AptCache *cache,
                              AptPackageManager *pm,
                              AptProgressCallback callback,
                              uintptr_t user_data,
                              bool download_only,
                              bool include_reinstall) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_INVALID_CACHE);
    }

    try {
        CallbackGuard cbGuard;
        ReInstallConfigGuard reinstallGuard;
        const bool hasReinstall = include_reinstall ? reinstallGuard.setIfNeeded(cache->dep_cache) : false;

        if (cache->dep_cache->BrokenCount() != 0) {
            for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
                if (pkgDepCache::StateCache &st = (*cache->dep_cache)[it]; st.InstBroken() || st.NowBroken()) {
                    std::string out = std::string(
                                          "Some broken packages were found while trying to process build-dependencies for ")
                                      + it.Name();
                    return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
                }
            }
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, APT_MSG_BROKEN_DEPS);
        }

        if (cache->dep_cache->DelCount() == 0 &&
            cache->dep_cache->InstCount() == 0 &&
            cache->dep_cache->BadCount() == 0 &&
            !hasReinstall) {
            return make_result(APT_SUCCESS, nullptr);
        }

        std::unique_ptr<pkgPackageManager> owned_pm;
        pkgPackageManager *active_pm = nullptr;

        if (pm && pm->pm) {
            active_pm = pm->pm.get();
        } else {
            owned_pm.reset(_system->CreatePM(cache->dep_cache));
            if (!owned_pm) {
                return make_result(APT_ERROR_INIT_FAILED, APT_MSG_INIT_PM_FAILED);
            }
            active_pm = owned_pm.get();
        }

        if (callback) {
            global_callback = callback;
            global_user_data = user_data;
        }

        FileFd Lock;
        if (!_config->FindB("Debug::NoLocking", false)) {
            Lock.Fd(GetLock(_config->FindDir("Dir::Cache::Archives") + "lock"));
            if (_error->PendingError()) {
                return make_result(APT_ERROR_LOCK_FAILED, APT_MSG_LOCK_DOWNLOAD_DIR);
            }
        }

        ProgressStatus status;
        pkgAcquire acquire(&status);
        pkgSourceList source_list;

        if (!source_list.ReadMainList()) {
            std::string err = collect_pending_errors();
            if (err.empty()) err = "The list of sources could not be read.";
            return make_result(APT_ERROR_INSTALL_FAILED, err.c_str());
        }

        pkgRecords records(*cache->dep_cache);

        if (!active_pm->GetArchives(&acquire, &source_list, &records)) {
            std::string err = collect_pending_errors();
            if (err.empty()) err = "Failed to get package archives";
            return make_result(APT_ERROR_INSTALL_FAILED, err.c_str());
        }

        const auto acquire_result = acquire.Run();

        if (global_callback != nullptr) {
            global_callback("", APT_CALLBACK_DOWNLOAD_STOP, 100, 100, 0, global_user_data);
        }

        if (acquire_result != pkgAcquire::Continue) {
            return make_result(APT_ERROR_INSTALL_FAILED, APT_MSG_DOWNLOAD_FAILED);
        }

        if (download_only) {
            return make_result(APT_SUCCESS, nullptr);
        }

        if (_system) {
            _system->UnLock();
        }

        CallbackBridge bridgeData;
        bridgeData.user_data = user_data;
        bridgeData.cache = cache;
        for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
            if (const auto &st = (*cache->dep_cache)[it]; st.NewInstall() || st.Upgrade() || st.Downgrade() || st.Delete() ||
                                                          (include_reinstall && (st.iFlags & pkgDepCache::ReInstall) != 0)) {
                bridgeData.planned.emplace_back(it.Name());
            }
        }

        const PackageManagerCallback_t apt_callback = create_common_progress_callback(&bridgeData);

        const auto result = active_pm->DoInstall(apt_callback, &bridgeData);

        if (_system) _system->Lock();

        switch (result) {
            case pkgPackageManager::Completed:
                break;
            case pkgPackageManager::Failed:
                {
                    std::string error_details = collect_pending_errors();
                    if (error_details.empty()) {
                        error_details = "Package manager operation failed";
                    }
                    return make_result(APT_ERROR_OPERATION_FAILED, error_details.c_str());
                }
            case pkgPackageManager::Incomplete:
                {
                    std::string error_details = collect_pending_errors();
                    if (error_details.empty()) {
                        error_details = "Package manager operation incomplete";
                    }
                    return make_result(APT_ERROR_OPERATION_INCOMPLETE, error_details.c_str());
                }
            default:
                {
                    std::string error_details = collect_pending_errors();
                    if (error_details.empty()) {
                        error_details = "Unknown package manager result";
                    }
                    return make_result(APT_ERROR_INSTALL_FAILED, error_details.c_str());
                }
        }

        if (bool update_result = active_pm->UpdateMarks(); !update_result) {
            return make_result(APT_ERROR_INSTALL_FAILED, APT_MSG_MARKS_UPDATE_FAILED);
        }

        if (_error->PendingError()) {
            return make_result(APT_ERROR_UNKNOWN, "MARKER_8_FINAL_PENDING");
        }
        return make_result(APT_SUCCESS);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_INSTALL_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

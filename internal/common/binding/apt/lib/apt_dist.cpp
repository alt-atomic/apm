#include "apt_internal.h"

#include <apt-pkg/algorithms.h>
#include <apt-pkg/pkgrecords.h>
#include <apt-pkg/sourcelist.h>

AptResult apt_dist_upgrade_with_progress(AptCache *cache,
                                         AptProgressCallback callback,
                                         void *user_data) {
    if (!cache || !cache->dep_cache) return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid cache for dist upgrade");

    try {
        if (!pkgDistUpgrade(*cache->dep_cache)) {
            return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Distribution upgrade failed");
        }

        if (cache->dep_cache->DelCount() == 0 &&
            cache->dep_cache->InstCount() == 0 &&
            cache->dep_cache->BadCount() == 0) {
            return make_result(APT_SUCCESS, nullptr);
        }

        std::unique_ptr<pkgPackageManager> pm;
        pm.reset(_system->CreatePM(cache->dep_cache));
        if (!pm) {
            return make_result(APT_ERROR_INIT_FAILED, "Failed to create package manager for dist upgrade");
        }

        bool cb_set = false;
        if (callback) {
            global_callback = callback;
            global_user_data = user_data;
            cb_set = true;
        }

        ProgressStatus status;
        pkgAcquire acquire(&status);
        pkgSourceList source_list;
        if (!source_list.ReadMainList()) {
            if (cb_set) {
                global_callback = nullptr;
                global_user_data = nullptr;
            }
            return make_result(APT_ERROR_INSTALL_FAILED, "Failed to read sources.list");
        }

        pkgRecords records(*cache->dep_cache);
        if (!pm->GetArchives(&acquire, &source_list, &records)) {
            if (cb_set) {
                global_callback = nullptr;
                global_user_data = nullptr;
            }
            return make_result(APT_ERROR_INSTALL_FAILED, "Failed to get package archives for dist upgrade");
        }
        if (acquire.Run() != pkgAcquire::Continue) {
            if (cb_set) {
                global_callback = nullptr;
                global_user_data = nullptr;
            }
            return make_result(APT_ERROR_INSTALL_FAILED, "Failed to download packages for dist upgrade");
        }

        if (_system) {
            _system->UnLock();
        }

        CallbackBridge bridgeData;
        bridgeData.user_data = user_data;
        bridgeData.cache = cache;
        if (cache && cache->dep_cache) {
            for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
                auto &st = (*cache->dep_cache)[it];
                if (st.NewInstall() || st.Upgrade() || st.Delete()) {
                    bridgeData.planned.emplace_back(it.Name());
                }
            }
        }

        // Use common progress callback implementation
        PackageManagerCallback_t apt_callback = create_common_progress_callback(&bridgeData);

        auto result = pm->DoInstall(apt_callback, &bridgeData);
        switch (result) {
            case pkgPackageManager::Completed:
                break;
            case pkgPackageManager::Failed:
                if (_system) _system->Lock();
                if (cb_set) {
                    global_callback = nullptr;
                    global_user_data = nullptr;
                }
                // Collect detailed error message from APT
                {
                    std::string error_details = collect_pending_errors();
                    if (error_details.empty()) {
                        error_details = "Package manager operation failed";
                    }
                    return make_result(APT_ERROR_OPERATION_FAILED, error_details.c_str());
                }
            case pkgPackageManager::Incomplete:
                if (_system) _system->Lock();
                if (cb_set) {
                    global_callback = nullptr;
                    global_user_data = nullptr;
                }
                {
                    std::string error_details = collect_pending_errors();
                    if (error_details.empty()) {
                        error_details = "Package manager operation incomplete";
                    }
                    return make_result(APT_ERROR_OPERATION_INCOMPLETE, error_details.c_str());
                }
            default:
                if (_system) _system->Lock();
                if (cb_set) {
                    global_callback = nullptr;
                    global_user_data = nullptr;
                }
                {
                    std::string error_details = collect_pending_errors();
                    if (error_details.empty()) {
                        error_details = "Unknown package manager result";
                    }
                    return make_result(APT_ERROR_INSTALL_FAILED, error_details.c_str());
                }
        }

        bool update_marks = pm->UpdateMarks();
        if (!update_marks) {
            if (cb_set) {
                global_callback = nullptr;
                global_user_data = nullptr;
            }
            return make_result(APT_ERROR_INSTALL_FAILED, "Failed to update package marks after dist upgrade");
        }

        if (!check_apt_errors()) {
            if (cb_set) {
                global_callback = nullptr;
                global_user_data = nullptr;
            }
            return make_result(last_error, nullptr);
        }
        if (cb_set) {
            global_callback = nullptr;
            global_user_data = nullptr;
        }
        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        global_callback = nullptr;
        global_user_data = nullptr;
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}

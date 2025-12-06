#include "apt_internal.h"

AptResult apt_install_packages(AptPackageManager *pm, AptProgressCallback callback, void *user_data) {
    if (!pm || !pm->pm) return make_result(APT_ERROR_INIT_FAILED, "Invalid package manager instance");

    try {
        if (pm->cache->dep_cache->BrokenCount() != 0) {
            // Attribute the error to a concrete broken package similar to cache open/simulate
            for (pkgCache::PkgIterator it = pm->cache->dep_cache->PkgBegin(); !it.end(); ++it) {
                pkgDepCache::StateCache &st = (*pm->cache->dep_cache)[it];
                if (st.InstBroken() || st.NowBroken()) {
                    std::string out = std::string(
                                          "Some broken packages were found while trying to process build-dependencies for ")
                                      + it.Name();
                    return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
                }
            }
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Broken dependencies");
        }

        if (pm->cache->dep_cache->DelCount() == 0 &&
            pm->cache->dep_cache->InstCount() == 0 &&
            pm->cache->dep_cache->BadCount() == 0) {
            return make_result(APT_SUCCESS, nullptr);
        }

        if (callback != nullptr) {
            global_callback = callback;
            global_user_data = user_data;
        }

        ProgressStatus status;
        pkgAcquire acquire(&status);
        pkgSourceList source_list;

        if (!source_list.ReadMainList()) {
            return make_result(APT_ERROR_INSTALL_FAILED, "Failed to read sources.list");
        }

        pkgRecords records(*pm->cache->dep_cache);

        if (!pm->pm->GetArchives(&acquire, &source_list, &records)) {
            return make_result(APT_ERROR_INSTALL_FAILED, "Failed to get package archives");
        }

        auto acquire_result = acquire.Run();

        // Send final download complete event
        if (global_callback != nullptr) {
            global_callback("", APT_CALLBACK_DOWNLOAD_STOP, 100, 100, global_user_data);
        }

        if (acquire_result != pkgAcquire::Continue) {
            return make_result(APT_ERROR_INSTALL_FAILED, "Failed to download packages");
        }

        if (_system) {
            _system->UnLock();
        }

        // Prepare planned package names for fallback (new installs or deletes)
        CallbackBridge bridgeData;
        bridgeData.user_data = user_data;
        bridgeData.cache = pm->cache;
        if (pm->cache && pm->cache->dep_cache) {
            for (pkgCache::PkgIterator it = pm->cache->dep_cache->PkgBegin(); !it.end(); ++it) {
                auto &st = (*pm->cache->dep_cache)[it];
                if (st.NewInstall() || st.Upgrade() || st.Delete()) {
                    bridgeData.planned.emplace_back(it.Name());
                }
            }
        }

        // Use common progress callback implementation
        PackageManagerCallback_t apt_callback = create_common_progress_callback(&bridgeData);

        auto result = pm->pm->DoInstall(apt_callback, &bridgeData);

        switch (result) {
            case pkgPackageManager::Completed:
                break;
            case pkgPackageManager::Failed:
                if (_system) _system->Lock();
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
                {
                    std::string error_details = collect_pending_errors();
                    if (error_details.empty()) {
                        error_details = "Package manager operation incomplete";
                    }
                    return make_result(APT_ERROR_OPERATION_INCOMPLETE, error_details.c_str());
                }
            default:
                if (_system) _system->Lock();
                {
                    std::string error_details = collect_pending_errors();
                    if (error_details.empty()) {
                        error_details = "Unknown package manager result";
                    }
                    return make_result(APT_ERROR_INSTALL_FAILED, error_details.c_str());
                }
        }

        bool update_result = pm->pm->UpdateMarks();
        if (!update_result) {
            return make_result(APT_ERROR_INSTALL_FAILED, "Failed to update package marks");
        }

        global_callback = nullptr;
        global_user_data = nullptr;

        return make_result(check_apt_errors() ? APT_SUCCESS : last_error, nullptr);
    } catch (const std::exception &e) {
        global_callback = nullptr;
        global_user_data = nullptr;
        if (_system) {
            _system->Lock();
        }
        return make_result(APT_ERROR_INSTALL_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

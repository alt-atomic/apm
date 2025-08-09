#include "apt_internal.h"

AptErrorCode apt_dist_upgrade_with_progress(AptCache* cache,
                                                       AptProgressCallback callback,
                                                       void* user_data) {
    if (!cache || !cache->dep_cache) return APT_ERROR_CACHE_OPEN_FAILED;

    try {
        if (!pkgDistUpgrade(*cache->dep_cache)) {
            set_error(APT_ERROR_CACHE_OPEN_FAILED, "Distribution upgrade failed");
            return APT_ERROR_CACHE_OPEN_FAILED;
        }

        if (cache->dep_cache->DelCount() == 0 &&
            cache->dep_cache->InstCount() == 0 &&
            cache->dep_cache->BadCount() == 0) {
            return APT_SUCCESS;
        }

        std::unique_ptr<pkgPackageManager> pm;
        pm.reset(_system->CreatePM(cache->dep_cache));
        if (!pm) {
            set_error(APT_ERROR_INIT_FAILED, "Failed to create package manager for dist-upgrade");
            return APT_ERROR_INIT_FAILED;
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
            set_error(APT_ERROR_INSTALL_FAILED, "Failed to read sources.list");
            if (cb_set) { global_callback = nullptr; global_user_data = nullptr; }
            return APT_ERROR_INSTALL_FAILED;
        }

        pkgRecords records(*cache->dep_cache);
        if (!pm->GetArchives(&acquire, &source_list, &records)) {
            set_error(APT_ERROR_INSTALL_FAILED, "Failed to get package archives for dist-upgrade");
            if (cb_set) { global_callback = nullptr; global_user_data = nullptr; }
            return APT_ERROR_INSTALL_FAILED;
        }
        if (acquire.Run() != pkgAcquire::Continue) {
            set_error(APT_ERROR_INSTALL_FAILED, "Failed to download packages for dist-upgrade");
            if (cb_set) { global_callback = nullptr; global_user_data = nullptr; }
            return APT_ERROR_INSTALL_FAILED;
        }

        if (_system) {
            _system->UnLock();
        }

        struct CallbackBridge {
            void* user_data {nullptr};
            AptCache* cache {nullptr};
            std::vector<std::string> planned;
            size_t current_idx {0};
            std::string current_name;
        } bridgeData;
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

        PackageManagerCallback_t apt_callback = [](const char *nevra, aptCallbackType what, uint64_t amount, uint64_t total, void *callbackData) {
            CallbackBridge* bd = static_cast<CallbackBridge*>(callbackData);
            AptCallbackType our_type = APT_CALLBACK_UNKNOWN;
            switch (what) {
                case APTCALLBACK_INST_PROGRESS: our_type = APT_CALLBACK_INST_PROGRESS; break;
                case APTCALLBACK_INST_START: our_type = APT_CALLBACK_INST_START; break;
                case APTCALLBACK_INST_STOP: our_type = APT_CALLBACK_INST_STOP; break;
                case APTCALLBACK_TRANS_PROGRESS: our_type = APT_CALLBACK_TRANS_PROGRESS; break;
                case APTCALLBACK_TRANS_START: our_type = APT_CALLBACK_TRANS_START; break;
                case APTCALLBACK_TRANS_STOP: our_type = APT_CALLBACK_TRANS_STOP; break;
                case APTCALLBACK_UNINST_PROGRESS: our_type = APT_CALLBACK_REMOVE_PROGRESS; break;
                case APTCALLBACK_UNINST_START: our_type = APT_CALLBACK_REMOVE_START; break;
                case APTCALLBACK_UNINST_STOP: our_type = APT_CALLBACK_REMOVE_STOP; break;
                case APTCALLBACK_ELEM_PROGRESS: our_type = APT_CALLBACK_ELEM_PROGRESS; break;
                default: our_type = APT_CALLBACK_UNKNOWN; break;
            }

            const bool has_nevra = (nevra != nullptr && nevra[0] != '\0');
            const char* effective_name = "";
            switch (what) {
                case APTCALLBACK_INST_START:
                case APTCALLBACK_UNINST_START:
                    if (has_nevra) {
                        bd->current_name = nevra;
                    } else {
                        if (bd && !bd->planned.empty()) {
                            if (bd->current_idx >= bd->planned.size()) bd->current_idx = bd->planned.size() - 1;
                            bd->current_name = bd->planned[bd->current_idx];
                        }
                    }
                    effective_name = bd->current_name.c_str();
                    break;
                case APTCALLBACK_INST_PROGRESS:
                case APTCALLBACK_UNINST_PROGRESS:
                case APTCALLBACK_ELEM_PROGRESS:
                    if (bd->current_name.empty()) {
                        if (has_nevra) bd->current_name = nevra; else if (bd && !bd->planned.empty()) {
                            if (bd->current_idx >= bd->planned.size()) bd->current_idx = bd->planned.size() - 1;
                            bd->current_name = bd->planned[bd->current_idx];
                        }
                    }
                    effective_name = bd->current_name.c_str();
                    break;
                case APTCALLBACK_INST_STOP:
                case APTCALLBACK_UNINST_STOP:
                    if (!bd->current_name.empty()) effective_name = bd->current_name.c_str(); else if (bd && !bd->planned.empty()) {
                        if (bd->current_idx >= bd->planned.size()) bd->current_idx = bd->planned.size() - 1;
                        effective_name = bd->planned[bd->current_idx].c_str();
                    }
                    if (bd->current_idx < bd->planned.size()) bd->current_idx++;
                    bd->current_name.clear();
                    break;
                default:
                    effective_name = has_nevra ? nevra : "";
                    break;
            }

            if (global_callback) {
                global_callback(effective_name, our_type, amount, total, global_user_data);
            }
        };

        auto result = pm->DoInstall(apt_callback, &bridgeData);
        switch (result) {
            case pkgPackageManager::Completed:
                break;
            case pkgPackageManager::Failed:
                set_error(APT_ERROR_OPERATION_FAILED, "Package manager operation failed");
                if (_system) _system->Lock();
                if (cb_set) { global_callback = nullptr; global_user_data = nullptr; }
                return APT_ERROR_OPERATION_FAILED;
            case pkgPackageManager::Incomplete:
                set_error(APT_ERROR_OPERATION_INCOMPLETE, "Package manager operation incomplete");
                if (_system) _system->Lock();
                if (cb_set) { global_callback = nullptr; global_user_data = nullptr; }
                return APT_ERROR_OPERATION_INCOMPLETE;
            default:
                set_error(APT_ERROR_INSTALL_FAILED, "Unknown package manager result");
                if (_system) _system->Lock();
                if (cb_set) { global_callback = nullptr; global_user_data = nullptr; }
                return APT_ERROR_INSTALL_FAILED;
        }

        bool update_marks = pm->UpdateMarks();
        if (!update_marks) {
            set_error(APT_ERROR_INSTALL_FAILED, "Failed to update package marks after dist-upgrade");
            if (cb_set) { global_callback = nullptr; global_user_data = nullptr; }
            return APT_ERROR_INSTALL_FAILED;
        }

        if (!check_apt_errors()) {
            if (cb_set) { global_callback = nullptr; global_user_data = nullptr; }
            return last_error;
        }
        if (cb_set) { global_callback = nullptr; global_user_data = nullptr; }
        return APT_SUCCESS;
    } catch (const std::exception& e) {
        set_error(APT_ERROR_UNKNOWN, std::string("Exception: ") + e.what());
        // Ensure we don't leak global callback on exceptions
        global_callback = nullptr;
        global_user_data = nullptr;
        return APT_ERROR_UNKNOWN;
    }
}



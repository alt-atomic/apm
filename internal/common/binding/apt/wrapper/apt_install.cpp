#include "apt_internal.h"

AptResult apt_install_packages(AptPackageManager* pm, AptProgressCallback callback, void* user_data) {
    if (!pm || !pm->pm) return make_result(APT_ERROR_INIT_FAILED, "Invalid package manager instance");

    try {
        if (pm->cache->dep_cache->BrokenCount() != 0) {
            // Attribute the error to a concrete broken package similar to cache open/simulate
            for (pkgCache::PkgIterator it = pm->cache->dep_cache->PkgBegin(); !it.end(); ++it) {
                pkgDepCache::StateCache &st = (*pm->cache->dep_cache)[it];
                if (st.InstBroken() || st.NowBroken()) {
                    std::string out = std::string("Some broken packages were found while trying to process build-dependencies for ") + it.Name();
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

        if (acquire.Run() != pkgAcquire::Continue) {
            return make_result(APT_ERROR_INSTALL_FAILED, "Failed to download packages");
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
        };

        // Prepare planned package names for fallback (new installs or deletes)
        CallbackBridge bridgeData{};
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
            // Resolve effective name robustly for multi-package operations
            auto pick_planned_current = [&]() -> const char* {
                if (bd == nullptr || bd->planned.empty()) return "";
                if (bd->current_idx >= bd->planned.size()) return bd->planned.back().c_str();
                return bd->planned[bd->current_idx].c_str();
            };

            const bool has_nevra = (nevra != nullptr && nevra[0] != '\0');
            const char* effective_name = "";

            switch (what) {
                case APTCALLBACK_INST_START:
                case APTCALLBACK_UNINST_START:
                    if (has_nevra) {
                        bd->current_name = nevra;
                    } else if (bd != nullptr) {
                        bd->current_name = pick_planned_current();
                    }
                    effective_name = bd ? bd->current_name.c_str() : (has_nevra ? nevra : "");
                    break;

                case APTCALLBACK_INST_PROGRESS:
                case APTCALLBACK_UNINST_PROGRESS:
                case APTCALLBACK_ELEM_PROGRESS:
                    if (bd != nullptr && bd->current_name.empty()) {
                        bd->current_name = has_nevra ? std::string(nevra) : std::string(pick_planned_current());
                    }
                    effective_name = bd ? bd->current_name.c_str() : (has_nevra ? nevra : "");
                    break;

                case APTCALLBACK_INST_STOP:
                case APTCALLBACK_UNINST_STOP:
                    effective_name = bd && !bd->current_name.empty() ? bd->current_name.c_str() : (has_nevra ? nevra : pick_planned_current());
                    if (bd != nullptr) {
                        // Advance to next planned item on stop; clear current
                        if (bd->current_idx < bd->planned.size()) bd->current_idx++;
                        bd->current_name.clear();
                    }
                    break;

                default:
                    effective_name = has_nevra ? nevra : "";
                    break;
            }

            if (global_callback) {
                global_callback(effective_name, our_type, amount, total, global_user_data);
            }
        };

        auto result = pm->pm->DoInstall(apt_callback, &bridgeData);

        switch (result) {
            case pkgPackageManager::Completed:
                break;
            case pkgPackageManager::Failed:
                if (_system) _system->Lock();
                return make_result(APT_ERROR_OPERATION_FAILED, "Package manager operation failed");
            case pkgPackageManager::Incomplete:
                if (_system) _system->Lock();
                return make_result(APT_ERROR_OPERATION_INCOMPLETE, "Package manager operation incomplete");
            default:
                if (_system) _system->Lock();
                return make_result(APT_ERROR_INSTALL_FAILED, "Unknown package manager result");
        }

        bool update_result = pm->pm->UpdateMarks();
        if (!update_result) {
            return make_result(APT_ERROR_INSTALL_FAILED, "Failed to update package marks");
        }

        global_callback = nullptr;
        global_user_data = nullptr;

        return make_result(check_apt_errors() ? APT_SUCCESS : last_error, nullptr);
    } catch (const std::exception& e) {
        global_callback = nullptr;
        global_user_data = nullptr;
        if (_system) {
            _system->Lock();
        }
        return make_result(APT_ERROR_INSTALL_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

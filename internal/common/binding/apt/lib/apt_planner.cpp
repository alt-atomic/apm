#include "planner.h"
#include "apt_cache.h"
#include "mark.h"
#include "dependency.h"
#include "changes.h"
#include "apt_ext_rpm.h"
#include "ext_rpm.h"

#include <apt-pkg/algorithms.h>

#include <cstdlib>
#include <cstring>

// Detects RPM files in arguments, adds them to APT::Arguments, refreshes cache.
static AptResult preprocess_rpm_files_if_needed(AptCache *cache,
                                                const char **install_names, const size_t install_count,
                                                const char **remove_names, const size_t remove_count) {
    bool has_rpm_files = false;

    if (install_names && install_count > 0) {
        for (size_t i = 0; i < install_count; i++) {
            if (install_names[i] && is_rpm_file(std::string(install_names[i]))) {
                has_rpm_files = true;
                break;
            }
        }
    }

    if (!has_rpm_files && remove_names && remove_count > 0) {
        for (size_t i = 0; i < remove_count; i++) {
            if (remove_names[i] && is_rpm_file(std::string(remove_names[i]))) {
                has_rpm_files = true;
                break;
            }
        }
    }

    if (!has_rpm_files) {
        return make_result(APT_SUCCESS, nullptr);
    }

    bool need_refresh = false;
    bool added_new = false;

    if (install_names && install_count > 0) {
        if (const AptResult preprocess_result = apt_preprocess_install_arguments(install_names, install_count, &added_new); preprocess_result.code != APT_SUCCESS) {
            return preprocess_result;
        }
        if (added_new) need_refresh = true;
    }

    if (remove_names && remove_count > 0) {
        if (const AptResult preprocess_result = apt_preprocess_install_arguments(remove_names, remove_count, &added_new); preprocess_result.code != APT_SUCCESS) {
            return preprocess_result;
        }
        if (added_new) need_refresh = true;
    }

    if (need_refresh) {
        if (const AptResult refresh_result = apt_cache_refresh(cache); refresh_result.code != APT_SUCCESS) {
            return refresh_result;
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

// Orchestrates mark > resolve > collect for a combined operation.
AptResult plan_change_internal(
    AptCache *cache,
    const char **install_names, size_t install_count,
    const char **remove_names, size_t remove_count,
    const char **reinstall_names, size_t reinstall_count,
    bool purge,
    bool remove_depends,
    bool apply,
    AptPackageChanges *changes) {
    if (!cache || !changes) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_PARAMS);
    }
    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_CACHE_FILE_NOT_AVAILABLE);
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        std::set<std::string> requested_install;
        std::set<std::string> requested_remove;
        std::set<std::string> requested_reinstall;
        std::vector<std::pair<std::string, pkgCache::PkgIterator> > remove_targets;

        AptResult preprocess_result = preprocess_rpm_files_if_needed(cache, install_names, install_count, remove_names,
                                                                     remove_count);
        if (preprocess_result.code != APT_SUCCESS) {
            return preprocess_result;
        }

        std::unique_ptr<CacheStateGuard> stateGuard;
        if (!apply) {
            stateGuard = std::make_unique<CacheStateGuard>(cache->dep_cache);
        }
        (void)stateGuard;

        AptResult result = process_package_installs(cache, install_names, install_count, requested_install);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = process_package_removals(cache, remove_names, remove_count, purge, requested_remove, remove_targets);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = process_package_reinstalls(cache, reinstall_names, reinstall_count, requested_reinstall);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = check_package_conflicts(cache, requested_install);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = preprocess_installs(cache, requested_install);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = finalize_dependency_resolution(cache, requested_install, requested_remove, remove_depends);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        std::vector<std::string> extra_installed;
        std::vector<std::string> upgraded;
        std::vector<std::string> new_installed;
        std::vector<std::string> removed;
        std::vector<std::string> kept_back;
        uint64_t download_size = 0;
        int64_t install_size = 0;

        collect_package_changes(cache, requested_install,
                                extra_installed, upgraded,
                                new_installed, removed, kept_back, download_size, install_size);

        if (!requested_install.empty() && requested_remove.empty() && requested_reinstall.empty()) {
            std::set<std::string> will_change;
            for (const auto &pkg : new_installed) will_change.insert(pkg);
            for (const auto &pkg : upgraded) will_change.insert(pkg);

            std::vector<std::string> already_installed;
            for (const auto &req : requested_install) {
                if (will_change.find(req) == will_change.end()) {
                    already_installed.push_back(req);
                }
            }

            if (!already_installed.empty() && already_installed.size() == requested_install.size()) {
                std::string msg = "Packages are already installed: ";
                for (size_t i = 0; i < already_installed.size(); ++i) {
                    if (i > 0) msg += ", ";
                    msg += already_installed[i];
                }
                return make_result(APT_ERROR_PACKAGES_ALREADY_INSTALLED, msg.c_str());
            }
        }

        const size_t total_not_upgraded = kept_back.size();
        kept_back.clear();

        std::vector<std::pair<std::string, std::string>> essential_list;
        collect_essential_packages(cache, essential_list);

        populate_changes_structure(changes, extra_installed, upgraded, new_installed, removed,
                                   kept_back, total_not_upgraded, essential_list, download_size, install_size);

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Combined simulation failed: ") + e.what()).c_str());
    }
}

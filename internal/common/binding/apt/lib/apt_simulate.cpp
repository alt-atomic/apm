#include "apt_package_operations.h"

#include <apt-pkg/algorithms.h>

#include <cstdlib>
#include <cstring>

// RAII guard for cache state management
class CacheStateGuard {
    std::unique_ptr<pkgDepCache::State> saved_state;

public:
    explicit CacheStateGuard(pkgDepCache *cache) : saved_state(std::make_unique<pkgDepCache::State>(cache)) {
    }

    ~CacheStateGuard() { if (saved_state) saved_state->Restore(); }
    void commit() { saved_state.reset(); }
};

// Helper function to preprocess RPM files from both install and remove arguments
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

    // Process RPM files - track if any new files were added
    bool need_refresh = false;
    bool added_new = false;

    if (install_names && install_count > 0) {
        const AptResult preprocess_result = apt_preprocess_install_arguments(install_names, install_count, &added_new);
        if (preprocess_result.code != APT_SUCCESS) {
            return preprocess_result;
        }
        if (added_new) need_refresh = true;
    }

    if (remove_names && remove_count > 0) {
        const AptResult preprocess_result = apt_preprocess_install_arguments(remove_names, remove_count, &added_new);
        if (preprocess_result.code != APT_SUCCESS) {
            return preprocess_result;
        }
        if (added_new) need_refresh = true;
    }

    // Only refresh cache if new RPM files were added to config
    if (need_refresh) {
        const AptResult refresh_result = apt_cache_refresh(cache);
        if (refresh_result.code != APT_SUCCESS) {
            return refresh_result;
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

AptResult apt_simulate_dist_upgrade(const AptCache *cache, AptPackageChanges *changes) {
    if (!cache || !changes) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for simulation");
    }

    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        CacheStateGuard stateGuard(cache->dep_cache);

        pkgDistUpgrade(*cache->dep_cache);

        // Try to resolve problems like apt-get does before declaring broken
        if (cache->dep_cache->BrokenCount() > 0) {
            pkgProblemResolver Fix2(cache->dep_cache);
            Fix2.InstallProtect();
            (void) Fix2.Resolve(true);
        }
        if (cache->dep_cache->BrokenCount() > 0) {
            for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
                pkgDepCache::StateCache &st = (*cache->dep_cache)[it];
                if (st.InstBroken() || st.NowBroken()) {
                    std::string out = std::string(
                                          "Some broken packages were found while trying to process build-dependencies for ")
                                      + it.Name();
                    return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
                }
            }
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Broken dependencies");
        }

        if (!check_apt_errors()) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, nullptr);
        }

        std::vector<std::string> upgraded;
        std::vector<std::string> new_installed;
        std::vector<std::string> removed;

        uint64_t download_size = 0;
        int64_t install_size = 0;

        for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin();
             !iter.end(); ++iter) {
            pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[iter];

            if (pkg_state.NewInstall()) {
                new_installed.emplace_back(iter.Name());

                if (pkg_state.CandidateVer != nullptr) {
                    download_size += static_cast<uint64_t>(pkg_state.CandidateVer->Size);
                    install_size += static_cast<int64_t>(pkg_state.CandidateVer->InstalledSize);
                }
            } else if (pkg_state.Upgrade()) {
                upgraded.emplace_back(iter.Name());

                if (pkg_state.CandidateVer != nullptr) {
                    download_size += static_cast<uint64_t>(pkg_state.CandidateVer->Size);
                    install_size += static_cast<int64_t>(pkg_state.CandidateVer->InstalledSize);
                    pkgCache::VerIterator currentVer = iter.CurrentVer();
                    if (!currentVer.end()) {
                        install_size -= static_cast<int64_t>(currentVer->InstalledSize);
                    }
                }
            } else if (pkg_state.Downgrade()) {
                upgraded.emplace_back(iter.Name());

                if (pkg_state.CandidateVer != nullptr) {
                    download_size += static_cast<uint64_t>(pkg_state.CandidateVer->Size);
                    install_size += static_cast<int64_t>(pkg_state.CandidateVer->InstalledSize);
                    pkgCache::VerIterator currentVer = iter.CurrentVer();
                    if (!currentVer.end()) {
                        install_size -= static_cast<int64_t>(currentVer->InstalledSize);
                    }
                }
            } else if (pkg_state.Delete()) {
                removed.emplace_back(iter.Name());

                if (pkg_state.InstallVer != nullptr) {
                    install_size -= static_cast<int64_t>(pkg_state.InstallVer->InstalledSize);
                }
            }
        }

        changes->extra_installed_count = 0;
        changes->upgraded_count = upgraded.size();
        changes->new_installed_count = new_installed.size();
        changes->removed_count = removed.size();
        changes->not_upgraded_count = 0;
        changes->download_size = download_size;
        changes->install_size = install_size;

        if (changes->new_installed_count > 0) {
            changes->new_installed_packages = static_cast<char **>(malloc(changes->new_installed_count * sizeof(char *)));
            for (size_t i = 0; i < changes->new_installed_count; i++) {
                changes->new_installed_packages[i] = safe_strdup(new_installed[i]);
            }
        }

        if (changes->upgraded_count > 0) {
            changes->upgraded_packages = static_cast<char **>(malloc(changes->upgraded_count * sizeof(char *)));
            for (size_t i = 0; i < changes->upgraded_count; i++) {
                changes->upgraded_packages[i] = safe_strdup(upgraded[i]);
            }
        }

        if (changes->removed_count > 0) {
            changes->removed_packages = static_cast<char **>(malloc(changes->removed_count * sizeof(char *)));
            for (size_t i = 0; i < changes->removed_count; i++) {
                changes->removed_packages[i] = safe_strdup(removed[i]);
            }
        }

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Dist upgrade simulation failed: ") + e.what()).c_str());
    }
}

AptResult apt_simulate_install(AptCache *cache, const char **package_names, const size_t count, AptPackageChanges *changes) {
    return apt_simulate_change(cache, package_names, count, nullptr, 0, false, false, changes);
}

AptResult apt_simulate_remove(AptCache *cache, const char **package_names, const size_t count, const bool purge,
                              const bool remove_depends, AptPackageChanges *changes) {
    return apt_simulate_change(cache, nullptr, 0, package_names, count, purge, remove_depends, changes);
}

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
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for change");
    }
    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        std::set<std::string> requested_install;
        std::set<std::string> requested_remove;
        std::set<std::string> requested_reinstall;
        std::vector<std::pair<std::string, pkgCache::PkgIterator> > remove_targets;

        // Preprocess RPM files BEFORE saving cache state
        AptResult preprocess_result = preprocess_rpm_files_if_needed(cache, install_names, install_count, remove_names,
                                                                     remove_count);
        if (preprocess_result.code != APT_SUCCESS) {
            return preprocess_result;
        }

        // RAII guard: destructor restores cache state
        std::unique_ptr<CacheStateGuard> stateGuard;
        if (!apply) {
            stateGuard = std::make_unique<CacheStateGuard>(cache->dep_cache);
        }
        (void)stateGuard;

        // Step 1: Process package installations (marks packages)
        AptResult result = process_package_installs(cache, install_names, install_count, requested_install);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        // Step 2: Process package removals (marks packages)
        result = process_package_removals(cache, remove_names, remove_count, purge, requested_remove, remove_targets);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        // Step 3: Process package reinstalls
        result = process_package_reinstalls(cache, reinstall_names, reinstall_count, requested_reinstall);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        // Step 4: Check for package conflicts FIRST
        result = check_package_conflicts(cache, requested_install);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        // Step 5: Preprocess additional install dependencies
        result = preprocess_installs(cache, requested_install);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        // Step 6: Finalize all dependency resolution (single Fix.Resolve for everything)
        result = finalize_dependency_resolution(cache, requested_install, requested_remove, remove_depends);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        // Collect changes
        std::vector<std::string> extra_installed;
        std::vector<std::string> extra_removed;
        std::vector<std::string> upgraded;
        std::vector<std::string> new_installed;
        std::vector<std::string> removed;
        std::vector<std::string> reinstalled;
        uint64_t download_size = 0;
        int64_t install_size = 0;

        collect_package_changes(cache, requested_install, requested_remove,
                                extra_installed, extra_removed, upgraded,
                                new_installed, removed, download_size, install_size);

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

        // Collect essential/important packages that will be removed
        std::vector<std::pair<std::string, std::string>> essential_list;
        collect_essential_packages(cache, essential_list);

        populate_changes_structure(changes, extra_installed, upgraded, new_installed, removed,
                                   essential_list, download_size, install_size);

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Combined simulation failed: ") + e.what()).c_str());
    }
}

AptResult apt_simulate_change(AptCache *cache,
                              const char **install_names, size_t install_count,
                              const char **remove_names, size_t remove_count,
                              bool purge,
                              bool remove_depends,
                              AptPackageChanges *changes) {
    return plan_change_internal(cache, install_names, install_count, remove_names, remove_count,
                                nullptr, 0, purge, remove_depends, false, changes);
}

AptResult apt_apply_changes(AptCache *cache,
                            const char **install_names, const size_t install_count,
                            const char **remove_names, const size_t remove_count,
                            const bool purge,
                            const bool remove_depends) {
    if (!cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache for apply changes");
    }

    AptPackageChanges dummy{};
    const AptResult r = plan_change_internal(cache, install_names, install_count,
                                       remove_names, remove_count,
                                       nullptr, 0,
                                       purge, remove_depends,
                                       true,
                                       &dummy);
    apt_free_package_changes(&dummy);
    return r;
}

AptResult apt_simulate_reinstall(AptCache *cache, const char **package_names, size_t count,
                                 AptPackageChanges *changes) {
    return plan_change_internal(cache, nullptr, 0, nullptr, 0,
                                package_names, count, false, false, false, changes);
}

AptResult apt_apply_reinstall(AptCache *cache, const char **package_names, size_t count) {
    if (!cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache for apply reinstall");
    }

    AptPackageChanges dummy{};
    const AptResult r = plan_change_internal(cache, nullptr, 0, nullptr, 0,
                                       package_names, count,
                                       false, false,
                                       true, // apply=true - commit changes to cache
                                       &dummy);
    apt_free_package_changes(&dummy);
    return r;
}

AptResult apt_simulate_autoremove(const AptCache *cache, AptPackageChanges *changes) {
    if (!cache || !changes) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for autoremove simulation");
    }

    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        // RAII guard for automatic cache state restoration
        CacheStateGuard stateGuard(cache->dep_cache);

        if (cache->dep_cache->BrokenCount() != 0) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Cache has broken packages");
        }

        std::set<std::string> kept_packages;
        std::set<std::string> unneeded_packages;

        if (!pkgAutoremoveGetKeptAndUnneededPackages(*cache->dep_cache, &kept_packages, &unneeded_packages)) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Failed to calculate autoremove packages");
        }

        // Mark unneeded packages for removal
        for (const std::string &pkg_name: unneeded_packages) {
            pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(pkg_name);
            if (!pkg.end() && pkg->CurrentState == pkgCache::State::Installed) {
                cache->dep_cache->MarkDelete(pkg, false);
            }
        }

        // Try to resolve problems
        if (cache->dep_cache->BrokenCount() > 0) {
            pkgProblemResolver Fix(cache->dep_cache);
            Fix.InstallProtect();
            if (!Fix.Resolve(false)) {
                return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Failed to resolve dependencies during autoremove");
            }
        }

        if (!check_apt_errors()) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, nullptr);
        }

        std::vector<std::string> removed;
        std::vector<std::string> upgraded;
        std::vector<std::string> new_installed;

        uint64_t download_size = 0;
        int64_t install_size = 0;

        for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
            pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[iter];

            if (pkg_state.Delete()) {
                removed.emplace_back(iter.Name());
                if (pkg_state.InstallVer != nullptr) {
                    install_size -= static_cast<int64_t>(pkg_state.InstallVer->InstalledSize);
                }
            } else if (pkg_state.NewInstall()) {
                new_installed.emplace_back(iter.Name());
                if (pkg_state.CandidateVer != nullptr) {
                    download_size += static_cast<uint64_t>(pkg_state.CandidateVer->Size);
                    install_size += static_cast<int64_t>(pkg_state.CandidateVer->InstalledSize);
                }
            } else if (pkg_state.Upgrade()) {
                upgraded.emplace_back(iter.Name());
                if (pkg_state.CandidateVer != nullptr) {
                    download_size += static_cast<uint64_t>(pkg_state.CandidateVer->Size);
                    install_size += static_cast<int64_t>(pkg_state.CandidateVer->InstalledSize);
                    pkgCache::VerIterator currentVer = iter.CurrentVer();
                    if (!currentVer.end()) {
                        install_size -= static_cast<int64_t>(currentVer->InstalledSize);
                    }
                }
            } else if (pkg_state.Downgrade()) {
                upgraded.emplace_back(iter.Name());
                if (pkg_state.CandidateVer != nullptr) {
                    download_size += static_cast<uint64_t>(pkg_state.CandidateVer->Size);
                    install_size += static_cast<int64_t>(pkg_state.CandidateVer->InstalledSize);
                    pkgCache::VerIterator currentVer = iter.CurrentVer();
                    if (!currentVer.end()) {
                        install_size -= static_cast<int64_t>(currentVer->InstalledSize);
                    }
                }
            }
        }

        // Fill results structure
        changes->extra_installed_count = 0;
        changes->upgraded_count = upgraded.size();
        changes->new_installed_count = new_installed.size();
        changes->removed_count = removed.size();
        changes->not_upgraded_count = 0;
        changes->download_size = download_size;
        changes->install_size = install_size;

        if (changes->removed_count > 0) {
            changes->removed_packages = static_cast<char **>(malloc(changes->removed_count * sizeof(char *)));
            for (size_t i = 0; i < changes->removed_count; i++) {
                changes->removed_packages[i] = safe_strdup(removed[i]);
            }
        }

        if (changes->upgraded_count > 0) {
            changes->upgraded_packages = static_cast<char **>(malloc(changes->upgraded_count * sizeof(char *)));
            for (size_t i = 0; i < changes->upgraded_count; i++) {
                changes->upgraded_packages[i] = safe_strdup(upgraded[i]);
            }
        }

        if (changes->new_installed_count > 0) {
            changes->new_installed_packages = static_cast<char **>(malloc(changes->new_installed_count * sizeof(char *)));
            for (size_t i = 0; i < changes->new_installed_count; i++) {
                changes->new_installed_packages[i] = safe_strdup(new_installed[i]);
            }
        }

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Autoremove simulation failed: ") + e.what()).c_str());
    }
}

#include "apt_internal.h"
#include "apt_package_operations.h"
#include <apt-pkg/version.h>
#include <set>
#include <memory>
#include <iostream>

// Helper function to preprocess RPM files from both install and remove arguments
static AptResult preprocess_rpm_files_if_needed(AptCache* cache,
                                                const char** install_names, size_t install_count,
                                                const char** remove_names, size_t remove_count) {
    // Check if we have any RPM files
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
    
    // Process RPM files and refresh cache ONLY ONCE
    if (install_names && install_count > 0) {
        AptResult preprocess_result = apt_preprocess_install_arguments(install_names, install_count);
        if (preprocess_result.code != APT_SUCCESS) {
            return preprocess_result;
        }
    }
    
    if (remove_names && remove_count > 0) {
        AptResult preprocess_result = apt_preprocess_install_arguments(remove_names, remove_count);
        if (preprocess_result.code != APT_SUCCESS) {
            return preprocess_result;
        }
    }
    
    AptResult refresh_result = apt_cache_refresh(cache);
    if (refresh_result.code != APT_SUCCESS) {
        return refresh_result;
    }
    
    return make_result(APT_SUCCESS, nullptr);
}

AptResult apt_simulate_dist_upgrade(AptCache* cache, AptPackageChanges* changes) {
    if (!cache || !changes) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for simulation");
    }

    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        pkgDistUpgrade(*cache->dep_cache);

        // Try to resolve problems like apt-get does before declaring broken
        if (cache->dep_cache->BrokenCount() > 0) {
            pkgProblemResolver Fix2(cache->dep_cache);
            Fix2.InstallProtect();
            (void)Fix2.Resolve(true);
        }
        if (cache->dep_cache->BrokenCount() > 0) {
            // Attribute error to a concrete broken package for clarity
            for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
                pkgDepCache::StateCache &st = (*cache->dep_cache)[it];
                if (st.InstBroken() || st.NowBroken()) {
                    std::string out = std::string("Some broken packages were found while trying to process build-dependencies for ") + it.Name();
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
        uint64_t install_size = 0;

        for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin();
             !iter.end(); ++iter) {

            pkgDepCache::StateCache& pkg_state = (*cache->dep_cache)[iter];

            if (pkg_state.NewInstall()) {
                new_installed.push_back(iter.Name());

                if (pkg_state.CandidateVer != 0) {
                    download_size += pkg_state.CandidateVer->Size;
                    install_size += pkg_state.CandidateVer->InstalledSize;
                }
            } else if (pkg_state.Upgrade()) {
                upgraded.push_back(iter.Name());

                if (pkg_state.CandidateVer != 0) {
                    download_size += pkg_state.CandidateVer->Size;
                    install_size += pkg_state.CandidateVer->InstalledSize;
                    if (pkg_state.InstallVer != 0) {
                        install_size -= pkg_state.InstallVer->InstalledSize;
                    }
                }
            } else if (pkg_state.Delete()) {
                removed.push_back(iter.Name());

                if (pkg_state.InstallVer != 0) {
                    install_size -= pkg_state.InstallVer->InstalledSize;
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
            changes->new_installed_packages = (char**)malloc(changes->new_installed_count * sizeof(char*));
            for (size_t i = 0; i < changes->new_installed_count; i++) {
                changes->new_installed_packages[i] = strdup(new_installed[i].c_str());
            }
        }

        if (changes->upgraded_count > 0) {
            changes->upgraded_packages = (char**)malloc(changes->upgraded_count * sizeof(char*));
            for (size_t i = 0; i < changes->upgraded_count; i++) {
                changes->upgraded_packages[i] = strdup(upgraded[i].c_str());
            }
        }

        if (changes->removed_count > 0) {
            changes->removed_packages = (char**)malloc(changes->removed_count * sizeof(char*));
            for (size_t i = 0; i < changes->removed_count; i++) {
                changes->removed_packages[i] = strdup(removed[i].c_str());
            }
        }

        return make_result(APT_SUCCESS, nullptr);

    } catch (const std::exception& e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Dist upgrade simulation failed: ") + e.what()).c_str());
    }
}

AptResult apt_simulate_install(AptCache* cache, const char** package_names, size_t count, AptPackageChanges* changes) {
    // Delegate to unified change simulator
    return apt_simulate_change(cache, package_names, count, nullptr, 0, false, false, changes);
}

AptResult apt_simulate_remove(AptCache* cache, const char** package_names, size_t count, bool purge, bool remove_depends, AptPackageChanges* changes) {
    // Delegate to unified change simulator
    return apt_simulate_change(cache, nullptr, 0, package_names, count, purge, remove_depends, changes);
}

AptResult plan_change_internal(
     AptCache* cache,
     const char** install_names, size_t install_count,
     const char** remove_names, size_t remove_count,
     bool purge,
     bool remove_depends,
     bool apply,
     AptPackageChanges* changes) {

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
         std::vector<std::pair<std::string, pkgCache::PkgIterator>> remove_targets;

         // Step 0: Preprocess RPM files BEFORE saving cache state
         AptResult preprocess_result = preprocess_rpm_files_if_needed(cache, install_names, install_count, remove_names, remove_count);
         if (preprocess_result.code != APT_SUCCESS) {
             return preprocess_result;
         }

         // Save cache state for simulation rollback AFTER RPM preprocessing
         std::unique_ptr<pkgDepCache::State> savedState;
         if (!apply) {
             savedState = std::make_unique<pkgDepCache::State>(cache->dep_cache);
         }

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

        // Step 3a: Check for package conflicts FIRST
        result = check_package_conflicts(cache, requested_install);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        // Step 3b: Preprocess additional install dependencies
        result = preprocess_installs(cache, requested_install);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        // Step 3c: Preprocess removal validations
        result = preprocess_removals(cache, requested_remove);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        // Step 3d: Finalize all dependency resolution (single Fix.Resolve for everything)
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
         uint64_t download_size = 0;
         uint64_t install_size = 0;

         collect_package_changes(cache, requested_install, requested_remove,
                               extra_installed, extra_removed, upgraded, 
                               new_installed, removed, download_size, install_size);

         // Populate changes structure
         populate_changes_structure(changes, extra_installed, upgraded, new_installed, removed, download_size, install_size);

         // Restore package states for simulation
         if (!apply && savedState) {
             savedState->Restore();
         }

         return make_result(APT_SUCCESS, nullptr);
     } catch (const std::exception& e) {
         return make_result(APT_ERROR_UNKNOWN, (std::string("Combined simulation failed: ") + e.what()).c_str());
     }
}

AptResult apt_simulate_change(AptCache* cache,
                              const char** install_names, size_t install_count,
                              const char** remove_names, size_t remove_count,
                              bool purge,
                              bool remove_depends,
                              AptPackageChanges* changes) {
    return plan_change_internal(cache, install_names, install_count, remove_names, remove_count, purge, remove_depends, false, changes);
}

AptResult apt_simulate_autoremove(AptCache* cache, AptPackageChanges* changes) {
    if (!cache || !changes) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for autoremove simulation");
    }

    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        // Check if cache is broken
        if (cache->dep_cache->BrokenCount() != 0) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Cache has broken packages");
        }

        // Use the existing pkgAutoremoveGetKeptAndUnneededPackages function
        std::set<std::string> kept_packages;
        std::set<std::string> unneeded_packages;
        
        if (!pkgAutoremoveGetKeptAndUnneededPackages(*cache->dep_cache, &kept_packages, &unneeded_packages)) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Failed to calculate autoremove packages");
        }

        // Mark unneeded packages for removal
        for (const std::string& pkg_name : unneeded_packages) {
            pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(pkg_name.c_str());
            if (!pkg.end() && pkg->CurrentState == pkgCache::State::Installed) {
                cache->dep_cache->MarkDelete(pkg, false); // false = don't purge, just remove
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

        // Collect results
        std::vector<std::string> removed;
        std::vector<std::string> upgraded;
        std::vector<std::string> new_installed;

        uint64_t download_size = 0;
        uint64_t install_size = 0;

        for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
            pkgDepCache::StateCache& pkg_state = (*cache->dep_cache)[iter];

            if (pkg_state.Delete()) {
                removed.push_back(iter.Name());
                if (pkg_state.InstallVer != 0) {
                    install_size -= pkg_state.InstallVer->InstalledSize;
                }
            } else if (pkg_state.NewInstall()) {
                new_installed.push_back(iter.Name());
                if (pkg_state.CandidateVer != 0) {
                    download_size += pkg_state.CandidateVer->Size;
                    install_size += pkg_state.CandidateVer->InstalledSize;
                }
            } else if (pkg_state.Upgrade()) {
                upgraded.push_back(iter.Name());
                if (pkg_state.CandidateVer != 0) {
                    download_size += pkg_state.CandidateVer->Size;
                    install_size += pkg_state.CandidateVer->InstalledSize;
                    if (pkg_state.InstallVer != 0) {
                        install_size -= pkg_state.InstallVer->InstalledSize;
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

        // Allocate and fill string arrays
        if (changes->removed_count > 0) {
            changes->removed_packages = (char**)malloc(changes->removed_count * sizeof(char*));
            for (size_t i = 0; i < changes->removed_count; i++) {
                changes->removed_packages[i] = strdup(removed[i].c_str());
            }
        }

        if (changes->upgraded_count > 0) {
            changes->upgraded_packages = (char**)malloc(changes->upgraded_count * sizeof(char*));
            for (size_t i = 0; i < changes->upgraded_count; i++) {
                changes->upgraded_packages[i] = strdup(upgraded[i].c_str());
            }
        }

        if (changes->new_installed_count > 0) {
            changes->new_installed_packages = (char**)malloc(changes->new_installed_count * sizeof(char*));
            for (size_t i = 0; i < changes->new_installed_count; i++) {
                changes->new_installed_packages[i] = strdup(new_installed[i].c_str());
            }
        }

        // Restore package states (undo the marking for simulation)
        for (const std::string& pkg_name : unneeded_packages) {
            pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(pkg_name.c_str());
            if (!pkg.end()) {
                cache->dep_cache->MarkKeep(pkg, false);
            }
        }

        return make_result(APT_SUCCESS, nullptr);

    } catch (const std::exception& e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Autoremove simulation failed: ") + e.what()).c_str());
    }
}

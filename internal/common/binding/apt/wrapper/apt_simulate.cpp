#include "apt_internal.h"

// Verbatim copies of simulation functions from apt_wrapper.cpp

AptErrorCode apt_simulate_dist_upgrade(AptCache* cache, AptPackageChanges* changes) {
    if (!cache || !changes) {
        set_error(APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for simulation");
        return APT_ERROR_INVALID_PARAMETERS;
    }

    if (!cache->cache_file) {
        set_error(APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
        return APT_ERROR_CACHE_OPEN_FAILED;
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        pkgDistUpgrade(*cache->dep_cache);

        if (!check_apt_errors()) {
            return APT_ERROR_DEPENDENCY_BROKEN;
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

        return APT_SUCCESS;

    } catch (const std::exception& e) {
        set_error(APT_ERROR_UNKNOWN, std::string("Dist-upgrade simulation failed: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

AptErrorCode apt_simulate_install(AptCache* cache, const char** package_names, size_t count, AptPackageChanges* changes) {
    if (!cache || !package_names || count == 0 || !changes) {
        set_error(AptErrorCode::APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for multi-package simulation");
        return AptErrorCode::APT_ERROR_INVALID_PARAMETERS;
    }

    if (!cache->cache_file) {
        set_error(AptErrorCode::APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
        return AptErrorCode::APT_ERROR_CACHE_OPEN_FAILED;
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        std::set<std::string> requested_packages;
        for (size_t i = 0; i < count; i++) {
            if (!package_names[i]) continue;

            std::string pkg_name(package_names[i]);
            requested_packages.insert(pkg_name);

            pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(pkg_name.c_str());
            if (pkg.end()) {
                set_error(AptErrorCode::APT_ERROR_PACKAGE_NOT_FOUND, std::string("Package not found: ") + pkg_name);
                return AptErrorCode::APT_ERROR_PACKAGE_NOT_FOUND;
            }

            cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, true);
        }

        if (!check_apt_errors()) {
            return AptErrorCode::APT_ERROR_DEPENDENCY_BROKEN;
        }

        std::vector<std::string> extra_installed;
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

                if (requested_packages.find(iter.Name()) == requested_packages.end()) {
                    extra_installed.push_back(iter.Name());
                }

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

        changes->extra_installed_count = extra_installed.size();
        changes->upgraded_count = upgraded.size();
        changes->new_installed_count = new_installed.size();
        changes->removed_count = removed.size();
        changes->not_upgraded_count = 0;
        changes->download_size = download_size;
        changes->install_size = install_size;

        if (changes->extra_installed_count > 0) {
            changes->extra_installed = (char**)malloc(changes->extra_installed_count * sizeof(char*));
            for (size_t i = 0; i < changes->extra_installed_count; i++) {
                changes->extra_installed[i] = strdup(extra_installed[i].c_str());
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

        if (changes->removed_count > 0) {
            changes->removed_packages = (char**)malloc(changes->removed_count * sizeof(char*));
            for (size_t i = 0; i < changes->removed_count; i++) {
                changes->removed_packages[i] = strdup(removed[i].c_str());
            }
        }

        for (size_t i = 0; i < count; i++) {
            if (!package_names[i]) continue;

            pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(package_names[i]);
            if (!pkg.end()) {
                cache->dep_cache->MarkKeep(pkg, false);
            }
        }

        return APT_SUCCESS;

    } catch (const std::exception& e) {
        set_error(APT_ERROR_UNKNOWN, std::string("Multi-package install simulation failed: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

AptErrorCode apt_simulate_remove(AptCache* cache, const char** package_names, size_t count, AptPackageChanges* changes) {
    if (!cache || !package_names || count == 0 || !changes) {
        set_error(APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for multi-package simulation");
        return APT_ERROR_INVALID_PARAMETERS;
    }

    if (!cache->cache_file) {
        set_error(APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
        return APT_ERROR_CACHE_OPEN_FAILED;
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        std::set<std::string> requested_packages;
        for (size_t i = 0; i < count; i++) {
            if (!package_names[i]) continue;

            std::string pkg_name(package_names[i]);
            requested_packages.insert(pkg_name);

            pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(pkg_name.c_str());
            if (pkg.end()) {
                set_error(APT_ERROR_PACKAGE_NOT_FOUND, std::string("Package not found: ") + pkg_name);
                return APT_ERROR_PACKAGE_NOT_FOUND;
            }

            if (pkg.CurrentVer().end()) {
                set_error(APT_ERROR_PACKAGE_NOT_FOUND, std::string("Package is not installed: ") + pkg_name);
                return APT_ERROR_PACKAGE_NOT_FOUND;
            }

            cache->dep_cache->MarkDelete(pkg, true); // true = purge
        }

        if (!check_apt_errors()) {
            return APT_ERROR_DEPENDENCY_BROKEN;
        }

        std::vector<std::string> extra_removed;
        std::vector<std::string> upgraded;
        std::vector<std::string> new_installed;
        std::vector<std::string> removed;

        uint64_t download_size = 0;
        uint64_t install_size = 0;

        for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin();
             !iter.end(); ++iter) {

            pkgDepCache::StateCache& pkg_state = (*cache->dep_cache)[iter];

            if (pkg_state.Delete()) {
                removed.push_back(iter.Name());

                if (requested_packages.find(iter.Name()) == requested_packages.end()) {
                    extra_removed.push_back(iter.Name());
                }

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

        // Allocate and fill results
        changes->extra_installed_count = extra_removed.size();
        changes->upgraded_count = upgraded.size();
        changes->new_installed_count = new_installed.size();
        changes->removed_count = removed.size();
        changes->not_upgraded_count = 0;
        changes->download_size = download_size;
        changes->install_size = install_size;

        // Allocate string arrays
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

        // Store extra removed in extra_installed array (reusing field)
        if (changes->extra_installed_count > 0) {
            changes->extra_installed = (char**)malloc(changes->extra_installed_count * sizeof(char*));
            for (size_t i = 0; i < changes->extra_installed_count; i++) {
                changes->extra_installed[i] = strdup(extra_removed[i].c_str());
            }
        }

        // Restore package states (undo the marking for simulation)
        for (size_t i = 0; i < count; i++) {
            if (!package_names[i]) continue;
            pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(package_names[i]);
            if (!pkg.end()) {
                cache->dep_cache->MarkKeep(pkg, false);
            }
        }

        return APT_SUCCESS;

    } catch (const std::exception& e) {
        set_error(APT_ERROR_UNKNOWN, std::string("Multi-package remove simulation failed: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}



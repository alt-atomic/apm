#include "apt_internal.h"

// Verbatim copies of simulation functions from apt_wrapper.cpp

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
            pkgProblemResolver Fix(cache->dep_cache);
            Fix.InstallProtect();
            (void)Fix.Resolve(true);
        }
        if (cache->dep_cache->BrokenCount() > 0) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Unmet dependencies");
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
    if (!cache || !package_names || count == 0 || !changes) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for multi-package simulation");
    }

    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
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
                // Resolve virtual name by scanning providers with installable candidate
                std::vector<pkgCache::PkgIterator> candidate_providers;
                for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); it.end() == false; ++it) {
                    pkgCache::VerIterator cand = (*cache->dep_cache)[it].CandidateVerIter(*cache->dep_cache);
                    if (cand.end()) continue;
                    for (pkgCache::PrvIterator prv = cand.ProvidesList(); prv.end() == false; ++prv) {
                        if (strcmp(prv.Name(), pkg_name.c_str()) == 0) {
                            candidate_providers.push_back(it);
                            break;
                        }
                    }
                }
                if (candidate_providers.empty()) {
                    return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package not found: ") + pkg_name).c_str());
                }
                if (candidate_providers.size() > 1) {
                    return make_result(APT_ERROR_DEPENDENCY_BROKEN,
                        (std::string("Virtual name '") + pkg_name + "' has multiple providers; specify exact package name").c_str());
                }
                pkg = candidate_providers.front();
            }

            // If no candidate but package provides others, try choose a suitable provider of this virtual
            pkgDepCache::StateCache &State = (*cache->dep_cache)[pkg];
            if (State.CandidateVer == 0 && pkg->ProvidesList != 0) {
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
                    return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                        (std::string("Virtual package ") + pkg_name + " has no installable providers").c_str());
                }
                if (GoodSolutions.size() > 1) {
                    return make_result(APT_ERROR_DEPENDENCY_BROKEN,
                        (std::string("Virtual package ") + pkg_name + " has multiple providers. Please select specific package.").c_str());
                }
                pkg = pkgCache::PkgIterator(*cache->dep_cache, GoodSolutions[0]);
            }

            cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, true);
        }

        // Try to resolve problems like apt-get does before declaring broken
        if (cache->dep_cache->BrokenCount() > 0) {
            pkgProblemResolver Fix(cache->dep_cache);
            Fix.InstallProtect();
            (void)Fix.Resolve(true);
        }
        if (cache->dep_cache->BrokenCount() > 0) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Unmet dependencies");
        }

        if (!check_apt_errors()) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, nullptr);
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

        return make_result(APT_SUCCESS, nullptr);

    } catch (const std::exception& e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Multi-package install simulation failed: ") + e.what()).c_str());
    }
}

AptResult apt_simulate_remove(AptCache* cache, const char** package_names, size_t count, AptPackageChanges* changes) {
    if (!cache || !package_names || count == 0 || !changes) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for multi-package simulation");
    }

    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
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
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package not found: ") + pkg_name).c_str());
            }

            // If target is a truly virtual name (no versions at all), find installed provider
            if (pkg.VersionList().end()) {
                std::vector<pkgCache::PkgIterator> InstalledProviders;
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
                    return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                        (std::string("Package ") + pkg_name + " has no installed providers").c_str());
                }
                if (InstalledProviders.size() > 1) {
                    return make_result(APT_ERROR_DEPENDENCY_BROKEN,
                        (std::string("Virtual package ") + pkg_name +
                         " has multiple installed providers: " + providersList +
                         ". Please remove specific package.").c_str());
                }
                pkg = InstalledProviders.front();
            }

            if (pkg.CurrentVer().end()) {
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package is not installed: ") + pkg_name).c_str());
            }

            cache->dep_cache->MarkDelete(pkg, true); // true = purge
        }

        // Try to resolve problems like apt-get does before declaring broken
        if (cache->dep_cache->BrokenCount() > 0) {
            pkgProblemResolver Fix(cache->dep_cache);
            Fix.InstallProtect();
            (void)Fix.Resolve(true);
        }
        if (cache->dep_cache->BrokenCount() > 0) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Unmet dependencies");
        }

        if (!check_apt_errors()) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, nullptr);
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
        changes->extra_installed_count = 0;
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

        // Restore package states (undo the marking for simulation)
        for (size_t i = 0; i < count; i++) {
            if (!package_names[i]) continue;
            pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(package_names[i]);
            if (!pkg.end()) {
                cache->dep_cache->MarkKeep(pkg, false);
            }
        }

        return make_result(APT_SUCCESS, nullptr);

    } catch (const std::exception& e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Multi-package remove simulation failed: ") + e.what()).c_str());
    }
}

AptResult apt_simulate_change(AptCache* cache,
                              const char** install_names, size_t install_count,
                              const char** remove_names, size_t remove_count,
                              bool purge,
                              AptPackageChanges* changes) {
    if (!cache || !changes) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid parameters for combined simulation");
    }
    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Cache file not available");
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        std::set<std::string> requested_install;
        std::set<std::string> requested_remove;

        // Mark installs
        if (install_names && install_count > 0) {
            for (size_t i = 0; i < install_count; i++) {
                if (!install_names[i]) continue;
                std::string name(install_names[i]);
                requested_install.insert(name);
                pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name.c_str());
                if (pkg.end()) {
                    return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package not found: ") + name).c_str());
                }
                cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, true);
            }
        }

        // Mark removals
        if (remove_names && remove_count > 0) {
            for (size_t i = 0; i < remove_count; i++) {
                if (!remove_names[i]) continue;
                std::string name(remove_names[i]);
                requested_remove.insert(name);
                pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name.c_str());
                if (pkg.end()) {
                    return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package not found: ") + name).c_str());
                }

                // Handle truly virtual package (no versions) by resolving installed provider
                if (pkg.VersionList().end()) {
                    std::vector<pkgCache::PkgIterator> InstalledProviders;
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
                        return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                            (std::string("Package ") + name + " has no installed providers").c_str());
                    }
                    if (InstalledProviders.size() > 1) {
                        return make_result(APT_ERROR_DEPENDENCY_BROKEN,
                            (std::string("Virtual package ") + name +
                             " has multiple installed providers: " + providersList +
                             ". Please remove specific package.").c_str());
                    }
                    pkg = InstalledProviders.front();
                }

                if (pkg.CurrentVer().end()) {
                    return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package is not installed: ") + name).c_str());
                }
                cache->dep_cache->MarkDelete(pkg, purge);
            }
        }

        // Try to resolve problems like apt-get does before declaring broken
        if (cache->dep_cache->BrokenCount() > 0) {
            pkgProblemResolver Fix(cache->dep_cache);
            Fix.InstallProtect();
            (void)Fix.Resolve(true);
        }
        if (cache->dep_cache->BrokenCount() > 0) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Unmet dependencies");
        }

        if (!check_apt_errors()) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, nullptr);
        }

        std::vector<std::string> extra_installed;
        std::vector<std::string> extra_removed;
        std::vector<std::string> upgraded;
        std::vector<std::string> new_installed;
        std::vector<std::string> removed;

        uint64_t download_size = 0;
        uint64_t install_size = 0;

        for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
            pkgDepCache::StateCache& st = (*cache->dep_cache)[iter];
            if (st.NewInstall()) {
                new_installed.push_back(iter.Name());
                if (requested_install.find(iter.Name()) == requested_install.end()) {
                    extra_installed.push_back(iter.Name());
                }
                if (st.CandidateVer != 0) {
                    download_size += st.CandidateVer->Size;
                    install_size += st.CandidateVer->InstalledSize;
                }
            } else if (st.Upgrade()) {
                upgraded.push_back(iter.Name());
                if (st.CandidateVer != 0) {
                    download_size += st.CandidateVer->Size;
                    install_size += st.CandidateVer->InstalledSize;
                    if (st.InstallVer != 0) install_size -= st.InstallVer->InstalledSize;
                }
            } else if (st.Delete()) {
                removed.push_back(iter.Name());
                if (requested_remove.find(iter.Name()) == requested_remove.end()) {
                    extra_removed.push_back(iter.Name());
                }
                if (st.InstallVer != 0) install_size -= st.InstallVer->InstalledSize;
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
            for (size_t i = 0; i < changes->extra_installed_count; ++i) changes->extra_installed[i] = strdup(extra_installed[i].c_str());
        }
        if (changes->removed_count > 0) {
            changes->removed_packages = (char**)malloc(changes->removed_count * sizeof(char*));
            for (size_t i = 0; i < changes->removed_count; ++i) changes->removed_packages[i] = strdup(removed[i].c_str());
        }
        if (changes->upgraded_count > 0) {
            changes->upgraded_packages = (char**)malloc(changes->upgraded_count * sizeof(char*));
            for (size_t i = 0; i < changes->upgraded_count; ++i) changes->upgraded_packages[i] = strdup(upgraded[i].c_str());
        }
        if (changes->new_installed_count > 0) {
            changes->new_installed_packages = (char**)malloc(changes->new_installed_count * sizeof(char*));
            for (size_t i = 0; i < changes->new_installed_count; ++i) changes->new_installed_packages[i] = strdup(new_installed[i].c_str());
        }

        // Restore marks
        if (install_names && install_count > 0) {
            for (size_t i = 0; i < install_count; i++) {
                if (!install_names[i]) continue;
                pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(install_names[i]);
                if (!pkg.end()) cache->dep_cache->MarkKeep(pkg, false);
            }
        }
        if (remove_names && remove_count > 0) {
            for (size_t i = 0; i < remove_count; i++) {
                if (!remove_names[i]) continue;
                pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(remove_names[i]);
                if (!pkg.end()) cache->dep_cache->MarkKeep(pkg, false);
            }
        }

        return make_result(APT_SUCCESS, nullptr);

    } catch (const std::exception& e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Combined simulation failed: ") + e.what()).c_str());
    }
}



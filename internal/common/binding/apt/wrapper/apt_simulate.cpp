#include "apt_internal.h"

// Verbatim copies of simulation functions from apt_wrapper.cpp

// For pkgVersioningSystem (VS().CheckDep)
#include <apt-pkg/version.h>


struct RequirementSpec {
    std::string name;
    bool has_version {false};
    int op {0};
    std::string version;
};

static RequirementSpec parse_requirement(const std::string& raw) {
    RequirementSpec r;
    // Support operators: ">=", "<=", "!=", "=", "<", ">"
    size_t pos = std::string::npos;
    int op = 0;
    if ((pos = raw.find("<=")) != std::string::npos) { op = pkgCache::Dep::LessEq; }
    else if ((pos = raw.find(">=")) != std::string::npos) { op = pkgCache::Dep::GreaterEq; }
    else if ((pos = raw.find("!=")) != std::string::npos) { op = pkgCache::Dep::NotEquals; }
    else if ((pos = raw.find('=')) != std::string::npos) { op = pkgCache::Dep::Equals; }
    else if ((pos = raw.find('<')) != std::string::npos) { op = pkgCache::Dep::Less; }
    else if ((pos = raw.find('>')) != std::string::npos) { op = pkgCache::Dep::Greater; }

    if (pos == std::string::npos) {
        r.name = raw;
        return r;
    }

    r.name = raw.substr(0, pos);
    size_t ver_start = pos;
    if (op == pkgCache::Dep::LessEq || op == pkgCache::Dep::GreaterEq || op == pkgCache::Dep::NotEquals) ver_start += 2;
    else ver_start += 1;
    while (ver_start < raw.size() && isspace(static_cast<unsigned char>(raw[ver_start]))) ver_start++;
    r.version = raw.substr(ver_start);
    while (!r.version.empty() && isspace(static_cast<unsigned char>(r.version.back()))) r.version.pop_back();
    r.has_version = !r.version.empty();
    r.op = op;
    return r;
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
    return apt_simulate_change(cache, package_names, count, nullptr, 0, false, changes);
}

AptResult apt_simulate_remove(AptCache* cache, const char** package_names, size_t count, bool purge, AptPackageChanges* changes) {
    // Delegate to unified change simulator
    return apt_simulate_change(cache, nullptr, 0, package_names, count, purge, changes);
}

AptResult plan_change_internal(
     AptCache* cache,
     const char** install_names, size_t install_count,
     const char** remove_names, size_t remove_count,
     bool purge,
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

         pkgProblemResolver Fix(cache->dep_cache);

         // Mark installs with provider scoring and version constraints
         if (install_names && install_count > 0) {
             for (size_t i = 0; i < install_count; i++) {
                 if (!install_names[i]) continue;
                 std::string raw(install_names[i]);
                 RequirementSpec req = parse_requirement(raw);
                 requested_install.insert(req.name);
                 pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(req.name.c_str());
                 if (pkg.end()) {
                     std::vector<pkgCache::Package*> provider_pkgs;
                     for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
                         for (int pass = 0; pass < 2; ++pass) {
                             pkgCache::VerIterator ver = (pass == 0)
                                 ? (*cache->dep_cache)[iter].CandidateVerIter(*cache->dep_cache)
                                 : iter.CurrentVer();
                             if (ver.end()) continue;
                             for (pkgCache::PrvIterator prv = ver.ProvidesList(); !prv.end(); ++prv) {
                                 if (strcmp(prv.Name(), req.name.c_str()) == 0) {
                                     if (req.has_version) {
                                         const char* pv = prv.ProvideVersion();
                                         if (pv == nullptr) continue;
                                         if (cache->dep_cache->VS().CheckDep(pv, req.op, req.version.c_str()) == false) continue;
                                     }
                                     provider_pkgs.push_back(iter);
                                     break;
                                 }
                             }
                         }
                     }
                     if (provider_pkgs.empty()) {
                         return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package not found: ") + raw).c_str());
                     }
                     Fix.MakeScores();
                     qsort(provider_pkgs.data(), provider_pkgs.size(), sizeof(provider_pkgs[0]), &(Fix.ScoreSort));
                     pkg = pkgCache::PkgIterator(*cache->dep_cache, provider_pkgs.front());
                 }

                 // If we resolved the name to a virtual package entry, pick a concrete provider
                 pkgDepCache::StateCache &State = (*cache->dep_cache)[pkg];
                 if (State.CandidateVer == 0 && pkg->ProvidesList != 0) {
                     std::vector<pkgCache::Package*> provider_pkgs;
                     for (pkgCache::PrvIterator Prv = pkg.ProvidesList(); !Prv.end(); ++Prv) {
                         if (req.has_version) {
                             const char* prvVer = Prv.ProvideVersion();
                             if (prvVer == nullptr) continue;
                             if (cache->dep_cache->VS().CheckDep(prvVer, req.op, req.version.c_str()) == false) continue;
                         }
                         provider_pkgs.push_back(Prv.OwnerPkg());
                     }
                     if (provider_pkgs.empty()) {
                         return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                             (std::string("Virtual package ") + req.name + " has no installable providers").c_str());
                     }
                     Fix.MakeScores();
                     qsort(provider_pkgs.data(), provider_pkgs.size(), sizeof(provider_pkgs[0]), &(Fix.ScoreSort));
                     pkg = pkgCache::PkgIterator(*cache->dep_cache, provider_pkgs.front());
                 }

                 cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, true);
             }
         }

         // Mark removals - now with automatic dependency removal like apt-get
         if (remove_names && remove_count > 0) {
             for (size_t i = 0; i < remove_count; i++) {
                 if (!remove_names[i]) continue;
                 std::string raw(remove_names[i]);
                 RequirementSpec req = parse_requirement(raw);
                 requested_remove.insert(req.name);
                 pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(req.name.c_str());

                 if (pkg.end()) {
                     // Try to find virtual package providers
                     std::vector<pkgCache::PkgIterator> candidate_providers;
                     for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
                         pkgCache::VerIterator current = iter.CurrentVer();
                         if (current.end()) continue;
                         for (pkgCache::PrvIterator prv = current.ProvidesList(); !prv.end(); ++prv) {
                             if (strcmp(prv.Name(), req.name.c_str()) == 0) {
                                 if (req.has_version) {
                                     const char* pv = prv.ProvideVersion();
                                     if (pv == nullptr) continue;
                                     if (cache->dep_cache->VS().CheckDep(pv, req.op, req.version.c_str()) == false) continue;
                                 }
                                 candidate_providers.push_back(iter);
                                 break;
                             }
                         }
                     }
                     if (candidate_providers.empty()) {
                         return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package ") + raw + " is not installed, so not removed").c_str());
                     }
                     if (candidate_providers.size() > 1) {
                         std::string providersList;
                         for (const auto& provider : candidate_providers) {
                             if (!providersList.empty()) providersList += ", ";
                             providersList += provider.Name();
                         }
                         return make_result(APT_ERROR_DEPENDENCY_BROKEN,
                             (std::string("Virtual package ") + raw +
                              " has multiple installed providers: " + providersList +
                              ". Please remove specific package.").c_str());
                     }
                     pkg = candidate_providers[0];
                 }

                 // Check if package is installed
                 if (pkg.CurrentVer().end()) {
                     return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package ") + raw + " is not installed, so not removed").c_str());
                 }

                 // Check if package is essential - still block this
                 if ((pkg->Flags & pkgCache::Flag::Essential) != 0) {
                     return make_result(APT_ERROR_OPERATION_INCOMPLETE,
                         (std::string("Cannot remove essential package: ") + pkg.Name()).c_str());
                 }

                 // Mark for deletion with autofix enabled to remove dependent packages
                 cache->dep_cache->MarkDelete(pkg, purge);

                 // Now mark all packages that depend on this one for removal too
                 // This mimics apt-get behavior
                 for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
                     if (it == pkg) continue; // Skip the package itself

                     pkgCache::VerIterator cur = it.CurrentVer();
                     if (cur.end()) continue; // Skip not installed packages

                     bool depends_on_removed = false;
                     for (pkgCache::DepIterator dep = cur.DependsList(); !dep.end(); ++dep) {
                         // Check only hard dependencies
                         if (dep->Type != pkgCache::Dep::Depends &&
                             dep->Type != pkgCache::Dep::PreDepends) continue;

                         // Check if this dependency points to our package
                         if (dep.TargetPkg() == pkg) {
                             depends_on_removed = true;
                             break;
                         }

                         // Also check if it depends on a virtual package that our package provides
                         for (pkgCache::PrvIterator prv = pkg.CurrentVer().ProvidesList(); !prv.end(); ++prv) {
                             if (strcmp(prv.Name(), dep.TargetPkg().Name()) == 0) {
                                 depends_on_removed = true;
                                 break;
                             }
                         }
                         if (depends_on_removed) break;
                     }

                     if (depends_on_removed) {
                         // Skip essential packages
                         if ((it->Flags & pkgCache::Flag::Essential) == 0) {
                             cache->dep_cache->MarkDelete(it, purge);
                         }
                     }
                 }
             }
         }

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

         // Collect changes
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
                     // This package is being removed as a dependency
                     extra_removed.push_back(iter.Name());
                 }
                 if (st.InstallVer != 0) install_size -= st.InstallVer->InstalledSize;
             }
         }

         // Fill the changes structure
         changes->extra_installed_count = extra_installed.size();
         changes->upgraded_count = upgraded.size();
         changes->new_installed_count = new_installed.size();
         changes->removed_count = removed.size();
         changes->not_upgraded_count = 0;
         changes->download_size = download_size;
         changes->install_size = install_size;

         if (changes->extra_installed_count > 0) {
             changes->extra_installed = (char**)malloc(changes->extra_installed_count * sizeof(char*));
             for (size_t i = 0; i < changes->extra_installed_count; ++i)
                 changes->extra_installed[i] = strdup(extra_installed[i].c_str());
         }
         if (changes->removed_count > 0) {
             changes->removed_packages = (char**)malloc(changes->removed_count * sizeof(char*));
             for (size_t i = 0; i < changes->removed_count; ++i)
                 changes->removed_packages[i] = strdup(removed[i].c_str());
         }
         if (changes->upgraded_count > 0) {
             changes->upgraded_packages = (char**)malloc(changes->upgraded_count * sizeof(char*));
             for (size_t i = 0; i < changes->upgraded_count; ++i)
                 changes->upgraded_packages[i] = strdup(upgraded[i].c_str());
         }
         if (changes->new_installed_count > 0) {
             changes->new_installed_packages = (char**)malloc(changes->new_installed_count * sizeof(char*));
             for (size_t i = 0; i < changes->new_installed_count; ++i)
                 changes->new_installed_packages[i] = strdup(new_installed[i].c_str());
         }

         // Restore marks if this is only a simulation
         if (!apply) {
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
                              AptPackageChanges* changes) {
    return plan_change_internal(cache, install_names, install_count, remove_names, remove_count, purge, false, changes);
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



#include "apt_package_operations.h"
#include "apt_internal.h"
#include <apt-pkg/version.h>

RequirementSpec parse_requirement(const std::string& raw) {
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

static AptResult find_install_package(AptCache* cache, const RequirementSpec& req, pkgCache::PkgIterator& result_pkg) {
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
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package not found: ") + req.name).c_str());
        }
        pkgProblemResolver Fix(cache->dep_cache);
        Fix.MakeScores();
        qsort(provider_pkgs.data(), provider_pkgs.size(), sizeof(provider_pkgs[0]), &(Fix.ScoreSort));
        pkg = pkgCache::PkgIterator(*cache->dep_cache, provider_pkgs.front());
    }

    result_pkg = pkg;
    return make_result(APT_SUCCESS, nullptr);
}

static AptResult resolve_virtual_package(AptCache* cache, const RequirementSpec& req, pkgCache::PkgIterator& pkg) {
    pkgDepCache::StateCache &State = (*cache->dep_cache)[pkg];
    if (State.CandidateVer == 0 && pkg->ProvidesList != 0) {
        std::vector<std::pair<std::string, std::string>> provider_info;
        std::vector<pkgCache::Package*> provider_pkgs;

        for (pkgCache::PrvIterator Prv = pkg.ProvidesList(); !Prv.end(); ++Prv) {
            pkgCache::PkgIterator prov = Prv.OwnerPkg();
            pkgCache::VerIterator prov_ver = (*cache->dep_cache)[prov].CandidateVerIter(*cache->dep_cache);

            if (!prov_ver.end()) {
                if (req.has_version) {
                    const char* prvVer = Prv.ProvideVersion();
                    if (prvVer == nullptr) continue;
                    if (cache->dep_cache->VS().CheckDep(prvVer, req.op, req.version.c_str()) == false) continue;
                }
                provider_pkgs.push_back(prov);

                std::string status = prov.CurrentVer().end() ? "" : "[Installed]";
                provider_info.push_back(std::make_pair(prov.Name(), status));
            }
        }

        if (provider_pkgs.empty()) {
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                (std::string("Virtual package ") + req.name + " has no installable providers").c_str());
        }

        if (provider_pkgs.size() > 1) {
            std::string msg = "Virtual package " + req.name + " is provided by:\n";
            for (const auto& info : provider_info) {
                msg += "  " + info.first + " " + info.second + "\n";
            }
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND, msg.c_str());
        }

        pkg = pkgCache::PkgIterator(*cache->dep_cache, provider_pkgs.front());
    }
    return make_result(APT_SUCCESS, nullptr);
}

AptResult process_package_installs(AptCache* cache, 
                                   const char** install_names, 
                                   size_t install_count,
                                   std::set<std::string>& requested_install) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }
    if (!install_names || install_count == 0) {
        return make_result(APT_SUCCESS, nullptr);
    }

    for (size_t i = 0; i < install_count; i++) {
        if (!install_names[i]) continue;
        
        std::string raw(install_names[i]);
        RequirementSpec req = parse_requirement(raw);
        requested_install.insert(req.name);
        
        pkgCache::PkgIterator pkg;
        AptResult result = find_install_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = resolve_virtual_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, true);
    }

    return make_result(APT_SUCCESS, nullptr);
}

static AptResult find_remove_package(AptCache* cache, const RequirementSpec& req, pkgCache::PkgIterator& result_pkg) {
    pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(req.name.c_str());

    if (pkg.end()) {
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
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package ") + req.name + " is not installed, so not removed").c_str());
        }
        if (candidate_providers.size() > 1) {
            std::string providersList;
            for (const auto& provider : candidate_providers) {
                if (!providersList.empty()) providersList += ", ";
                providersList += provider.Name();
            }
            return make_result(APT_ERROR_DEPENDENCY_BROKEN,
                (std::string("Virtual package ") + req.name +
                 " has multiple installed providers: " + providersList +
                 ". Please remove specific package.").c_str());
        }
        pkg = candidate_providers[0];
    }

    result_pkg = pkg;
    return make_result(APT_SUCCESS, nullptr);
}

static AptResult resolve_virtual_remove_package(AptCache* cache, const RequirementSpec& req, pkgCache::PkgIterator& pkg) {
    if (pkg.CurrentVer().end()) {
        std::vector<pkgCache::PkgIterator> installed_providers;
        std::string providersList;
        for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
            pkgCache::VerIterator current = iter.CurrentVer();
            if (current.end()) continue;
            for (pkgCache::PrvIterator prv = current.ProvidesList(); !prv.end(); ++prv) {
                if (strcmp(prv.Name(), req.name.c_str()) != 0) continue;
                if (req.has_version) {
                    const char* pv = prv.ProvideVersion();
                    if (pv == nullptr) continue;
                    if (cache->dep_cache->VS().CheckDep(pv, req.op, req.version.c_str()) == false) continue;
                }
                installed_providers.push_back(iter);
                if (!providersList.empty()) providersList += ", ";
                providersList += iter.Name();
                break;
            }
        }
        if (installed_providers.empty()) {
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package ") + req.name + " is not installed, so not removed").c_str());
        }
        if (installed_providers.size() > 1) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN,
                (std::string("Virtual package ") + req.name +
                 " has multiple installed providers: " + providersList +
                 ". Please remove specific package.").c_str());
        }
        pkg = installed_providers.front();
    }
    return make_result(APT_SUCCESS, nullptr);
}

static void mark_dependent_packages_for_removal(AptCache* cache, pkgCache::PkgIterator initial_pkg, bool purge) {
    std::set<pkgCache::PkgIterator> to_remove;
    std::set<pkgCache::PkgIterator> processed;
    to_remove.insert(initial_pkg);

    while (!to_remove.empty()) {
        pkgCache::PkgIterator current = *to_remove.begin();
        to_remove.erase(to_remove.begin());
        
        if (processed.find(current) != processed.end()) continue;
        processed.insert(current);

        for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
            if (processed.find(it) != processed.end()) continue;

            pkgCache::VerIterator cur = it.CurrentVer();
            if (cur.end()) continue;

            pkgDepCache::StateCache &it_st = (*cache->dep_cache)[it];
            if (it_st.Delete()) continue;

            bool depends_on_current = false;
            for (pkgCache::DepIterator dep = cur.DependsList(); !dep.end(); ++dep) {
                if (dep->Type != pkgCache::Dep::Depends && 
                    dep->Type != pkgCache::Dep::PreDepends) continue;

                if (dep.TargetPkg() == current) {
                    depends_on_current = true;
                    break;
                }

                pkgCache::VerIterator cur_ver = current.CurrentVer();
                if (!cur_ver.end()) {
                    for (pkgCache::PrvIterator prv = cur_ver.ProvidesList(); !prv.end(); ++prv) {
                        if (strcmp(prv.Name(), dep.TargetPkg().Name()) == 0) {
                            depends_on_current = true;
                            break;
                        }
                    }
                }
                if (depends_on_current) break;
            }

            if (depends_on_current) {
                if ((it->Flags & pkgCache::Flag::Essential) == 0) {
                    cache->dep_cache->MarkDelete(it, purge);
                    to_remove.insert(it);
                }
            }
        }
    }
}

AptResult process_package_removals(AptCache* cache,
                                   const char** remove_names,
                                   size_t remove_count,
                                   bool purge,
                                   std::set<std::string>& requested_remove,
                                   std::vector<std::pair<std::string, pkgCache::PkgIterator>>& remove_targets) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }
    if (!remove_names || remove_count == 0) {
        return make_result(APT_SUCCESS, nullptr);
    }

    for (size_t i = 0; i < remove_count; i++) {
        if (!remove_names[i]) continue;
        
        std::string raw(remove_names[i]);
        RequirementSpec req = parse_requirement(raw);
        requested_remove.insert(req.name);
        
        pkgCache::PkgIterator pkg;
        AptResult result = find_remove_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = resolve_virtual_remove_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        if ((pkg->Flags & pkgCache::Flag::Essential) != 0) {
            return make_result(APT_ERROR_OPERATION_INCOMPLETE,
                (std::string("Cannot remove essential package: ") + pkg.Name()).c_str());
        }

        cache->dep_cache->MarkDelete(pkg, purge);
        remove_targets.emplace_back(req.name, pkg);

        mark_dependent_packages_for_removal(cache, pkg, purge);
    }

    return make_result(APT_SUCCESS, nullptr);
}

AptResult resolve_dependencies(AptCache* cache) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }

    // Check if we have broken packages that need resolution
    if (cache->dep_cache->BrokenCount() > 0) {
        // Create resolver after packages are marked
        pkgProblemResolver Fix(cache->dep_cache);
        Fix.InstallProtect();
        
        // Let the resolver try to fix the problems
        if (!Fix.Resolve(true)) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Failed to resolve dependencies");
        }
    }
    
    // Final check for any remaining broken packages
    if (cache->dep_cache->BrokenCount() != 0) {
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

    return make_result(APT_SUCCESS, nullptr);
}

void collect_package_changes(AptCache* cache,
                            const std::set<std::string>& requested_install,
                            const std::set<std::string>& requested_remove,
                            std::vector<std::string>& extra_installed,
                            std::vector<std::string>& extra_removed,
                            std::vector<std::string>& upgraded,
                            std::vector<std::string>& new_installed,
                            std::vector<std::string>& removed,
                            uint64_t& download_size,
                            uint64_t& install_size) {
    
    download_size = 0;
    install_size = 0;

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
}

AptResult validate_install_requests(AptCache* cache, const std::set<std::string>& requested_install,
                                    const std::vector<std::string>& new_installed,
                                    const std::vector<std::string>& upgraded) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }

    // If we requested packages to install but nothing was marked for installation or upgrade,
    // check if all requested packages are already installed at the correct version
    if (!requested_install.empty() && new_installed.empty() && upgraded.empty()) {
        
        for (const auto &name : requested_install) {
            pkgCache::PkgIterator p = cache->dep_cache->FindPkg(name.c_str());
            if (p.end()) {
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND, 
                    (std::string("Package not found: ") + name).c_str());
            }
            
            // Check if package is directly installed
            if (!p.CurrentVer().end()) {
                // Package is directly installed
                continue;
            }
            
            // Check if this is a virtual package with installed providers
            bool has_installed_provider = false;
            if (p->ProvidesList != 0) {
                for (pkgCache::PrvIterator prv = p.ProvidesList(); !prv.end(); ++prv) {
                    pkgCache::PkgIterator prov = prv.OwnerPkg();
                    if (!prov.CurrentVer().end()) {
                        // Found an installed provider
                        has_installed_provider = true;
                        break;
                    }
                }
            }
            
            if (!has_installed_provider) {
                // Neither the package itself nor any of its providers are installed
                // Check if the package exists but has broken dependencies
                pkgDepCache::StateCache &State = (*cache->dep_cache)[p];
                if (State.CandidateVer != 0) {
                    // Package exists but wasn't installed - check dependencies
                    bool has_broken_deps = false;
                    pkgCache::VerIterator ver = State.CandidateVerIter(*cache->dep_cache);
                    if (!ver.end()) {
                        for (pkgCache::DepIterator dep = ver.DependsList(); !dep.end(); ++dep) {
                            if (dep->Type != pkgCache::Dep::Depends && dep->Type != pkgCache::Dep::PreDepends) {
                                continue;
                            }
                            
                            // Check if this dependency is satisfied
                            bool dep_satisfied = false;
                            pkgCache::PkgIterator target_pkg = dep.TargetPkg();
                            if (!target_pkg.end()) {
                                // Check if target package is installed or installable
                                if (!target_pkg.CurrentVer().end()) {
                                    dep_satisfied = true;
                                } else {
                                    pkgDepCache::StateCache &target_state = (*cache->dep_cache)[target_pkg];
                                    if (target_state.CandidateVer != 0) {
                                        dep_satisfied = true;
                                    }
                                }
                            }
                            
                            if (!dep_satisfied) {
                                has_broken_deps = true;
                                break;
                            }
                            
                            // Skip OR groups
                            while (!dep.end() && dep->CompareOp == pkgCache::Dep::Or) {
                                ++dep;
                            }
                        }
                    }
                    
                    if (has_broken_deps) {
                        return make_result(APT_ERROR_DEPENDENCY_BROKEN,
                            (std::string("Some broken packages were found while trying to process build-dependencies for ") + name).c_str());
                    } else {
                        // Package exists and dependencies look OK, but still wasn't marked
                        // This is unexpected
                        return make_result(APT_ERROR_DEPENDENCY_BROKEN,
                            (std::string("Package ") + name + " is not installed and was not marked for installation").c_str());
                    }
                } else {
                    // Package doesn't exist or is not installable
                    return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                        (std::string("Package ") + name + " is not available for installation").c_str());
                }
            }
        }
        
        // All requested packages are already installed - this is actually success
        return make_result(APT_SUCCESS, nullptr);
    }
    
    return make_result(APT_SUCCESS, nullptr);
}

AptResult validate_remove_requests(AptCache* cache,
                                   const std::vector<std::pair<std::string, pkgCache::PkgIterator>>& remove_targets,
                                   const std::vector<std::string>& removed) {
    if (!remove_targets.empty()) {
        std::vector<std::string> blocked_packages;
        std::set<std::string> blocking_packages;
        
        for (const auto &entry : remove_targets) {
            const std::string &rawName = entry.first;
            pkgCache::PkgIterator pkg = entry.second;
            if (pkg.end()) continue;
            
            pkgDepCache::StateCache &st = (*cache->dep_cache)[pkg];
            if (!st.Delete()) {
                blocked_packages.push_back(rawName);
                
                for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
                    if (it == pkg) continue;
                    
                    pkgDepCache::StateCache &it_st = (*cache->dep_cache)[it];
                    if (it_st.Delete()) continue;
                    
                    pkgCache::VerIterator cur = it.CurrentVer();
                    if (cur.end()) continue;
                    
                    for (pkgCache::DepIterator dep = cur.DependsList(); !dep.end(); ++dep) {
                        if (dep->Type != pkgCache::Dep::Depends && 
                            dep->Type != pkgCache::Dep::PreDepends) continue;
                        
                        if (dep.TargetPkg() == pkg) {
                            blocking_packages.insert(it.Name());
                            break;
                        }
                    }
                }
            }
        }
        
        if (!blocked_packages.empty() && removed.empty()) {
            std::string msg = "Cannot remove ";
            for (size_t i = 0; i < blocked_packages.size(); ++i) {
                if (i > 0) msg += ", ";
                msg += blocked_packages[i];
            }
            
            if (!blocking_packages.empty()) {
                msg += ". Try removing together: ";
                blocking_packages.insert(blocked_packages.begin(), blocked_packages.end());
                bool first = true;
                for (const auto &name : blocking_packages) {
                    if (!first) msg += " ";
                    first = false;
                    msg += name;
                }
            }
            
            return make_result(APT_ERROR_OPERATION_INCOMPLETE, msg.c_str());
        }
    }
    return make_result(APT_SUCCESS, nullptr);
}

void populate_changes_structure(AptPackageChanges* changes,
                                const std::vector<std::string>& extra_installed,
                                const std::vector<std::string>& upgraded,
                                const std::vector<std::string>& new_installed,
                                const std::vector<std::string>& removed,
                                uint64_t download_size,
                                uint64_t install_size) {
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
}

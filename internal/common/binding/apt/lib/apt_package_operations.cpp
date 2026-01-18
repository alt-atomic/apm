#include "apt_package_operations.h"

#include <apt-pkg/algorithms.h>
#include <apt-pkg/error.h>
#include <apt-pkg/version.h>

#include <cctype>
#include <cstring>
#include <sys/stat.h>

RequirementSpec parse_requirement(const std::string &raw) {
    RequirementSpec r;
    // Support operators: ">=", "<=", "!=", "=", "<", ">"
    size_t pos = std::string::npos;
    int op = 0;
    if ((pos = raw.find("<=")) != std::string::npos) { op = pkgCache::Dep::LessEq; } else if (
        (pos = raw.find(">=")) != std::string::npos) { op = pkgCache::Dep::GreaterEq; } else if (
        (pos = raw.find("!=")) != std::string::npos) { op = pkgCache::Dep::NotEquals; } else if (
        (pos = raw.find('=')) != std::string::npos) { op = pkgCache::Dep::Equals; } else if (
        (pos = raw.find('<')) != std::string::npos) { op = pkgCache::Dep::Less; } else if (
        (pos = raw.find('>')) != std::string::npos) { op = pkgCache::Dep::Greater; }

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

static AptResult find_install_package(AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &result_pkg) {
    pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(req.name.c_str());

    if (pkg.end()) {
        std::vector<pkgCache::Package *> provider_pkgs;
        for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
            for (int pass = 0; pass < 2; ++pass) {
                pkgCache::VerIterator ver = (pass == 0)
                                                ? (*cache->dep_cache)[iter].CandidateVerIter(*cache->dep_cache)
                                                : iter.CurrentVer();
                if (ver.end()) continue;
                for (pkgCache::PrvIterator prv = ver.ProvidesList(); !prv.end(); ++prv) {
                    if (strcmp(prv.Name(), req.name.c_str()) == 0) {
                        if (req.has_version) {
                            const char *pv = prv.ProvideVersion();
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

static AptResult resolve_virtual_package(AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &pkg) {
    pkgDepCache::StateCache &State = (*cache->dep_cache)[pkg];

    if (pkg->VersionList == 0 && pkg->ProvidesList != 0) {
        std::vector<pkgCache::Package *> GoodSolutions;
        unsigned long Size = 0;

        // Count providers
        for (pkgCache::PrvIterator Prv = pkg.ProvidesList(); !Prv.end(); ++Prv) {
            Size++;
        }

        // Create array for sorting
        std::vector<pkgCache::Package *> PList(Size);
        pkgCache::Package **PEnd = PList.data();
        for (pkgCache::PrvIterator Prv = pkg.ProvidesList(); !Prv.end(); ++Prv) {
            *PEnd++ = Prv.OwnerPkg();
        }

        pkgProblemResolver TempFix(cache->dep_cache);
        TempFix.MakeScores();
        qsort(PList.data(), PList.size(), sizeof(PList[0]), &(TempFix.ScoreSort));

        bool instVirtual = _config->FindB("APT::Install::Virtual", false);

        for (unsigned int p = 0; p < Size; ++p) {
            pkgCache::PkgIterator PrvPkg = pkgCache::PkgIterator(*pkg.Cache(), PList[p]);
            pkgCache::PrvIterator Prv = pkg.ProvidesList();

            // Find the provide that matches this package
            for (; !Prv.end() && Prv.OwnerPkg() != PrvPkg; ++Prv) {
            }
            if (Prv.end()) continue;

            bool AlreadySeen = false;
            for (unsigned int i = 0; i != GoodSolutions.size(); i++) {
                pkgCache::PkgIterator GoodPkg(*pkg.Cache(), GoodSolutions[i]);
                if (PrvPkg == GoodPkg) {
                    AlreadySeen = true;
                    break;
                }
            }
            if (AlreadySeen) continue;

            if (PrvPkg.CurrentVer() == Prv.OwnerVer()) {
                GoodSolutions.push_back(PrvPkg);
                if (instVirtual) break;
                continue;
            }

            pkgCache::VerIterator PrvPkgCandVer = (*cache->dep_cache)[PrvPkg].CandidateVerIter(*cache->dep_cache);
            if (PrvPkgCandVer.end()) {
                continue;
            }

            if (req.has_version) {
                const char *prvVer = Prv.ProvideVersion();
                if (prvVer == nullptr) continue;
                if (cache->dep_cache->VS().CheckDep(prvVer, req.op, req.version.c_str()) == false) continue;
            }

            // Check if provides points to any candidate version
            bool good = false;
            for (; PrvPkgCandVer.end() == false; ++PrvPkgCandVer) {
                if (PrvPkgCandVer == Prv.OwnerVer()) {
                    good = true;
                    GoodSolutions.push_back(PrvPkg);
                }
            }

            if (good && instVirtual) break;
        }

        if (GoodSolutions.size() == 1) {
            pkg = pkgCache::PkgIterator(*pkg.Cache(), GoodSolutions[0]);
        } else if (GoodSolutions.empty()) {
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                               (std::string("Virtual package ") + req.name + " has no installable providers").c_str());
        } else {
            std::string msg = "Virtual package " + req.name + " is provided by:\n";
            for (unsigned int i = 0; i != GoodSolutions.size(); i++) {
                pkgCache::PkgIterator GoodPkg(*pkg.Cache(), GoodSolutions[i]);
                std::string status = GoodPkg.CurrentVer().end() ? "" : " [Installed]";
                pkgCache::VerIterator CandVer = (*cache->dep_cache)[GoodPkg].CandidateVerIter(*cache->dep_cache);
                std::string version = CandVer.end() ? "" : CandVer.VerStr();
                msg += "  " + std::string(GoodPkg.Name()) + " " + version + status + "\n";
            }
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND, msg.c_str());
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

AptResult process_package_installs(AptCache *cache,
                                   const char **install_names,
                                   size_t install_count,
                                   std::set<std::string> &requested_install) {
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

        pkgCache::PkgIterator pkg;
        AptResult result = find_install_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = resolve_virtual_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        // Store the actual package name (not the file path or requirement string)
        requested_install.insert(pkg.Name());

        cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, false);

        // Install it with autoinstalling enabled if broken
        pkgDepCache::StateCache &State = (*cache->dep_cache)[pkg];
        if (State.InstBroken() == true) {
            cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::DontChange, true);
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

AptResult process_package_reinstalls(AptCache *cache,
                                     const char **reinstall_names,
                                     size_t reinstall_count,
                                     std::set<std::string> &requested_reinstall) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }
    if (!reinstall_names || reinstall_count == 0) {
        return make_result(APT_SUCCESS, nullptr);
    }

    for (size_t i = 0; i < reinstall_count; i++) {
        if (!reinstall_names[i]) continue;

        std::string raw(reinstall_names[i]);
        RequirementSpec req = parse_requirement(raw);

        pkgCache::PkgIterator pkg;

        if (is_rpm_file(raw)) {
            std::string pkg_name;
            bool found = false;

            // Scan cache to find package with matching RPM file
            for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
                for (pkgCache::VerIterator ver = iter.VersionList(); !ver.end(); ++ver) {
                    // Check if this version came from our RPM file
                    for (pkgCache::VerFileIterator vf = ver.FileList(); !vf.end(); ++vf) {
                        pkgCache::PkgFileIterator file = vf.File();
                        if (file.FileName() && raw.find(file.FileName()) != std::string::npos) {
                            pkg_name = iter.Name();
                            found = true;
                            goto found_reinstall_pkg;
                        }
                    }
                }
            }
        found_reinstall_pkg:

            if (!found) {
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                                   (std::string("Unable to find package from RPM file: ") + raw).c_str());
            }

            // Find the installed package by name
            pkg = cache->dep_cache->FindPkg(pkg_name.c_str());
            if (pkg.end()) {
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                                   (std::string("Package ") + pkg_name + " is not installed, so cannot be reinstalled").
                                   c_str());
            }
        } else {
            // For regular package names, use standard lookup
            AptResult result = find_install_package(cache, req, pkg);
            if (result.code != APT_SUCCESS) {
                return result;
            }
        }

        // Check if package is installed
        if (pkg->CurrentVer == 0) {
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                               (std::string("Package ") + pkg.Name() + " is not installed, so cannot be reinstalled").
                               c_str());
        }

        if (!is_rpm_file(raw)) {
            pkgCache::VerIterator currentVer = pkg.CurrentVer();
            if (!currentVer.Downloadable()) {
                return make_result(APT_ERROR_DOWNLOAD_FAILED,
                                   (std::string("Reinstallation of ") + pkg.Name() + " " +
                                    currentVer.VerStr() + " is not possible, it cannot be downloaded").c_str());
            }
        }

        requested_reinstall.insert(pkg.Name());
        cache->dep_cache->SetReInstall(pkg, true);
    }

    return make_result(APT_SUCCESS, nullptr);
}

static AptResult find_remove_package(AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &result_pkg) {
    pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(req.name.c_str());

    if (pkg.end()) {
        std::vector<pkgCache::PkgIterator> candidate_providers;
        for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
            pkgCache::VerIterator current = iter.CurrentVer();
            if (current.end()) continue;
            for (pkgCache::PrvIterator prv = current.ProvidesList(); !prv.end(); ++prv) {
                if (strcmp(prv.Name(), req.name.c_str()) == 0) {
                    if (req.has_version) {
                        const char *pv = prv.ProvideVersion();
                        if (pv == nullptr) continue;
                        if (cache->dep_cache->VS().CheckDep(pv, req.op, req.version.c_str()) == false) continue;
                    }
                    candidate_providers.push_back(iter);
                    break;
                }
            }
        }
        if (candidate_providers.empty()) {
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                               (std::string("Package ") + req.name + " is not installed, so not removed").c_str());
        }
        if (candidate_providers.size() > 1) {
            std::string providersList;
            for (const auto &provider: candidate_providers) {
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

static AptResult
resolve_virtual_remove_package(AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &pkg) {
    if (pkg.CurrentVer().end()) {
        std::vector<pkgCache::PkgIterator> installed_providers;
        std::string providersList;
        for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
            pkgCache::VerIterator current = iter.CurrentVer();
            if (current.end()) continue;
            for (pkgCache::PrvIterator prv = current.ProvidesList(); !prv.end(); ++prv) {
                if (strcmp(prv.Name(), req.name.c_str()) != 0) continue;
                if (req.has_version) {
                    const char *pv = prv.ProvideVersion();
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
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                               (std::string("Package ") + req.name + " is not installed, so not removed").c_str());
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

AptResult process_package_removals(AptCache *cache,
                                   const char **remove_names,
                                   size_t remove_count,
                                   bool purge,
                                   std::set<std::string> &requested_remove,
                                   std::vector<std::pair<std::string, pkgCache::PkgIterator> > &remove_targets) {
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

        // delete only one package
        // mark_dependent_packages_for_removal(cache, pkg, purge);
    }

    //    Save for history TODO удалить после тестирования комментарий
    //    std::set<pkgCache::PkgIterator> being_removed;
    //    for (const auto& target : remove_targets) {
    //        being_removed.insert(target.second);
    //    }
    //    mark_orphan_packages_for_removal(cache, being_removed, purge);

    return make_result(APT_SUCCESS, nullptr);
}

// Check for package conflicts ONLY between explicitly requested packages
AptResult check_package_conflicts(AptCache *cache, const std::set<std::string> &requested_install) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }

    if (requested_install.size() < 2) {
        return make_result(APT_SUCCESS, nullptr);
    }

    std::vector<pkgCache::PkgIterator> requested_packages;

    for (const auto &name: requested_install) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name.c_str());
        if (!pkg.end()) {
            requested_packages.push_back(pkg);
        }
    }

    for (size_t i = 0; i < requested_packages.size(); i++) {
        pkgCache::PkgIterator pkg1 = requested_packages[i];
        pkgDepCache::StateCache &state1 = (*cache->dep_cache)[pkg1];

        pkgCache::VerIterator ver1 = state1.CandidateVerIter(*cache->dep_cache);
        if (ver1.end()) continue;

        for (pkgCache::DepIterator dep = ver1.DependsList(); !dep.end(); ++dep) {
            if (dep->Type != pkgCache::Dep::Conflicts) continue;

            pkgCache::PkgIterator target_pkg = dep.TargetPkg();
            if (target_pkg.end()) continue;

            bool target_is_requested = false;
            pkgCache::VerIterator target_ver;

            for (size_t j = 0; j < requested_packages.size(); j++) {
                if (j != i && requested_packages[j] == target_pkg) {
                    target_is_requested = true;
                    // Get the candidate version of the target package
                    pkgDepCache::StateCache &target_state = (*cache->dep_cache)[target_pkg];
                    target_ver = target_state.CandidateVerIter(*cache->dep_cache);
                    break;
                }
            }

            if (target_is_requested && !target_ver.end()) {
                if (cache->dep_cache->VS().CheckDep(target_ver.VerStr(), dep)) {
                    std::string error_msg = "Conflicting packages: " + std::string(pkg1.Name()) + " and " + std::string(
                                                target_pkg.Name());

                    return make_result(APT_ERROR_DEPENDENCY_BROKEN, error_msg.c_str());
                }
            }
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

AptResult preprocess_installs(AptCache *cache, const std::set<std::string> &requested_install) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }

    for (const auto &name: requested_install) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name.c_str());
        if (pkg.end()) continue;

        pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[pkg];
        if (!pkg_state.Install()) continue;

        pkgCache::VerIterator InstVer = pkg_state.InstVerIter(*cache->dep_cache);
        if (InstVer.end()) continue;

        for (pkgCache::DepIterator D = InstVer.DependsList(); !D.end();) {
            pkgCache::DepIterator Start, End;
            D.GlobOr(Start, End);

            // We only worry about critical deps
            if (!End.IsCritical()) {
                continue;
            }

            // Skip if dep is already satisfied
            if (((*cache->dep_cache)[End] & pkgDepCache::DepGInstall) == pkgDepCache::DepGInstall) {
                continue;
            }

            // Try to satisfy this dependency by marking packages for install
            bool DepSatisfied = false;
            for (pkgCache::DepIterator CurDep = Start; !DepSatisfied;) {
                pkgCache::PkgIterator DepPkg = CurDep.SmartTargetPkg();
                if (!DepPkg.end()) {
                    pkgDepCache::StateCache &DepState = (*cache->dep_cache)[DepPkg];

                    if (((*cache->dep_cache)[CurDep] & pkgDepCache::DepCVer) == pkgDepCache::DepCVer) {
                        if (!DepState.Install() && DepState.CandidateVer != 0) {
                            cache->dep_cache->MarkInstall(DepPkg, pkgDepCache::AutoMarkFlag::Auto, true);
                        }
                        DepSatisfied = true;
                        break;
                    }
                }

                if (CurDep == End) break;
                ++CurDep;
            }
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

AptResult preprocess_removals(AptCache *cache, const std::set<std::string> &requested_remove) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }

    if (requested_remove.empty()) {
        return make_result(APT_SUCCESS, nullptr);
    }

    for (const auto &name: requested_remove) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name.c_str());
        if (pkg.end()) continue;

        pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[pkg];
        if (!pkg_state.Delete()) continue;

        if ((pkg->Flags & pkgCache::Flag::Essential) != 0) {
            return make_result(APT_ERROR_OPERATION_INCOMPLETE,
                               (std::string("Cannot remove essential package: ") + pkg.Name()).c_str());
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

AptResult finalize_dependency_resolution(AptCache *cache, const std::set<std::string> &requested_install,
                                         const std::set<std::string> &requested_remove, bool remove_depends) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }

    pkgProblemResolver Fix(cache->dep_cache);

    if (remove_depends || (_config && _config->FindB("APT::Remove-Depends", false))) {
        Fix.RemoveDepends();
    }

    for (const auto &name: requested_install) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name.c_str());
        if (pkg.end()) continue;

        pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[pkg];
        if (!pkg_state.Install()) continue;

        Fix.Clear(pkg);
        Fix.Protect(pkg);
    }

    for (const auto &name: requested_remove) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name.c_str());
        if (pkg.end()) continue;

        pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[pkg];
        if (!pkg_state.Delete()) continue;

        Fix.Clear(pkg);
        Fix.Protect(pkg);
        Fix.Remove(pkg);
    }

    Fix.InstallProtect();
    if (!Fix.Resolve(true)) {
        _error->Discard();
    }

    // Check if any requested packages were unmarked by the resolver
    // If Fix.Resolve() couldn't satisfy dependencies, it unmarked the package (MarkKeep)
    // This results in BrokenCount=0 but the package is not installed - we must detect this
    for (const auto &name: requested_install) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name.c_str());
        if (pkg.end()) continue;

        pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[pkg];

        if (!pkg_state.Install() && pkg.CurrentVer().end()) {
            cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, false);
        }
    }

    if (cache->dep_cache->BrokenCount() != 0) {
        std::string broken_info;
        for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
            pkgDepCache::StateCache &st = (*cache->dep_cache)[it];
            if (st.InstBroken() || st.NowBroken()) {
                if (!broken_info.empty()) broken_info += "; ";
                broken_info += it.Name();
                pkgCache::VerIterator InstVer = st.InstVerIter(*cache->dep_cache);
                if (!InstVer.end()) {
                    for (pkgCache::DepIterator D = InstVer.DependsList(); !D.end(); ++D) {
                        if (D->Type != pkgCache::Dep::Depends && D->Type != pkgCache::Dep::PreDepends) continue;
                        if (((*cache->dep_cache)[D] & pkgDepCache::DepInstall) == pkgDepCache::DepInstall) continue;

                        broken_info += " (depends: ";
                        broken_info += D.TargetPkg().Name();
                        if (D.TargetVer() != nullptr) {
                            broken_info += " ";
                            broken_info += D.CompType();
                            broken_info += " ";
                            broken_info += D.TargetVer();
                        }
                        broken_info += ")";
                        break;
                    }
                }
            }
        }

        if (!broken_info.empty()) {
            std::string out = "Some packages could not be installed. Broken: " + broken_info;
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
        }
        return make_result(APT_ERROR_DEPENDENCY_BROKEN,
                           "Some packages could not be installed. This may mean that you have requested an impossible situation");
    }

    if (!check_apt_errors()) {
        return make_result(APT_ERROR_DEPENDENCY_BROKEN, nullptr);
    }

    return make_result(APT_SUCCESS, nullptr);
}

void collect_package_changes(AptCache *cache,
                             const std::set<std::string> &requested_install,
                             const std::set<std::string> &requested_remove,
                             std::vector<std::string> &extra_installed,
                             std::vector<std::string> &extra_removed,
                             std::vector<std::string> &upgraded,
                             std::vector<std::string> &new_installed,
                             std::vector<std::string> &removed,
                             uint64_t &download_size,
                             uint64_t &install_size) {
    download_size = 0;
    install_size = 0;

    for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
        pkgDepCache::StateCache &st = (*cache->dep_cache)[iter];
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
        } else if ((st.iFlags & pkgDepCache::ReInstall) != 0) {
            new_installed.push_back(iter.Name());
            pkgCache::VerIterator currentVer = iter.CurrentVer();
            if (!currentVer.end()) {
                download_size += currentVer->Size;
            }
        }
    }
}

void populate_changes_structure(AptPackageChanges *changes,
                                const std::vector<std::string> &extra_installed,
                                const std::vector<std::string> &upgraded,
                                const std::vector<std::string> &new_installed,
                                const std::vector<std::string> &removed,
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
        changes->extra_installed = (char **) malloc(changes->extra_installed_count * sizeof(char *));
        for (size_t i = 0; i < changes->extra_installed_count; ++i)
            changes->extra_installed[i] = strdup(extra_installed[i].c_str());
    }
    if (changes->removed_count > 0) {
        changes->removed_packages = (char **) malloc(changes->removed_count * sizeof(char *));
        for (size_t i = 0; i < changes->removed_count; ++i)
            changes->removed_packages[i] = strdup(removed[i].c_str());
    }
    if (changes->upgraded_count > 0) {
        changes->upgraded_packages = (char **) malloc(changes->upgraded_count * sizeof(char *));
        for (size_t i = 0; i < changes->upgraded_count; ++i)
            changes->upgraded_packages[i] = strdup(upgraded[i].c_str());
    }
    if (changes->new_installed_count > 0) {
        changes->new_installed_packages = (char **) malloc(changes->new_installed_count * sizeof(char *));
        for (size_t i = 0; i < changes->new_installed_count; ++i)
            changes->new_installed_packages[i] = strdup(new_installed[i].c_str());
    }
}

#include "apt_package_operations.h"

#include <apt-pkg/algorithms.h>
#include <apt-pkg/error.h>
#include <apt-pkg/version.h>
#include <apt-pkg/versionmatch.h>

#include <cctype>
#include <cstring>
#include <list>
#include <sys/stat.h>

RequirementSpec parse_requirement(const std::string &raw) {
    RequirementSpec r;
    size_t pos;
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

// Helper class for sorting versions - best (newest) versions first
class BestVersionOrder {
    pkgDepCache &cache_;
    pkgProblemResolver &fix_;

public:
    BestVersionOrder(pkgDepCache &cache, pkgProblemResolver &fix)
        : cache_(cache), fix_(fix) {}

    bool operator()(const pkgCache::VerIterator &a, const pkgCache::VerIterator &b) const {
        int cmp = cache_.VS().CmpVersion(a.VerStr(), b.VerStr());
        if (cmp == 0) {
            const pkgCache::Package *A = &(*a.ParentPkg());
            const pkgCache::Package *B = &(*b.ParentPkg());
            cmp = fix_.ScoreSort(&B, &A);
        }
        return cmp > 0;
    }
};

// Set candidate version for package based on version requirement
static AptResult set_candidate_version_for_requirement(
    const AptCache *cache,
    pkgCache::PkgIterator &pkg,
    const RequirementSpec &req) {

    pkgVersionMatch match(req.version, pkgVersionMatch::Version, req.op);
    std::list<pkgCache::VerIterator> found = match.FindAll(pkg);

    if (found.empty()) {
        return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                           (std::string("Version '") + req.version + "' for '" + req.name + "' was not found").c_str());
    }

    if (found.size() > 1) {
        pkgProblemResolver fix(cache->dep_cache);
        fix.MakeScores();
        const BestVersionOrder order(*cache->dep_cache, fix);
        found.sort(order);
        found.unique();
    }

    const pkgCache::VerIterator selectedVer = found.front();

    cache->dep_cache->SetCandidateVersion(selectedVer);

    pkg = selectedVer.ParentPkg();

    return make_result(APT_SUCCESS, nullptr);
}

static AptResult find_install_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &result_pkg) {
    pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(req.name);

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

static AptResult resolve_virtual_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &pkg) {
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

        const bool instVirtual = _config->FindB("APT::Install::Virtual", false);

        for (unsigned int p = 0; p < Size; ++p) {
            auto PrvPkg = pkgCache::PkgIterator(*pkg.Cache(), PList[p]);
            pkgCache::PrvIterator Prv = pkg.ProvidesList();

            // Find the provide that matches this package
            for (; !Prv.end() && Prv.OwnerPkg() != PrvPkg; ++Prv) {
            }
            if (Prv.end()) continue;

            bool AlreadySeen = false;
            for (const auto & GoodSolution : GoodSolutions) {
                pkgCache::PkgIterator GoodPkg(*pkg.Cache(), GoodSolution);
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
            for (const auto & GoodSolution : GoodSolutions) {
                pkgCache::PkgIterator GoodPkg(*pkg.Cache(), GoodSolution);
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

AptResult process_package_installs(const AptCache *cache,
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

        // If version is specified, set the candidate version BEFORE MarkInstall.
        // Skip for virtual packages: after resolution, pkg points to the real provider
        if (req.has_version && req.name == pkg.Name()) {
            result = set_candidate_version_for_requirement(cache, pkg, req);
            if (result.code != APT_SUCCESS) {
                return result;
            }
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

AptResult process_package_reinstalls(const AptCache *cache,
                                     const char **reinstall_names,
                                     const size_t reinstall_count,
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
            pkg = cache->dep_cache->FindPkg(pkg_name);
            if (pkg.end()) {
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                                   (std::string("Package ") + pkg_name + " is not installed, so cannot be reinstalled").
                                   c_str());
            }
        } else {
            // For regular package names, use standard lookup
            const AptResult result = find_install_package(cache, req, pkg);
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

static AptResult find_remove_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &result_pkg) {
    pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(req.name);

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
resolve_virtual_remove_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &pkg) {
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

AptResult process_package_removals(const AptCache *cache,
                                   const char **remove_names,
                                   const size_t remove_count,
                                   const bool purge,
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

        pkgCache::PkgIterator pkg;
        AptResult result = find_remove_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = resolve_virtual_remove_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        requested_remove.insert(pkg.Name());

        cache->dep_cache->MarkDelete(pkg, purge);
        remove_targets.emplace_back(pkg.Name(), pkg);

    }

    return make_result(APT_SUCCESS, nullptr);
}

// Check for package conflicts ONLY between explicitly requested packages
AptResult check_package_conflicts(const AptCache *cache, const std::set<std::string> &requested_install) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }

    if (requested_install.size() < 2) {
        return make_result(APT_SUCCESS, nullptr);
    }

    std::vector<pkgCache::PkgIterator> requested_packages;

    for (const auto &name: requested_install) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name);
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
                    const std::string error_msg = "Conflicting packages: " + std::string(pkg1.Name()) + " and " + std::string(
                                                target_pkg.Name());

                    return make_result(APT_ERROR_DEPENDENCY_BROKEN, error_msg.c_str());
                }
            }
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

AptResult preprocess_installs(const AptCache *cache, const std::set<std::string> &requested_install) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }

    for (const auto &name: requested_install) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name);
        if (pkg.end()) continue;

        pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[pkg];
        if (!pkg_state.Install()) continue;

        pkgCache::VerIterator InstVer = pkg_state.InstVerIter(*cache->dep_cache);
        if (InstVer.end()) continue;

        for (pkgCache::DepIterator D = InstVer.DependsList(); !D.end();) {
            pkgCache::DepIterator Start, End;
            D.GlobOr(Start, End);

            if (!End.IsCritical()) {
                continue;
            }

            if (((*cache->dep_cache)[End] & pkgDepCache::DepGInstall) == pkgDepCache::DepGInstall) {
                continue;
            }

            // Try to satisfy this dependency by marking packages for install
            for (pkgCache::DepIterator CurDep = Start; ; ++CurDep) {
                pkgCache::PkgIterator DepPkg = CurDep.SmartTargetPkg();
                if (!DepPkg.end()) {
                    if (((*cache->dep_cache)[CurDep] & pkgDepCache::DepCVer) == pkgDepCache::DepCVer) {
                        pkgDepCache::StateCache &DepState = (*cache->dep_cache)[DepPkg];
                        if (!DepState.Install() && DepState.CandidateVer != nullptr) {
                            cache->dep_cache->MarkInstall(DepPkg, pkgDepCache::AutoMarkFlag::Auto, true);
                        }
                        break;
                    }
                }

                if (CurDep == End) break;
            }
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

AptResult finalize_dependency_resolution(const AptCache *cache, const std::set<std::string> &requested_install,
                                         const std::set<std::string> &requested_remove, const bool remove_depends) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, "Invalid cache");
    }

    pkgProblemResolver Fix(cache->dep_cache);

    if (remove_depends || (_config && _config->FindB("APT::Remove-Depends", false))) {
        Fix.RemoveDepends();
    }

    for (const auto &name: requested_install) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name);
        if (pkg.end()) continue;

        pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[pkg];
        if (!pkg_state.Install()) continue;

        Fix.Clear(pkg);
        Fix.Protect(pkg);
    }

    for (const auto &name: requested_remove) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name);
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
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name);
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
            const std::string out = "Some packages could not be installed. Broken: " + broken_info;
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

void collect_package_changes(const AptCache *cache,
                             const std::set<std::string> &requested_install,
                             std::vector<std::string> &extra_installed,
                             std::vector<std::string> &upgraded,
                             std::vector<std::string> &new_installed,
                             std::vector<std::string> &removed,
                             std::vector<std::string> &kept_back,
                             uint64_t &download_size,
                             int64_t &install_size) {
    download_size = 0;
    install_size = 0;

    for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
        pkgDepCache::StateCache &st = (*cache->dep_cache)[iter];

        if (iter->CurrentVer != 0 && st.Upgradable() && !st.Upgrade() && !st.Delete()) {
            kept_back.emplace_back(iter.Name());
            continue;
        }

        if (st.NewInstall()) {
            new_installed.emplace_back(iter.Name());
            if (requested_install.find(iter.Name()) == requested_install.end()) {
                extra_installed.emplace_back(iter.Name());
            }
            if (st.CandidateVer != nullptr) {
                download_size += static_cast<uint64_t>(st.CandidateVer->Size);
                install_size += static_cast<int64_t>(st.CandidateVer->InstalledSize);
            }
        } else if (st.Upgrade()) {
            upgraded.emplace_back(iter.Name());
            if (st.CandidateVer != nullptr) {
                download_size += static_cast<uint64_t>(st.CandidateVer->Size);
                install_size += static_cast<int64_t>(st.CandidateVer->InstalledSize);
                pkgCache::VerIterator currentVer = iter.CurrentVer();
                if (!currentVer.end()) {
                    install_size -= static_cast<int64_t>(currentVer->InstalledSize);
                }
            }
        } else if (st.Downgrade()) {
            upgraded.emplace_back(iter.Name());
            if (st.CandidateVer != nullptr) {
                download_size += static_cast<uint64_t>(st.CandidateVer->Size);
                install_size += static_cast<int64_t>(st.CandidateVer->InstalledSize);
                pkgCache::VerIterator currentVer = iter.CurrentVer();
                if (!currentVer.end()) {
                    install_size -= static_cast<int64_t>(currentVer->InstalledSize);
                }
            }
        } else if (st.Delete()) {
            removed.emplace_back(iter.Name());
            if (st.InstallVer != nullptr) install_size -= static_cast<int64_t>(st.InstallVer->InstalledSize);
        } else if ((st.iFlags & pkgDepCache::ReInstall) != 0) {
            new_installed.emplace_back(iter.Name());
            pkgCache::VerIterator currentVer = iter.CurrentVer();
            if (!currentVer.end()) {
                download_size += static_cast<uint64_t>(currentVer->Size);
            }
        }
    }
}

// Check if package is being obsoleted by something being installed
static bool is_obsoleted(const AptCache *cache, const pkgCache::PkgIterator &pkg) {
    if (pkg.CurrentVer().end()) return false;

    for (pkgCache::DepIterator D = pkg.RevDependsList(); !D.end(); ++D) {
        if (D->Type == pkgCache::Dep::Obsoletes &&
            (*cache->dep_cache)[D.ParentPkg()].Install() &&
            (static_cast<pkgCache::Version *>(D.ParentVer()) == (*cache->dep_cache)[D.ParentPkg()].InstallVer ||
             static_cast<pkgCache::Version *>(D.ParentVer()) == static_cast<pkgCache::Version *>(D.ParentPkg().CurrentVer())) &&
            cache->dep_cache->VS().CheckDep(pkg.CurrentVer().VerStr(), D)) {
            return true;
        }
    }
    return false;
}

void collect_essential_packages(const AptCache *cache,
                                std::vector<std::pair<std::string, std::string>> &essential_list) {
    if (!cache || !cache->dep_cache) return;

    std::vector<bool> added(cache->dep_cache->Head().PackageCount, false);

    for (pkgCache::PkgIterator I = cache->dep_cache->PkgBegin(); !I.end(); ++I) {
        if (((I->Flags & pkgCache::Flag::Essential) == 0) &&
            ((I->Flags & pkgCache::Flag::Important) == 0)) {
            continue;
        }

        if ((*cache->dep_cache)[I].Delete() && !added[I->ID] && !is_obsoleted(cache, I)) {
            added[I->ID] = true;
            essential_list.emplace_back(I.Name(), "");
        }

        if (I->CurrentVer == 0) continue;

        for (pkgCache::DepIterator D = I.CurrentVer().DependsList(); !D.end(); ++D) {
            if (D->Type != pkgCache::Dep::PreDepends &&
                D->Type != pkgCache::Dep::Depends) {
                continue;
            }

            pkgCache::PkgIterator P = D.SmartTargetPkg();
            if (P.end() || !(*cache->dep_cache)[P].Delete() || added[P->ID]) {
                continue;
            }

            if (!is_obsoleted(cache, P)) {
                added[P->ID] = true;
                essential_list.emplace_back(P.Name(), I.Name());
            }
        }
    }
}

void populate_changes_structure(AptPackageChanges *changes,
                                const std::vector<std::string> &extra_installed,
                                const std::vector<std::string> &upgraded,
                                const std::vector<std::string> &new_installed,
                                const std::vector<std::string> &removed,
                                const std::vector<std::string> &kept_back,
                                const size_t not_upgraded_count,
                                const std::vector<std::pair<std::string, std::string>> &essential_list,
                                const uint64_t download_size,
                                const int64_t install_size) {
    changes->extra_installed_count = extra_installed.size();
    changes->upgraded_count = upgraded.size();
    changes->new_installed_count = new_installed.size();
    changes->removed_count = removed.size();
    changes->kept_back_count = kept_back.size();
    changes->not_upgraded_count = not_upgraded_count;
    changes->download_size = download_size;
    changes->install_size = install_size;

    if (changes->extra_installed_count > 0) {
        changes->extra_installed = static_cast<char **>(malloc(changes->extra_installed_count * sizeof(char *)));
        for (size_t i = 0; i < changes->extra_installed_count; ++i)
            changes->extra_installed[i] = safe_strdup(extra_installed[i]);
    }
    if (changes->removed_count > 0) {
        changes->removed_packages = static_cast<char **>(malloc(changes->removed_count * sizeof(char *)));
        for (size_t i = 0; i < changes->removed_count; ++i)
            changes->removed_packages[i] = safe_strdup(removed[i]);
    }
    if (changes->upgraded_count > 0) {
        changes->upgraded_packages = static_cast<char **>(malloc(changes->upgraded_count * sizeof(char *)));
        for (size_t i = 0; i < changes->upgraded_count; ++i)
            changes->upgraded_packages[i] = safe_strdup(upgraded[i]);
    }
    if (changes->new_installed_count > 0) {
        changes->new_installed_packages = static_cast<char **>(malloc(changes->new_installed_count * sizeof(char *)));
        for (size_t i = 0; i < changes->new_installed_count; ++i)
            changes->new_installed_packages[i] = safe_strdup(new_installed[i]);
    }
    if (changes->kept_back_count > 0) {
        changes->kept_back_packages = static_cast<char **>(malloc(changes->kept_back_count * sizeof(char *)));
        for (size_t i = 0; i < changes->kept_back_count; ++i)
            changes->kept_back_packages[i] = safe_strdup(kept_back[i]);
    }

    changes->essential_packages_count = essential_list.size();
    if (changes->essential_packages_count > 0) {
        changes->essential_packages = static_cast<AptEssentialPackage *>(malloc(
            changes->essential_packages_count * sizeof(AptEssentialPackage)));
        for (size_t i = 0; i < changes->essential_packages_count; ++i) {
            changes->essential_packages[i].name = safe_strdup(essential_list[i].first);
            changes->essential_packages[i].reason = safe_strdup(essential_list[i].second);
        }
    }
}

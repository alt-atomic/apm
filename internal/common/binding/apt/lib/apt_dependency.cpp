#include "dependency.h"
#include "error.h"

#include <apt-pkg/algorithms.h>
#include <apt-pkg/error.h>
#include <apt-pkg/version.h>

#include <vector>

// Detects mutual Conflicts among requested packages before marking.
AptResult check_package_conflicts(const AptCache *cache, const std::set<std::string> &requested_install) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_CACHE);
    }

    if (requested_install.size() < 2) {
        return make_result(APT_SUCCESS, nullptr);
    }

    std::vector<pkgCache::PkgIterator> requested_packages;

    for (const auto &name: requested_install) {
        if (pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name); !pkg.end()) {
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

// Walks critical deps of requested packages and auto-marks missing ones.
AptResult preprocess_installs(const AptCache *cache, const std::set<std::string> &requested_install) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_CACHE);
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

            for (pkgCache::DepIterator CurDep = Start; ; ++CurDep) {
                if (pkgCache::PkgIterator DepPkg = CurDep.SmartTargetPkg(); !DepPkg.end()) {
                    if (((*cache->dep_cache)[CurDep] & pkgDepCache::DepCVer) == pkgDepCache::DepCVer) {
                        if (pkgDepCache::StateCache &DepState = (*cache->dep_cache)[DepPkg]; !DepState.Install() && DepState.CandidateVer != nullptr) {
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

// Runs the problem resolver and reports any remaining broken dependencies.
AptResult finalize_dependency_resolution(const AptCache *cache, const std::set<std::string> &requested_install,
                                         const std::set<std::string> &requested_remove, const bool remove_depends) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_CACHE);
    }

    pkgProblemResolver Fix(cache->dep_cache);

    if (remove_depends || (_config && _config->FindB("APT::Remove-Depends", false))) {
        Fix.RemoveDepends();
    }

    for (const auto &name: requested_install) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name);
        if (pkg.end()) continue;

        if (pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[pkg]; !pkg_state.Install()) continue;

        Fix.Clear(pkg);
        Fix.Protect(pkg);
    }

    for (const auto &name: requested_remove) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name);
        if (pkg.end()) continue;

        if (pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[pkg]; !pkg_state.Delete()) continue;

        Fix.Clear(pkg);
        Fix.Protect(pkg);
        Fix.Remove(pkg);
    }

    Fix.InstallProtect();
    if (!Fix.Resolve(true)) {
        _error->Discard();
    }

    for (const auto &name: requested_install) {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(name);
        if (pkg.end()) continue;

        if (pkgDepCache::StateCache &pkg_state = (*cache->dep_cache)[pkg]; !pkg_state.Install() && pkg.CurrentVer().end()) {
            cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, false);
        }
    }

    if (cache->dep_cache->BrokenCount() != 0) {
        std::string broken_info;
        for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
            if (pkgDepCache::StateCache &st = (*cache->dep_cache)[it]; st.InstBroken() || st.NowBroken()) {
                if (!broken_info.empty()) broken_info += "; ";
                broken_info += it.Name();
                if (pkgCache::VerIterator InstVer = st.InstVerIter(*cache->dep_cache); !InstVer.end()) {
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

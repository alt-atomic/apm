#include "resolve.h"

#include <apt-pkg/algorithms.h>
#include <apt-pkg/error.h>
#include <apt-pkg/pkgrecords.h>
#include <apt-pkg/version.h>
#include <apt-pkg/versionmatch.h>

#include <cctype>
#include <cstring>
#include <list>
#include <sys/stat.h>

// Splits "name>=version" into name, operator, and version components.
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

// Comparator that sorts versions by version number, then by problem resolver score.
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

// Finds all matching versions and selects the best one as the candidate.
AptResult set_candidate_version_for_requirement(
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

// Looks up a package by name; falls back to searching Provides lists.
AptResult find_install_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &result_pkg) {
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
        qsort(provider_pkgs.data(), provider_pkgs.size(), sizeof(pkgCache::Package *), &pkgProblemResolver::ScoreSort);
        pkg = pkgCache::PkgIterator(*cache->dep_cache, provider_pkgs.front());
    }

    result_pkg = pkg;
    return make_result(APT_SUCCESS, nullptr);
}

// Picks the best real provider for a virtual package using problem resolver scores.
AptResult resolve_virtual_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &pkg) {
    if (pkg->VersionList == 0 && pkg->ProvidesList != 0) {
        std::vector<pkgCache::Package *> GoodSolutions;
        unsigned long Size = 0;

        for (pkgCache::PrvIterator Prv = pkg.ProvidesList(); !Prv.end(); ++Prv) {
            Size++;
        }

        std::vector<pkgCache::Package *> PList(Size);
        pkgCache::Package **PEnd = PList.data();
        for (pkgCache::PrvIterator Prv = pkg.ProvidesList(); !Prv.end(); ++Prv) {
            *PEnd++ = Prv.OwnerPkg();
        }

        pkgProblemResolver TempFix(cache->dep_cache);
        TempFix.MakeScores();
        qsort(PList.data(), PList.size(), sizeof(pkgCache::Package *), &pkgProblemResolver::ScoreSort);

        const bool instVirtual = _config->FindB("APT::Install::Virtual", false);

        for (unsigned int p = 0; p < Size; ++p) {
            auto PrvPkg = pkgCache::PkgIterator(*pkg.Cache(), PList[p]);
            pkgCache::PrvIterator Prv = pkg.ProvidesList();

            for (; !Prv.end() && Prv.OwnerPkg() != PrvPkg; ++Prv) {
            }
            if (Prv.end()) continue;

            bool AlreadySeen = false;
            for (const auto & GoodSolution : GoodSolutions) {
                if (pkgCache::PkgIterator GoodPkg(*pkg.Cache(), GoodSolution); PrvPkg == GoodPkg) {
                    AlreadySeen = true;
                    break;
                }
            }
            if (AlreadySeen) continue;

            if (PrvPkg.CurrentVer() == Prv.OwnerVer()) {
                if (req.has_version) {
                    if (const char *prvVer = Prv.ProvideVersion(); prvVer == nullptr ||
                                                                   !cache->dep_cache->VS().CheckDep(prvVer, req.op, req.version.c_str()))
                        continue;
                }
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
                if (!cache->dep_cache->VS().CheckDep(prvVer, req.op, req.version.c_str())) continue;
            }

            bool good = false;
            for (; !PrvPkgCandVer.end(); ++PrvPkgCandVer) {
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
                msg += "  ";
                msg += GoodPkg.Name();
                msg += " ";
                msg += version;
                msg += status;
                msg += "\n";
            }
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND, msg.c_str());
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

// Scans all package records to find which package owns `name` (a file path).
AptResult resolve_file_to_package(const AptCache *cache, std::string &name) {
    pkgRecords recs(*cache->cache_file);
    if (_error->PendingError()) {
        _error->Discard();
        return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
            (std::string("Package not found: ") + name).c_str());
    }

    for (pkgCache::PkgIterator pkg = cache->dep_cache->PkgBegin(); !pkg.end(); ++pkg) {
        pkgCache::VerIterator ver = (*cache->dep_cache)[pkg].CandidateVerIter(*cache->dep_cache);
        if (ver.end()) continue;

        pkgCache::VerFileIterator vf = ver.FileList();
        if (vf.end()) continue;

        if (pkgRecords::Parser &parse = recs.Lookup(vf); parse.HasFile(name.c_str())) {
            emit_log(std::string("Selecting ") + pkg.Name() + " for '" + name + "'");
            name = pkg.Name();
            return make_result(APT_SUCCESS, nullptr);
        }
    }

    return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
        (std::string("Package not found: ") + name).c_str());
}

// Finds an installed package by name; falls back to installed Provides.
AptResult find_remove_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &result_pkg) {
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

// If `pkg` is not installed, searches for a single installed provider.
AptResult
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

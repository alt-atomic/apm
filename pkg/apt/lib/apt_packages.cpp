#include "apt_package.h"
#include "internal.h"
#include "error.h"
#include "package_fill.h"

#include <apt-pkg/error.h>

#include <cstdlib>
#include <cstring>
#include <regex.h>
#include <set>

// RAII wrapper for POSIX regex compilation and cleanup.
class RegexGuard {
    regex_t re_{};
    bool compiled_ = false;
public:
    bool compile(const char *pattern, const int flags) {
        if (regcomp(&re_, pattern, flags) != 0) return false;
        compiled_ = true;
        return true;
    }
    regex_t *get() { return &re_; }
    ~RegexGuard() { if (compiled_) regfree(&re_); }
};

// Searches packages by regex, matching against name and description.
AptResult apt_packages_search(AptCache *cache, const char *pattern, AptPackageList *result) {
    if (!cache || !cache->dep_cache || !pattern || !result) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_PACKAGES_INVALID_PARAMS);
    }
    result->packages = nullptr;
    result->count = 0;

    try {
        LocaleGuard locale;
        pkgCache &Cache = cache->dep_cache->GetCache();

        pkgDepCache::Policy policy;

        RegexGuard compiled_pattern;
        if (!compiled_pattern.compile(pattern, REG_EXTENDED | REG_ICASE | REG_NOSUB)) {
            return make_result(APT_ERROR_UNKNOWN, APT_MSG_PACKAGES_REGEX_FAILED);
        }

        pkgRecords Recs(Cache);
        if (_error->PendingError()) {
            return make_result(APT_ERROR_UNKNOWN, APT_MSG_PACKAGES_RECORDS_FAILED);
        }

        struct ExVerFile {
            pkgCache::VerFile *Vf;
            bool NameMatch;
        };

        std::unique_ptr<ExVerFile[]> VFList(new ExVerFile[Cache.HeaderP->PackageCount + 1]());

        for (pkgCache::PkgIterator P = Cache.PkgBegin(); !P.end(); ++P) {
            VFList[P->ID].NameMatch = false;
            VFList[P->ID].Vf = nullptr;

            if (regexec(compiled_pattern.get(), P.Name(), 0, nullptr, 0) == 0) {
                VFList[P->ID].NameMatch = true;
            }

            if (pkgCache::VerIterator V = policy.GetCandidateVer(P); !V.end()) {
                VFList[P->ID].Vf = V.FileList();
            }
        }

        for (pkgCache::PkgIterator P = Cache.PkgBegin(); !P.end(); ++P) {
            if (!VFList[P->ID].NameMatch)
                continue;

            for (pkgCache::PrvIterator Prv = P.ProvidesList(); !Prv.end(); ++Prv) {
                if (pkgCache::VerIterator V = policy.GetCandidateVer(Prv.OwnerPkg()); !V.end()) {
                    VFList[Prv.OwnerPkg()->ID].Vf = V.FileList();
                    VFList[Prv.OwnerPkg()->ID].NameMatch = true;
                }
            }
        }

        const auto LocalityCompare = [](const void *a, const void *b) -> int {
            const auto *A = static_cast<const ExVerFile *>(a);
            const auto *B = static_cast<const ExVerFile *>(b);
            if (A->Vf == nullptr && B->Vf == nullptr) return 0;
            if (A->Vf == nullptr) return 1;
            if (B->Vf == nullptr) return -1;
            if (A->Vf->File < B->Vf->File) return -1;
            if (A->Vf->File > B->Vf->File) return 1;
            return 0;
        };

        qsort(VFList.get(), Cache.HeaderP->PackageCount, sizeof(ExVerFile), LocalityCompare);

        std::vector<AptPackageInfo> matched_packages;
        std::set<std::string> seen_packages;
        AptFileListCache hdlist_cache;

        for (ExVerFile *J = VFList.get(); J->Vf != nullptr; J++) {
            pkgCache::VerFileIterator VF(Cache, J->Vf);

            try {
                bool Match = true;
                pkgRecords::Parser &Parser = Recs.Lookup(VF);
                const std::string pkg_name = Parser.Name();

                if (!J->NameMatch) {
                    const std::string LongDesc = Parser.LongDesc();
                    const std::string ShortDesc = Parser.ShortDesc();

                    Match = false;
                    if (regexec(compiled_pattern.get(), LongDesc.c_str(), 0, nullptr, 0) == 0 ||
                        regexec(compiled_pattern.get(), ShortDesc.c_str(), 0, nullptr, 0) == 0) {
                        Match = true;
                    }
                }

                if (Match && !pkg_name.empty()) {
                    // Skip i586- variant if base package exists with a candidate
                    if (pkg_name.rfind("i586-", 0) == 0) {
                        std::string base_name = pkg_name.substr(5);
                        if (pkgCache::PkgIterator base_pkg = Cache.FindPkg(base_name); !base_pkg.end()) {
                            if (pkgCache::VerIterator base_ver = policy.GetCandidateVer(base_pkg); !base_ver.end()) {
                                continue;
                            }
                        }
                    }

                    // Determine effective name (strip i586- prefix when base has no candidate)
                    std::string effective_name = pkg_name;
                    if (pkg_name.rfind("i586-", 0) == 0) {
                        std::string base_name = pkg_name.substr(5);
                        if (pkgCache::PkgIterator base_pkg = Cache.FindPkg(base_name); !base_pkg.end()) {
                            if (pkgCache::VerIterator base_ver = policy.GetCandidateVer(base_pkg); base_ver.end()) {
                                effective_name = base_name;
                            }
                        }
                    }

                    if (seen_packages.find(effective_name) == seen_packages.end()) {
                        seen_packages.insert(effective_name);

                        pkgCache::PkgIterator Pkg = Cache.FindPkg(pkg_name);
                        pkgCache::VerIterator Ver = Pkg.end()
                            ? pkgCache::VerIterator()
                            : policy.GetCandidateVer(Pkg);

                        AptPackageInfo info = {};
                        fill_package_info(Cache, Pkg, Ver, VF, Recs, policy,
                                          effective_name, &info, hdlist_cache);

                        matched_packages.push_back(info);

                        if (matched_packages.size() >= 150000) {
                            break;
                        }
                    }
                }
            } catch (const std::exception &) {
                // Skip this version file if we can't parse it
            }
        }

        if (matched_packages.empty()) {
            return make_result(APT_SUCCESS);
        }

        result->count = matched_packages.size();
        result->packages = static_cast<AptPackageInfo *>(calloc(result->count, sizeof(AptPackageInfo)));
        if (!result->packages) {
            for (auto &pkg : matched_packages) {
                apt_package_free(&pkg);
            }
            result->count = 0;
            return make_result(APT_ERROR_UNKNOWN, APT_MSG_PACKAGES_ALLOC_FAILED);
        }

        for (size_t i = 0; i < matched_packages.size(); ++i) {
            result->packages[i] = matched_packages[i];
        }

        return make_result(APT_SUCCESS);
    } catch (const std::exception &e) {
        if (result->packages) {
            for (size_t i = 0; i < result->count; ++i) {
                apt_package_free(&result->packages[i]);
            }
            free(result->packages);
            result->packages = nullptr;
        }
        result->count = 0;
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    } catch (...) {
        if (result && result->packages) {
            for (size_t i = 0; i < result->count; ++i) {
                apt_package_free(&result->packages[i]);
            }
            free(result->packages);
            result->packages = nullptr;
        }
        if (result) result->count = 0;
        return make_result(APT_ERROR_UNKNOWN, APT_MSG_PACKAGES_UNKNOWN);
    }
}

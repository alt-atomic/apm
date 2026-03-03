#include "apt_internal.h"

#include <apt-pkg/error.h>
#include <apt-pkg/pkgrecords.h>

#include <cstdlib>
#include <cstring>
#include <regex.h>
#include <set>

// RAII wrapper
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

AptResult apt_search_packages(AptCache *cache, const char *pattern, AptPackageList *result) {
    if (!cache || !cache->dep_cache || !pattern || !result) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid parameters for search");
    }

    result->packages = nullptr;
    result->count = 0;

    try {
        pkgCache &Cache = cache->dep_cache->GetCache();

        pkgDepCache::Policy policy;

        RegexGuard compiled_pattern;
        if (!compiled_pattern.compile(pattern, REG_EXTENDED | REG_ICASE | REG_NOSUB)) {
            return make_result(APT_ERROR_UNKNOWN, "Failed to compile regex pattern");
        }

        pkgRecords Recs(Cache);
        if (_error->PendingError() == true) {
            return make_result(APT_ERROR_UNKNOWN, "Failed to create package records parser");
        }

        struct ExVerFile {
            pkgCache::VerFile *Vf;
            bool NameMatch;
        };

        std::unique_ptr<ExVerFile[]> VFList(new ExVerFile[Cache.HeaderP->PackageCount + 1]());

        for (pkgCache::PkgIterator P = Cache.PkgBegin(); P.end() == false; ++P) {
            VFList[P->ID].NameMatch = false;
            VFList[P->ID].Vf = nullptr;

            if (regexec(compiled_pattern.get(), P.Name(), 0, nullptr, 0) == 0) {
                VFList[P->ID].NameMatch = true;
            }

            pkgCache::VerIterator V = policy.GetCandidateVer(P);
            if (V.end() == false) {
                VFList[P->ID].Vf = V.FileList();
            }
        }

        for (pkgCache::PkgIterator P = Cache.PkgBegin(); P.end() == false; ++P) {
            if (VFList[P->ID].NameMatch == false)
                continue;

            for (pkgCache::PrvIterator Prv = P.ProvidesList(); Prv.end() == false; ++Prv) {
                pkgCache::VerIterator V = policy.GetCandidateVer(Prv.OwnerPkg());
                if (V.end() == false) {
                    VFList[Prv.OwnerPkg()->ID].Vf = V.FileList();
                    VFList[Prv.OwnerPkg()->ID].NameMatch = true;
                }
            }
        }

        auto LocalityCompare = [](const void *a, const void *b) -> int {
            const auto A = static_cast<const ExVerFile *>(a);
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

        for (ExVerFile *J = VFList.get(); J->Vf != nullptr; J++) {
            pkgCache::VerFileIterator VF(Cache, J->Vf);

            try {
                bool Match = true;
                pkgRecords::Parser &Parser = Recs.Lookup(VF);
                std::string pkg_name = Parser.Name();

                if (J->NameMatch == false) {
                    std::string LongDesc = Parser.LongDesc();
                    std::string ShortDesc = Parser.ShortDesc();

                    Match = false;
                    if (regexec(compiled_pattern.get(), LongDesc.c_str(), 0, nullptr, 0) == 0 ||
                        regexec(compiled_pattern.get(), ShortDesc.c_str(), 0, nullptr, 0) == 0) {
                        Match = true;
                    }
                }

                if (Match == true && !pkg_name.empty()) {
                    if (pkg_name.rfind("i586-", 0) == 0) {
                        std::string base_name = pkg_name.substr(5);
                        pkgCache::PkgIterator base_pkg = Cache.FindPkg(base_name);
                        if (!base_pkg.end()) {
                            pkgCache::VerIterator base_ver = policy.GetCandidateVer(base_pkg);
                            if (!base_ver.end()) {
                                continue;
                            }
                        }
                    }
                    std::string effective_name = pkg_name;
                    bool is_i586 = (pkg_name.rfind("i586-", 0) == 0);
                    if (is_i586) {
                        std::string base_name = pkg_name.substr(5);
                        pkgCache::PkgIterator base_pkg = Cache.FindPkg(base_name);
                        if (!base_pkg.end()) {
                            pkgCache::VerIterator base_ver = policy.GetCandidateVer(base_pkg);
                            if (base_ver.end()) {
                                effective_name = base_name;
                            }
                        }
                    }

                    if (seen_packages.find(effective_name) == seen_packages.end()) {
                        seen_packages.insert(effective_name);

                        // Find the package in cache for proper info
                        pkgCache::PkgIterator Pkg = Cache.FindPkg(pkg_name);

                        AptPackageInfo info = {};

                        std::string long_desc = Parser.LongDesc();
                        std::string short_desc = Parser.ShortDesc();

                        info.description = safe_strdup(long_desc.c_str());
                        info.short_description = safe_strdup(short_desc.c_str());
                        info.maintainer = safe_strdup(Parser.Maintainer().c_str());
                        info.source_package = safe_strdup(Parser.SourcePkg().c_str());
                        info.md5_hash = safe_strdup(Parser.MD5Hash().c_str());
                        info.blake2b_hash = safe_strdup(Parser.BLAKE2b().c_str());
                        info.filename = safe_strdup(Parser.FileName().c_str());
                        info.changelog = safe_strdup(Parser.Changelog().c_str());

                        // Parse fields from full record (Homepage, Version, Architecture)
                        const char *rec_start, *rec_stop;
                        Parser.GetRec(rec_start, rec_stop);
                        std::string record(rec_start, rec_stop - rec_start);

                        // Version
                        std::string record_version;
                        {
                            size_t pos = record.find("Version: ");
                            if (pos != std::string::npos) {
                                size_t start = pos + 9;
                                size_t end = record.find('\n', start);
                                if (end == std::string::npos) end = record.length();
                                record_version = record.substr(start, end - start);
                            }
                        }
                        // Architecture
                        std::string record_arch;
                        {
                            size_t pos = record.find("Architecture: ");
                            if (pos != std::string::npos) {
                                size_t start = pos + 14;
                                size_t end = record.find('\n', start);
                                if (end == std::string::npos) end = record.length();
                                record_arch = record.substr(start, end - start);
                            }
                        }

                        // Provides
                        std::string record_provides;
                        {
                            size_t pos = record.find("Provides: ");
                            if (pos != std::string::npos) {
                                size_t start = pos + 10;
                                size_t end = record.find('\n', start);
                                if (end == std::string::npos) end = record.length();
                                record_provides = record.substr(start, end - start);
                            }
                        }

                        size_t homepage_pos = record.find("Homepage: ");
                        if (homepage_pos != std::string::npos) {
                            size_t start = homepage_pos + 10;
                            size_t end = record.find('\n', start);
                            if (end == std::string::npos) end = record.length();
                            std::string homepage = record.substr(start, end - start);
                            info.homepage = safe_strdup(homepage.c_str());
                        } else {
                            // leave homepage as nullptr if not present
                        }

                        info.name = safe_strdup(effective_name.c_str());
                        info.aliases = nullptr;
                        info.alias_count = 0;

                        if (!Pkg.end()) {
                            info.package_id = Pkg->ID;
                            if (Pkg.Section() != nullptr && *Pkg.Section() != '\0') {
                                info.section = safe_strdup(Pkg.Section());
                            }
                            info.essential = (Pkg->Flags & pkgCache::Flag::Essential) != 0;
                            info.auto_installed = (Pkg->Flags & pkgCache::Flag::Auto) != 0;

                            // Package state - check CurrentVer()
                            if (!Pkg.CurrentVer().end()) {
                                switch (Pkg->CurrentState) {
                                    case pkgCache::State::Installed:
                                        info.state = APT_PKG_STATE_INSTALLED;
                                        break;
                                    case pkgCache::State::ConfigFiles:
                                        info.state = APT_PKG_STATE_CONFIG_FILES;
                                        break;
                                    case pkgCache::State::UnPacked:
                                        info.state = APT_PKG_STATE_UNPACKED;
                                        break;
                                    case pkgCache::State::HalfConfigured:
                                        info.state = APT_PKG_STATE_HALF_CONFIGURED;
                                        break;
                                    case pkgCache::State::HalfInstalled:
                                        info.state = APT_PKG_STATE_HALF_INSTALLED;
                                        break;
                                    default:
                                        info.state = APT_PKG_STATE_INSTALLED;
                                        break;
                                }
                            } else {
                                info.state = APT_PKG_STATE_NOT_INSTALLED;
                            }

                            // Resolve version: candidate; else fallback to record Version field
                            pkgCache::VerIterator Ver = policy.GetCandidateVer(Pkg);
                            if (!Ver.end()) {
                                if (Ver.VerStr() != nullptr && *Ver.VerStr() != '\0') {
                                    info.version = safe_strdup(Ver.VerStr());
                                }
                                if (Ver.Arch() != nullptr && *Ver.Arch() != '\0') {
                                    info.architecture = safe_strdup(Ver.Arch());
                                }
                                if (pkgCache::Priority(Ver->Priority) != nullptr && *pkgCache::Priority(Ver->Priority)
                                    != '\0') {
                                    info.priority = safe_strdup(pkgCache::Priority(Ver->Priority));
                                }
                                info.installed_size = Ver->InstalledSize;
                                info.download_size = Ver->Size;
                                {
                                    std::set<std::string> prov_names;
                                    for (pkgCache::PrvIterator prv = Ver.ProvidesList(); !prv.end(); ++prv) {
                                        const char *n = prv.Name();
                                        if (n && *n) prov_names.insert(n);
                                    }
                                    if (!prov_names.empty()) {
                                        std::string joined;
                                        for (const auto & prov_name : prov_names) {
                                            if (!joined.empty()) joined += ", ";
                                            joined += prov_name;
                                        }
                                        info.provides = safe_strdup(joined.c_str());
                                    }
                                }
                                {
                                    std::set<std::string> dep_names;
                                    for (pkgCache::DepIterator dep = Ver.DependsList(); !dep.end(); ++dep) {
                                        if (dep->Type != pkgCache::Dep::Depends && dep->Type !=
                                            pkgCache::Dep::PreDepends)
                                            continue;
                                        pkgCache::PkgIterator tpkg = dep.TargetPkg();
                                        if (!tpkg.end() && tpkg.Name() != nullptr) {
                                            dep_names.insert(tpkg.Name());
                                        }
                                    }
                                    if (!dep_names.empty()) {
                                        std::string joined;
                                        for (const auto & dep_name : dep_names) {
                                            if (!joined.empty()) joined += ", ";
                                            joined += dep_name;
                                        }
                                        info.depends = safe_strdup(joined.c_str());
                                    }
                                }
                            } else {
                                if (!record_version.empty()) {
                                    info.version = safe_strdup(record_version.c_str());
                                }
                                if (!record_arch.empty()) {
                                    info.architecture = safe_strdup(record_arch.c_str());
                                }
                                info.installed_size = 0;
                                info.download_size = 0;
                                // Fallback depends from record line if present
                                {
                                    size_t pos = record.find("Depends: ");
                                    if (pos != std::string::npos) {
                                        size_t start = pos + 9;
                                        size_t end = record.find('\n', start);
                                        if (end == std::string::npos) end = record.length();
                                        std::string deps = record.substr(start, end - start);
                                        if (!deps.empty()) info.depends = safe_strdup(deps.c_str());
                                    }
                                }
                            }
                        } else {
                            info.package_id = 0;
                            info.essential = false;
                            info.auto_installed = false;
                            info.state = APT_PKG_STATE_NOT_INSTALLED;
                            if (!record_version.empty()) {
                                info.version = safe_strdup(record_version.c_str());
                            }
                            if (!record_arch.empty()) {
                                info.architecture = safe_strdup(record_arch.c_str());
                            }
                            info.installed_size = 0;
                            info.download_size = 0;
                            {
                                size_t pos = record.find("Depends: ");
                                if (pos != std::string::npos) {
                                    size_t start = pos + 9;
                                    size_t end = record.find('\n', start);
                                    if (end == std::string::npos) end = record.length();
                                    std::string deps = record.substr(start, end - start);
                                    if (!deps.empty()) info.depends = safe_strdup(deps.c_str());
                                }
                            }
                        }

                        // Finalize auxiliary fields (use record-provides if nothing collected from cache)
                        if (info.provides == nullptr && !record_provides.empty())
                            info.provides = safe_strdup(record_provides.c_str());

                        {
                            std::vector<std::string> aliases;
                            std::string current_name = Parser.Name();
                            const std::string& base_name_for_alias = effective_name;

                            // If this package is i586- prefixed and base exists, add that relation
                            bool has_i586_prefix = (!current_name.empty() && current_name.rfind("i586-", 0) == 0);
                            if (has_i586_prefix) {
                                std::string stripped = current_name.substr(5);
                                pkgCache::PkgIterator base_pkg = Cache.FindPkg(stripped);
                                if (!base_pkg.end()) {
                                    aliases.push_back(current_name);
                                    aliases.push_back(current_name + ".32bit");
                                }
                            } else {
                                // Current is base name. If i586- variant exists or arch is 32-bit, add aliases
                                std::string i586_variant = std::string("i586-") + base_name_for_alias;
                                pkgCache::PkgIterator i586_pkg = Cache.FindPkg(i586_variant);
                                bool has_i586_variant = !i586_pkg.end();

                                bool is_32bit_arch = false;
                                if (!Pkg.end()) {
                                    pkgCache::VerIterator cand = policy.GetCandidateVer(Pkg);
                                    if (!cand.end() && cand.Arch() != nullptr) {
                                        const char *a = cand.Arch();
                                        if (strcmp(a, "i586") == 0 || strcmp(a, "i386") == 0) {
                                            is_32bit_arch = true;
                                        }
                                    }
                                }

                                if (has_i586_variant || is_32bit_arch) {
                                    aliases.push_back(i586_variant);
                                    aliases.push_back(i586_variant + ".32bit");
                                }
                            }

                            if (!aliases.empty()) {
                                info.alias_count = aliases.size();
                                info.aliases = static_cast<char **>(calloc(info.alias_count, sizeof(char *)));
                                for (size_t ai = 0; ai < aliases.size(); ++ai) {
                                    info.aliases[ai] = safe_strdup(aliases[ai].c_str());
                                }
                            }
                        }

                        matched_packages.push_back(info);

                        // Limit results to prevent excessive memory usage
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
            result->count = 0;
            return make_result(APT_ERROR_UNKNOWN, "Failed to allocate memory for search results");
        }

        for (size_t i = 0; i < matched_packages.size(); ++i) {
            result->packages[i] = matched_packages[i];
        }

        return make_result(APT_SUCCESS);
    } catch (const std::exception &e) {
        if (result->packages) {
            for (size_t i = 0; i < result->count; ++i) {
                apt_free_package_info(&result->packages[i]);
            }
            free(result->packages);
            result->packages = nullptr;
        }
        result->count = 0;
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    } catch (...) {
        if (result && result->packages) {
            for (size_t i = 0; i < result->count; ++i) {
                apt_free_package_info(&result->packages[i]);
            }
            free(result->packages);
            result->packages = nullptr;
        }
        if (result) result->count = 0;
        return make_result(APT_ERROR_UNKNOWN, "Unknown exception in search");
    }
}

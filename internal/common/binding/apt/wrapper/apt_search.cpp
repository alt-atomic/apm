#include "apt_internal.h"

AptResult apt_search_packages(AptCache* cache, const char* pattern, AptPackageList* result) {
    if (!cache || !cache->dep_cache || !pattern || !result) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid parameters for search");
    }

    result->packages = nullptr;
    result->count = 0;

    try {
        pkgCache& Cache = cache->dep_cache->GetCache();

        pkgDepCache::Policy Plcy;

        regex_t compiled_pattern;
        if (regcomp(&compiled_pattern, pattern, REG_EXTENDED | REG_ICASE | REG_NOSUB) != 0) {
            return make_result(APT_ERROR_UNKNOWN, "Failed to compile regex pattern");
        }

        pkgRecords Recs(Cache);
        if (_error->PendingError() == true) {
            regfree(&compiled_pattern);
            return make_result(APT_ERROR_UNKNOWN, "Failed to create package records parser");
        }

        struct ExVerFile {
            pkgCache::VerFile *Vf;
            bool NameMatch;
        };

        ExVerFile *VFList = new ExVerFile[Cache.HeaderP->PackageCount + 1]();

        for (pkgCache::PkgIterator P = Cache.PkgBegin(); P.end() == false; P++) {
            VFList[P->ID].NameMatch = false;
            VFList[P->ID].Vf = nullptr;

            if (regexec(&compiled_pattern, P.Name(), 0, 0, 0) == 0) {
                VFList[P->ID].NameMatch = true;
            }

            pkgCache::VerIterator V = Plcy.GetCandidateVer(P);
            if (V.end() == false) {
                VFList[P->ID].Vf = V.FileList();
            }
        }

        for (pkgCache::PkgIterator P = Cache.PkgBegin(); P.end() == false; P++) {
            if (VFList[P->ID].NameMatch == false)
                continue;

            for (pkgCache::PrvIterator Prv = P.ProvidesList(); Prv.end() == false; Prv++) {
                pkgCache::VerIterator V = Plcy.GetCandidateVer(Prv.OwnerPkg());
                if (V.end() == false) {
                    VFList[Prv.OwnerPkg()->ID].Vf = V.FileList();
                    VFList[Prv.OwnerPkg()->ID].NameMatch = true;
                }
            }
        }

        auto LocalityCompare = [](const void *a, const void *b) -> int {
            const ExVerFile *A = (const ExVerFile *)a;
            const ExVerFile *B = (const ExVerFile *)b;
            if (A->Vf == nullptr && B->Vf == nullptr) return 0;
            if (A->Vf == nullptr) return 1;
            if (B->Vf == nullptr) return -1;
            return A->Vf->File - B->Vf->File;
        };

        qsort(VFList, Cache.HeaderP->PackageCount, sizeof(*VFList), LocalityCompare);

        std::vector<AptPackageInfo> matched_packages;
        std::set<std::string> seen_packages; // To avoid duplicates

        // Iterate over all the version records and check them (like in apt-cache)
        for (ExVerFile *J = VFList; J->Vf != nullptr; J++) {

            bool Match = true;
            pkgCache::VerFileIterator VF(Cache, J->Vf);

            try {
                pkgRecords::Parser &Parser = Recs.Lookup(VF);
                std::string pkg_name = Parser.Name();

                if (J->NameMatch == false) {
                    std::string LongDesc = Parser.LongDesc();
                    std::string ShortDesc = Parser.ShortDesc();

                    Match = false;
                    if (regexec(&compiled_pattern, LongDesc.c_str(), 0, 0, 0) == 0 ||
                        regexec(&compiled_pattern, ShortDesc.c_str(), 0, 0, 0) == 0) {
                        Match = true;
                    }
                }

                if (Match == true && !pkg_name.empty()) {
                    // Fold i586-* packages into their base if base REAL package exists (has candidate version)
                    // Do not emit separate entry in that case. If base is virtual-only, keep this entry (we will retitle name below).
                    if (pkg_name.rfind("i586-", 0) == 0) {
                        std::string base_name = pkg_name.substr(5);
                        pkgCache::PkgIterator base_pkg = Cache.FindPkg(base_name);
                        if (!base_pkg.end()) {
                            pkgCache::VerIterator base_ver = Plcy.GetCandidateVer(base_pkg);
                            if (!base_ver.end()) {
                                // Real base package exists, skip this i586-* duplicate
                                continue;
                            }
                        }
                    }
                    // Determine effective display name: if this is i586-* and base is virtual-only, use base name
                    std::string effective_name = pkg_name;
                    bool is_i586 = (pkg_name.rfind("i586-", 0) == 0);
                    if (is_i586) {
                        std::string base_name = pkg_name.substr(5);
                        pkgCache::PkgIterator base_pkg = Cache.FindPkg(base_name);
                        if (!base_pkg.end()) {
                            pkgCache::VerIterator base_ver = Plcy.GetCandidateVer(base_pkg);
                            if (base_ver.end()) {
                                effective_name = base_name;
                            }
                        }
                    }

                    if (seen_packages.find(effective_name) == seen_packages.end()) {
                        seen_packages.insert(effective_name);

                        // Find the package in cache for proper info
                        pkgCache::PkgIterator Pkg = Cache.FindPkg(pkg_name);

                        // Create package info
                        AptPackageInfo info;
                        memset(&info, 0, sizeof(AptPackageInfo));

                        // Get detailed information from parser first
                        std::string long_desc = Parser.LongDesc();
                        std::string short_desc = Parser.ShortDesc();

                        info.description = strdup(long_desc.c_str());
                        info.short_description = strdup(short_desc.c_str());
                        info.maintainer = strdup(Parser.Maintainer().c_str());
                        info.source_package = strdup(Parser.SourcePkg().c_str());
                        info.md5_hash = strdup(Parser.MD5Hash().c_str());
                        info.blake2b_hash = strdup(Parser.BLAKE2b().c_str());
                        info.filename = strdup(Parser.FileName().c_str());
                        info.changelog = strdup(Parser.Changelog().c_str());

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
                            info.homepage = strdup(homepage.c_str());
                        } else {
                            // leave homepage as nullptr if not present
                        }

                        // Basic package information: use effective_name as primary name
                        info.name = strdup(effective_name.c_str());
                        info.aliases = nullptr;
                        info.alias_count = 0;

                        if (!Pkg.end()) {
                            info.package_id = Pkg->ID;
                            if (Pkg.Section() != nullptr && *Pkg.Section() != '\0') {
                                info.section = strdup(Pkg.Section());
                            }
                            info.essential = (Pkg->Flags & pkgCache::Flag::Essential) != 0;
                            info.auto_installed = (Pkg->Flags & pkgCache::Flag::Auto) != 0;

                            // Package state
                            switch (Pkg->CurrentState) {
                                case pkgCache::State::NotInstalled:
                                    info.state = APT_PKG_STATE_NOT_INSTALLED;
                                    break;
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
                                    info.state = APT_PKG_STATE_NOT_INSTALLED;
                                    break;
                            }

                            // Resolve version: candidate; else fallback to record Version field
                            pkgCache::VerIterator Ver = Plcy.GetCandidateVer(Pkg);
                            if (!Ver.end()) {
                                if (Ver.VerStr() != nullptr && *Ver.VerStr() != '\0') {
                                    info.version = strdup(Ver.VerStr());
                                }
                                if (Ver.Arch() != nullptr && *Ver.Arch() != '\0') {
                                    info.architecture = strdup(Ver.Arch());
                                }
                                if (pkgCache::Priority(Ver->Priority) != nullptr && *pkgCache::Priority(Ver->Priority) != '\0') {
                                    info.priority = strdup(pkgCache::Priority(Ver->Priority));
                                }
                                info.installed_size = Ver->InstalledSize;
                                info.download_size = Ver->Size;
                                // Collect provided virtual names from this version
                                {
                                    std::set<std::string> prov_names;
                                    for (pkgCache::PrvIterator prv = Ver.ProvidesList(); !prv.end(); ++prv) {
                                        const char* n = prv.Name();
                                        if (n && *n) prov_names.insert(n);
                                    }
                                    if (!prov_names.empty()) {
                                        std::string joined;
                                        for (auto it = prov_names.begin(); it != prov_names.end(); ++it) {
                                            if (!joined.empty()) joined += ", ";
                                            joined += *it;
                                        }
                                        info.provides = strdup(joined.c_str());
                                    }
                                }
                                // Collect hard dependencies (Depends/PreDepends) from this version
                                {
                                    std::set<std::string> dep_names;
                                    for (pkgCache::DepIterator dep = Ver.DependsList(); !dep.end(); ++dep) {
                                        if (dep->Type != pkgCache::Dep::Depends && dep->Type != pkgCache::Dep::PreDepends) continue;
                                        pkgCache::PkgIterator tpkg = dep.TargetPkg();
                                        if (!tpkg.end() && tpkg.Name() != nullptr) {
                                            dep_names.insert(tpkg.Name());
                                        }
                                    }
                                    if (!dep_names.empty()) {
                                        std::string joined;
                                        for (auto it = dep_names.begin(); it != dep_names.end(); ++it) {
                                            if (!joined.empty()) joined += ", ";
                                            joined += *it;
                                        }
                                        info.depends = strdup(joined.c_str());
                                    }
                                }
                            } else {
                                if (!record_version.empty()) {
                                    info.version = strdup(record_version.c_str());
                                }
                                if (!record_arch.empty()) {
                                    info.architecture = strdup(record_arch.c_str());
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
                                        if (!deps.empty()) info.depends = strdup(deps.c_str());
                                    }
                                }
                            }
                        } else {
                            // Package not found in cache
                            info.package_id = 0;
                            // leave section nullptr
                            info.essential = false;
                            info.auto_installed = false;
                            info.state = APT_PKG_STATE_NOT_INSTALLED;
                            if (!record_version.empty()) {
                                info.version = strdup(record_version.c_str());
                            }
                            if (!record_arch.empty()) {
                                info.architecture = strdup(record_arch.c_str());
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
                                    if (!deps.empty()) info.depends = strdup(deps.c_str());
                                }
                            }
                        }

                        // Finalize auxiliary fields (use record-provides if nothing collected from cache)
                        if (info.provides == nullptr && !record_provides.empty()) info.provides = strdup(record_provides.c_str());

                        // Build aliases list for biarch naming (ALT): i586-<name>, <name>.32bit, i586-<name>.32bit
                        {
                            std::vector<std::string> aliases;
                            std::string current_name = Parser.Name();
                            std::string base_name_for_alias = effective_name;

                            // If this package is i586- prefixed and base exists, add that relation
                            bool has_i586_prefix = (!current_name.empty() && current_name.rfind("i586-", 0) == 0);
                            if (has_i586_prefix) {
                                std::string stripped = current_name.substr(5);
                                pkgCache::PkgIterator base_pkg = Cache.FindPkg(stripped);
                                if (!base_pkg.end()) {
                                    // Current is i586-<name>. Always add the provider name itself and its .32bit alias
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
                                    pkgCache::VerIterator cand = Plcy.GetCandidateVer(Pkg);
                                    if (!cand.end() && cand.Arch() != nullptr) {
                                        const char* a = cand.Arch();
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
                                info.aliases = (char**)calloc(info.alias_count, sizeof(char*));
                                for (size_t ai = 0; ai < aliases.size(); ++ai) {
                                    info.aliases[ai] = strdup(aliases[ai].c_str());
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
            } catch (...) {
                // Skip this version file if we can't parse it
                continue;
            }
        }

        // Clean up VFList and regex
        delete[] VFList;
        regfree(&compiled_pattern);

        if (matched_packages.empty()) {
            return make_result(APT_SUCCESS);
        }

        // Allocate result array
        result->count = matched_packages.size();
        result->packages = (AptPackageInfo*)calloc(result->count, sizeof(AptPackageInfo));
        if (!result->packages) {
            result->count = 0;
            return make_result(APT_ERROR_UNKNOWN, "Failed to allocate memory for search results");
        }

        // Copy results from matched_packages vector
        for (size_t i = 0; i < matched_packages.size(); ++i) {
            result->packages[i] = matched_packages[i];
        }

        return make_result(APT_SUCCESS);

    } catch (const std::exception& e) {
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
        if (result->packages) {
            for (size_t i = 0; i < result->count; ++i) {
                apt_free_package_info(&result->packages[i]);
            }
            free(result->packages);
            result->packages = nullptr;
        }
        result->count = 0;
        return make_result(APT_ERROR_UNKNOWN, "Unknown exception in search");
    }
}



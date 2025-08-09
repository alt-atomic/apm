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
                    if (seen_packages.find(pkg_name) == seen_packages.end()) {
                        seen_packages.insert(pkg_name);

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

                        // Parse homepage from full record
                        const char *rec_start, *rec_stop;
                        Parser.GetRec(rec_start, rec_stop);
                        std::string record(rec_start, rec_stop - rec_start);

                        size_t homepage_pos = record.find("Homepage: ");
                        if (homepage_pos != std::string::npos) {
                            size_t start = homepage_pos + 10;
                            size_t end = record.find('\n', start);
                            if (end == std::string::npos) end = record.length();
                            std::string homepage = record.substr(start, end - start);
                            info.homepage = strdup(homepage.c_str());
                        } else {
                            info.homepage = strdup("");
                        }

                        // Basic package information - use Parser.Name() directly
                        info.name = strdup(Parser.Name().c_str());

                        if (!Pkg.end()) {
                            info.package_id = Pkg->ID;
                            info.section = strdup(Pkg.Section() ? Pkg.Section() : "unknown");
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

                            // Get candidate version
                            pkgCache::VerIterator Ver = Plcy.GetCandidateVer(Pkg);
                            if (!Ver.end()) {
                                info.version = strdup(Ver.VerStr() ? Ver.VerStr() : "unknown");
                                info.architecture = strdup(Ver.Arch() ? Ver.Arch() : "unknown");
                                info.priority = strdup(pkgCache::Priority(Ver->Priority));
                                info.installed_size = Ver->InstalledSize;
                                info.download_size = Ver->Size;
                            } else {
                                info.version = strdup("unknown");
                                info.architecture = strdup("unknown");
                                info.priority = strdup("unknown");
                                info.installed_size = 0;
                                info.download_size = 0;
                            }
                        } else {
                            // Package not found in cache
                            info.package_id = 0;
                            info.section = strdup("unknown");
                            info.essential = false;
                            info.auto_installed = false;
                            info.state = APT_PKG_STATE_NOT_INSTALLED;
                            info.version = strdup("unknown");
                            info.architecture = strdup("unknown");
                            info.priority = strdup("unknown");
                            info.installed_size = 0;
                            info.download_size = 0;
                        }

                        // Simplified dependency info
                        info.depends = strdup("");
                        info.provides = strdup("");
                        info.conflicts = strdup("");
                        info.obsoletes = strdup("");
                        info.recommends = strdup("");
                        info.suggests = strdup("");

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



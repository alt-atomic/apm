#include "apt_internal.h"

AptResult apt_get_package_info(AptCache* cache, const char* package_name, AptPackageInfo* info) {
    if (!cache || !cache->dep_cache || !package_name || !info) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid parameters for get_package_info");
    }

    try {
        memset(info, 0, sizeof(AptPackageInfo));
        info->aliases = nullptr;
        info->alias_count = 0;

        std::string input(package_name);
        std::string requested;
        
        // Check if this is an RPM file path
        if (is_rpm_file(input)) {
            // This is an RPM file - we need to preprocess it and find the corresponding package
            const char* rpm_path = input.c_str();
            AptResult preprocess_result = apt_preprocess_install_arguments(&rpm_path, 1);
            if (preprocess_result.code != APT_SUCCESS) {
                return preprocess_result;
            }

            // Refresh cache to pick up the temporary index
            AptResult refresh_result = apt_cache_refresh(cache);
            if (refresh_result.code != APT_SUCCESS) {
                return refresh_result;
            }

            // Find the package that was added from the RPM file
            bool found_package = false;
            for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
                pkgDepCache::StateCache &state = (*cache->dep_cache)[iter];
                if (state.CandidateVer != 0) {
                    pkgCache::VerIterator ver = state.CandidateVerIter(*cache->dep_cache);
                    if (!ver.end()) {
                        for (pkgCache::VerFileIterator vf = ver.FileList(); !vf.end(); ++vf) {
                            pkgCache::PkgFileIterator file = vf.File();
                            // Check if this file corresponds to our RPM file
                            if (file.FileName() && input.find(file.FileName()) != std::string::npos) {
                                // This package comes from our RPM file
                                requested = iter.Name();
                                found_package = true;
                                goto found_rpm_package;
                            }
                        }
                    }
                }
            }
            found_rpm_package:
            
            if (!found_package) {
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND, 
                                  (std::string("Unable to find package from RPM file: ") + input).c_str());
            }
        } else {
            // Regular package name - normalize: strip .32bit alias suffix
            requested = input;
            if (!requested.empty() && requested.size() > 7 && requested.rfind(".32bit") == requested.size() - 7) {
                requested.erase(requested.size() - 7);
            }
        }

        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(requested.c_str());
        if (pkg.end()) {
            // Try common ALT biarch provider name: i586-<requested>
            std::string i586_name = std::string("i586-") + requested;
            pkg = cache->dep_cache->FindPkg(i586_name.c_str());
        }
        if (pkg.end()) {
            // Fallback: resolve via providers (candidate versions)
            pkgDepCache::Policy Plcy;
            for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); it.end() == false; ++it) {
                pkgCache::VerIterator cand = Plcy.GetCandidateVer(it);
                if (cand.end()) continue;
                for (pkgCache::PrvIterator prv = cand.ProvidesList(); !prv.end(); ++prv) {
                    const char* prov_name = prv.Name();
                    if (prov_name && (requested == prov_name || (package_name && std::string(package_name) == prov_name))) {
                        pkg = it;
                        break;
                    }
                }
                if (!pkg.end()) break;
            }
        }
        if (pkg.end()) {
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package not found: ") + requested).c_str());
        }

        // Use the requested normalized name as primary display name when it differs (to honor aliases)
        if (!requested.empty()) {
            info->name = strdup(requested.c_str());
        } else {
            info->name = strdup(pkg.Name());
        }
        info->package_id = pkg->ID;

        info->essential = (pkg->Flags & pkgCache::Flag::Essential) != 0;
        info->auto_installed = (pkg->Flags & pkgCache::Flag::Auto) != 0;

        info->section = strdup(pkg.Section() ? pkg.Section() : "unknown");

        bool is_installed = (pkg->CurrentState == pkgCache::State::Installed) || (pkg.CurrentVer().end() == false);
        if (!is_installed) {
            for (pkgCache::PrvIterator prv_iter = pkg.ProvidesList(); prv_iter.end() == false; ++prv_iter) {
                pkgCache::PkgIterator provider_pkg = prv_iter.OwnerPkg();
                if (provider_pkg->CurrentState == pkgCache::State::Installed || provider_pkg.CurrentVer().end() == false) {
                    is_installed = true;
                    break;
                }
            }
        }
        if (!is_installed) {
            for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); it.end() == false; ++it) {
                if (it->CurrentState != pkgCache::State::Installed && it.CurrentVer().end() == true) {
                    continue;
                }
                for (pkgCache::PrvIterator prv = it.CurrentVer().ProvidesList(); prv.end() == false; ++prv) {
                    if (strcmp(prv.Name(), package_name) == 0) {
                        is_installed = true;
                        break;
                    }
                }
                if (is_installed) break;
            }
        }
        if (is_installed) {
            info->state = APT_PKG_STATE_INSTALLED;
        } else {
            switch (pkg->CurrentState) {
                case pkgCache::State::ConfigFiles:
                    info->state = APT_PKG_STATE_CONFIG_FILES;
                    break;
                default:
                    info->state = APT_PKG_STATE_NOT_INSTALLED;
                    break;
            }
        }

        pkgDepCache::Policy Plcy;
        pkgCache::VerIterator candidate_ver = Plcy.GetCandidateVer(pkg);

        if (!candidate_ver.end()) {
            info->version = strdup(candidate_ver.VerStr());

            info->architecture = strdup(candidate_ver.Arch());

            for (pkgCache::VerFileIterator vf = candidate_ver.FileList(); !vf.end(); ++vf) {
                if (vf.File().Archive() != nullptr) {
                    info->download_size = candidate_ver->Size;
                    info->installed_size = candidate_ver->InstalledSize;
                    break;
                }
            }

            pkgRecords records(*cache->dep_cache);
            for (pkgCache::VerFileIterator vf = candidate_ver.FileList(); !vf.end(); ++vf) {
                pkgRecords::Parser& parser = records.Lookup(vf);

                std::string desc = parser.LongDesc();
                std::string short_desc = parser.ShortDesc();

                if (!desc.empty()) {
                    info->description = strdup(desc.c_str());
                }
                if (!short_desc.empty()) {
                    info->short_description = strdup(short_desc.c_str());
                }

                std::string maintainer = parser.Maintainer();
                if (!maintainer.empty()) {
                    info->maintainer = strdup(maintainer.c_str());
                }

                // Add missing fields from apt_search_packages
                std::string source_pkg = parser.SourcePkg();
                if (!source_pkg.empty()) {
                    info->source_package = strdup(source_pkg.c_str());
                }
                
                std::string md5_hash = parser.MD5Hash();
                if (!md5_hash.empty()) {
                    info->md5_hash = strdup(md5_hash.c_str());
                }
                
                std::string blake2b_hash = parser.BLAKE2b();
                if (!blake2b_hash.empty()) {
                    info->blake2b_hash = strdup(blake2b_hash.c_str());
                }
                
                std::string filename = parser.FileName();
                if (!filename.empty()) {
                    info->filename = strdup(filename.c_str());
                }
                
                std::string changelog = parser.Changelog();
                if (!changelog.empty()) {
                    info->changelog = strdup(changelog.c_str());
                }
                
                // Get Homepage from full record like in apt_search_packages
                const char *rec_start, *rec_stop;
                parser.GetRec(rec_start, rec_stop);
                std::string record(rec_start, rec_stop - rec_start);
                
                size_t homepage_pos = record.find("Homepage: ");
                if (homepage_pos != std::string::npos) {
                    size_t start = homepage_pos + 10;
                    size_t end = record.find('\n', start);
                    if (end == std::string::npos) end = record.length();
                    std::string homepage = record.substr(start, end - start);
                    info->homepage = strdup(homepage.c_str());
                }
                
                // Get Provides field
                size_t provides_pos = record.find("Provides: ");
                if (provides_pos != std::string::npos) {
                    size_t start = provides_pos + 10;
                    size_t end = record.find('\n', start);
                    if (end == std::string::npos) end = record.length();
                    std::string provides = record.substr(start, end - start);
                    info->provides = strdup(provides.c_str());
                }

                break;
            }
        } else {
            info->version = strdup("unknown");
            info->architecture = strdup("unknown");
            info->download_size = 0;
            info->installed_size = 0;
        }

        info->priority = strdup("normal");

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception& e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}



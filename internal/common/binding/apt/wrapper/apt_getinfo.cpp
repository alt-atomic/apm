#include "apt_internal.h"

AptResult apt_get_package_info(AptCache* cache, const char* package_name, AptPackageInfo* info) {
    if (!cache || !cache->dep_cache || !package_name || !info) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid parameters for get_package_info");
    }

    try {
        memset(info, 0, sizeof(AptPackageInfo));

        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(package_name);
        if (pkg.end()) {
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Package not found: ") + package_name).c_str());
        }

        info->name = strdup(pkg.Name());
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

        pkgDepCache::StateCache& state = (*cache->dep_cache)[pkg];
        pkgCache::VerIterator candidate_ver = state.CandidateVerIter(*cache->dep_cache);

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



#include "apt_internal.h"
#include "apt_filelist.h"

#include <apt-pkg/pkgrecords.h>

#include <cstring>

AptResult apt_get_package_info(AptCache *cache, const char *package_name, AptPackageInfo *info) {
    if (!cache || !cache->dep_cache || !package_name || !info) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid parameters for get_package_info");
    }

    try {
        LocaleGuard locale;
        memset(info, 0, sizeof(AptPackageInfo));
        info->aliases = nullptr;
        info->alias_count = 0;

        std::string input(package_name);
        std::string requested;

        // Check if this is an RPM file path
        if (is_rpm_file(input)) {
            const char *rpm_path = input.c_str();
            bool added_new = false;
            AptResult preprocess_result = apt_preprocess_install_arguments(&rpm_path, 1, &added_new);
            if (preprocess_result.code != APT_SUCCESS) {
                return preprocess_result;
            }

            if (added_new) {
                AptResult refresh_result = apt_cache_refresh(cache);
                if (refresh_result.code != APT_SUCCESS) {
                    return refresh_result;
                }
            }

            bool found_package = false;
            for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
                for (pkgCache::VerIterator ver = iter.VersionList(); !ver.end(); ++ver) {
                    for (pkgCache::VerFileIterator vf = ver.FileList(); !vf.end(); ++vf) {
                        pkgCache::PkgFileIterator file = vf.File();
                        if (file.FileName() && input.find(file.FileName()) != std::string::npos) {
                            requested = iter.Name();
                            found_package = true;
                            goto found_rpm_package;
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
            requested = input;
            if (!requested.empty() && requested.size() > 7 && requested.rfind(".32bit") == requested.size() - 7) {
                requested.erase(requested.size() - 7);
            }
        }

        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(requested);
        if (pkg.end()) {
            std::string i586_name = std::string("i586-") + requested;
            pkg = cache->dep_cache->FindPkg(i586_name);
        }
        if (pkg.end()) {
            // Fallback: resolve via providers (candidate versions)
            pkgDepCache::Policy policy;
            for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); it.end() == false; ++it) {
                pkgCache::VerIterator cand = policy.GetCandidateVer(it);
                if (cand.end()) continue;
                for (pkgCache::PrvIterator prv = cand.ProvidesList(); !prv.end(); ++prv) {
                    const char *prov_name = prv.Name();
                    if (prov_name && (requested == prov_name ||
                                      std::string(package_name) == prov_name)) {
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
            info->name = safe_strdup(requested.c_str());
        } else {
            info->name = safe_strdup(pkg.Name());
        }
        info->package_id = pkg->ID;

        info->essential = (pkg->Flags & pkgCache::Flag::Essential) != 0;
        info->auto_installed = (pkg->Flags & pkgCache::Flag::Auto) != 0;

        info->section = safe_strdup(pkg.Section() ? pkg.Section() : "unknown");

        // Package state - check CurrentVer()
        if (!pkg.CurrentVer().end()) {
            switch (pkg->CurrentState) {
                case pkgCache::State::Installed:
                    info->state = APT_PKG_STATE_INSTALLED;
                    break;
                case pkgCache::State::ConfigFiles:
                    info->state = APT_PKG_STATE_CONFIG_FILES;
                    break;
                case pkgCache::State::UnPacked:
                    info->state = APT_PKG_STATE_UNPACKED;
                    break;
                case pkgCache::State::HalfConfigured:
                    info->state = APT_PKG_STATE_HALF_CONFIGURED;
                    break;
                case pkgCache::State::HalfInstalled:
                    info->state = APT_PKG_STATE_HALF_INSTALLED;
                    break;
                default:
                    info->state = APT_PKG_STATE_INSTALLED;
                    break;
            }
        } else {
            info->state = APT_PKG_STATE_NOT_INSTALLED;
        }

        pkgDepCache::Policy Policy;
        pkgCache::VerIterator candidate_ver = Policy.GetCandidateVer(pkg);

        if (!candidate_ver.end()) {
            info->version = safe_strdup(candidate_ver.VerStr());

            info->architecture = safe_strdup(candidate_ver.Arch());

            for (pkgCache::VerFileIterator vf = candidate_ver.FileList(); !vf.end(); ++vf) {
                if (vf.File().Archive() != nullptr) {
                    info->download_size = candidate_ver->Size;
                    info->installed_size = candidate_ver->InstalledSize;
                    break;
                }
            }

            pkgRecords records(*cache->dep_cache);
            pkgCache::VerFileIterator vf = candidate_ver.FileList();
            if (!vf.end()) {
                pkgRecords::Parser &parser = records.Lookup(vf);

                std::string desc = parser.LongDesc();
                std::string short_desc = parser.ShortDesc();

                if (!desc.empty()) {
                    info->description = safe_strdup(desc.c_str());
                }
                if (!short_desc.empty()) {
                    info->short_description = safe_strdup(short_desc.c_str());
                }

                std::string maintainer = parser.Maintainer();
                if (!maintainer.empty()) {
                    info->maintainer = safe_strdup(maintainer.c_str());
                }

                std::string source_pkg = parser.SourcePkg();
                if (!source_pkg.empty()) {
                    info->source_package = safe_strdup(source_pkg.c_str());
                }

                std::string md5_hash = parser.MD5Hash();
                if (!md5_hash.empty()) {
                    info->md5_hash = safe_strdup(md5_hash.c_str());
                }

                std::string blake2b_hash = parser.BLAKE2b();
                if (!blake2b_hash.empty()) {
                    info->blake2b_hash = safe_strdup(blake2b_hash.c_str());
                }

                std::string filename = parser.FileName();
                if (!filename.empty()) {
                    info->filename = safe_strdup(filename.c_str());
                }

                std::string changelog = parser.Changelog();
                if (!changelog.empty()) {
                    info->changelog = safe_strdup(changelog.c_str());
                }

                const char *rec_start, *rec_stop;
                parser.GetRec(rec_start, rec_stop);
                std::string record(rec_start, rec_stop - rec_start);

                size_t homepage_pos = record.find("Homepage: ");
                if (homepage_pos != std::string::npos) {
                    size_t start = homepage_pos + 10;
                    size_t end = record.find('\n', start);
                    if (end == std::string::npos) end = record.length();
                    std::string homepage = record.substr(start, end - start);
                    info->homepage = safe_strdup(homepage.c_str());
                }

                size_t provides_pos = record.find("Provides: ");
                if (provides_pos != std::string::npos) {
                    size_t start = provides_pos + 10;
                    size_t end = record.find('\n', start);
                    if (end == std::string::npos) end = record.length();
                    std::string provides = record.substr(start, end - start);
                    info->provides = safe_strdup(provides.c_str());
                }
            }
        } else {
            info->version = safe_strdup("unknown");
            info->architecture = safe_strdup("unknown");
            info->download_size = 0;
            info->installed_size = 0;
        }

        info->priority = safe_strdup("normal");

        // Fill file list from repository index
        if (!candidate_ver.end()) {
            pkgCache::VerFileIterator vf = candidate_ver.FileList();
            AptFileListCache fl_cache;
            apt_filelist_fill(vf, info, fl_cache);
        }

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}

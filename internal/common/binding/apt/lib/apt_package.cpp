#include "apt_package.h"
#include "apt_cache.h"
#include "apt_ext_rpm.h"
#include "internal.h"
#include "error.h"
#include "package_fill.h"
#include "ext_rpm.h"

#include <cstring>

// Resolves a package (by name, virtual, or RPM file) and fills all metadata.
AptResult apt_package_get(AptCache *cache, const char *package_name, AptPackageInfo *info) {
    if (!cache || !cache->dep_cache || !package_name || !info) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_PACKAGE_INVALID_PARAMS);
    }

    try {
        LocaleGuard locale;
        memset(info, 0, sizeof(AptPackageInfo));

        const std::string input(package_name);
        std::string requested;

        if (is_rpm_file(input)) {
            const char *rpm_path = input.c_str();
            bool added_new = false;
            if (const AptResult preprocess_result = apt_preprocess_install_arguments(&rpm_path, 1, &added_new); preprocess_result.code != APT_SUCCESS) {
                return preprocess_result;
            }

            if (added_new) {
                if (const AptResult refresh_result = apt_cache_refresh(cache); refresh_result.code != APT_SUCCESS) {
                    return refresh_result;
                }
            }

            bool found_package = false;
            for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
                for (pkgCache::VerIterator ver = iter.VersionList(); !ver.end(); ++ver) {
                    for (pkgCache::VerFileIterator vf = ver.FileList(); !vf.end(); ++vf) {
                        if (pkgCache::PkgFileIterator file = vf.File(); file.FileName() && input.find(file.FileName()) != std::string::npos) {
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
            const std::string i586_name = std::string("i586-") + requested;
            pkg = cache->dep_cache->FindPkg(i586_name);
        }
        if (pkg.end()) {
            pkgDepCache::Policy policy;
            for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); it.end() == false; ++it) {
                pkgCache::VerIterator cand = policy.GetCandidateVer(it);
                if (cand.end()) continue;
                for (pkgCache::PrvIterator prv = cand.ProvidesList(); !prv.end(); ++prv) {
                    if (const char *prov_name = prv.Name(); prov_name && (requested == prov_name ||
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

        pkgDepCache::Policy policy;
        pkgCache::VerIterator candidate_ver = policy.GetCandidateVer(pkg);
        pkgRecords records(*cache->dep_cache);
        AptFileListCache fl_cache;

        pkgCache::VerFileIterator vf = candidate_ver.end()
            ? pkgCache::VerFileIterator()
            : candidate_ver.FileList();

        fill_package_info(cache->dep_cache->GetCache(), pkg, candidate_ver, vf,
                          records, policy, requested, info, fl_cache);

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}

// Frees all heap-allocated fields and zeroes the struct.
void apt_package_free(AptPackageInfo *info) {
    if (!info) return;

    free(info->name);
    free(info->version);
    free(info->description);
    free(info->short_description);
    free(info->section);
    free(info->architecture);
    free(info->maintainer);
    free(info->homepage);
    free(info->priority);
    free(info->md5_hash);
    free(info->blake2b_hash);
    free(info->source_package);
    free(info->changelog);
    free(info->filename);
    free(info->depends);
    free(info->provides);
    free(info->conflicts);
    free(info->obsoletes);
    free(info->recommends);
    free(info->suggests);
    if (info->aliases) {
        for (size_t i = 0; i < info->alias_count; ++i) {
            free(info->aliases[i]);
        }
        free(info->aliases);
    }
    if (info->files) {
        for (size_t i = 0; i < info->file_count; ++i) {
            free(info->files[i]);
        }
        free(info->files);
    }

    memset(info, 0, sizeof(AptPackageInfo));
}

// Frees each package info entry and the array itself.
void apt_packages_free(AptPackageList *list) {
    if (!list || !list->packages) return;

    for (size_t i = 0; i < list->count; ++i) {
        apt_package_free(&list->packages[i]);
    }

    free(list->packages);
    list->packages = nullptr;
    list->count = 0;
}

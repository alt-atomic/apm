#include "mark.h"
#include "resolve.h"
#include "ext_rpm.h"

#include <cstring>

// Resolves each name (incl. file paths, version specs), marks for install.
AptResult process_package_installs(const AptCache *cache,
                                   const char **install_names,
                                   const size_t install_count,
                                   std::set<std::string> &requested_install) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_CACHE);
    }
    if (!install_names || install_count == 0) {
        return make_result(APT_SUCCESS, nullptr);
    }

    for (size_t i = 0; i < install_count; i++) {
        if (!install_names[i]) continue;

        std::string raw(install_names[i]);

        if (!raw.empty() && raw[0] == '/' && !is_rpm_file(raw)) {
            if (const AptResult result = resolve_file_to_package(cache, raw); result.code != APT_SUCCESS) {
                return result;
            }
        }

        RequirementSpec req = parse_requirement(raw);

        pkgCache::PkgIterator pkg;
        AptResult result = find_install_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = resolve_virtual_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        if (req.has_version && req.name == pkg.Name()) {
            result = set_candidate_version_for_requirement(cache, pkg, req);
            if (result.code != APT_SUCCESS) {
                return result;
            }
        }

        requested_install.insert(pkg.Name());

        cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, false);

        // Install it with autoinstalling enabled if broken
        if (pkgDepCache::StateCache &State = (*cache->dep_cache)[pkg]; State.InstBroken()) {
            cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::DontChange, true);
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

// Validates packages are installed and downloadable, then marks for reinstall.
AptResult process_package_reinstalls(const AptCache *cache,
                                     const char **reinstall_names,
                                     const size_t reinstall_count,
                                     std::set<std::string> &requested_reinstall) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_CACHE);
    }
    if (!reinstall_names || reinstall_count == 0) {
        return make_result(APT_SUCCESS, nullptr);
    }

    for (size_t i = 0; i < reinstall_count; i++) {
        if (!reinstall_names[i]) continue;

        std::string raw(reinstall_names[i]);
        RequirementSpec req = parse_requirement(raw);

        pkgCache::PkgIterator pkg;

        if (is_rpm_file(raw)) {
            std::string pkg_name;
            bool found = false;

            for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
                for (pkgCache::VerIterator ver = iter.VersionList(); !ver.end(); ++ver) {
                    for (pkgCache::VerFileIterator vf = ver.FileList(); !vf.end(); ++vf) {
                        if (pkgCache::PkgFileIterator file = vf.File(); file.FileName() && raw.find(file.FileName()) != std::string::npos) {
                            pkg_name = iter.Name();
                            found = true;
                            goto found_reinstall_pkg;
                        }
                    }
                }
            }
        found_reinstall_pkg:

            if (!found) {
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                                   (std::string("Unable to find package from RPM file: ") + raw).c_str());
            }

            pkg = cache->dep_cache->FindPkg(pkg_name);
            if (pkg.end()) {
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                                   (std::string("Package ") + pkg_name + " is not installed, so cannot be reinstalled").
                                   c_str());
            }
        } else {
            if (const AptResult result = find_install_package(cache, req, pkg); result.code != APT_SUCCESS) {
                return result;
            }
        }

        if (pkg->CurrentVer == 0) {
            return make_result(APT_ERROR_PACKAGE_NOT_FOUND,
                               (std::string("Package ") + pkg.Name() + " is not installed, so cannot be reinstalled").
                               c_str());
        }

        if (!is_rpm_file(raw)) {
            if (pkgCache::VerIterator currentVer = pkg.CurrentVer(); !currentVer.Downloadable()) {
                return make_result(APT_ERROR_DOWNLOAD_FAILED,
                                   (std::string("Reinstallation of ") + pkg.Name() + " " +
                                    currentVer.VerStr() + " is not possible, it cannot be downloaded").c_str());
            }
        }

        requested_reinstall.insert(pkg.Name());
        cache->dep_cache->SetReInstall(pkg, true);
    }

    return make_result(APT_SUCCESS, nullptr);
}

// Resolves each name (incl. virtual packages), marks for deletion.
AptResult process_package_removals(const AptCache *cache,
                                   const char **remove_names,
                                   const size_t remove_count,
                                   const bool purge,
                                   std::set<std::string> &requested_remove,
                                   std::vector<std::pair<std::string, pkgCache::PkgIterator> > &remove_targets) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_CACHE);
    }
    if (!remove_names || remove_count == 0) {
        return make_result(APT_SUCCESS, nullptr);
    }

    for (size_t i = 0; i < remove_count; i++) {
        if (!remove_names[i]) continue;

        std::string raw(remove_names[i]);
        RequirementSpec req = parse_requirement(raw);

        pkgCache::PkgIterator pkg;
        AptResult result = find_remove_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        result = resolve_virtual_remove_package(cache, req, pkg);
        if (result.code != APT_SUCCESS) {
            return result;
        }

        requested_remove.insert(pkg.Name());

        cache->dep_cache->MarkDelete(pkg, purge);
        remove_targets.emplace_back(pkg.Name(), pkg);

    }

    return make_result(APT_SUCCESS, nullptr);
}

#include "mark.h"
#include "resolve.h"
#include "ext_rpm.h"

#include <cstring>

// Expands globs in place; a glob with no matches stays as is so the resolver reports it
static std::vector<std::string> expand_globs(const AptCache *cache, const std::vector<std::string> &names, const bool installed_only) {
    std::vector<std::string> expanded;
    expanded.reserve(names.size());
    for (const auto &name : names) {
        if (!is_glob(name)) {
            expanded.push_back(name);
            continue;
        }
        const size_t before = expanded.size();
        expand_glob(cache, name, installed_only, expanded);
        if (expanded.size() == before) expanded.push_back(name);
    }
    return expanded;
}

// Resolves each name (incl. file paths, version specs, globs), marks for install.
AptResult process_package_installs(const AptCache *cache,
                                   const std::vector<std::string> &install_names,
                                   std::set<std::string> &requested_install) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_CACHE);
    }
    if (install_names.empty()) {
        return make_result(APT_SUCCESS, nullptr);
    }

    const std::vector<std::string> names = expand_globs(cache, install_names, false);
    std::vector<pkgCache::PkgIterator> marked;
    marked.reserve(names.size());

    for (const auto &name : names) {
        std::string raw(name);

        // try the path as a file only when no package (incl. virtual) has this name
        if (!raw.empty() && raw[0] == '/' && !is_rpm_file(raw) && cache->dep_cache->FindPkg(raw).end()) {
            const AptResult result = resolve_file_to_package(cache, raw);
            free(result.message);
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

        cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::DontChange, false);
        marked.push_back(pkg);
    }

    for (pkgCache::PkgIterator &pkg: marked) {
        cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::Manual, false);
        if (pkgDepCache::StateCache &State = (*cache->dep_cache)[pkg]; State.InstBroken()) {
            cache->dep_cache->MarkInstall(pkg, pkgDepCache::AutoMarkFlag::DontChange, true);
        }
    }

    return make_result(APT_SUCCESS, nullptr);
}

// Validates packages are installed and downloadable, then marks for reinstall.
AptResult process_package_reinstalls(const AptCache *cache,
                                     const std::vector<std::string> &reinstall_names,
                                     std::set<std::string> &requested_reinstall) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_CACHE);
    }
    if (reinstall_names.empty()) {
        return make_result(APT_SUCCESS, nullptr);
    }

    for (const auto &name : expand_globs(cache, reinstall_names, true)) {
        std::string raw(name);

        // a local rpm provides its own path and resolves like a virtual package
        RequirementSpec req;
        if (is_rpm_file(raw)) {
            req.name = raw;
        } else {
            req = parse_requirement(raw);
        }

        pkgCache::PkgIterator pkg;
        AptResult result = find_install_package(cache, req, pkg);
        if (result.code == APT_SUCCESS) {
            result = resolve_virtual_package(cache, req, pkg);
        }
        if (result.code != APT_SUCCESS) {
            return result;
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

// Resolves each name (incl. virtual packages and globs), marks for deletion.
AptResult process_package_removals(const AptCache *cache,
                                   const std::vector<std::string> &remove_names,
                                   const bool purge,
                                   const bool idempotent,
                                   std::set<std::string> &requested_remove,
                                   std::vector<std::string> &skipped) {
    if (!cache || !cache->dep_cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_CACHE);
    }
    if (remove_names.empty()) {
        return make_result(APT_SUCCESS, nullptr);
    }

    for (const auto &raw : expand_globs(cache, remove_names, true)) {
        RequirementSpec req = parse_requirement(raw);

        pkgCache::PkgIterator pkg;
        AptResult result = find_remove_package(cache, req, pkg);
        if (result.code == APT_SUCCESS) {
            result = resolve_virtual_remove_package(cache, req, pkg);
        }

        // a known package that is not installed is not an error, an unknown name is
        if (result.code == APT_ERROR_PACKAGE_NOT_FOUND && idempotent) {
            free(result.message);
            if (!is_glob(raw) && cache->dep_cache->FindPkg(req.name).end()) {
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND, (std::string("Couldn't find package ") + req.name).c_str());
            }
            skipped.push_back(raw);
            continue;
        }
        if (result.code != APT_SUCCESS) {
            return result;
        }

        requested_remove.insert(pkg.Name());
        cache->dep_cache->MarkDelete(pkg, purge);
    }

    return make_result(APT_SUCCESS, nullptr);
}

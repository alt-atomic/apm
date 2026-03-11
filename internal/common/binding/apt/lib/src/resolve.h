#pragma once

#include "internal.h"
#include "error.h"

#include <apt-pkg/pkgcache.h>

#include <string>

// Parsed version requirement (e.g., "pkg>=1.0" -> name="pkg", op=">=", version="1.0").
struct RequirementSpec {
    std::string name;
    bool has_version{false};
    int op{0};
    std::string version;
};

// Parses a package requirement string like "name>=version" into a RequirementSpec.
RequirementSpec parse_requirement(const std::string &raw);

// Finds a package for installation, handling virtual packages and version constraints.
AptResult find_install_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &result_pkg);

// Resolves a virtual package to its best real provider.
AptResult resolve_virtual_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &pkg);

// Resolves a file path to its owning package name (modifies `name` in place).
AptResult resolve_file_to_package(const AptCache *cache, std::string &name);

// Finds an installed package for removal.
AptResult find_remove_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &result_pkg);

// Resolves a virtual package for removal to its installed provider.
AptResult resolve_virtual_remove_package(const AptCache *cache, const RequirementSpec &req, pkgCache::PkgIterator &pkg);

// Pins the candidate version of `pkg` to match `req`.
AptResult set_candidate_version_for_requirement(const AptCache *cache, pkgCache::PkgIterator &pkg, const RequirementSpec &req);

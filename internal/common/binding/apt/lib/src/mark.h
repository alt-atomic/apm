#pragma once

#include "internal.h"

#include <set>
#include <string>
#include <utility>
#include <vector>

// Resolves and marks `install_names` for installation in the dep cache.
// Populates `requested_install` with resolved package names.
AptResult process_package_installs(const AptCache *cache,
                                   const char **install_names,
                                   size_t install_count,
                                   std::set<std::string> &requested_install);

// Resolves and marks `reinstall_names` for reinstallation.
// Validates that each package is installed and downloadable.
AptResult process_package_reinstalls(const AptCache *cache,
                                     const char **reinstall_names,
                                     size_t reinstall_count,
                                     std::set<std::string> &requested_reinstall);

// Resolves and marks `remove_names` for removal. If `purge` is true, also
// removes configuration files. Populates `remove_targets` with resolved iterators.
AptResult process_package_removals(const AptCache *cache,
                                   const char **remove_names,
                                   size_t remove_count,
                                   bool purge,
                                   std::set<std::string> &requested_remove,
                                   std::vector<std::pair<std::string, pkgCache::PkgIterator> > &remove_targets);

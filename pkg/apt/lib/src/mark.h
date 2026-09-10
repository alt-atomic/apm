#pragma once

#include "internal.h"

#include <set>
#include <string>
#include <vector>

// Resolves and marks `install_names` for installation; resolved names go to `requested_install`.
AptResult process_package_installs(const AptCache *cache,
                                   const std::vector<std::string> &install_names,
                                   std::set<std::string> &requested_install);

// Resolves and marks `reinstall_names` for reinstallation; each must be installed and downloadable.
AptResult process_package_reinstalls(const AptCache *cache,
                                     const std::vector<std::string> &reinstall_names,
                                     std::set<std::string> &requested_reinstall);

// Resolves and marks `remove_names` for removal; with `idempotent`, not installed names go to `skipped`.
AptResult process_package_removals(const AptCache *cache,
                                   const std::vector<std::string> &remove_names,
                                   bool purge,
                                   bool idempotent,
                                   std::set<std::string> &requested_remove,
                                   std::vector<std::string> &skipped);

#pragma once

#include "internal.h"

#include <set>
#include <string>

// Checks for mutual conflicts among `requested_install` packages.
AptResult check_package_conflicts(const AptCache *cache, const std::set<std::string> &requested_install);

// Auto-marks critical dependencies for `requested_install` packages.
AptResult preprocess_installs(const AptCache *cache, const std::set<std::string> &requested_install);

// Runs the problem resolver and verifies no broken dependencies remain.
AptResult finalize_dependency_resolution(const AptCache *cache, const std::set<std::string> &requested_install,
                                         const std::set<std::string> &requested_remove, bool remove_depends = false);

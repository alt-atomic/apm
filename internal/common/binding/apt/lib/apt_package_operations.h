#pragma once

#include "apt_internal.h"
#include <apt-pkg/algorithms.h>
#include <set>
#include <vector>
#include <string>
#include <utility>

struct RequirementSpec {
    std::string name;
    bool has_version{false};
    int op{0};
    std::string version;
};

RequirementSpec parse_requirement(const std::string &raw);

AptResult process_package_installs(AptCache *cache,
                                   const char **install_names,
                                   size_t install_count,
                                   std::set<std::string> &requested_install);

AptResult process_package_reinstalls(AptCache *cache,
                                     const char **reinstall_names,
                                     size_t reinstall_count,
                                     std::set<std::string> &requested_reinstall);

AptResult process_package_removals(AptCache *cache,
                                   const char **remove_names,
                                   size_t remove_count,
                                   bool purge,
                                   std::set<std::string> &requested_remove,
                                   std::vector<std::pair<std::string, pkgCache::PkgIterator> > &remove_targets);

AptResult check_package_conflicts(AptCache* cache, const std::set<std::string>& requested_install);

AptResult preprocess_installs(AptCache *cache, const std::set<std::string> &requested_install);

AptResult preprocess_removals(AptCache *cache, const std::set<std::string> &requested_remove);

AptResult finalize_dependency_resolution(AptCache *cache, const std::set<std::string> &requested_install, const std::set<std::string> &requested_remove, bool remove_depends = false);

void collect_package_changes(AptCache *cache,
                             const std::set<std::string> &requested_install,
                             const std::set<std::string> &requested_remove,
                             std::vector<std::string> &extra_installed,
                             std::vector<std::string> &extra_removed,
                             std::vector<std::string> &upgraded,
                             std::vector<std::string> &new_installed,
                             std::vector<std::string> &removed,
                             uint64_t &download_size,
                             uint64_t &install_size);

void populate_changes_structure(AptPackageChanges *changes,
                                const std::vector<std::string> &extra_installed,
                                const std::vector<std::string> &upgraded,
                                const std::vector<std::string> &new_installed,
                                const std::vector<std::string> &removed,
                                uint64_t download_size,
                                uint64_t install_size);

#pragma once

#include "apt_internal.h"

#include <apt-pkg/pkgcache.h>

#include <set>
#include <string>
#include <utility>
#include <vector>

struct RequirementSpec {
    std::string name;
    bool has_version{false};
    int op{0};
    std::string version;
};

RequirementSpec parse_requirement(const std::string &raw);

AptResult process_package_installs(const AptCache *cache,
                                   const char **install_names,
                                   size_t install_count,
                                   std::set<std::string> &requested_install);

AptResult process_package_reinstalls(const AptCache *cache,
                                     const char **reinstall_names,
                                     size_t reinstall_count,
                                     std::set<std::string> &requested_reinstall);

AptResult process_package_removals(const AptCache *cache,
                                   const char **remove_names,
                                   size_t remove_count,
                                   bool purge,
                                   std::set<std::string> &requested_remove,
                                   std::vector<std::pair<std::string, pkgCache::PkgIterator> > &remove_targets);

AptResult check_package_conflicts(const AptCache *cache, const std::set<std::string> &requested_install);

AptResult preprocess_installs(const AptCache *cache, const std::set<std::string> &requested_install);

AptResult finalize_dependency_resolution(const AptCache *cache, const std::set<std::string> &requested_install,
                                         const std::set<std::string> &requested_remove, bool remove_depends = false);

void collect_essential_packages(const AptCache *cache,
                                std::vector<std::pair<std::string, std::string>> &essential_list);

void collect_package_changes(const AptCache *cache,
                             const std::set<std::string> &requested_install,
                             std::vector<std::string> &extra_installed,
                             std::vector<std::string> &upgraded,
                             std::vector<std::string> &new_installed,
                             std::vector<std::string> &removed,
                             std::vector<std::string> &kept_back,
                             uint64_t &download_size,
                             int64_t &install_size);

void populate_changes_structure(AptPackageChanges *changes,
                                const std::vector<std::string> &extra_installed,
                                const std::vector<std::string> &upgraded,
                                const std::vector<std::string> &new_installed,
                                const std::vector<std::string> &removed,
                                const std::vector<std::string> &kept_back,
                                size_t not_upgraded_count,
                                const std::vector<std::pair<std::string, std::string>> &essential_list,
                                uint64_t download_size,
                                int64_t install_size);

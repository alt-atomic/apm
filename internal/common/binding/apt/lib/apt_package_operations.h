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

AptResult process_package_removals(AptCache *cache,
                                   const char **remove_names,
                                   size_t remove_count,
                                   bool purge,
                                   std::set<std::string> &requested_remove,
                                   std::vector<std::pair<std::string, pkgCache::PkgIterator> > &remove_targets);

AptResult check_package_conflicts(AptCache* cache, const std::set<std::string>& requested_install);

AptResult preprocess_dependencies(AptCache *cache, const std::set<std::string> &requested_install);

AptResult resolve_dependencies(AptCache *cache, bool remove_depends = false);

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

//AptResult validate_install_requests(AptCache* cache,
//                                    const std::set<std::string>& requested_install,
//                                    const std::vector<std::string>& new_installed,
//                                    const std::vector<std::string>& upgraded);

AptResult validate_remove_requests(AptCache *cache,
                                   const std::vector<std::pair<std::string, pkgCache::PkgIterator> > &remove_targets,
                                   const std::vector<std::string> &removed);

void populate_changes_structure(AptPackageChanges *changes,
                                const std::vector<std::string> &extra_installed,
                                const std::vector<std::string> &upgraded,
                                const std::vector<std::string> &new_installed,
                                const std::vector<std::string> &removed,
                                uint64_t download_size,
                                uint64_t install_size);

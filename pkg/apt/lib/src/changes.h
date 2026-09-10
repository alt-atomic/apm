#pragma once

#include "internal.h"

#include <set>
#include <string>
#include <utility>
#include <vector>

// Marked changes gathered from the dep cache before they are copied into AptPackageChanges
struct PlanChanges {
    std::vector<std::string> extra_installed, upgraded, new_installed, removed, kept_back;
    std::vector<std::pair<std::string, std::string>> essential;
    uint64_t download_size = 0;
    int64_t install_size = 0;
};

// Collects marked changes and essential packages; extra_installed is new_installed minus requested_install
PlanChanges collect_plan_changes(const AptCache *cache, const std::set<std::string> &requested_install);

// Copies plan into changes; kept_back_packages is filled only when keep_kept_back, its count always
void populate_changes(AptPackageChanges *changes, const PlanChanges &plan,
                      const std::vector<std::string> &skipped,
                      const std::vector<std::string> &requested_install,
                      const std::vector<std::string> &requested_remove,
                      bool keep_kept_back);

// Categorizes dep cache changes into install/upgrade/remove/kept-back lists.
// Computes total `download_size` and net `install_size`.
void collect_package_changes(const AptCache *cache,
                             const std::set<std::string> &requested_install,
                             std::vector<std::string> &extra_installed,
                             std::vector<std::string> &upgraded,
                             std::vector<std::string> &new_installed,
                             std::vector<std::string> &removed,
                             std::vector<std::string> &kept_back,
                             uint64_t &download_size,
                             int64_t &install_size);

// Finds essential packages marked for removal and appends (name, reason) pairs.
void collect_essential_packages(const AptCache *cache,
                                std::vector<std::pair<std::string, std::string>> &essential_list);

// Converts C++ vectors into the C AptPackageChanges structure.
void populate_changes_structure(AptPackageChanges *changes,
                                const std::vector<std::string> &extra_installed,
                                const std::vector<std::string> &upgraded,
                                const std::vector<std::string> &new_installed,
                                const std::vector<std::string> &removed,
                                const std::vector<std::string> &kept_back,
                                const std::vector<std::string> &skipped,
                                const std::vector<std::string> &requested_install,
                                const std::vector<std::string> &requested_remove,
                                size_t not_upgraded_count,
                                const std::vector<std::pair<std::string, std::string>> &essential_list,
                                uint64_t download_size,
                                int64_t install_size);

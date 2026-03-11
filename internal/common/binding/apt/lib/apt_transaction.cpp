#include "apt_transaction.h"
#include "planner.h"
#include "executor.h"
#include "changes.h"

#include <apt-pkg/algorithms.h>
#include <apt-pkg/depcache.h>

#include <memory>
#include <vector>
#include <set>
#include <string>
#include <cstring>

// Accumulates package names and flags for a single transaction.
struct AptTransaction {
    AptCache *cache{};
    std::vector<std::string> install_names;
    std::vector<std::string> remove_names;
    std::vector<std::string> reinstall_names;
    bool purge = false;
    bool remove_depends = false;
    bool is_dist_upgrade = false;
    bool is_autoremove = false;
};

// Allocates a new transaction bound to the given cache.
AptResult apt_transaction_new(AptCache *cache, AptTransaction **tx) {
    if (!cache || !tx) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_PARAMS);
    }

    *tx = new (std::nothrow) AptTransaction();
    if (!*tx) {
        return make_result(APT_ERROR_UNKNOWN, APT_MSG_TX_ALLOC_FAILED);
    }
    (*tx)->cache = cache;

    return make_result(APT_SUCCESS, nullptr);
}

// Frees a previously allocated transaction.
void apt_transaction_free(const AptTransaction *tx) {
    delete tx;
}

// Appends package names to the installation list.
AptResult apt_transaction_install(AptTransaction *tx, const char **names, const size_t count) {
    if (!tx) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NULL_TRANSACTION);
    if (!names || count == 0) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NO_PACKAGE_NAMES);

    for (size_t i = 0; i < count; i++) {
        if (names[i]) {
            tx->install_names.emplace_back(names[i]);
        }
    }
    return make_result(APT_SUCCESS, nullptr);
}

// Appends package names to the remove list with purge/depends on flags.
AptResult apt_transaction_remove(AptTransaction *tx, const char **names, const size_t count,
                                  const bool purge, const bool remove_depends) {
    if (!tx) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NULL_TRANSACTION);
    if (!names || count == 0) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NO_PACKAGE_NAMES);

    for (size_t i = 0; i < count; i++) {
        if (names[i]) {
            tx->remove_names.emplace_back(names[i]);
        }
    }
    tx->purge = purge;
    tx->remove_depends = remove_depends;
    return make_result(APT_SUCCESS, nullptr);
}

// Appends package names to the reinstallation list.
AptResult apt_transaction_reinstall(AptTransaction *tx, const char **names, size_t count) {
    if (!tx) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NULL_TRANSACTION);
    if (!names || count == 0) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NO_PACKAGE_NAMES);

    for (size_t i = 0; i < count; i++) {
        if (names[i]) {
            tx->reinstall_names.emplace_back(names[i]);
        }
    }
    return make_result(APT_SUCCESS, nullptr);
}

// Marks the transaction as a distribution upgrade.
AptResult apt_transaction_dist_upgrade(AptTransaction *tx) {
    if (!tx) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NULL_TRANSACTION);
    tx->is_dist_upgrade = true;
    return make_result(APT_SUCCESS, nullptr);
}

// Marks the transaction as an auto remove operation.
AptResult apt_transaction_autoremove(AptTransaction *tx) {
    if (!tx) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NULL_TRANSACTION);
    tx->is_autoremove = true;
    return make_result(APT_SUCCESS, nullptr);
}

// RAII guard that restores dep cache state on destruction.
class CacheStateGuard {
    std::unique_ptr<pkgDepCache::State> saved_state;

 public:
    explicit CacheStateGuard(pkgDepCache *cache) : saved_state(std::make_unique<pkgDepCache::State>(cache)) {
    }
    ~CacheStateGuard() { if (saved_state) saved_state->Restore(); }
    CacheStateGuard(const CacheStateGuard &) = delete;
    CacheStateGuard &operator=(const CacheStateGuard &) = delete;
};

// Converts a vector of strings to a vector of C string pointers.
static std::vector<const char *> to_cstr_array(const std::vector<std::string> &v) {
    std::vector<const char *> result(v.size());
    for (size_t i = 0; i < v.size(); i++) {
        result[i] = v[i].c_str();
    }
    return result;
}

// Simulates a distribution upgrade and collects the resulting changes.
static AptResult plan_dist_upgrade(const AptCache *cache, AptPackageChanges *changes) {
    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_CACHE_FILE_NOT_AVAILABLE);
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        CacheStateGuard stateGuard(cache->dep_cache);

        pkgDistUpgrade(*cache->dep_cache);

        if (cache->dep_cache->BrokenCount() > 0) {
            pkgProblemResolver Fix2(cache->dep_cache);
            Fix2.InstallProtect();
            (void) Fix2.Resolve(true);
        }
        if (cache->dep_cache->BrokenCount() > 0) {
            for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
                pkgDepCache::StateCache &st = (*cache->dep_cache)[it];
                if (st.InstBroken() || st.NowBroken()) {
                    const std::string out = std::string(
                                          "Some broken packages were found while trying to process build-dependencies for ")
                                      + it.Name();
                    return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
                }
            }
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, APT_MSG_BROKEN_DEPS);
        }

        if (!check_apt_errors()) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, nullptr);
        }

        const std::set<std::string> empty_set;
        std::vector<std::string> extra_installed, upgraded, new_installed, removed, kept_back;
        uint64_t download_size = 0;
        int64_t install_size = 0;

        collect_package_changes(cache, empty_set,
                                extra_installed, upgraded,
                                new_installed, removed, kept_back, download_size, install_size);
        extra_installed.clear();

        std::vector<std::pair<std::string, std::string>> essential_list;
        collect_essential_packages(cache, essential_list);

        populate_changes_structure(changes, extra_installed, upgraded, new_installed, removed,
                                   kept_back, kept_back.size(), essential_list, download_size, install_size);

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Dist upgrade simulation failed: ") + e.what()).c_str());
    }
}

// Simulates autoremove and collects packages that would be removed.
static AptResult plan_autoremove(const AptCache *cache, AptPackageChanges *changes) {
    if (!cache->cache_file) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_CACHE_FILE_NOT_AVAILABLE);
    }

    try {
        memset(changes, 0, sizeof(AptPackageChanges));

        CacheStateGuard stateGuard(cache->dep_cache);

        if (cache->dep_cache->BrokenCount() != 0) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, APT_MSG_BROKEN_CACHE);
        }

        std::set<std::string> kept_packages;
        std::set<std::string> unneeded_packages;

        if (!pkgAutoremoveGetKeptAndUnneededPackages(*cache->dep_cache, &kept_packages, &unneeded_packages)) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, APT_MSG_AUTOREMOVE_CALC_FAILED);
        }

        pkgProblemResolver Fix(cache->dep_cache);

        for (pkgCache::PkgIterator pkg = cache->dep_cache->PkgBegin(); !pkg.end(); ++pkg) {
            if (pkg->CurrentState != pkgCache::State::Installed) {
                continue;
            }

            if (kept_packages.count(pkg.Name()) != 0) {
                cache->dep_cache->MarkKeep(pkg);
                Fix.Protect(pkg);
            } else {
                cache->dep_cache->MarkDelete(pkg, false);
            }
        }

        if (!Fix.Resolve(false)) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, APT_MSG_AUTOREMOVE_RESOLVE_FAILED);
        }

        if (!check_apt_errors()) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, nullptr);
        }

        const std::set<std::string> empty_set;
        std::vector<std::string> extra_installed, upgraded, new_installed, removed, kept_back;
        uint64_t download_size = 0;
        int64_t install_size = 0;

        collect_package_changes(cache, empty_set,
                                extra_installed, upgraded,
                                new_installed, removed, kept_back, download_size, install_size);

        extra_installed.clear();
        const size_t total_not_upgraded = kept_back.size();
        kept_back.clear();

        std::vector<std::pair<std::string, std::string>> essential_list;
        collect_essential_packages(cache, essential_list);

        populate_changes_structure(changes, extra_installed, upgraded, new_installed, removed,
                                   kept_back, total_not_upgraded, essential_list, download_size, install_size);

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Autoremove simulation failed: ") + e.what()).c_str());
    }
}

// Dispatches to the appropriate planner based on transaction type.
AptResult apt_transaction_plan(const AptTransaction *tx, AptPackageChanges *changes) {
    if (!tx || !changes) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_PARAMS);

    if (tx->is_dist_upgrade) {
        return plan_dist_upgrade(tx->cache, changes);
    }

    if (tx->is_autoremove) {
        return plan_autoremove(tx->cache, changes);
    }

    auto inst = to_cstr_array(tx->install_names);
    auto rem = to_cstr_array(tx->remove_names);
    auto reinst = to_cstr_array(tx->reinstall_names);

    return plan_change_internal(
        tx->cache,
        inst.empty() ? nullptr : inst.data(), inst.size(),
        rem.empty() ? nullptr : rem.data(), rem.size(),
        reinst.empty() ? nullptr : reinst.data(), reinst.size(),
        tx->purge, tx->remove_depends,
        false,
        changes);
}

// Plans changes in apply mode, then downloads and installs packages.
AptResult apt_transaction_execute(const AptTransaction *tx,
                                   const AptProgressCallback callback, const uintptr_t user_data,
                                   const bool download_only) {
    if (!tx) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NULL_TRANSACTION);

    if (tx->is_dist_upgrade) {
        if (!pkgDistUpgrade(*tx->cache->dep_cache)) {
            std::string err = collect_pending_errors();
            if (err.empty()) err = "Distribution upgrade failed";
            return make_result(APT_ERROR_CACHE_OPEN_FAILED, err.c_str());
        }

        if (tx->cache->dep_cache->DelCount() == 0 &&
            tx->cache->dep_cache->InstCount() == 0 &&
            tx->cache->dep_cache->BadCount() == 0) {
            return make_result(APT_SUCCESS, nullptr);
        }

        return execute_transaction(tx->cache, nullptr, callback, user_data, download_only, false);
    }

    auto inst = to_cstr_array(tx->install_names);
    auto rem = to_cstr_array(tx->remove_names);
    auto reinst = to_cstr_array(tx->reinstall_names);

    AptPackageChanges dummy{};
    const AptResult r = plan_change_internal(
        tx->cache,
        inst.empty() ? nullptr : inst.data(), inst.size(),
        rem.empty() ? nullptr : rem.data(), rem.size(),
        reinst.empty() ? nullptr : reinst.data(), reinst.size(),
        tx->purge, tx->remove_depends,
        true,
        &dummy);
    apt_free_package_changes(&dummy);

    if (r.code != APT_SUCCESS) {
        return r;
    }

    const bool include_reinstall = !tx->reinstall_names.empty();
    return execute_transaction(tx->cache, nullptr, callback, user_data, download_only, include_reinstall);
}

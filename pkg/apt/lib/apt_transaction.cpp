#include "apt_transaction.h"
#include "planner.h"
#include "executor.h"
#include "changes.h"
#include "ext_rpm.h"

#include <apt-pkg/algorithms.h>
#include <apt-pkg/depcache.h>
#include <apt-pkg/error.h>

#include <memory>
#include <vector>
#include <set>
#include <string>
#include <cstring>

// Accumulates package names and flags for a single transaction.
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

// Classifies like apt-get DoInstall: a full name known to the cache wins over a trailing +/-
AptResult apt_transaction_add_apt_get_args(AptTransaction *tx, const char **args, const size_t count) {
    if (!tx) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NULL_TRANSACTION);
    if (!args || count == 0) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NO_PACKAGE_NAMES);

    for (size_t i = 0; i < count; i++) {
        if (!args[i] || !*args[i]) continue;
        std::string arg(args[i]);

        if (is_rpm_file(arg) || !tx->cache->dep_cache->FindPkg(arg).end()) {
            tx->install_names.push_back(arg);
            continue;
        }

        const char suffix = arg.back();
        if (suffix == '-' || suffix == '+') arg.pop_back();
        if (arg.empty()) continue;

        (suffix == '-' ? tx->remove_names : tx->install_names).push_back(arg);
    }
    return make_result(APT_SUCCESS, nullptr);
}

void apt_transaction_set_options(AptTransaction *tx, const bool purge, const bool remove_depends, const bool idempotent) {
    if (!tx) return;
    tx->purge = purge;
    tx->remove_depends = remove_depends;
    tx->idempotent = idempotent;
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

// Appends package names to the remove list; flags come from apt_transaction_set_options.
AptResult apt_transaction_remove(AptTransaction *tx, const char **names, const size_t count) {
    if (!tx) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NULL_TRANSACTION);
    if (!names || count == 0) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NO_PACKAGE_NAMES);

    for (size_t i = 0; i < count; i++) {
        if (names[i]) {
            tx->remove_names.emplace_back(names[i]);
        }
    }
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

// Converts a vector of strings to a vector of C string pointers.
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
                    const std::string out = std::string(APT_MSG_BROKEN_DEPS) + ": " + it.Name();
                    return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
                }
            }
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, APT_MSG_BROKEN_DEPS);
        }

        if (_error->PendingError()) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN);
        }

        PlanChanges plan = collect_plan_changes(cache, {});
        plan.extra_installed.clear();
        populate_changes(changes, plan, {}, {}, {}, true);

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

        if (_error->PendingError()) {
            return make_result(APT_ERROR_DEPENDENCY_BROKEN);
        }

        PlanChanges plan = collect_plan_changes(cache, {});
        plan.extra_installed.clear();
        populate_changes(changes, plan, {}, {}, {}, false);

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

    return plan_change_internal(tx->cache, *tx, false, changes);
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

    AptPackageChanges dummy{};
    const AptResult r = plan_change_internal(tx->cache, *tx, true, &dummy);
    apt_free_package_changes(&dummy);

    if (r.code != APT_SUCCESS) {
        return r;
    }

    const bool include_reinstall = !tx->reinstall_names.empty();
    return execute_transaction(tx->cache, nullptr, callback, user_data, download_only, include_reinstall);
}

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

// Simulates the transaction: marks it, collects changes, rolls the dep cache back.
AptResult apt_transaction_plan(const AptTransaction *tx, AptPackageChanges *changes) {
    if (!tx || !changes) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_PARAMS);
    memset(changes, 0, sizeof(AptPackageChanges));

    try {
        if (const AptResult r = prepare_transaction(tx->cache, *tx); r.code != APT_SUCCESS) {
            return r;
        }

        CacheStateGuard stateGuard(tx->cache->dep_cache);
        return apply_transaction_marks(tx->cache, *tx, changes);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Transaction planning failed: ") + e.what()).c_str());
    }
}

// Marks the transaction as plan does, then downloads and installs; rolls marks back on failure.
AptResult apt_transaction_execute(const AptTransaction *tx,
                                   const AptProgressCallback callback, const uintptr_t user_data,
                                   const bool download_only) {
    if (!tx) return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_NULL_TRANSACTION);

    try {
        if (const AptResult r = prepare_transaction(tx->cache, *tx); r.code != APT_SUCCESS) {
            return r;
        }

        CacheStateGuard stateGuard(tx->cache->dep_cache);

        AptPackageChanges dummy{};
        const AptResult marked = apply_transaction_marks(tx->cache, *tx, &dummy);
        apt_free_package_changes(&dummy);
        if (marked.code != APT_SUCCESS) {
            return marked;
        }

        const bool include_reinstall = !tx->reinstall_names.empty();
        const AptResult result = execute_transaction(tx->cache, nullptr, callback, user_data,
                                                     download_only, include_reinstall);
        if (result.code == APT_SUCCESS && !download_only) {
            stateGuard.commit();
        }
        return result;
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_INSTALL_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

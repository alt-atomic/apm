#ifndef APT_TRANSACTION_H
#define APT_TRANSACTION_H
#ifdef __cplusplus
extern "C" {
#endif

#include "apt_common.h"

// Creates a new transaction bound to `cache`.
AptResult apt_transaction_new(AptCache *cache, AptTransaction **tx);

// Frees the transaction.
void apt_transaction_free(const AptTransaction *tx);

// Marks `names` (array of `count` strings) for installation.
AptResult apt_transaction_install(AptTransaction *tx, const char **names, size_t count);

// Marks `names` for removal. If `purge` is true, removes config files too.
// If `remove_depends` is true, also removes reverse dependencies.
AptResult apt_transaction_remove(AptTransaction *tx, const char **names, size_t count,
                                  bool purge, bool remove_depends);

// Marks `names` for reinstallation.
AptResult apt_transaction_reinstall(AptTransaction *tx, const char **names, size_t count);

// Marks all upgradable packages for a distribution upgrade.
AptResult apt_transaction_dist_upgrade(AptTransaction *tx);

// Marks automatically installed packages with no dependents for removal.
AptResult apt_transaction_autoremove(AptTransaction *tx);

// Simulates the transaction and fills `changes` with the planned result.
AptResult apt_transaction_plan(const AptTransaction *tx, AptPackageChanges *changes);

// Executes the transaction. If `download_only` is true, only downloads packages.
AptResult apt_transaction_execute(const AptTransaction *tx,
                                   AptProgressCallback callback, uintptr_t user_data,
                                   bool download_only);

#ifdef __cplusplus
}
#endif

#endif

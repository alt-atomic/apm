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

// Sets transaction flags; idempotent reports missing removes in `skipped_packages` and tolerates installed packages.
void apt_transaction_set_options(AptTransaction *tx, bool purge, bool remove_depends, bool idempotent);

// Marks `names` for installation: name, name>=1.0, glob*, /path, file.rpm; +/- suffixes are not interpreted.
AptResult apt_transaction_install(AptTransaction *tx, const char **names, size_t count);

// Marks `names` for removal; flags come from apt_transaction_set_options.
AptResult apt_transaction_remove(AptTransaction *tx, const char **names, size_t count);

// Marks `names` for reinstallation.
AptResult apt_transaction_reinstall(AptTransaction *tx, const char **names, size_t count);

// Adds command line arguments as apt-get does: name installs, name- removes, name+ installs.
AptResult apt_transaction_add_apt_get_args(AptTransaction *tx, const char **args, size_t count);

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

#pragma once

#include "internal.h"
#include "error.h"

// Registers local .rpm files of `tx` as an APT source; may reopen the cache,
// so it must run before a CacheStateGuard is taken.
AptResult prepare_transaction(AptCache *cache, const AptTransaction &tx);

// Marks `tx` (install/remove/reinstall, dist-upgrade or autoremove) in the dep cache
// and fills `changes`. Plan and execute share it; rollback is up to the caller.
// `changes` must be zeroed by the caller.
AptResult apply_transaction_marks(const AptCache *cache, const AptTransaction &tx, AptPackageChanges *changes);

#pragma once

#include "internal.h"
#include "error.h"

// Plans (and optionally applies) a combined install/remove/reinstall operation.
// When `apply` is false, simulates changes and fills `changes` without modifying
// the system. When `apply` is true, marks packages but does not execute.
AptResult plan_change_internal(
    AptCache *cache,
    const char **install_names, size_t install_count,
    const char **remove_names, size_t remove_count,
    const char **reinstall_names, size_t reinstall_count,
    bool purge,
    bool remove_depends,
    bool apply,
    AptPackageChanges *changes);

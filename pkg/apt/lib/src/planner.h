#pragma once

#include "internal.h"
#include "error.h"

// Plans (and optionally applies) a combined install/remove/reinstall operation.
// When `apply` is false, simulates changes and fills `changes` without modifying
// the system. When `apply` is true, marks packages but does not execute.
AptResult plan_change_internal(AptCache *cache, const AptTransaction &tx, bool apply, AptPackageChanges *changes);

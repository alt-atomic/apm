#pragma once

#include "internal.h"
#include "error.h"
#include "progress.h"

// Downloads and installs packages using the package manager `pm`.
// If `download_only` is true, skips the install phase.
// If `include_reinstall` is true, sets APT::Get::ReInstall during execution.
AptResult execute_transaction(AptCache *cache,
                              AptPackageManager *pm,
                              AptProgressCallback callback,
                              uintptr_t user_data,
                              bool download_only,
                              bool include_reinstall);

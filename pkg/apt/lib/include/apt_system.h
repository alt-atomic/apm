#ifndef APT_SYSTEM_H
#define APT_SYSTEM_H
#ifdef __cplusplus
extern "C" {
#endif

#include "apt_common.h"

// Initializes APT configuration (pkgInitConfig).
AptResult apt_init_config();

// Initializes the APT system and stores the handle in `*system`.
AptResult apt_init_system(AptSystem **system);

// Releases resources held by `system` and unlocks the APT system.
void apt_cleanup_system(const AptSystem *system);

#ifdef __cplusplus
}
#endif

#endif

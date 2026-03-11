#ifndef APT_CONFIG_H
#define APT_CONFIG_H
#ifdef __cplusplus
extern "C" {
#endif

#include "apt_common.h"

// Sets an APT configuration `key` to `value`.
AptErrorCode apt_set_config(const char *key, const char *value);

// Returns the entire APT configuration as a string. Caller must free().
char *apt_config_dump(void);

// Saves a snapshot of the current APT configuration. Free with apt_config_restore().
void *apt_config_snapshot(void);

// Restores APT configuration from a previously saved `snapshot` and frees it.
void apt_config_restore(void *snapshot);

#ifdef __cplusplus
}
#endif

#endif
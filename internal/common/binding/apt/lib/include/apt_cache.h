#ifndef APT_CACHE_H
#define APT_CACHE_H
#ifdef __cplusplus
extern "C" {
#endif

#include "apt_common.h"

// Opens the package cache. If `with_lock` is true, acquires the APT lock.
AptResult apt_cache_open(const AptSystem *system, AptCache **cache, bool with_lock);

// Closes and frees the package cache.
void apt_cache_close(AptCache *cache);

// Reopens the cache to pick up changes on disk.
AptResult apt_cache_refresh(AptCache *cache);

// Downloads fresh package lists from configured repositories.
AptResult apt_cache_update(AptCache *cache);


#ifdef __cplusplus
}
#endif

#endif

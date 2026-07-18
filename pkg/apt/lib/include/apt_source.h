#ifndef APT_SOURCE_H
#define APT_SOURCE_H
#ifdef __cplusplus
extern "C" {
#endif

#include "apt_common.h"

// A downloaded source package (.src.rpm).
typedef struct {
    char *name;
    char *version;
    char *file;
    uint64_t size;
} AptSourcePackage;

// Result of a source download. Free with apt_free_source_result().
typedef struct {
    AptSourcePackage *items;
    size_t count;
} AptSourceResult;

// Downloads .src.rpm files for the given names into dest_dir (CWD if empty).
AptResult apt_source_download(AptCache *cache, const char **names, size_t count,
                              const char *dest_dir, AptSourceResult *result);

// Frees all heap-allocated fields within `result`.
void apt_free_source_result(AptSourceResult *result);

// Returns fields of the source package at `index`.
inline void apt_get_source_package(const AptSourceResult *result, const size_t index,
                                   const char **out_name, const char **out_version,
                                   const char **out_file, uint64_t *out_size) {
    *out_name = result->items[index].name;
    *out_version = result->items[index].version;
    *out_file = result->items[index].file;
    *out_size = result->items[index].size;
}

#ifdef __cplusplus
}
#endif

#endif

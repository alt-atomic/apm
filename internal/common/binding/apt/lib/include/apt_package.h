#ifndef APT_PACKAGE_H
#define APT_PACKAGE_H
#ifdef __cplusplus
extern "C" {
#endif

#include "apt_common.h"

// Detailed metadata for a single package. Free with apt_package_free().
typedef struct {
    char *name;
    char *version;
    char *description;
    char *short_description;
    char *section;
    char *architecture;
    char *maintainer;
    char *homepage;
    char *priority;
    char *md5_hash;
    char *blake2b_hash;
    char *source_package;
    char *changelog;
    char *filename;
    char *depends;
    char *provides;
    char *conflicts;
    char *obsoletes;
    char *recommends;
    char *suggests;
    AptPackageState state;
    bool auto_installed;
    bool essential;
    uint64_t installed_size;
    uint64_t download_size;
    uint32_t package_id;
    char **aliases;
    size_t alias_count;
    char **files;
    size_t file_count;
} AptPackageInfo;

// A list of package info entries. Free with apt_packages_free().
typedef struct {
    AptPackageInfo *packages;
    size_t count;
} AptPackageList;

// Fills `info` with metadata for `package_name`.
AptResult apt_package_get(AptCache *cache, const char *package_name, AptPackageInfo *info);

// Frees all heap-allocated fields within `info`.
void apt_package_free(AptPackageInfo *info);

// Searches packages matching `pattern` (extended regex) and fills `result`.
AptResult apt_packages_search(AptCache *cache, const char *pattern, AptPackageList *result);

// Frees all entries in `list` and the array itself.
void apt_packages_free(AptPackageList *list);

#ifdef __cplusplus
}
#endif

#endif

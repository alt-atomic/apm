#pragma once

#include "apt_common.h"

#include <apt-pkg/cachefile.h>
#include <apt-pkg/packagemanager.h>
#include <apt-pkg/pkgsystem.h>

#include <clocale>
#include <cstdlib>
#include <cstring>
#include <memory>
#include <string>

// RAII guard that saves the current locale, sets LC_ALL to "", and restores
// the original locale on destruction.
struct LocaleGuard {
    char *saved;
    LocaleGuard() : saved(nullptr) {
        if (const char *cur = setlocale(LC_ALL, nullptr)) saved = strdup(cur);
        setlocale(LC_ALL, "");
    }
    ~LocaleGuard() {
        if (saved) {
            setlocale(LC_ALL, saved);
            free(saved);
        }
    }
    LocaleGuard(const LocaleGuard &) = delete;
    LocaleGuard &operator=(const LocaleGuard &) = delete;
};

// Wraps the global pkgSystem pointer.
struct AptSystem {
    pkgSystem *system;

    AptSystem() : system(nullptr) {
    }

    ~AptSystem() = default;
};

// Owns the APT cache file and its dependency cache.
struct AptCache {
    std::unique_ptr<CacheFile> cache_file;
    pkgDepCache *dep_cache;

    AptCache() : dep_cache(nullptr) {
    }
};

// Owns the package manager used to execute transactions.
struct AptPackageManager {
    std::unique_ptr<pkgPackageManager> pm;
    AptCache *cache;

    explicit AptPackageManager(AptCache *c) : cache(c) {
    }
};

// RAII guard that saves dep cache state and restores it on destruction.
// Call commit() to keep changes instead of rolling back.
class CacheStateGuard {
    std::unique_ptr<pkgDepCache::State> saved_state;

 public:
    explicit CacheStateGuard(pkgDepCache *cache) : saved_state(std::make_unique<pkgDepCache::State>(cache)) {
    }

    ~CacheStateGuard() { if (saved_state) saved_state->Restore(); }
    void commit() { saved_state.reset(); }
    CacheStateGuard(const CacheStateGuard &) = delete;
    CacheStateGuard &operator=(const CacheStateGuard &) = delete;
};

// Returns a strdup'd copy of `s`, or nullptr if `s` is null or empty.
inline char *safe_strdup(const char *s) {
    if (!s || !*s) return nullptr;
    char *p = strdup(s);
    return p;
}

// Returns a strdup'd copy of `s`, or nullptr if `s` is empty.
inline char *safe_strdup(const std::string &s) {
    if (s.empty()) return nullptr;
    char *p = strdup(s.c_str());
    return p;
}

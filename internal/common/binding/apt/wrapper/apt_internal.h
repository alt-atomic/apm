#pragma once

#include "apt_wrapper.h"

#include <apt-pkg/init.h>
#include <apt-pkg/cachefile.h>
#include <apt-pkg/packagemanager.h>
#include <apt-pkg/depcache.h>
#include <apt-pkg/pkgcache.h>
#include <apt-pkg/configuration.h>
#include <apt-pkg/error.h>
#include <apt-pkg/pkgsystem.h>
#include <apt-pkg/acquire.h>
#include <apt-pkg/sourcelist.h>
#include <apt-pkg/pkgrecords.h>
#include <apt-pkg/algorithms.h>
#include <apt-pkg/fileutl.h>

#include <memory>
#include <string>
#include <vector>
#include <cstring>
#include <iostream>
#include <sstream>
#include <algorithm>
#include <cctype>
#include <regex.h>
#include <cstdlib>
#include <set>
#include <unistd.h>

// Internal structures originally defined in apt_wrapper.cpp
struct AptSystem {
    pkgSystem* system;

    AptSystem() : system(nullptr) {}
    ~AptSystem() {}
};

struct AptCache {
    std::unique_ptr<CacheFile> cache_file;
    pkgDepCache* dep_cache;

    AptCache() : dep_cache(nullptr) {}
};

struct AptPackageManager {
    std::unique_ptr<pkgPackageManager> pm;
    AptCache* cache;

    AptPackageManager(AptCache* c) : cache(c) {}
};

extern AptProgressCallback global_callback;
extern void* global_user_data;

extern AptLogCallback g_log_callback;
extern void* g_log_user_data;

extern AptErrorCode last_error;
extern std::string last_error_message;

void emit_log(const std::string& msg);
// Deprecated: avoid using set_error; return make_result(...) directly instead
inline void set_error(AptErrorCode code, const std::string& message = "") {
    (void)code; (void)message;
}
bool check_apt_errors();
// Build structured result from explicit message or current APT error stack
AptResult make_result(AptErrorCode code, const char* message = nullptr);

class ProgressStatus : public pkgAcquireStatus {
public:
    bool MediaChange(std::string, std::string) override { return false; }
    bool Pulse(pkgAcquire *Owner) override;
    void Fetch(pkgAcquire::ItemDesc &Itm) override;
    void Done(pkgAcquire::ItemDesc &Itm) override;
    void Fail(pkgAcquire::ItemDesc &Itm) override;

private:
    bool has_active_item_ = false;
    std::string active_name_;
};



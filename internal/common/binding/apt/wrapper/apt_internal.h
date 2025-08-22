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
#include <apt-pkg/indexfile.h>
#include <sys/stat.h>

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
bool check_apt_errors();
AptResult make_result(AptErrorCode code, const char* message = nullptr);

// RPM file detection utility
bool is_rpm_file(const std::string& path);

// Stream used by APT components (e.g. CacheFile) to write informational messages.
// Implemented to forward data into emit_log() so higher layers may capture them.
std::ostream& apt_log_stream();

class ProgressStatus : public pkgAcquireStatus {
public:
    bool MediaChange(std::string, std::string) override { return false; }
    bool Pulse(pkgAcquire *Owner) override;
    void Fetch(pkgAcquire::ItemDesc &Itm) override;
    void Done(pkgAcquire::ItemDesc &Itm) override;
    void Fail(pkgAcquire::ItemDesc &Itm) override;
    void Stop() override;

private:
    bool has_active_item_ = false;
    std::string active_name_;
};

// Unified internal planner used by simulation and marking
AptResult plan_change_internal(
    AptCache* cache,
    const char** install_names, size_t install_count,
    const char** remove_names, size_t remove_count,
    bool purge,
    bool apply,
    AptPackageChanges* changes);

// Common callback bridge structure for progress handling
struct CallbackBridge {
    void* user_data;
    AptCache* cache;
    std::vector<std::string> planned;
    size_t current_idx;
    std::string current_name;
    
    CallbackBridge() : user_data(nullptr), cache(nullptr), current_idx(0) {}
};

// Common progress callback function used by both install and dist-upgrade
PackageManagerCallback_t create_common_progress_callback(CallbackBridge* bridgeData);



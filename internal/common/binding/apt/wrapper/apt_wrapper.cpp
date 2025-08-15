#include "apt_internal.h"

// Include APT library headers
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
// For pkgVersioningSystem (VS().CheckDep)
#include <apt-pkg/version.h>

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
#include <cstring>
#include <vector>

// Error handling
AptErrorCode last_error = APT_SUCCESS;
std::string last_error_message;

// Optional logging callback (bridge to Go/Rust callers)
AptLogCallback g_log_callback = nullptr;
void* g_log_user_data = nullptr;

void emit_log(const std::string& msg) {
    if (g_log_callback) {
        g_log_callback(msg.c_str(), g_log_user_data);
    } else {
        std::cerr << msg << std::endl;
    }
}

// A streambuf that forwards output into emit_log in whole lines
class EmitLogBuf : public std::streambuf {
public:
    EmitLogBuf() = default;

protected:
    int overflow(int ch) override {
        if (ch == EOF) return 0;
        if (ch == '\n') {
            flush_line();
        } else if (ch == '\r') {
            flush_line();
        } else {
            buffer_.push_back(static_cast<char>(ch));
        }
        return ch;
    }

    int sync() override {
        flush_line();
        return 0;
    }

private:
    void flush_line() {
        if (!buffer_.empty()) {
            emit_log(buffer_);
            buffer_.clear();
        }
    }

    std::string buffer_;
};

static EmitLogBuf g_emit_buf;
static std::ostream g_emit_stream(&g_emit_buf);

std::ostream& apt_log_stream() {
    return g_emit_stream;
}

static bool g_stdio_captured = false;
static std::streambuf* g_prev_cout = nullptr;
static std::streambuf* g_prev_cerr = nullptr;
static std::streambuf* g_prev_clog = nullptr;

extern "C" void apt_capture_stdio(int enable) {
    if (enable) {
        if (!g_stdio_captured) {
            // Redirect only C++ iostreams to avoid OS-level fd hacks
            g_prev_cout = std::cout.rdbuf();
            g_prev_cerr = std::cerr.rdbuf();
            g_prev_clog = std::clog.rdbuf();
            std::cout.rdbuf(g_emit_stream.rdbuf());
            std::cerr.rdbuf(g_emit_stream.rdbuf());
            std::clog.rdbuf(g_emit_stream.rdbuf());
            g_stdio_captured = true;
        }
    } else {
        if (g_stdio_captured) {
            // Flush any pending content in the emit stream and standard streams
            try {
                g_emit_stream.flush();
                std::cout.flush();
                std::cerr.flush();
                std::clog.flush();
            } catch (...) {
                // ignore flush errors
            }
            // Restore C++ iostreams
            std::cout.rdbuf(g_prev_cout);
            std::cerr.rdbuf(g_prev_cerr);
            std::clog.rdbuf(g_prev_clog);
            g_prev_cout = nullptr;
            g_prev_cerr = nullptr;
            g_prev_clog = nullptr;
            g_stdio_captured = false;
        }
    }
}

bool check_apt_errors() {
    if (_error->PendingError()) {
        std::string error_msg;
        std::string all_errors;

        while (_error->PopMessage(error_msg)) {
            if (!all_errors.empty()) {
                all_errors += "; ";
            }
            all_errors += error_msg;
        }

        AptErrorCode error_code = APT_ERROR_UNKNOWN;
        if (all_errors.find("lock") != std::string::npos &&
            (all_errors.find("Resource temporarily unavailable") != std::string::npos ||
             all_errors.find("another process using it") != std::string::npos ||
             all_errors.find("EAGAIN") != std::string::npos ||
             all_errors.find("EACCES") != std::string::npos)) {
            error_code = APT_ERROR_LOCK_FAILED;
        }

        // inline set_error to avoid using it directly
        last_error = error_code;
        last_error_message = all_errors;
        if (!all_errors.empty()) {
            emit_log(std::string("APT Error: ") + all_errors);
        }
        return false;
    }
    return true;
}

static std::string collect_pending_errors() {
    std::string all_errors;
    if (_error->PendingError()) {
        std::string msg;
        while (_error->PopMessage(msg)) {
            if (!all_errors.empty()) all_errors += "; ";
            all_errors += msg;
        }
    }
    return all_errors;
}

static const char* find_first_broken_pkg(pkgDepCache* dep) {
    if (dep == nullptr) return nullptr;
    for (pkgCache::PkgIterator it = dep->PkgBegin(); !it.end(); ++it) {
        pkgDepCache::StateCache &st = (*dep)[it];
        if (st.InstBroken() || st.NowBroken()) {
            return it.Name();
        }
    }
    return nullptr;
}

static char* dup_cstr(const std::string& s) {
    if (s.empty()) return nullptr;
    char* p = (char*)malloc(s.size() + 1);
    if (!p) return nullptr;
    memcpy(p, s.c_str(), s.size() + 1);
    return p;
}

AptResult make_result(AptErrorCode code, const char* explicit_msg) {
    AptResult r{};
    r.code = code;
    if (code == APT_SUCCESS) {
        r.message = nullptr;
        return r;
    }
    std::string msg;
    if (explicit_msg && *explicit_msg) {
        msg = explicit_msg;
    } else {
        msg = collect_pending_errors();
    }
    if (msg.empty()) {
        if (!last_error_message.empty() && last_error == code) msg = last_error_message;
    }
    if (msg.empty()) msg = apt_error_string(code);
    r.message = dup_cstr(msg);
    return r;
}

// System initialization
AptResult apt_init_config() {
    try {
        if (!pkgInitConfig(*_config)) {
            return make_result(APT_ERROR_INIT_FAILED, "Failed to initialize APT configuration");
        }
        return make_result(check_apt_errors() ? APT_SUCCESS : last_error, nullptr);
    } catch (const std::exception& e) {
        return make_result(APT_ERROR_INIT_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

AptResult apt_init_system(AptSystem** system) {
    if (!system) return make_result(APT_ERROR_INIT_FAILED, "Invalid system pointer");

    try {
        if (!pkgInitSystem(*_config, _system)) {
            return make_result(APT_ERROR_INIT_FAILED, "Failed to initialize APT system");
        }

        // Create wrapper that points to global system
        *system = new AptSystem();
        (*system)->system = _system;

        return make_result(check_apt_errors() ? APT_SUCCESS : last_error, nullptr);
    } catch (const std::exception& e) {
        if (*system) {
            delete *system;
            *system = nullptr;
        }
        return make_result(APT_ERROR_INIT_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

void apt_cleanup_system(AptSystem* system) {
    if (system) {
        if (system->system) {
            system->system->UnLock(true);
        }
        delete system;
    }
}

// Cache management
AptResult apt_cache_open(AptSystem* system, AptCache** cache, bool with_lock) {
    if (!system || !cache) return make_result(APT_ERROR_INIT_FAILED, "Invalid arguments for cache_open");

    if (!system->system) {
        return make_result(APT_ERROR_INIT_FAILED, "System not properly initialized");
    }

    // Check if system can be locked before attempting to open cache
    if (with_lock) {
        if (!system->system->Lock()) {
            if (_error->PendingError()) {
                std::string error_msg;
                std::string all_errors;
                while (_error->PopMessage(error_msg)) {
                    if (!all_errors.empty()) {
                        all_errors += "; ";
                    }
                    all_errors += error_msg;
                }
                return make_result(APT_ERROR_LOCK_FAILED, all_errors.c_str());
            } else {
                return make_result(APT_ERROR_LOCK_FAILED, "Unable to acquire APT system lock - another process may be using APT");
            }
        }
        system->system->UnLock(true);
    }

    try {
        *cache = new AptCache();

        static std::ostringstream nullstream;
        (*cache)->cache_file = std::make_unique<CacheFile>(nullstream, with_lock);

        if (!(*cache)->cache_file->Open()) {
            delete *cache;
            *cache = nullptr;
            return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Failed to open APT cache");
        }

        if (!(*cache)->cache_file->CheckDeps()) {
            const char* broken = find_first_broken_pkg((*cache)->cache_file->operator->());
            std::string out;
            if (broken && *broken) {
                out = std::string("Some broken packages were found while trying to process build-dependencies for ") + broken;
            } else {
                out = "Broken dependencies";
            }
            delete *cache;
            *cache = nullptr;
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
        }

        (*cache)->dep_cache = (*cache)->cache_file->operator->();
        if (!(*cache)->dep_cache) {
            delete *cache;
            *cache = nullptr;
            return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Failed to get dependency cache");
        }
        
        return make_result(check_apt_errors() ? APT_SUCCESS : last_error);
    } catch (const std::exception& e) {
        if (*cache) {
            delete *cache;
            *cache = nullptr;
        }
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

void apt_cache_close(AptCache* cache) {
    if (cache) {
        if (cache->cache_file) {
            cache->cache_file.reset();
        }
        delete cache;
    }
}

AptResult apt_cache_refresh(AptCache* cache) {
    if (!cache) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED);
    }

    try {
        cache->cache_file.reset();

        static std::ostringstream nullstream;
        cache->cache_file = std::make_unique<CacheFile>(nullstream, true);

        if (!cache->cache_file->Open()) {
            return make_result(APT_ERROR_CACHE_REFRESH_FAILED, "Failed to reopen cache after refresh");
        }

        if (!cache->cache_file->CheckDeps()) {
            const char* broken = find_first_broken_pkg(cache->cache_file->operator->());
            if (broken && *broken) {
                std::string out = std::string("Some broken packages were found while trying to process build-dependencies for ") + broken + ".";
                return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
            }
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Broken dependencies");
        }

        cache->dep_cache = cache->cache_file->operator->();
        if (!cache->dep_cache) {
            return make_result(APT_ERROR_CACHE_REFRESH_FAILED, "Failed to get dependency cache after refresh");
        }

        return make_result(check_apt_errors() ? APT_SUCCESS : last_error);
    } catch (const std::exception& e) {
        return make_result(APT_ERROR_CACHE_REFRESH_FAILED, (std::string("Exception during cache refresh: ") + e.what()).c_str());
    }
}

AptResult apt_cache_update(AptCache* cache) {
    if (!cache || !cache->cache_file) return make_result(APT_ERROR_CACHE_OPEN_FAILED);

    try {
        ProgressStatus status;
        pkgAcquire acquire(&status);
        pkgSourceList source_list;

        if (!source_list.ReadMainList()) {
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, "Failed to read sources.list");
        }

        if (!source_list.GetIndexes(&acquire)) {
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, "Failed to get package indexes");
        }

        auto fetch_result = acquire.Run();
        if (fetch_result != pkgAcquire::Continue) {
            return make_result(APT_ERROR_DOWNLOAD_FAILED, "Failed to download package lists");
        }

        if (!cache->cache_file->BuildCaches()) {
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, "Failed to rebuild caches");
        }
        
        return make_result(check_apt_errors() ? APT_SUCCESS : last_error);
    } catch (const std::exception& e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}

// Package manager
AptResult apt_package_manager_create(AptCache* cache, AptPackageManager** pm) {
    if (!cache || !cache->dep_cache || !pm) return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid cache or output pointer for pm create");

    try {
        *pm = new AptPackageManager(cache);

        (*pm)->pm.reset(_system->CreatePM(cache->dep_cache));

        if (!(*pm)->pm) {
            delete *pm;
            *pm = nullptr;
            return make_result(APT_ERROR_INIT_FAILED, "Failed to create package manager");
        }

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception& e) {
        if (*pm) {
            delete *pm;
            *pm = nullptr;
        }
        return make_result(APT_ERROR_INIT_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

void apt_package_manager_destroy(AptPackageManager* pm) {
    delete pm;
}

AptResult apt_mark_install(AptCache* cache, const char* package_name) {
    if (!cache || !cache->dep_cache || !package_name) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid arguments for mark_install");
    }

    try {
        // Delegate to unified planner to guarantee parity with simulation
        const char* install_names[1] = { package_name };
        AptPackageChanges dummy{};
        AptResult r = plan_change_internal(cache, install_names, 1, nullptr, 0, false, true, &dummy);
        apt_free_package_changes(&dummy);
        return r;
    } catch (const std::exception& e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}

AptResult apt_mark_remove(AptCache* cache, const char* package_name, bool purge) {
    if (!cache || !cache->dep_cache || !package_name) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid arguments for mark_remove");
    }

    try {
        // Delegate to unified planner to guarantee parity with simulation
        const char* remove_names[1] = { package_name };
        AptPackageChanges dummy{};
        AptResult r = plan_change_internal(cache, nullptr, 0, remove_names, 1, purge, true, &dummy);
        apt_free_package_changes(&dummy);
        return r;
    } catch (const std::exception& e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}

// Progress callback implementation
AptProgressCallback global_callback = nullptr;
void* global_user_data = nullptr;

// Simple progress callback that handles package manager operations
class SimpleProgressCallback {
public:
    static void InstallProgress(const char* package, int current, int total) {
        if (global_callback && package) {
            global_callback(package, APT_CALLBACK_INST_PROGRESS, current, total, global_user_data);
        }
    }

    static void InstallStart(const char* package) {
        if (global_callback && package) {
            global_callback(package, APT_CALLBACK_INST_START, 0, 0, global_user_data);
        }
    }

    static void InstallStop(const char* package) {
        if (global_callback && package) {
            global_callback(package, APT_CALLBACK_INST_STOP, 0, 0, global_user_data);
        }
    }
};

// ProgressStatus implementations (after global_callback is defined)
bool ProgressStatus::Pulse(pkgAcquire *Owner) {
    if (global_callback != nullptr) {
        // Send overall download progress
        if (TotalBytes > 0) {
            // Use "" as package name to indicate overall progress
            global_callback("", APT_CALLBACK_DOWNLOAD_PROGRESS,
                            static_cast<uint64_t>(CurrentBytes),
                            static_cast<uint64_t>(TotalBytes),
                            global_user_data);
        }
    }
    return pkgAcquireStatus::Pulse(Owner);
}

void ProgressStatus::Fetch(pkgAcquire::ItemDesc &Itm) {
    const char *name = Itm.ShortDesc.empty() ? Itm.URI.c_str() : Itm.ShortDesc.c_str();
    active_name_.assign(name ? name : "");
    has_active_item_ = true;
    if (global_callback != nullptr) {
        global_callback(active_name_.c_str(), APT_CALLBACK_DOWNLOAD_START, 0, 0, global_user_data);
    }
    pkgAcquireStatus::Fetch(Itm);
}

void ProgressStatus::Done(pkgAcquire::ItemDesc &Itm) {
    if (global_callback != nullptr) {
        global_callback(active_name_.c_str(), APT_CALLBACK_DOWNLOAD_STOP, 0, 0, global_user_data);
    }
    has_active_item_ = false;
    active_name_.clear();
    pkgAcquireStatus::Done(Itm);
}

void ProgressStatus::Fail(pkgAcquire::ItemDesc &Itm) {
    if (global_callback != nullptr) {
        global_callback(active_name_.c_str(), APT_CALLBACK_ERROR, 0, 0, global_user_data);
    }
    has_active_item_ = false;
    active_name_.clear();
    pkgAcquireStatus::Fail(Itm);
}


void apt_free_package_info(AptPackageInfo* info) {
    if (!info) return;

    free(info->name);
    free(info->version);
    free(info->description);
    free(info->short_description);
    free(info->section);
    free(info->architecture);
    free(info->maintainer);
    free(info->homepage);
    free(info->priority);
    free(info->md5_hash);
    free(info->blake2b_hash);
    free(info->source_package);
    free(info->changelog);
    free(info->filename);
    free(info->depends);
    free(info->provides);
    free(info->conflicts);
    free(info->obsoletes);
    free(info->recommends);
    free(info->suggests);
    if (info->aliases) {
        for (size_t i = 0; i < info->alias_count; ++i) {
            free(info->aliases[i]);
        }
        free(info->aliases);
    }

    memset(info, 0, sizeof(AptPackageInfo));
}

void apt_free_package_list(AptPackageList* list) {
    if (!list || !list->packages) return;

    for (size_t i = 0; i < list->count; ++i) {
        apt_free_package_info(&list->packages[i]);
    }

    free(list->packages);
    list->packages = nullptr;
    list->count = 0;
}

// Utility functions
const char* apt_error_string(AptErrorCode error) {
    // Prefer the last detailed message captured via set_error if it matches the code
    if (last_error == error && !last_error_message.empty()) {
        return last_error_message.c_str();
    }
    switch (error) {
        case APT_SUCCESS: return "Success";

        case APT_ERROR_INIT_FAILED: return "Initialization failed";

        case APT_ERROR_CACHE_OPEN_FAILED: return "Cache open failed";
        case APT_ERROR_CACHE_REFRESH_FAILED: return "Cache refresh failed";
        case APT_ERROR_CACHE_UPDATE_FAILED: return "Cache update failed";

        case APT_ERROR_PACKAGE_NOT_FOUND: return "Package not found";
        case APT_ERROR_DEPENDENCY_BROKEN: return "Broken dependencies";

        case APT_ERROR_OPERATION_FAILED: return "Package manager operation failed";
        case APT_ERROR_OPERATION_INCOMPLETE: return "Package manager operation incomplete";
        case APT_ERROR_INSTALL_FAILED: return "Installation failed";
        case APT_ERROR_DOWNLOAD_FAILED: return "Download failed";

        case APT_ERROR_LOCK_FAILED: return "Unable to acquire lock - another APT process is running";
        case APT_ERROR_INVALID_PARAMETERS: return "Invalid parameters";

        case APT_ERROR_UNKNOWN:
        default: return "Unknown error";
    }
}

bool apt_has_broken_packages(AptCache* cache) {
    if (!cache || !cache->dep_cache) return false;
    return cache->dep_cache->BrokenCount() > 0;
}

uint32_t apt_get_broken_count(AptCache* cache) {
    if (!cache || !cache->dep_cache) return 0;
    return cache->dep_cache->BrokenCount();
}

// Debug function to test FindPkg logic like in original apt-get
bool apt_test_findpkg(AptCache* cache, const char* package_name) {
    if (!cache || !cache->dep_cache || !package_name) return false;

    try {
        pkgCache::PkgIterator pkg = cache->dep_cache->FindPkg(package_name);

        emit_log(std::string("=== FindPkg Test for '") + package_name + "' ===");
        emit_log(std::string("pkg.end(): ") + (pkg.end() ? "true" : "false"));

        if (!pkg.end()) {
            emit_log("Package found in cache!");
            emit_log(std::string("Name: ") + pkg.Name());
            emit_log(std::string("ID: ") + std::to_string(pkg->ID));

            pkgDepCache::StateCache& state = (*cache->dep_cache)[pkg];
            pkgCache::VerIterator candidate = state.CandidateVerIter(*cache->dep_cache);
            emit_log(std::string("CandidateVer.end(): ") + (candidate.end() ? "true" : "false"));

            emit_log(std::string("ProvidesList: ") + (pkg->ProvidesList == 0 ? "empty" : "has provides"));

            if (pkg->ProvidesList != 0) {
                emit_log("This is a virtual package! Providers:");
                for (pkgCache::PrvIterator prv = pkg.ProvidesList(); !prv.end(); ++prv) {
                    pkgCache::PkgIterator provider = prv.OwnerPkg();
                    std::string line = std::string("  - ") + provider.Name();
                    if (!provider.CurrentVer().end()) {
                        line += std::string(" (INSTALLED: ") + provider.CurrentVer().VerStr() + ")";
                    }
                    pkgCache::VerIterator candVer = (*cache->dep_cache)[provider].CandidateVerIter(*cache->dep_cache);
                    if (!candVer.end()) {
                        line += std::string(" (CANDIDATE: ") + candVer.VerStr() + ")";
                    }
                    emit_log(line);
                }
            }

            return true;
        } else {
            emit_log("Package NOT found in cache.");
            return false;
        }

    } catch (const std::exception& e) {
        emit_log(std::string("Exception: ") + e.what());
        return false;
    }
}

extern "C" void apt_set_log_callback(AptLogCallback callback, void* user_data) {
    g_log_callback = callback;
    g_log_user_data = user_data;
}

// Global/default progress callback registration
extern "C" void apt_register_progress_callback(AptProgressCallback callback, void* user_data) {
    global_callback = callback;
    global_user_data = user_data;
}

extern "C" void goAptProgressCallback(const char *package_name,
                                       int callback_type,
                                       uint64_t current,
                                       uint64_t total,
                                       void *user_data);

extern "C" void goAptLogCallback(const char *message, void *user_data);


extern "C" void apt_use_go_progress_callback(void* user_data) {
    global_callback = (AptProgressCallback)goAptProgressCallback;
    global_user_data = user_data;
}

extern "C" void apt_enable_go_log_callback(void* user_data) {
    g_log_callback = (AptLogCallback)goAptLogCallback;
    g_log_user_data = user_data;
}

// Configuration
AptErrorCode apt_set_config(const char* key, const char* value) {
    if (!key || !value) return APT_ERROR_INIT_FAILED;

    try {
        _config->Set(key, value);
        return APT_SUCCESS;
    } catch (const std::exception& e) {
        emit_log(std::string("Exception: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

const char* apt_get_config(const char* key, const char* default_value) {
    if (!key) return default_value;

    try {
        static std::string result;
        result = _config->Find(key, default_value ? default_value : "");
        return result.c_str();
    } catch (const std::exception& e) {
        return default_value;
    }
}

// Force unlock function to clean up hanging locks
void apt_force_unlock() {
    try {
        pkgSystem* system = _system;
        if (system) {
            system->UnLock(true);
        }
    } catch (...) {
        // Ignore any exceptions during force unlock
    }
}

void apt_free_package_changes(AptPackageChanges* changes) {
    if (!changes) return;

    if (changes->extra_installed) {
        for (size_t i = 0; i < changes->extra_installed_count; i++) {
            free(changes->extra_installed[i]);
        }
        free(changes->extra_installed);
    }

    if (changes->upgraded_packages) {
        for (size_t i = 0; i < changes->upgraded_count; i++) {
            free(changes->upgraded_packages[i]);
        }
        free(changes->upgraded_packages);
    }

    if (changes->new_installed_packages) {
        for (size_t i = 0; i < changes->new_installed_count; i++) {
            free(changes->new_installed_packages[i]);
        }
        free(changes->new_installed_packages);
    }

    if (changes->removed_packages) {
        for (size_t i = 0; i < changes->removed_count; i++) {
            free(changes->removed_packages[i]);
        }
        free(changes->removed_packages);
    }

    memset(changes, 0, sizeof(AptPackageChanges));
}

// Common progress callback implementation
PackageManagerCallback_t create_common_progress_callback(CallbackBridge* /* bridgeData - parameter not used in function signature, only as lambda context */) {
    return [](const char *nevra, aptCallbackType what, uint64_t amount, uint64_t total, void *callbackData) {
        CallbackBridge* bd = static_cast<CallbackBridge*>(callbackData);
        AptCallbackType our_type = APT_CALLBACK_UNKNOWN;
        
        // Map internal APT callback types to public API types
        switch (what) {
            case APTCALLBACK_INST_PROGRESS: our_type = APT_CALLBACK_INST_PROGRESS; break;
            case APTCALLBACK_INST_START: our_type = APT_CALLBACK_INST_START; break;
            case APTCALLBACK_INST_STOP: our_type = APT_CALLBACK_INST_STOP; break;
            case APTCALLBACK_TRANS_PROGRESS: our_type = APT_CALLBACK_TRANS_PROGRESS; break;
            case APTCALLBACK_TRANS_START: our_type = APT_CALLBACK_TRANS_START; break;
            case APTCALLBACK_TRANS_STOP: our_type = APT_CALLBACK_TRANS_STOP; break;
            case APTCALLBACK_UNINST_PROGRESS: our_type = APT_CALLBACK_REMOVE_PROGRESS; break;
            case APTCALLBACK_UNINST_START: our_type = APT_CALLBACK_REMOVE_START; break;
            case APTCALLBACK_UNINST_STOP: our_type = APT_CALLBACK_REMOVE_STOP; break;
            case APTCALLBACK_ELEM_PROGRESS: our_type = APT_CALLBACK_ELEM_PROGRESS; break;
            default: our_type = APT_CALLBACK_UNKNOWN; break;
        }

        // Helper function to pick current planned package
        auto pick_planned_current = [&]() -> const char* {
            if (bd == nullptr || bd->planned.empty()) return "";
            if (bd->current_idx >= bd->planned.size()) return bd->planned.back().c_str();
            return bd->planned[bd->current_idx].c_str();
        };

        const bool has_nevra = (nevra != nullptr && nevra[0] != '\0');
        const char* effective_name = "";

        // Determine effective package name based on event type
        switch (what) {
            case APTCALLBACK_INST_START:
            case APTCALLBACK_UNINST_START:
                if (has_nevra) {
                    bd->current_name = nevra;
                } else if (bd != nullptr) {
                    bd->current_name = pick_planned_current();
                }
                effective_name = bd ? bd->current_name.c_str() : (has_nevra ? nevra : "");
                break;

            case APTCALLBACK_INST_PROGRESS:
            case APTCALLBACK_UNINST_PROGRESS:
            case APTCALLBACK_ELEM_PROGRESS:
                if (bd != nullptr && bd->current_name.empty()) {
                    bd->current_name = has_nevra ? std::string(nevra) : std::string(pick_planned_current());
                }
                effective_name = bd ? bd->current_name.c_str() : (has_nevra ? nevra : "");
                break;

            case APTCALLBACK_INST_STOP:
            case APTCALLBACK_UNINST_STOP:
                // For stop events, prioritize nevra (from RPM header) as it's most reliable
                // If nevra is not available, use current name or fallback to planned name
                if (has_nevra) {
                    effective_name = nevra;
                } else if (bd && !bd->current_name.empty()) {
                    effective_name = bd->current_name.c_str();
                } else {
                    effective_name = pick_planned_current();
                }
                // Note: We advance index and clear name AFTER the callback is invoked
                break;

            default:
                effective_name = has_nevra ? nevra : "";
                break;
        }

        // Invoke the global callback
        if (global_callback) {
            global_callback(effective_name, our_type, amount, total, global_user_data);
        }

        // Handle state advancement for stop events AFTER the callback is invoked
        if ((what == APTCALLBACK_INST_STOP || what == APTCALLBACK_UNINST_STOP) && bd != nullptr) {
            // Advance to next planned item on stop; clear current
            if (bd->current_idx < bd->planned.size()) bd->current_idx++;
            bd->current_name.clear();
        }
    };
}

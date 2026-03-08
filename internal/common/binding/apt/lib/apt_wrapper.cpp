#include "apt_internal.h"

#include <apt-pkg/acquire-item.h>
#include <apt-pkg/error.h>
#include <apt-pkg/init.h>
#include <apt-pkg/sourcelist.h>
#include <apt-pkg/acquire-worker.h>

#include <atomic>
#include <cstdio>
#include <cstring>
#include <fcntl.h>
#include <sstream>
#include <sys/select.h>
#include <sys/stat.h>
#include <thread>
#include <unistd.h>

// Error handling
AptErrorCode last_error = APT_SUCCESS;
std::string last_error_message;

// Optional logging callback (bridge to Go callers)
AptLogCallback g_log_callback = nullptr;
uintptr_t g_log_user_data = 0;

void emit_log(const std::string &msg) {
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
    int overflow(const int ch) override {
        if (ch == EOF) return 0;
        if (ch == '\n' || ch == '\r') {
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

std::ostream &apt_log_stream() {
    return g_emit_stream;
}

static bool g_stdio_captured = false;
static std::streambuf *g_prev_cout = nullptr;
static std::streambuf *g_prev_cerr = nullptr;
static std::streambuf *g_prev_clog = nullptr;

// File descriptor level capture for C stdio (RPM output, printf, fprintf, etc.)
static int g_saved_stderr_fd = -1;
static int g_pipe_read_fd = -1;
static int g_pipe_write_fd = -1;
static std::string g_captured_stderr;
static std::thread g_reader_thread;
static std::atomic<bool> g_reader_running{false};

static void stderr_reader_thread() {
    char buffer[1024];
    while (g_reader_running.load()) {
        fd_set fds;
        FD_ZERO(&fds);
        FD_SET(g_pipe_read_fd, &fds);
        timeval tv = {0, 100000};
        const int ret = select(g_pipe_read_fd + 1, &fds, nullptr, nullptr, &tv);
        if (ret > 0 && FD_ISSET(g_pipe_read_fd, &fds)) {
            const ssize_t n = read(g_pipe_read_fd, buffer, sizeof(buffer) - 1);
            if (n > 0) {
                buffer[n] = '\0';
                g_captured_stderr += buffer;
            }
        }
    }
    // Drain remaining data
    while (true) {
        const ssize_t n = read(g_pipe_read_fd, buffer, sizeof(buffer) - 1);
        if (n <= 0) break;
        buffer[n] = '\0';
        g_captured_stderr += buffer;
    }
}

extern "C" void apt_capture_stdio(int enable) {
    if (enable) {
        if (!g_stdio_captured) {
            g_prev_cout = std::cout.rdbuf();
            g_prev_cerr = std::cerr.rdbuf();
            g_prev_clog = std::clog.rdbuf();
            std::cout.rdbuf(g_emit_stream.rdbuf());
            std::cerr.rdbuf(g_emit_stream.rdbuf());
            std::clog.rdbuf(g_emit_stream.rdbuf());

            fflush(stderr);
            g_captured_stderr.clear();
            int pipefd[2];
            if (pipe(pipefd) == 0) {
                g_pipe_read_fd = pipefd[0];
                g_pipe_write_fd = pipefd[1];
                fcntl(g_pipe_read_fd, F_SETFL, O_NONBLOCK);
                g_saved_stderr_fd = dup(STDERR_FILENO);
                dup2(g_pipe_write_fd, STDERR_FILENO);
                g_reader_running.store(true);
                g_reader_thread = std::thread(stderr_reader_thread);
            }

            g_stdio_captured = true;
        }
    } else {
        if (g_stdio_captured) {
            try {
                g_emit_stream.flush();
                std::cout.flush();
                std::cerr.flush();
                std::clog.flush();
            } catch (...) {
                // ignore flush errors
            }

            // Restore C stderr and stop reader
            fflush(stderr);
            if (g_saved_stderr_fd >= 0) {
                dup2(g_saved_stderr_fd, STDERR_FILENO);
                close(g_saved_stderr_fd);
                g_saved_stderr_fd = -1;
            }
            if (g_pipe_write_fd >= 0) {
                close(g_pipe_write_fd);
                g_pipe_write_fd = -1;
            }
            if (g_reader_running.load()) {
                g_reader_running.store(false);
                if (g_reader_thread.joinable()) {
                    g_reader_thread.join();
                }
            }
            if (g_pipe_read_fd >= 0) {
                close(g_pipe_read_fd);
                g_pipe_read_fd = -1;
            }

            // Send captured stderr to log handler for error analysis
            if (!g_captured_stderr.empty() && g_log_callback) {
                std::istringstream stream(g_captured_stderr);
                std::string line;
                while (std::getline(stream, line)) {
                    if (!line.empty()) {
                        g_log_callback(line.c_str(), g_log_user_data);
                    }
                }
            }
            g_captured_stderr.clear();

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

std::string collect_pending_errors() {
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

static const char *find_first_broken_pkg(pkgDepCache *dep) {
    if (dep == nullptr) return nullptr;
    for (pkgCache::PkgIterator it = dep->PkgBegin(); !it.end(); ++it) {
        pkgDepCache::StateCache &st = (*dep)[it];
        if (st.InstBroken() || st.NowBroken()) {
            return it.Name();
        }
    }
    return nullptr;
}


AptResult make_result(const AptErrorCode code, const char *message) {
    AptResult r{};
    r.code = code;
    if (code == APT_SUCCESS) {
        r.message = nullptr;
        return r;
    }
    std::string msg;
    if (message && *message) {
        msg = message;
    } else {
        msg = collect_pending_errors();
    }
    if (msg.empty()) {
        if (!last_error_message.empty() && last_error == code) msg = last_error_message;
    }
    if (msg.empty()) msg = apt_error_string(code);
    r.message = safe_strdup(msg);
    return r;
}

// System initialization
AptResult apt_init_config() {
    try {
        if (!pkgInitConfig(*_config)) {
            return make_result(APT_ERROR_INIT_FAILED, "Failed to initialize APT configuration");
        }
        return make_result(check_apt_errors() ? APT_SUCCESS : last_error, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_INIT_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

AptResult apt_init_system(AptSystem **system) {
    if (!system) return make_result(APT_ERROR_INIT_FAILED, "Invalid system pointer");

    try {
        if (!pkgInitSystem(*_config, _system)) {
            return make_result(APT_ERROR_INIT_FAILED, "Failed to initialize APT system");
        }

        // Create wrapper that points to global system
        *system = new AptSystem();
        (*system)->system = _system;

        return make_result(check_apt_errors() ? APT_SUCCESS : last_error, nullptr);
    } catch (const std::exception &e) {
        if (*system) {
            delete *system;
            *system = nullptr;
        }
        return make_result(APT_ERROR_INIT_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

void apt_cleanup_system(const AptSystem *system) {
    if (system) {
        if (system->system) {
            system->system->UnLock(true);
        }
        delete system;
    }
}

// Cache management
AptResult apt_cache_open(const AptSystem *system, AptCache **cache, bool with_lock) {
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
                return make_result(APT_ERROR_LOCK_FAILED,
                                   "Unable to acquire APT system lock - another process may be using APT");
            }
        }
        system->system->UnLock(true);
    }

    try {
        *cache = new AptCache();

        (*cache)->cache_file = std::make_unique<CacheFile>(apt_log_stream(), with_lock);

        if (!(*cache)->cache_file->Open()) {
            std::string err = collect_pending_errors();
            if (err.empty()) err = "Failed to open APT cache";
            delete *cache;
            *cache = nullptr;
            return make_result(APT_ERROR_CACHE_OPEN_FAILED, err.c_str());
        }

        if (!(*cache)->cache_file->CheckDeps()) {
            const char *broken = find_first_broken_pkg((*cache)->cache_file->operator->());
            std::string out;
            if (broken && *broken) {
                out = std::string("Some broken packages were found while trying to process build-dependencies for ") +
                      broken;
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
    } catch (const std::exception &e) {
        if (*cache) {
            delete *cache;
            *cache = nullptr;
        }
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

void apt_cache_close(AptCache *cache) {
    if (cache) {
        if (cache->cache_file) {
            cache->cache_file.reset();
        }
        delete cache;
    }
}

AptResult apt_cache_refresh(AptCache *cache) {
    if (!cache) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED);
    }

    try {
        cache->cache_file.reset();

        cache->cache_file = std::make_unique<CacheFile>(apt_log_stream(), true);

        if (!cache->cache_file->Open()) {
            std::string err = collect_pending_errors();
            if (err.empty()) err = "Failed to reopen cache after refresh";
            return make_result(APT_ERROR_CACHE_REFRESH_FAILED, err.c_str());
        }

        if (!cache->cache_file->CheckDeps()) {
            const char *broken = find_first_broken_pkg(cache->cache_file->operator->());
            if (broken && *broken) {
                std::string out = std::string(
                                      "Some broken packages were found while trying to process build-dependencies for ")
                                  + broken + ".";
                return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
            }
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, "Broken dependencies");
        }

        cache->dep_cache = cache->cache_file->operator->();
        if (!cache->dep_cache) {
            return make_result(APT_ERROR_CACHE_REFRESH_FAILED, "Failed to get dependency cache after refresh");
        }

        return make_result(check_apt_errors() ? APT_SUCCESS : last_error);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_CACHE_REFRESH_FAILED,
                           (std::string("Exception during cache refresh: ") + e.what()).c_str());
    }
}

AptResult apt_cache_update(AptCache *cache) {
    if (!cache || !cache->cache_file) return make_result(APT_ERROR_CACHE_OPEN_FAILED);

    try {
        FileFd Lock;
        if (!_config->FindB("Debug::NoLocking", false)) {
            Lock.Fd(GetLock(_config->FindDir("Dir::State::Lists") + "lock"));
            if (_error->PendingError()) {
                return make_result(APT_ERROR_LOCK_FAILED, "Unable to lock the list directory");
            }
        }

        ProgressStatus status;
        pkgAcquire acquire(&status);
        pkgSourceList source_list;

        if (!source_list.ReadMainList()) {
            std::string err = collect_pending_errors();
            if (err.empty()) err = "The list of sources could not be read.";
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, err.c_str());
        }

        if (!source_list.InvalidateReleases()) {
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, "Failed to invalidate releases");
        }

        if (!source_list.GetReleases(&acquire)) {
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, "Failed to get release files");
        }

        auto fetch_result = acquire.Run();
        if (fetch_result != pkgAcquire::Continue) {
            for (pkgAcquire::ItemCIterator I = acquire.ItemsBegin(); I != acquire.ItemsEnd(); ++I) {
                if ((*I)->Status == pkgAcquire::Item::StatError && !(*I)->ErrorText.empty()) {
                    std::string error_msg = "Repository update failed: " + (*I)->DescURI() + " " + (*I)->ErrorText;
                    return make_result(APT_ERROR_DOWNLOAD_FAILED, error_msg.c_str());
                }
            }
            return make_result(APT_ERROR_DOWNLOAD_FAILED, "Repository update failed: Unable to download release files");
        }

        for (pkgAcquire::ItemCIterator I = acquire.ItemsBegin(); I != acquire.ItemsEnd(); ++I) {
            if ((*I)->Status == pkgAcquire::Item::StatError && !(*I)->ErrorText.empty()) {
                std::string error_msg = "Repository update failed: " + (*I)->DescURI() + " " + (*I)->ErrorText;
                return make_result(APT_ERROR_DOWNLOAD_FAILED, error_msg.c_str());
            }
        }

        if (!source_list.GetIndexes(&acquire)) {
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, "Failed to get package indexes");
        }

        fetch_result = acquire.Run();
        if (fetch_result != pkgAcquire::Continue) {
            for (pkgAcquire::ItemCIterator I = acquire.ItemsBegin(); I != acquire.ItemsEnd(); ++I) {
                if ((*I)->Status == pkgAcquire::Item::StatError && !(*I)->ErrorText.empty()) {
                    std::string error_msg = "Package index update failed: " + (*I)->DescURI() + " " + (*I)->ErrorText;
                    return make_result(APT_ERROR_DOWNLOAD_FAILED, error_msg.c_str());
                }
            }
            return make_result(APT_ERROR_DOWNLOAD_FAILED,
                               "Package index update failed: Unable to download package lists");
        }

        for (pkgAcquire::ItemCIterator I = acquire.ItemsBegin(); I != acquire.ItemsEnd(); ++I) {
            if ((*I)->Status == pkgAcquire::Item::StatError && !(*I)->ErrorText.empty()) {
                std::string error_msg = "Package index update failed: " + (*I)->DescURI() + " " + (*I)->ErrorText;
                return make_result(APT_ERROR_DOWNLOAD_FAILED, error_msg.c_str());
            }
        }

        if (!cache->cache_file->BuildCaches()) {
            std::string err = collect_pending_errors();
            if (err.empty()) err = "Failed to rebuild caches";
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, err.c_str());
        }

        return make_result(check_apt_errors() ? APT_SUCCESS : last_error);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}

// Package manager
AptResult apt_package_manager_create(AptCache *cache, AptPackageManager **pm) {
    if (!cache || !cache->dep_cache || !pm)
        return make_result(APT_ERROR_CACHE_OPEN_FAILED,
                           "Invalid cache or output pointer for pm create");

    try {
        *pm = new AptPackageManager(cache);

        (*pm)->pm.reset(_system->CreatePM(cache->dep_cache));

        if (!(*pm)->pm) {
            delete *pm;
            *pm = nullptr;
            return make_result(APT_ERROR_INIT_FAILED, "Failed to create package manager");
        }

        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        if (*pm) {
            delete *pm;
            *pm = nullptr;
        }
        return make_result(APT_ERROR_INIT_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

void apt_package_manager_destroy(const AptPackageManager *pm) {
    delete pm;
}

AptResult apt_mark_install(AptCache *cache, const char *package_name) {
    if (!cache || !cache->dep_cache || !package_name) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid arguments for mark_install");
    }

    try {
        // Delegate to unified planner to guarantee parity with simulation
        const char *install_names[1] = {package_name};
        AptPackageChanges dummy{};
        AptResult r = plan_change_internal(cache, install_names, 1, nullptr, 0, nullptr, 0, false, false, true, &dummy);
        apt_free_package_changes(&dummy);
        return r;
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}

AptResult apt_mark_remove(AptCache *cache, const char *package_name, bool purge, bool remove_depends) {
    if (!cache || !cache->dep_cache || !package_name) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, "Invalid arguments for mark_remove");
    }

    try {
        // Delegate to unified planner to guarantee parity with simulation
        const char *remove_names[1] = {package_name};
        AptPackageChanges dummy{};
        AptResult r = plan_change_internal(cache, nullptr, 0, remove_names, 1, nullptr, 0, purge, remove_depends, true,
                                           &dummy);
        apt_free_package_changes(&dummy);
        return r;
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}

// Progress callback implementation
AptProgressCallback global_callback = nullptr;
uintptr_t global_user_data = 0;

// Simple progress callback that handles package manager operations
class SimpleProgressCallback {
public:
    static void InstallProgress(const char *package, int current, int total) {
        if (global_callback && package) {
            global_callback(package, APT_CALLBACK_INST_PROGRESS, current, total, 0, global_user_data);
        }
    }

    static void InstallStart(const char *package) {
        if (global_callback && package) {
            global_callback(package, APT_CALLBACK_INST_START, 0, 0, 0, global_user_data);
        }
    }

    static void InstallStop(const char *package) {
        if (global_callback && package) {
            global_callback(package, APT_CALLBACK_INST_STOP, 0, 0, 0, global_user_data);
        }
    }
};

// ProgressStatus implementations (after global_callback is defined)
bool ProgressStatus::Pulse(pkgAcquire *Owner) {
    // Call parent first — it computes CurrentCPS (bytes/sec)
    const bool ret = pkgAcquireStatus::Pulse(Owner);
    if (global_callback != nullptr) {
        if (TotalBytes > 0) {
            global_callback("", APT_CALLBACK_DOWNLOAD_PROGRESS,
                            static_cast<uint64_t>(CurrentBytes),
                            static_cast<uint64_t>(TotalBytes),
                            static_cast<uint64_t>(CurrentCPS),
                            global_user_data);
        }

        for (const pkgAcquire::Worker *W = Owner->WorkersBegin(); W != nullptr; W = Owner->WorkerStep(W)) {
            if (W->CurrentItem == nullptr) continue;
            const std::string &desc = W->CurrentItem->Description;
            if (desc.empty()) continue;
            const uint64_t cur = W->CurrentSize;
            const uint64_t tot = W->TotalSize;
            if (tot == 0) continue;
            global_callback(desc.c_str(), APT_CALLBACK_DOWNLOAD_ITEM_PROGRESS,
                            cur, tot,
                            static_cast<uint64_t>(CurrentCPS),
                            global_user_data);
        }
    }
    return ret;
}

void ProgressStatus::Fetch(pkgAcquire::ItemDesc &Itm) {
    const char *name = Itm.Description.empty() ? Itm.URI.c_str() : Itm.Description.c_str();
    active_name_.assign(name ? name : "");
    has_active_item_ = true;
    if (global_callback != nullptr) {
        global_callback(active_name_.c_str(), APT_CALLBACK_DOWNLOAD_START, 0, 0, 0, global_user_data);
    }
    pkgAcquireStatus::Fetch(Itm);
}

void ProgressStatus::Done(pkgAcquire::ItemDesc &Itm) {
    const char *name = Itm.Description.empty() ? active_name_.c_str() : Itm.Description.c_str();
    if (global_callback != nullptr) {
        global_callback(name, APT_CALLBACK_DOWNLOAD_STOP, 0, 0, 0, global_user_data);
    }
    has_active_item_ = false;
    active_name_.clear();
    pkgAcquireStatus::Done(Itm);
}

void ProgressStatus::Fail(pkgAcquire::ItemDesc &Itm) {
    has_active_item_ = false;
    active_name_.clear();
    pkgAcquireStatus::Fail(Itm);
}

void ProgressStatus::Stop() {
    if (global_callback != nullptr) {
        global_callback("", APT_CALLBACK_DOWNLOAD_COMPLETE, 100, 100, 0, global_user_data);
    }
    pkgAcquireStatus::Stop();
}

void apt_free_package_info(AptPackageInfo *info) {
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
    if (info->files) {
        for (size_t i = 0; i < info->file_count; ++i) {
            free(info->files[i]);
        }
        free(info->files);
    }

    memset(info, 0, sizeof(AptPackageInfo));
}

void apt_free_package_list(AptPackageList *list) {
    if (!list || !list->packages) return;

    for (size_t i = 0; i < list->count; ++i) {
        apt_free_package_info(&list->packages[i]);
    }

    free(list->packages);
    list->packages = nullptr;
    list->count = 0;
}

// Utility functions
const char *apt_error_string(const AptErrorCode error) {
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

bool apt_has_broken_packages(const AptCache *cache) {
    if (!cache || !cache->dep_cache) return false;
    return cache->dep_cache->BrokenCount() > 0;
}

uint32_t apt_get_broken_count(const AptCache *cache) {
    if (!cache || !cache->dep_cache) return 0;
    return cache->dep_cache->BrokenCount();
}

extern "C" void apt_set_log_callback(AptLogCallback callback, uintptr_t user_data) {
    g_log_callback = callback;
    g_log_user_data = user_data;
}

extern "C" void goAptProgressCallback(const char *package_name,
                                      AptCallbackType callback_type,
                                      uint64_t current,
                                      uint64_t total,
                                      uint64_t speed_bps,
                                      uintptr_t user_data);

extern "C" void goAptLogCallback(const char *message, uintptr_t user_data);


extern "C" void apt_use_go_progress_callback(const uintptr_t user_data) {
    global_callback = goAptProgressCallback;
    global_user_data = user_data;
}

extern "C" void apt_enable_go_log_callback(const uintptr_t user_data) {
    g_log_callback = static_cast<AptLogCallback>(goAptLogCallback);
    g_log_user_data = user_data;
}

// Configuration
AptErrorCode apt_set_config(const char *key, const char *value) {
    if (!key || !value) return APT_ERROR_INIT_FAILED;

    try {
        _config->Set(key, value);
        return APT_SUCCESS;
    } catch (const std::exception &e) {
        emit_log(std::string("Exception: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

char *apt_dump_config() {
    std::ostringstream ss;
    _config->Dump(ss);
    return strdup(ss.str().c_str());
}

void *apt_config_snapshot() {
    try {
        return new Configuration(*_config);
    } catch (...) {
        return nullptr;
    }
}

void apt_config_restore(void *snapshot) {
    if (!snapshot) return;
    delete _config;
    _config = static_cast<Configuration *>(snapshot);
}

void apt_free_package_changes(AptPackageChanges *changes) {
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

    if (changes->kept_back_packages) {
        for (size_t i = 0; i < changes->kept_back_count; i++) {
            free(changes->kept_back_packages[i]);
        }
        free(changes->kept_back_packages);
    }

    if (changes->essential_packages) {
        for (size_t i = 0; i < changes->essential_packages_count; i++) {
            free(changes->essential_packages[i].name);
            free(changes->essential_packages[i].reason);
        }
        free(changes->essential_packages);
    }

    memset(changes, 0, sizeof(AptPackageChanges));
}

// Optimized progress callback implementation - simplified for performance
PackageManagerCallback_t create_common_progress_callback(CallbackBridge *) {
    return [](const char *nevra, const aptCallbackType what, const uint64_t amount, const uint64_t total,
              void *callbackData) {
        switch (what) {
            case APTCALLBACK_UNPACK_ERROR:
            case APTCALLBACK_CPIO_ERROR:
            case APTCALLBACK_SCRIPT_ERROR:
                if (nevra && nevra[0]) {
                    emit_log(std::string(nevra));
                }
                return;
            default:
                break;
        }

        AptCallbackType our_type;
        switch (what) {
            case APTCALLBACK_INST_PROGRESS: our_type = APT_CALLBACK_INST_PROGRESS;
                break;
            case APTCALLBACK_INST_START: our_type = APT_CALLBACK_INST_START;
                break;
            case APTCALLBACK_INST_STOP: our_type = APT_CALLBACK_INST_STOP;
                break;
            default: return;
        }

        auto *bd = static_cast<CallbackBridge *>(callbackData);
        auto effective_name = "";

        if (nevra && nevra[0]) {
            effective_name = nevra;
        } else if (bd && !bd->planned.empty()) {
            const size_t idx = (bd->current_idx < bd->planned.size()) ? bd->current_idx : 0;
            effective_name = bd->planned[idx].c_str();
        }

        if (what == APTCALLBACK_INST_STOP && bd) {
            if (bd->current_idx < bd->planned.size()) {
                bd->current_idx++;
            }
        }

        if (global_callback) {
            global_callback(effective_name, our_type, amount, total, 0, global_user_data);
        }
    };
}

// RPM file detection utility (shared implementation)
bool is_rpm_file(const std::string &path) {
    if (path.length() <= 4 || path.substr(path.length() - 4) != ".rpm") {
        return false;
    }

    struct stat st{};
    if (stat(path.c_str(), &st) != 0 || !S_ISREG(st.st_mode)) {
        return false;
    }

    return true;
}

// File installation support preprocess arguments to detect and handle RPM files
AptResult apt_preprocess_install_arguments(const char **install_names, size_t install_count, bool *added_new) {
    if (added_new) *added_new = false;

    if (!install_names || install_count == 0) {
        return make_result(APT_SUCCESS, nullptr);
    }

    try {
        std::vector<std::string> existing_args = _config->FindVector("APT::Arguments");
        std::set<std::string> existing_set(existing_args.begin(), existing_args.end());

        bool any_added = false;

        // Process arguments and add RPM files to APT::Arguments configuration
        for (size_t i = 0; i < install_count; i++) {
            if (!install_names[i]) continue;

            std::string arg(install_names[i]);

            // Use shared RPM file detection logic
            if (is_rpm_file(arg)) {
                if (existing_set.find(arg) == existing_set.end()) {
                    _config->Set("APT::Arguments::", arg);
                    existing_set.insert(arg);
                    any_added = true;
                }
            }
        }

        if (added_new) *added_new = any_added;
        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception in preprocess: ") + e.what()).c_str());
    }
}

// Clear RPM file entries from APT::Arguments to prevent stale state in long-running processes
void apt_clear_install_arguments() {
    _config->Clear("APT::Arguments");
}

// Helper function to check if a lock file can be acquired
static bool check_lock_file(const std::string &path, int &holder_pid) {
    holder_pid = -1;

    const int fd = open(path.c_str(), O_RDWR);
    if (fd < 0) {
        if (errno == ENOENT || errno == EACCES) {
            return true;
        }
        return false;
    }

    flock fl{};
    fl.l_type = F_WRLCK;
    fl.l_whence = SEEK_SET;
    fl.l_start = 0;
    fl.l_len = 0;
    fl.l_pid = 0;

    if (fcntl(fd, F_GETLK, &fl) == -1) {
        close(fd);
        return false;
    }

    close(fd);

    if (fl.l_type == F_UNLCK) {
        return true;
    }

    if (fl.l_pid > 0) {
        holder_pid = fl.l_pid;
    }
    return false;
}

// Helper function to get process name by PID
static std::string get_process_name(int pid) {
    if (pid <= 0) return "";

    char path[64];
    snprintf(path, sizeof(path), "/proc/%d/comm", pid);

    FILE *f = fopen(path, "r");
    if (!f) return "";

    char name[256];
    if (fgets(name, sizeof(name), f)) {
        fclose(f);
        const size_t len = strlen(name);
        if (len > 0 && name[len - 1] == '\n') {
            name[len - 1] = '\0';
        }
        return name;
    }

    fclose(f);
    return "";
}

// Internal structure for lock paths (not exported)
struct LockPaths {
    char *archives_lock;
    char *lists_lock;
};

// Get paths to lock files based on APT configuration
static LockPaths get_lock_paths() {
    LockPaths paths{};

    const std::string archives_dir = _config->FindDir("Dir::Cache::Archives", "/var/cache/apt/archives/");
    paths.archives_lock = safe_strdup(archives_dir + "lock");

    const std::string lists_dir = _config->FindDir("Dir::State::lists", "/var/lib/apt/lists/");
    paths.lists_lock = safe_strdup(lists_dir + "lock");

    return paths;
}

// Free lock paths structure
static void free_lock_paths(LockPaths &paths) {
    free(paths.archives_lock);
    free(paths.lists_lock);
    memset(&paths, 0, sizeof(LockPaths));
}

// Check if APT locks can be acquired without actually acquiring them
AptLockStatus apt_check_lock_status() {
    AptLockStatus status{};
    status.is_locked = false;
    status.can_acquire = true;
    status.lock_pid = -1;

    try {
        LockPaths paths = get_lock_paths();

        // Check archives lock (main lock for install/upgrade operations)
        int archives_holder_pid = -1;
        if (paths.archives_lock && !check_lock_file(paths.archives_lock, archives_holder_pid)) {
            status.is_locked = true;
            status.can_acquire = false;
            status.lock_pid = archives_holder_pid;
            status.lock_file_path = safe_strdup(paths.archives_lock);

            if (archives_holder_pid > 0) {
                const std::string proc_name = get_process_name(archives_holder_pid);
                if (!proc_name.empty()) {
                    status.lock_holder = safe_strdup(proc_name);
                }
            }

            free_lock_paths(paths);
            return status;
        }

        // Check lists lock (lock for update operations)
        int lists_holder_pid = -1;
        if (paths.lists_lock && !check_lock_file(paths.lists_lock, lists_holder_pid)) {
            status.is_locked = true;
            status.can_acquire = false;
            status.lock_pid = lists_holder_pid;
            status.lock_file_path = safe_strdup(paths.lists_lock);

            if (lists_holder_pid > 0) {
                const std::string proc_name = get_process_name(lists_holder_pid);
                if (!proc_name.empty()) {
                    status.lock_holder = safe_strdup(proc_name);
                }
            }

            free_lock_paths(paths);
            return status;
        }

        free_lock_paths(paths);
    } catch (const std::exception &e) {
        status.error_message = safe_strdup(std::string("Exception: ") + e.what());
        status.can_acquire = false;
    }

    return status;
}

// Free lock status structure
void apt_free_lock_status(AptLockStatus *status) {
    if (!status) return;
    free(status->lock_holder);
    free(status->lock_file_path);
    free(status->error_message);
    memset(status, 0, sizeof(AptLockStatus));
}

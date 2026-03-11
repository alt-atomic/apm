#include "apt_cache.h"
#include "internal.h"
#include "error.h"
#include "progress.h"

#include <apt-pkg/acquire-item.h>
#include <apt-pkg/error.h>
#include <apt-pkg/sourcelist.h>

// Returns the name of the first broken package, or nullptr.
static const char *find_first_broken_pkg(pkgDepCache *dep) {
    if (dep == nullptr) return nullptr;
    for (pkgCache::PkgIterator it = dep->PkgBegin(); !it.end(); ++it) {
        if (pkgDepCache::StateCache &st = (*dep)[it]; st.InstBroken() || st.NowBroken()) {
            return it.Name();
        }
    }
    return nullptr;
}

// Opens the cache, optionally acquires the system lock, and checks deps.
AptResult apt_cache_open(const AptSystem *system, AptCache **cache, bool with_lock) {
    if (!system || !cache) return make_result(APT_ERROR_INIT_FAILED, APT_MSG_CACHE_INVALID_ARGS);

    if (!system->system) {
        return make_result(APT_ERROR_INIT_FAILED, APT_MSG_CACHE_SYSTEM_NOT_INIT);
    }

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
            }
            return make_result(APT_ERROR_LOCK_FAILED,
                               "Unable to acquire APT system lock - another process may be using APT");
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
                out = APT_MSG_BROKEN_DEPS;
            }
            delete *cache;
            *cache = nullptr;
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
        }

        (*cache)->dep_cache = (*cache)->cache_file->operator->();
        if (!(*cache)->dep_cache) {
            delete *cache;
            *cache = nullptr;
            return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_CACHE_DEP_FAILED);
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

// Destroys and reopens the cache to pick up on-disk changes.
AptResult apt_cache_refresh(AptCache *cache) {
    if (!cache) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_CACHE_NOT_AVAILABLE);
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
            if (const char *broken = find_first_broken_pkg(cache->cache_file->operator->()); broken && *broken) {
                const std::string out = std::string(
                                      "Some broken packages were found while trying to process build-dependencies for ")
                                  + broken + ".";
                return make_result(APT_ERROR_DEPENDENCY_BROKEN, out.c_str());
            }
            return make_result(APT_ERROR_DEPENDENCY_BROKEN, APT_MSG_BROKEN_DEPS);
        }

        cache->dep_cache = cache->cache_file->operator->();
        if (!cache->dep_cache) {
            return make_result(APT_ERROR_CACHE_REFRESH_FAILED, APT_MSG_CACHE_REFRESH_DEP_FAILED);
        }

        return make_result(check_apt_errors() ? APT_SUCCESS : last_error);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_CACHE_REFRESH_FAILED,
                           (std::string("Exception during cache refresh: ") + e.what()).c_str());
    }
}

// Downloads release files and package indexes, then rebuilds caches.
AptResult apt_cache_update(AptCache *cache) {
    if (!cache || !cache->cache_file) return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_CACHE_NOT_AVAILABLE);

    try {
        FileFd Lock;
        if (!_config->FindB("Debug::NoLocking", false)) {
            Lock.Fd(GetLock(_config->FindDir("Dir::State::Lists") + "lock"));
            if (_error->PendingError()) {
                return make_result(APT_ERROR_LOCK_FAILED, APT_MSG_LOCK_LIST_DIR);
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
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, APT_MSG_CACHE_INVALIDATE_FAILED);
        }

        if (!source_list.GetReleases(&acquire)) {
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, APT_MSG_CACHE_RELEASE_FAILED);
        }

        auto fetch_result = acquire.Run();
        if (fetch_result != pkgAcquire::Continue) {
            for (auto I = acquire.ItemsBegin(); I != acquire.ItemsEnd(); ++I) {
                if ((*I)->Status == pkgAcquire::Item::StatError && !(*I)->ErrorText.empty()) {
                    std::string error_msg = "Repository update failed: " + (*I)->DescURI() + " " + (*I)->ErrorText;
                    return make_result(APT_ERROR_DOWNLOAD_FAILED, error_msg.c_str());
                }
            }
            return make_result(APT_ERROR_DOWNLOAD_FAILED, APT_MSG_CACHE_DOWNLOAD_FAILED);
        }

        for (auto I = acquire.ItemsBegin(); I != acquire.ItemsEnd(); ++I) {
            if ((*I)->Status == pkgAcquire::Item::StatError && !(*I)->ErrorText.empty()) {
                std::string error_msg = "Repository update failed: " + (*I)->DescURI() + " " + (*I)->ErrorText;
                return make_result(APT_ERROR_DOWNLOAD_FAILED, error_msg.c_str());
            }
        }

        if (!source_list.GetIndexes(&acquire)) {
            return make_result(APT_ERROR_CACHE_UPDATE_FAILED, APT_MSG_CACHE_INDEX_FAILED);
        }

        fetch_result = acquire.Run();
        if (fetch_result != pkgAcquire::Continue) {
            for (auto I = acquire.ItemsBegin(); I != acquire.ItemsEnd(); ++I) {
                if ((*I)->Status == pkgAcquire::Item::StatError && !(*I)->ErrorText.empty()) {
                    std::string error_msg = "Package index update failed: " + (*I)->DescURI() + " " + (*I)->ErrorText;
                    return make_result(APT_ERROR_DOWNLOAD_FAILED, error_msg.c_str());
                }
            }
            return make_result(APT_ERROR_DOWNLOAD_FAILED,
                               "Package index update failed: Unable to download package lists");
        }

        for (auto I = acquire.ItemsBegin(); I != acquire.ItemsEnd(); ++I) {
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


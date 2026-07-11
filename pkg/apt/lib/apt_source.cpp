#include "apt_source.h"
#include "internal.h"
#include "error.h"
#include "progress.h"

#include <apt-pkg/acquire-item.h>
#include <apt-pkg/error.h>
#include <apt-pkg/fileutl.h>
#include <apt-pkg/pkgrecords.h>
#include <apt-pkg/sourcelist.h>
#include <apt-pkg/srcrecords.h>
#include <apt-pkg/version.h>

#include <string>
#include <vector>

// Resolves a package name (optionally "name=version") to the best source record.
static pkgSrcRecords::Parser *find_src(const char *name, pkgRecords &recs,
                                       pkgSrcRecords &src_recs, pkgDepCache &cache) {
    std::string ver_tag;
    std::string tmp_src = name;
    if (const auto eq = tmp_src.rfind('='); eq != std::string::npos) {
        ver_tag = tmp_src.substr(eq + 1);
        tmp_src = tmp_src.substr(0, eq);
    }

    // Map a binary package name to its source package name
    std::string src;
    if (ver_tag.empty()) {
        if (const pkgCache::PkgIterator pkg = cache.FindPkg(tmp_src); !pkg.end()) {
            if (const pkgCache::VerIterator ver = cache.GetCandidateVer(pkg); !ver.end()) {
                src = recs.Lookup(ver.FileList()).SourcePkg();
            }
        }
    }
    if (src.empty()) src = tmp_src;

    // Pick the exact version or the newest record
    pkgSrcRecords::Parser *last = nullptr;
    off_t offset = 0;
    std::string version;
    bool is_match = !ver_tag.empty();

    src_recs.Restart();
    pkgSrcRecords::Parser *parse;
    while ((parse = src_recs.Find(src.c_str(), false)) != nullptr) {
        const std::string ver = parse->Version();

        if (is_match && parse->Package() != src) continue;

        if (!ver_tag.empty()) {
            if (cache.VS().CmpVersion(ver_tag, ver) != 0) continue;
            last = parse;
            offset = parse->Offset();
            break;
        }

        if (last == nullptr || cache.VS().CmpVersion(version, ver) < 0 ||
            (parse->Package() == src && !is_match)) {
            is_match = parse->Package() == src;
            last = parse;
            offset = parse->Offset();
            version = ver;
        }
    }

    if (last == nullptr || !last->Jump(offset)) return nullptr;
    return last;
}

static void free_source_items(std::vector<AptSourcePackage> &items) {
    for (auto &item : items) {
        free(item.name);
        free(item.version);
        free(item.file);
    }
    items.clear();
}

AptResult apt_source_download(AptCache *cache, const char **names, size_t count,
                              const char *dest_dir, AptSourceResult *result) {
    if (!cache || !cache->dep_cache || !names || count == 0 || !result) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_PARAMS);
    }

    result->items = nullptr;
    result->count = 0;

    std::vector<AptSourcePackage> items;
    try {
        pkgSourceList list;
        if (!list.ReadMainList()) {
            std::string err = collect_pending_errors();
            if (err.empty()) err = "The list of sources could not be read";
            return make_result(APT_ERROR_OPERATION_FAILED, err.c_str());
        }

        pkgRecords recs(*cache->dep_cache);
        pkgSrcRecords src_recs(list);
        if (_error->PendingError()) {
            return make_result(APT_ERROR_OPERATION_FAILED, collect_pending_errors().c_str());
        }

        ProgressStatus status;
        pkgAcquire fetcher(&status);
        const std::string dest = (dest_dir && *dest_dir) ? dest_dir : "";

        for (size_t i = 0; i < count; ++i) {
            pkgSrcRecords::Parser *last = find_src(names[i], recs, src_recs, *cache->dep_cache);
            if (last == nullptr) {
                free_source_items(items);
                const std::string msg = std::string("Unable to find a source package for ") + names[i];
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND, msg.c_str());
            }

            std::vector<pkgSrcRecords::File> files;
            if (!last->Files(files)) {
                free_source_items(items);
                std::string err = collect_pending_errors();
                if (err.empty()) err = std::string("Failed to read source file list for ") + names[i];
                return make_result(APT_ERROR_OPERATION_FAILED, err.c_str());
            }

            bool queued = false;
            for (const auto &f : files) {
                if (f.Type != "srpm") continue;

                new pkgAcqFile(&fetcher, last->Index().ArchiveURI(f.Path),
                               f.MD5Hash, f.Size,
                               last->Index().SourceInfo(*last, f),
                               last->Package(), dest);

                AptSourcePackage item{};
                item.name = safe_strdup(last->Package());
                item.version = safe_strdup(last->Version());
                const std::string base = flNotDir(f.Path);
                item.file = safe_strdup(dest.empty() ? base : flCombine(dest, base));
                item.size = f.Size;
                items.push_back(item);
                queued = true;
            }

            if (!queued) {
                free_source_items(items);
                const std::string msg = std::string("No .src.rpm file in source record for ") + names[i];
                return make_result(APT_ERROR_PACKAGE_NOT_FOUND, msg.c_str());
            }
        }

        if (fetcher.Run() != pkgAcquire::Continue) {
            free_source_items(items);
            std::string err = collect_pending_errors();
            if (err.empty()) err = APT_MSG_DOWNLOAD_FAILED;
            return make_result(APT_ERROR_DOWNLOAD_FAILED, err.c_str());
        }

        for (auto I = fetcher.ItemsBegin(); I != fetcher.ItemsEnd(); ++I) {
            if ((*I)->Status == pkgAcquire::Item::StatDone && (*I)->Complete) continue;
            free_source_items(items);
            const std::string msg = "Failed to fetch " + (*I)->DescURI() + " " + (*I)->ErrorText;
            return make_result(APT_ERROR_DOWNLOAD_FAILED, msg.c_str());
        }

        result->items = static_cast<AptSourcePackage *>(calloc(items.size(), sizeof(AptSourcePackage)));
        if (result->items == nullptr) {
            free_source_items(items);
            return make_result(APT_ERROR_UNKNOWN, "Failed to allocate source download result");
        }
        for (size_t i = 0; i < items.size(); ++i) {
            result->items[i] = items[i];
        }
        result->count = items.size();

        return make_result(APT_SUCCESS);
    } catch (const std::exception &e) {
        free_source_items(items);
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception: ") + e.what()).c_str());
    }
}

void apt_free_source_result(AptSourceResult *result) {
    if (result == nullptr || result->items == nullptr) return;
    for (size_t i = 0; i < result->count; ++i) {
        free(result->items[i].name);
        free(result->items[i].version);
        free(result->items[i].file);
    }
    free(result->items);
    result->items = nullptr;
    result->count = 0;
}

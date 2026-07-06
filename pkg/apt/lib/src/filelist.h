#pragma once

#include "internal.h"
#include "apt_package.h"

#include <string>
#include <vector>
#include <unordered_map>

#include <rpm/rpmio.h>

// Caches open RPM file descriptors for efficient repeated reads.
// Closes all handles on destruction.
struct AptFileListCache {
    std::unordered_map<std::string, FD_t> fds;

    ~AptFileListCache() { close(); }

    void close() {
        for (const auto &pair : fds) {
            if (pair.second) Fclose(pair.second);
        }
        fds.clear();
    }
};

// Reads the file list from an RPM header at `offset` in `hd_list_path`.
bool apt_filelist_read(const char *hd_list_path, off_t offset,
                       std::vector<std::string> &out_files,
                       AptFileListCache &cache);

// Populates `info->files` from the RPM header referenced by `vf`.
void apt_filelist_fill(pkgCache::VerFileIterator &vf, AptPackageInfo *info,
                       AptFileListCache &cache);

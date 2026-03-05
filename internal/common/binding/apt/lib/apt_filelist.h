#pragma once

#include "apt_internal.h"

#include <string>
#include <vector>
#include <unordered_map>

#include <rpm/rpmio.h>

// Cache of open hdlist file descriptors to avoid reopening on each package.
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

// Read file list for a package version from the repository index
bool apt_filelist_read(const char *hd_list_path, off_t offset,
                       std::vector<std::string> &out_files,
                       AptFileListCache &cache);

// Fill info->files and info->file_count from the candidate version's
void apt_filelist_fill(pkgCache::VerFileIterator &vf, AptPackageInfo *info,
                       AptFileListCache &cache);

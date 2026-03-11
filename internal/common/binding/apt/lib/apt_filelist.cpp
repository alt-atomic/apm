#include "filelist.h"

#include <rpm/rpmlib.h>
#include <rpm/rpmio.h>
#include <rpm/header.h>
#include <rpm/rpmtd.h>
#include <rpm/rpmtag.h>

#include <unistd.h>
#include <cstdlib>

// Opens (or reuses cached) RPM header list file and reads file names at offset.
bool apt_filelist_read(const char *hd_list_path, const off_t offset,
                       std::vector<std::string> &out_files,
                       AptFileListCache &cache)
{
    if (!hd_list_path || !*hd_list_path)
        return false;

    const std::string path(hd_list_path);
    FD_t fd = nullptr;

    if (const auto it = cache.fds.find(path); it != cache.fds.end()) {
        fd = it->second;
    } else {
        fd = Fopen(hd_list_path, "r");
        if (!fd || Ferror(fd)) {
            if (fd) Fclose(fd);
            return false;
        }
        cache.fds[path] = fd;
    }

    if (lseek(Fileno(fd), offset, SEEK_SET) != offset)
        return false;

    const auto hdr = headerRead(fd, HEADER_MAGIC_YES);
    if (!hdr)
        return false;

    const auto td = rpmtdNew();
    if (headerGet(hdr, RPMTAG_FILENAMES, td, HEADERGET_EXT)) {
        const char *fn;
        while ((fn = rpmtdNextString(td)) != nullptr) {
            out_files.emplace_back(fn);
        }
    } else if (headerGet(hdr, RPMTAG_OLDFILENAMES, td, HEADERGET_MINMEM)) {
        const char *fn;
        while ((fn = rpmtdNextString(td)) != nullptr) {
            out_files.emplace_back(fn);
        }
    }

    rpmtdFree(td);
    headerFree(hdr);
    return true;
}

// Reads file list from RPM header and allocates info->files array.
void apt_filelist_fill(pkgCache::VerFileIterator &vf, AptPackageInfo *info,
                       AptFileListCache &cache)
{
    if (vf.end())
        return;

    const pkgCache::PkgFileIterator pf = vf.File();
    const char *hd_list_path = pf.FileName();
    if (!hd_list_path || !*hd_list_path)
        return;

    std::vector<std::string> files;
    if (!apt_filelist_read(hd_list_path, vf->Offset, files, cache) || files.empty())
        return;

    info->file_count = files.size();
    info->files = static_cast<char **>(calloc(info->file_count, sizeof(char *)));
    if (!info->files) {
        info->file_count = 0;
        return;
    }
    for (size_t i = 0; i < files.size(); ++i) {
        info->files[i] = safe_strdup(files[i]);
    }
}

#pragma once

#include "internal.h"
#include "apt_package.h"
#include "filelist.h"

#include <apt-pkg/pkgrecords.h>

#include <string>

// Determines package installation state from cache iterator.
void fill_package_state(pkgCache::PkgIterator &pkg, AptPackageInfo *info);

// Fills basic package flags from cache iterator.
void fill_package_flags(pkgCache::PkgIterator &pkg, AptPackageInfo *info);

// Fills metadata fields from pkgRecords::Parser (description, maintainer, hashes, etc.).
void fill_package_metadata(pkgRecords::Parser &parser, AptPackageInfo *info);

// Parses fields from raw record text that aren't available via Parser API.
void fill_package_from_record(const std::string &record, AptPackageInfo *info);

// Extracts a single field value from raw record text by tag name.
std::string extract_record_field(const std::string &record, const char *tag);

// Fills version, architecture, sizes, provides and depends from candidate version.
void fill_package_version(pkgCache::PkgIterator &pkg,
                          pkgCache::VerIterator &ver,
                          const std::string &record,
                          AptPackageInfo *info);

// Fills aliases for i586/32bit variant packages.
void fill_package_aliases(pkgCache &cache,
                          const pkgCache::PkgIterator &pkg,
                          pkgDepCache::Policy &policy,
                          const std::string &effective_name,
                          AptPackageInfo *info);

// Resolves a virtual package to its real provider.
// If pkg has no candidate version, searches ProvidesList for the real owner package.
bool resolve_virtual_package(pkgCache::PkgIterator &pkg,
                             pkgCache::VerIterator &ver,
                             pkgCache::VerFileIterator &vf,
                             pkgDepCache::Policy &policy);

// Fills all package info fields using shared helpers.
void fill_package_info(pkgCache &cache,
                       pkgCache::PkgIterator &pkg,
                       pkgCache::VerIterator &ver,
                       pkgCache::VerFileIterator &vf,
                       pkgRecords &records,
                       pkgDepCache::Policy &policy,
                       const std::string &effective_name,
                       AptPackageInfo *info,
                       AptFileListCache &fl_cache);

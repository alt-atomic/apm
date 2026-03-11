#include "package_fill.h"

#include <cstring>
#include <set>
#include <vector>

void fill_package_state(pkgCache::PkgIterator &pkg, AptPackageInfo *info) {
    if (!pkg.CurrentVer().end()) {
        switch (pkg->CurrentState) {
            case pkgCache::State::Installed:
                info->state = APT_PKG_STATE_INSTALLED;
                break;
            case pkgCache::State::ConfigFiles:
                info->state = APT_PKG_STATE_CONFIG_FILES;
                break;
            case pkgCache::State::UnPacked:
                info->state = APT_PKG_STATE_UNPACKED;
                break;
            case pkgCache::State::HalfConfigured:
                info->state = APT_PKG_STATE_HALF_CONFIGURED;
                break;
            case pkgCache::State::HalfInstalled:
                info->state = APT_PKG_STATE_HALF_INSTALLED;
                break;
            default:
                info->state = APT_PKG_STATE_INSTALLED;
                break;
        }
    } else {
        info->state = APT_PKG_STATE_NOT_INSTALLED;
    }
}

void fill_package_flags(pkgCache::PkgIterator &pkg, AptPackageInfo *info) {
    info->package_id = pkg->ID;
    info->essential = (pkg->Flags & pkgCache::Flag::Essential) != 0;
    info->auto_installed = (pkg->Flags & pkgCache::Flag::Auto) != 0;
    info->section = safe_strdup(pkg.Section() ? pkg.Section() : "unknown");
}

void fill_package_metadata(pkgRecords::Parser &parser, AptPackageInfo *info) {
    info->description = safe_strdup(parser.LongDesc());
    info->short_description = safe_strdup(parser.ShortDesc());
    info->maintainer = safe_strdup(parser.Maintainer());
    info->source_package = safe_strdup(parser.SourcePkg());
    info->md5_hash = safe_strdup(parser.MD5Hash());
    info->blake2b_hash = safe_strdup(parser.BLAKE2b());
    info->filename = safe_strdup(parser.FileName());
    info->changelog = safe_strdup(parser.Changelog());
}

void fill_package_from_record(const std::string &record, AptPackageInfo *info) {
    if (const size_t pos = record.find("Homepage: "); pos != std::string::npos) {
        const size_t start = pos + 10;
        size_t end = record.find('\n', start);
        if (end == std::string::npos) end = record.length();
        info->homepage = safe_strdup(record.substr(start, end - start));
    }

    if (!info->provides) {
        if (const size_t pos = record.find("Provides: "); pos != std::string::npos) {
            const size_t start = pos + 10;
            size_t end = record.find('\n', start);
            if (end == std::string::npos) end = record.length();
            info->provides = safe_strdup(record.substr(start, end - start));
        }
    }
}

std::string extract_record_field(const std::string &record, const char *tag) {
    const std::string prefix = std::string(tag) + ": ";
    const size_t pos = record.find(prefix);
    if (pos == std::string::npos) return {};
    const size_t start = pos + prefix.size();
    size_t end = record.find('\n', start);
    if (end == std::string::npos) end = record.length();
    return record.substr(start, end - start);
}

void fill_package_version(pkgCache::PkgIterator &pkg,
                          pkgCache::VerIterator &ver,
                          const std::string &record,
                          AptPackageInfo *info) {
    if (!ver.end()) {
        info->version = safe_strdup(ver.VerStr());
        info->architecture = safe_strdup(ver.Arch());
        info->installed_size = ver->InstalledSize;
        info->download_size = ver->Size;

        if (ver.Arch() != nullptr && *ver.Arch() != '\0') {
            if (const char *pri = pkgCache::Priority(ver->Priority); pri && *pri) {
                info->priority = safe_strdup(pri);
            }
        }

        // Provides from iterator
        {
            std::set<std::string> prov_names;
            for (pkgCache::PrvIterator prv = ver.ProvidesList(); !prv.end(); ++prv) {
                if (const char *n = prv.Name(); n && *n) prov_names.insert(n);
            }
            if (!prov_names.empty()) {
                std::string joined;
                for (const auto &prov_name : prov_names) {
                    if (!joined.empty()) joined += ", ";
                    joined += prov_name;
                }
                info->provides = safe_strdup(joined);
            }
        }

        // Depends from iterator
        {
            std::set<std::string> dep_names;
            for (pkgCache::DepIterator dep = ver.DependsList(); !dep.end(); ++dep) {
                if (dep->Type != pkgCache::Dep::Depends && dep->Type != pkgCache::Dep::PreDepends)
                    continue;
                if (pkgCache::PkgIterator tpkg = dep.TargetPkg(); !tpkg.end() && tpkg.Name() != nullptr) {
                    dep_names.insert(tpkg.Name());
                }
            }
            if (!dep_names.empty()) {
                std::string joined;
                for (const auto &dep_name : dep_names) {
                    if (!joined.empty()) joined += ", ";
                    joined += dep_name;
                }
                info->depends = safe_strdup(joined);
            }
        }
    } else {
        // Fallback to raw record
        if (const std::string v = extract_record_field(record, "Version"); !v.empty()) info->version = safe_strdup(v);
        if (const std::string a = extract_record_field(record, "Architecture"); !a.empty()) info->architecture = safe_strdup(a);
        if (const std::string d = extract_record_field(record, "Depends"); !d.empty()) info->depends = safe_strdup(d);

        info->installed_size = 0;
        info->download_size = 0;
    }

    if (!info->priority) {
        info->priority = safe_strdup("normal");
    }
}

void fill_package_aliases(pkgCache &cache,
                          const pkgCache::PkgIterator &pkg,
                          pkgDepCache::Policy &policy,
                          const std::string &effective_name,
                          AptPackageInfo *info) {
    std::vector<std::string> aliases;

    if (const std::string current_name = pkg.end() ? effective_name : std::string(pkg.Name()); !current_name.empty() && current_name.rfind("i586-", 0) == 0) {
        const std::string stripped = current_name.substr(5);
        if (const pkgCache::PkgIterator base_pkg = cache.FindPkg(stripped); !base_pkg.end()) {
            aliases.push_back(current_name);
            aliases.push_back(current_name + ".32bit");
        }
    } else {
        const std::string i586_variant = std::string("i586-") + effective_name;
        const pkgCache::PkgIterator i586_pkg = cache.FindPkg(i586_variant);
        const bool has_i586_variant = !i586_pkg.end();

        bool is_32bit_arch = false;
        if (!pkg.end()) {
            if (const pkgCache::VerIterator cand = policy.GetCandidateVer(pkg); !cand.end() && cand.Arch() != nullptr) {
                if (const char *a = cand.Arch(); strcmp(a, "i586") == 0 || strcmp(a, "i386") == 0) {
                    is_32bit_arch = true;
                }
            }
        }

        if (has_i586_variant || is_32bit_arch) {
            aliases.push_back(i586_variant);
            aliases.push_back(i586_variant + ".32bit");
        }
    }

    if (!aliases.empty()) {
        info->alias_count = aliases.size();
        info->aliases = static_cast<char **>(calloc(info->alias_count, sizeof(char *)));
        for (size_t i = 0; i < aliases.size(); ++i) {
            info->aliases[i] = safe_strdup(aliases[i].c_str());
        }
    }
}

void fill_package_info(pkgCache &cache,
                       pkgCache::PkgIterator &pkg,
                       pkgCache::VerIterator &ver,
                       pkgCache::VerFileIterator &vf,
                       pkgRecords &records,
                       pkgDepCache::Policy &policy,
                       const std::string &effective_name,
                       AptPackageInfo *info,
                       AptFileListCache &fl_cache) {
    info->name = safe_strdup(effective_name.c_str());

    if (!pkg.end()) {
        fill_package_flags(pkg, info);
        fill_package_state(pkg, info);
    } else {
        info->state = APT_PKG_STATE_NOT_INSTALLED;
    }

    if (!vf.end()) {
        pkgRecords::Parser &parser = records.Lookup(vf);
        fill_package_metadata(parser, info);

        const char *rec_start, *rec_stop;
        parser.GetRec(rec_start, rec_stop);
        const std::string record(rec_start, rec_stop - rec_start);

        fill_package_version(pkg, ver, record, info);
        fill_package_from_record(record, info);
    } else {
        info->version = safe_strdup("unknown");
        info->architecture = safe_strdup("unknown");
        info->priority = safe_strdup("normal");
    }

    fill_package_aliases(cache, pkg, policy, effective_name, info);

    if (!vf.end()) {
        apt_filelist_fill(vf, info, fl_cache);
    }
}

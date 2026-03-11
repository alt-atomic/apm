#include "changes.h"
#include <apt-pkg/version.h>
#include <cstring>

// Returns true if `pkg` is obsoleted by another package being installed.
static bool is_obsoleted(const AptCache *cache, const pkgCache::PkgIterator &pkg) {
    if (pkg.CurrentVer().end()) return false;

    for (pkgCache::DepIterator D = pkg.RevDependsList(); !D.end(); ++D) {
        if (D->Type == pkgCache::Dep::Obsoletes &&
            (*cache->dep_cache)[D.ParentPkg()].Install() &&
            (static_cast<pkgCache::Version *>(D.ParentVer()) == (*cache->dep_cache)[D.ParentPkg()].InstallVer ||
             static_cast<pkgCache::Version *>(D.ParentVer()) == static_cast<pkgCache::Version *>(D.ParentPkg().CurrentVer())) &&
            cache->dep_cache->VS().CheckDep(pkg.CurrentVer().VerStr(), D)) {
            return true;
        }
    }
    return false;
}

// Walks the dep cache and categorizes every changed package.
void collect_package_changes(const AptCache *cache,
                             const std::set<std::string> &requested_install,
                             std::vector<std::string> &extra_installed,
                             std::vector<std::string> &upgraded,
                             std::vector<std::string> &new_installed,
                             std::vector<std::string> &removed,
                             std::vector<std::string> &kept_back,
                             uint64_t &download_size,
                             int64_t &install_size) {
    download_size = 0;
    install_size = 0;

    for (pkgCache::PkgIterator iter = cache->dep_cache->PkgBegin(); !iter.end(); ++iter) {
        pkgDepCache::StateCache &st = (*cache->dep_cache)[iter];

        if (iter->CurrentVer != 0 && st.Upgradable() && !st.Upgrade() && !st.Delete()) {
            kept_back.emplace_back(iter.Name());
            continue;
        }

        if (st.NewInstall()) {
            new_installed.emplace_back(iter.Name());
            if (requested_install.find(iter.Name()) == requested_install.end()) {
                extra_installed.emplace_back(iter.Name());
            }
            if (st.CandidateVer != nullptr) {
                download_size += static_cast<uint64_t>(st.CandidateVer->Size);
                install_size += static_cast<int64_t>(st.CandidateVer->InstalledSize);
            }
        } else if (st.Upgrade()) {
            upgraded.emplace_back(iter.Name());
            if (st.CandidateVer != nullptr) {
                download_size += static_cast<uint64_t>(st.CandidateVer->Size);
                install_size += static_cast<int64_t>(st.CandidateVer->InstalledSize);
                if (pkgCache::VerIterator currentVer = iter.CurrentVer(); !currentVer.end()) {
                    install_size -= static_cast<int64_t>(currentVer->InstalledSize);
                }
            }
        } else if (st.Downgrade()) {
            upgraded.emplace_back(iter.Name());
            if (st.CandidateVer != nullptr) {
                download_size += static_cast<uint64_t>(st.CandidateVer->Size);
                install_size += static_cast<int64_t>(st.CandidateVer->InstalledSize);
                if (pkgCache::VerIterator currentVer = iter.CurrentVer(); !currentVer.end()) {
                    install_size -= static_cast<int64_t>(currentVer->InstalledSize);
                }
            }
        } else if (st.Delete()) {
            removed.emplace_back(iter.Name());
            if (st.InstallVer != nullptr) install_size -= static_cast<int64_t>(st.InstallVer->InstalledSize);
        } else if ((st.iFlags & pkgDepCache::ReInstall) != 0) {
            new_installed.emplace_back(iter.Name());
            if (pkgCache::VerIterator currentVer = iter.CurrentVer(); !currentVer.end()) {
                download_size += static_cast<uint64_t>(currentVer->Size);
            }
        }
    }
}

// Collects essential/important packages marked for deletion.
void collect_essential_packages(const AptCache *cache,
                                std::vector<std::pair<std::string, std::string>> &essential_list) {
    if (!cache || !cache->dep_cache) return;

    std::vector<bool> added(cache->dep_cache->Head().PackageCount, false);

    for (pkgCache::PkgIterator I = cache->dep_cache->PkgBegin(); !I.end(); ++I) {
        if (((I->Flags & pkgCache::Flag::Essential) == 0) &&
            ((I->Flags & pkgCache::Flag::Important) == 0)) {
            continue;
        }

        if ((*cache->dep_cache)[I].Delete() && !added[I->ID] && !is_obsoleted(cache, I)) {
            added[I->ID] = true;
            essential_list.emplace_back(I.Name(), "");
        }

        if (I->CurrentVer == 0) continue;

        for (pkgCache::DepIterator D = I.CurrentVer().DependsList(); !D.end(); ++D) {
            if (D->Type != pkgCache::Dep::PreDepends &&
                D->Type != pkgCache::Dep::Depends) {
                continue;
            }

            pkgCache::PkgIterator P = D.SmartTargetPkg();
            if (P.end() || !(*cache->dep_cache)[P].Delete() || added[P->ID]) {
                continue;
            }

            if (!is_obsoleted(cache, P)) {
                added[P->ID] = true;
                essential_list.emplace_back(P.Name(), I.Name());
            }
        }
    }
}

// Allocates and fills the C AptPackageChanges struct from C++ vectors.
void populate_changes_structure(AptPackageChanges *changes,
                                const std::vector<std::string> &extra_installed,
                                const std::vector<std::string> &upgraded,
                                const std::vector<std::string> &new_installed,
                                const std::vector<std::string> &removed,
                                const std::vector<std::string> &kept_back,
                                const size_t not_upgraded_count,
                                const std::vector<std::pair<std::string, std::string>> &essential_list,
                                const uint64_t download_size,
                                const int64_t install_size) {
    changes->extra_installed_count = extra_installed.size();
    changes->upgraded_count = upgraded.size();
    changes->new_installed_count = new_installed.size();
    changes->removed_count = removed.size();
    changes->kept_back_count = kept_back.size();
    changes->not_upgraded_count = not_upgraded_count;
    changes->download_size = download_size;
    changes->install_size = install_size;

    if (changes->extra_installed_count > 0) {
        changes->extra_installed = static_cast<char **>(malloc(changes->extra_installed_count * sizeof(char *)));
        for (size_t i = 0; i < changes->extra_installed_count; ++i)
            changes->extra_installed[i] = safe_strdup(extra_installed[i]);
    }
    if (changes->removed_count > 0) {
        changes->removed_packages = static_cast<char **>(malloc(changes->removed_count * sizeof(char *)));
        for (size_t i = 0; i < changes->removed_count; ++i)
            changes->removed_packages[i] = safe_strdup(removed[i]);
    }
    if (changes->upgraded_count > 0) {
        changes->upgraded_packages = static_cast<char **>(malloc(changes->upgraded_count * sizeof(char *)));
        for (size_t i = 0; i < changes->upgraded_count; ++i)
            changes->upgraded_packages[i] = safe_strdup(upgraded[i]);
    }
    if (changes->new_installed_count > 0) {
        changes->new_installed_packages = static_cast<char **>(malloc(changes->new_installed_count * sizeof(char *)));
        for (size_t i = 0; i < changes->new_installed_count; ++i)
            changes->new_installed_packages[i] = safe_strdup(new_installed[i]);
    }
    if (changes->kept_back_count > 0) {
        changes->kept_back_packages = static_cast<char **>(malloc(changes->kept_back_count * sizeof(char *)));
        for (size_t i = 0; i < changes->kept_back_count; ++i)
            changes->kept_back_packages[i] = safe_strdup(kept_back[i]);
    }

    changes->essential_packages_count = essential_list.size();
    if (changes->essential_packages_count > 0) {
        changes->essential_packages = static_cast<AptEssentialPackage *>(malloc(
            changes->essential_packages_count * sizeof(AptEssentialPackage)));
        for (size_t i = 0; i < changes->essential_packages_count; ++i) {
            changes->essential_packages[i].name = safe_strdup(essential_list[i].first);
            changes->essential_packages[i].reason = safe_strdup(essential_list[i].second);
        }
    }
}

// Frees all string arrays and essential entries, then zeroes the struct.
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

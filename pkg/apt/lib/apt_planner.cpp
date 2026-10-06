#include "planner.h"
#include "apt_cache.h"
#include "mark.h"
#include "dependency.h"
#include "changes.h"
#include "apt_ext_rpm.h"
#include "ext_rpm.h"

#include <apt-pkg/algorithms.h>
#include <apt-pkg/error.h>

// True when any name is a local .rpm file.
static bool has_rpm_file(const std::vector<std::string> &names) {
    for (const auto &name : names) {
        if (is_rpm_file(name)) return true;
    }
    return false;
}

static AptResult register_rpm_files(const std::vector<std::string> &names, bool &need_refresh) {
    if (names.empty()) return make_result(APT_SUCCESS, nullptr);

    std::vector<const char *> raw(names.size());
    for (size_t i = 0; i < names.size(); i++) raw[i] = names[i].c_str();

    bool added_new = false;
    const AptResult result = apt_preprocess_install_arguments(raw.data(), raw.size(), &added_new);
    if (result.code == APT_SUCCESS && added_new) need_refresh = true;
    return result;
}

// Registers local .rpm files as an APT source and refreshes the cache when new ones appear
AptResult prepare_transaction(AptCache *cache, const AptTransaction &tx) {
    if (!cache) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_PARAMS);
    }
    if (!cache->cache_file || !cache->dep_cache) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_CACHE_FILE_NOT_AVAILABLE);
    }
    if (!has_rpm_file(tx.install_names) && !has_rpm_file(tx.remove_names) && !has_rpm_file(tx.reinstall_names)) {
        return make_result(APT_SUCCESS, nullptr);
    }

    bool need_refresh = false;
    if (const AptResult r = register_rpm_files(tx.install_names, need_refresh); r.code != APT_SUCCESS) return r;
    if (const AptResult r = register_rpm_files(tx.remove_names, need_refresh); r.code != APT_SUCCESS) return r;
    if (const AptResult r = register_rpm_files(tx.reinstall_names, need_refresh); r.code != APT_SUCCESS) return r;

    if (need_refresh) {
        return apt_cache_refresh(cache);
    }
    return make_result(APT_SUCCESS, nullptr);
}

// Dependency failure with pending APT errors drained into the message
static AptResult dependency_failure(std::string msg) {
    if (const std::string pending = collect_pending_errors(); !pending.empty()) msg += "\n" + pending;
    return make_result(APT_ERROR_DEPENDENCY_BROKEN, msg.c_str());
}

// Names the first broken package
static AptResult broken_result(const AptCache *cache) {
    std::string out = APT_MSG_BROKEN_DEPS;
    for (pkgCache::PkgIterator it = cache->dep_cache->PkgBegin(); !it.end(); ++it) {
        if (pkgDepCache::StateCache &st = (*cache->dep_cache)[it]; st.InstBroken() || st.NowBroken()) {
            out += std::string(": ") + it.Name();
            break;
        }
    }
    return dependency_failure(out);
}

// Marks a distribution upgrade
static AptResult mark_dist_upgrade(const AptCache *cache, AptPackageChanges *changes) {
    pkgDistUpgrade(*cache->dep_cache);

    if (cache->dep_cache->BrokenCount() > 0) {
        pkgProblemResolver Fix(cache->dep_cache);
        Fix.InstallProtect();
        (void) Fix.Resolve(true);
    }
    if (cache->dep_cache->BrokenCount() > 0) {
        return broken_result(cache);
    }
    if (_error->PendingError()) {
        return make_result(APT_ERROR_DEPENDENCY_BROKEN);
    }

    PlanChanges plan = collect_plan_changes(cache, {});
    plan.extra_installed.clear();
    populate_changes(changes, plan, {}, {}, {}, true);
    return make_result(APT_SUCCESS, nullptr);
}

// Marks automatically installed packages nobody depends on for removal
static AptResult mark_autoremove(const AptCache *cache, AptPackageChanges *changes) {
    if (cache->dep_cache->BrokenCount() != 0) {
        return make_result(APT_ERROR_DEPENDENCY_BROKEN, APT_MSG_BROKEN_CACHE);
    }

    std::set<std::string> kept_packages;
    std::set<std::string> unneeded_packages;

    if (!pkgAutoremoveGetKeptAndUnneededPackages(*cache->dep_cache, &kept_packages, &unneeded_packages)) {
        return dependency_failure(APT_MSG_AUTOREMOVE_CALC_FAILED);
    }

    pkgProblemResolver Fix(cache->dep_cache);

    for (pkgCache::PkgIterator pkg = cache->dep_cache->PkgBegin(); !pkg.end(); ++pkg) {
        if (pkg->CurrentState != pkgCache::State::Installed) {
            continue;
        }

        if (kept_packages.count(pkg.Name()) != 0) {
            cache->dep_cache->MarkKeep(pkg);
            Fix.Protect(pkg);
        } else {
            cache->dep_cache->MarkDelete(pkg, false);
        }
    }

    if (!Fix.Resolve(false)) {
        return dependency_failure(APT_MSG_AUTOREMOVE_RESOLVE_FAILED);
    }

    if (_error->PendingError()) {
        return make_result(APT_ERROR_DEPENDENCY_BROKEN);
    }

    PlanChanges plan = collect_plan_changes(cache, {});
    plan.extra_installed.clear();
    populate_changes(changes, plan, {}, {}, {}, false);
    return make_result(APT_SUCCESS, nullptr);
}

// Marks a combined install/remove/reinstall operation
static AptResult mark_changes(const AptCache *cache, const AptTransaction &tx, AptPackageChanges *changes) {
    std::set<std::string> requested_install;
    std::set<std::string> requested_remove;
    std::set<std::string> requested_reinstall;
    std::vector<std::string> skipped;

    AptResult result = process_package_installs(cache, tx.install_names, requested_install);
    if (result.code != APT_SUCCESS) {
        return result;
    }

    result = process_package_removals(cache, tx.remove_names, tx.purge, tx.idempotent, requested_remove, skipped);
    if (result.code != APT_SUCCESS) {
        return result;
    }

    result = process_package_reinstalls(cache, tx.reinstall_names, requested_reinstall);
    if (result.code != APT_SUCCESS) {
        return result;
    }

    result = check_package_conflicts(cache, requested_install);
    if (result.code != APT_SUCCESS) {
        return result;
    }

    result = preprocess_installs(cache, requested_install);
    if (result.code != APT_SUCCESS) {
        return result;
    }

    result = finalize_dependency_resolution(cache, requested_install, requested_remove, tx.remove_depends);
    if (result.code != APT_SUCCESS) {
        return result;
    }

    const PlanChanges plan = collect_plan_changes(cache, requested_install);

    // install-only request with nothing to do is an error unless the transaction is idempotent
    if (!tx.idempotent && !requested_install.empty() && requested_remove.empty() && requested_reinstall.empty()) {
        std::set<std::string> will_change;
        for (const auto &pkg : plan.new_installed) will_change.insert(pkg);
        for (const auto &pkg : plan.upgraded) will_change.insert(pkg);

        std::vector<std::string> already_installed;
        for (const auto &req : requested_install) {
            if (will_change.find(req) == will_change.end()) {
                already_installed.push_back(req);
            }
        }

        if (!already_installed.empty() && already_installed.size() == requested_install.size()) {
            std::string msg = "Packages are already installed: ";
            for (size_t i = 0; i < already_installed.size(); ++i) {
                if (i > 0) msg += ", ";
                msg += already_installed[i];
            }
            return make_result(APT_ERROR_PACKAGES_ALREADY_INSTALLED, msg.c_str());
        }
    }

    populate_changes(changes, plan, skipped,
                     {requested_install.begin(), requested_install.end()},
                     {requested_remove.begin(), requested_remove.end()},
                     false);

    return make_result(APT_SUCCESS, nullptr);
}

AptResult apply_transaction_marks(const AptCache *cache, const AptTransaction &tx, AptPackageChanges *changes) {
    if (!cache || !changes) {
        return make_result(APT_ERROR_INVALID_PARAMETERS, APT_MSG_INVALID_PARAMS);
    }
    if (!cache->cache_file || !cache->dep_cache) {
        return make_result(APT_ERROR_CACHE_OPEN_FAILED, APT_MSG_CACHE_FILE_NOT_AVAILABLE);
    }

    const char *failure_prefix = tx.is_dist_upgrade ? "Dist upgrade simulation failed: "
                                 : tx.is_autoremove ? "Autoremove simulation failed: "
                                 : "Combined simulation failed: ";
    try {
        if (tx.is_dist_upgrade) {
            return mark_dist_upgrade(cache, changes);
        }
        if (tx.is_autoremove) {
            return mark_autoremove(cache, changes);
        }
        return mark_changes(cache, tx, changes);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string(failure_prefix) + e.what()).c_str());
    }
}

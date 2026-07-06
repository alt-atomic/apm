#include "progress.h"
#include "error.h"

#include <apt-pkg/acquire-item.h>
#include <apt-pkg/acquire-worker.h>

AptProgressCallback global_callback = nullptr;
uintptr_t global_user_data = 0;

// Static helpers that forward install-phase events to the global callback.
class SimpleProgressCallback {
public:
    static void InstallProgress(const char *package, const int current, const int total) {
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

// Reports aggregate and per-item download progress to the global callback.
bool ProgressStatus::Pulse(pkgAcquire *Owner) {
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

// Maps APT install callback types to our AptCallbackType and forwards them.
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

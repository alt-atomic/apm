#pragma once

#include "internal.h"
#include "error.h"

#include <apt-pkg/acquire.h>

#include <vector>
#include <string>

// Global progress callback set during transaction execution.
extern AptProgressCallback global_callback;
extern uintptr_t global_user_data;

// RAII guard that clears the global progress callback on destruction.
struct CallbackGuard {
    ~CallbackGuard() { global_callback = nullptr; global_user_data = 0; }
};

// Tracks download progress via the pkgAcquireStatus interface.
class ProgressStatus : public pkgAcquireStatus {
 public:
    bool MediaChange(std::string /*media*/, std::string /*drive*/) override { return false; }

    // Reports overall download progress to the global callback.
    bool Pulse(pkgAcquire *Owner) override;

    // Called when an item download starts.
    void Fetch(pkgAcquire::ItemDesc &Itm) override;

    // Called when an item download completes successfully.
    void Done(pkgAcquire::ItemDesc &Itm) override;

    // Called when an item download fails.
    void Fail(pkgAcquire::ItemDesc &Itm) override;

    // Called when the entire download phase finishes.
    void Stop() override;

 private:
    bool has_active_item_ = false;
    std::string active_name_;
};

// Data passed through the install-phase callback bridge.
struct CallbackBridge {
    uintptr_t user_data;
    AptCache *cache;
    std::vector<std::string> planned;
    size_t current_idx;
    std::string current_name;

    CallbackBridge() : user_data(0), cache(nullptr), current_idx(0) {
    }
};

// Creates a PackageManagerCallback that reports install progress via `bridgeData`.
PackageManagerCallback_t create_common_progress_callback(CallbackBridge *bridgeData);

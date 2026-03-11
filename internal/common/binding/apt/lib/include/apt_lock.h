#ifndef APT_LOCK_H
#define APT_LOCK_H
#ifdef __cplusplus
extern "C" {
#endif

#include "apt_common.h"

// Non-blocking APT lock status. Free with apt_free_lock_status().
typedef struct {
    bool is_locked;        // True if any APT lock file is held.
    bool can_acquire;      // True if locks can be acquired by this process.
    int lock_pid;          // PID of the process holding the lock, or 0.
    char *lock_holder;     // Name of the lock-holding process, or nullptr.
    char *lock_file_path;  // Path to the locked file, or nullptr.
    char *error_message;   // Human-readable error, or nullptr.
} AptLockStatus;

// Checks whether APT lock files are currently held (non-blocking).
AptLockStatus apt_check_lock_status();

// Frees heap-allocated strings within `status`.
void apt_free_lock_status(AptLockStatus *status);

#ifdef __cplusplus
}
#endif

#endif

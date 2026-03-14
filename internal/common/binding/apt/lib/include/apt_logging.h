#ifndef APT_LOGGING_H
#define APT_LOGGING_H
#ifdef __cplusplus
extern "C" {
#endif

#include "apt_common.h"

// Sets the log callback for APT diagnostic messages.
void apt_set_log_callback(AptLogCallback callback, uintptr_t user_data);

// Registers the Go progress callback bridge with `user_data`.
void apt_use_go_progress_callback(uintptr_t user_data);

// Registers the Go log callback bridge with `user_data`.
void apt_enable_go_log_callback(uintptr_t user_data);

// Enables (`enable` != 0) or disables C stdout/stderr capture to the log callback.
void apt_capture_stdio(int enable);

#ifdef __cplusplus
}
#endif

#endif

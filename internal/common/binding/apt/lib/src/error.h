#pragma once

#include "internal.h"

#include <string>

// Global error state set by the last failed APT operation.
extern AptErrorCode last_error;
extern std::string last_error_message;

// Global log callback and its opaque user data.
extern AptLogCallback g_log_callback;
extern uintptr_t g_log_user_data;

// Sends `msg` to the registered log callback, if any.
void emit_log(const std::string &msg);

// Returns true and updates globals if APT has pending errors.
bool check_apt_errors();

// Collects all pending APT error/warning messages into a single string.
std::string collect_pending_errors();

// Creates an AptResult with the given `code` and optional `message`.
AptResult make_result(AptErrorCode code, const char *message = nullptr);

// Returns the output stream used for APT log messages.
std::ostream &apt_log_stream();

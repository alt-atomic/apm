#include "error.h"

#include <apt-pkg/error.h>

#include <cstring>

AptErrorCode last_error = APT_SUCCESS;
std::string last_error_message;

// Pops all pending APT errors, detects lock errors, and updates globals.
bool check_apt_errors() {
    if (_error->PendingError()) {
        std::string error_msg;
        std::string all_errors;

        while (_error->PopMessage(error_msg)) {
            if (!all_errors.empty()) {
                all_errors += "; ";
            }
            all_errors += error_msg;
        }

        AptErrorCode error_code = APT_ERROR_UNKNOWN;
        if (all_errors.find("lock") != std::string::npos &&
            (all_errors.find("Resource temporarily unavailable") != std::string::npos ||
             all_errors.find("another process using it") != std::string::npos ||
             all_errors.find("EAGAIN") != std::string::npos ||
             all_errors.find("EACCES") != std::string::npos)) {
            error_code = APT_ERROR_LOCK_FAILED;
        }

        last_error = error_code;
        last_error_message = all_errors;
        if (!all_errors.empty()) {
            emit_log(std::string("APT Error: ") + all_errors);
        }
        return false;
    }
    return true;
}

// Drains all pending APT messages into a semicolon-separated string.
std::string collect_pending_errors() {
    std::string all_errors;
    if (_error->PendingError()) {
        std::string msg;
        while (_error->PopMessage(msg)) {
            if (!all_errors.empty()) all_errors += "; ";
            all_errors += msg;
        }
    }
    return all_errors;
}

// Builds an AptResult. On failure, collects pending errors if no message given.
AptResult make_result(const AptErrorCode code, const char *message) {
    AptResult r{};
    r.code = code;
    if (code == APT_SUCCESS) {
        r.message = nullptr;
        return r;
    }
    std::string msg;
    if (message && *message) {
        msg = message;
    } else {
        msg = collect_pending_errors();
    }
    if (msg.empty()) {
        if (!last_error_message.empty() && last_error == code) msg = last_error_message;
    }
    r.message = safe_strdup(msg);
    return r;
}

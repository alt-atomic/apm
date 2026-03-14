#include "error.h"

#include <apt-pkg/error.h>

#include <cstring>

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
    r.message = safe_strdup(msg);
    return r;
}

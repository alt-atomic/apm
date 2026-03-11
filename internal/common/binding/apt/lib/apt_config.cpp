#include "apt_config.h"
#include "error.h"

#include <sstream>

// Sets a single APT configuration key.
AptErrorCode apt_set_config(const char *key, const char *value) {
    if (!key || !value) return APT_ERROR_INIT_FAILED;

    try {
        _config->Set(key, value);
        return APT_SUCCESS;
    } catch (const std::exception &e) {
        emit_log(std::string("Exception: ") + e.what());
        return APT_ERROR_UNKNOWN;
    }
}

// Dumps the full APT config tree as a string. Caller owns the result.
char *apt_config_dump() {
    std::ostringstream ss;
    _config->Dump(ss);
    return strdup(ss.str().c_str());
}

// Deep-copies _config for later restore.
void *apt_config_snapshot() {
    try {
        return new Configuration(*_config);
    } catch (...) {
        return nullptr;
    }
}

// Replaces the global _config with a previously saved snapshot.
void apt_config_restore(void *snapshot) {
    if (!snapshot) return;
    delete _config;
    _config = static_cast<Configuration *>(snapshot);
}

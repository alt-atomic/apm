#include "apt_system.h"
#include "internal.h"
#include "error.h"

#include <apt-pkg/error.h>
#include <apt-pkg/init.h>

// Wraps pkgInitConfig and checks for pending errors.
AptResult apt_init_config() {
    try {
        if (!pkgInitConfig(*_config)) {
            return make_result(APT_ERROR_INIT_FAILED, APT_MSG_INIT_CONFIG_FAILED);
        }
        if (_error->PendingError()) {
            return make_result(APT_ERROR_UNKNOWN);
        }
        return make_result(APT_SUCCESS);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_INIT_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

// Wraps pkgInitSystem, allocates and returns an AptSystem handle.
AptResult apt_init_system(AptSystem **system) {
    if (!system) return make_result(APT_ERROR_INIT_FAILED, APT_MSG_INIT_SYSTEM_INVALID_PTR);

    try {
        if (!pkgInitSystem(*_config, _system)) {
            return make_result(APT_ERROR_INIT_FAILED, APT_MSG_INIT_SYSTEM_FAILED);
        }

        *system = new AptSystem();
        (*system)->system = _system;

        if (_error->PendingError()) {
            return make_result(APT_ERROR_UNKNOWN);
        }
        return make_result(APT_SUCCESS);
    } catch (const std::exception &e) {
        if (*system) {
            delete *system;
            *system = nullptr;
        }
        return make_result(APT_ERROR_INIT_FAILED, (std::string("Exception: ") + e.what()).c_str());
    }
}

// Unlocks the APT system and deletes the handle.
void apt_cleanup_system(const AptSystem *system) {
    if (system) {
        if (system->system) {
            system->system->UnLock(true);
        }
        delete system;
    }
}

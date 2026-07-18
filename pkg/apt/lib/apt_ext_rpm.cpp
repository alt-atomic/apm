#include "apt_ext_rpm.h"
#include "error.h"
#include "ext_rpm.h"

#include <set>
#include <sys/stat.h>

// Checks file extension and existence on disk.
bool is_rpm_file(const std::string &path) {
    if (path.length() <= 4 || path.substr(path.length() - 4) != ".rpm") {
        return false;
    }

    struct stat st{};
    if (stat(path.c_str(), &st) != 0 || !S_ISREG(st.st_mode)) {
        return false;
    }

    return true;
}

// Scans `install_names` for RPM files and adds new ones to APT::Arguments.
AptResult apt_preprocess_install_arguments(const char **install_names, const size_t install_count, bool *added_new) {
    if (added_new) *added_new = false;

    if (!install_names || install_count == 0) {
        return make_result(APT_SUCCESS, nullptr);
    }

    try {
        std::vector<std::string> existing_args = _config->FindVector("APT::Arguments");
        std::set existing_set(existing_args.begin(), existing_args.end());

        bool any_added = false;

        for (size_t i = 0; i < install_count; i++) {
            if (!install_names[i]) continue;

            if (std::string arg(install_names[i]); is_rpm_file(arg)) {
                if (existing_set.find(arg) == existing_set.end()) {
                    _config->Set("APT::Arguments::", arg);
                    existing_set.insert(arg);
                    any_added = true;
                }
            }
        }

        if (added_new) *added_new = any_added;
        return make_result(APT_SUCCESS, nullptr);
    } catch (const std::exception &e) {
        return make_result(APT_ERROR_UNKNOWN, (std::string("Exception in preprocess: ") + e.what()).c_str());
    }
}

// Removes all entries from APT::Arguments.
void apt_clear_install_arguments() {
    _config->Clear("APT::Arguments");
}

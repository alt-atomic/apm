#ifndef APT_EXT_RPM_H
#define APT_EXT_RPM_H
#ifdef __cplusplus
extern "C" {
#endif

#include "apt_common.h"

// Detects RPM files among `install_names` and adds them to APT::Arguments.
// Sets `*added_new` to true if any RPM files were added.
AptResult apt_preprocess_install_arguments(const char **install_names, size_t install_count, bool *added_new);

// Clears RPM file arguments previously added by apt_preprocess_install_arguments().
void apt_clear_install_arguments(void);

#ifdef __cplusplus
}
#endif

#endif

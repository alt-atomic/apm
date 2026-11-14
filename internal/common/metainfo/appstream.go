package metainfo

/*
#cgo pkg-config: appstream
#include <appstream.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// CurrentOSComponentID uses the same OS identity as appstreamcli sysinfo.
func CurrentOSComponentID() (string, error) {
	id := C.as_get_current_distro_component_id()
	if id == nil {
		return "", fmt.Errorf("libappstream could not determine the operating-system component ID")
	}
	defer C.g_free(C.gpointer(id))

	value := C.GoString(id)
	if value == "" {
		return "", fmt.Errorf("libappstream returned an empty operating-system component ID")
	}

	return value, nil
}

// OSComponentExists checks whether the AppStream pool contains an operating-system component with the given ID.
func OSComponentExists(id string) (bool, error) {
	pool := C.as_pool_new()
	defer C.g_object_unref(C.gpointer(pool))

	C.as_pool_set_flags(pool, C.AS_POOL_FLAG_LOAD_OS_CATALOG|C.AS_POOL_FLAG_LOAD_OS_METAINFO|C.AS_POOL_FLAG_IGNORE_CACHE_AGE)

	var loadError *C.GError
	if C.as_pool_load(pool, nil, &loadError) == C.FALSE {
		if loadError != nil {
			defer C.g_error_free(loadError)
			return false, fmt.Errorf("load AppStream components: %s", C.GoString(loadError.message))
		}
		return false, fmt.Errorf("load AppStream components failed")
	}

	componentID := C.CString(id)
	defer C.free(unsafe.Pointer(componentID))

	components := C.as_pool_get_components_by_id(pool, componentID)
	if components == nil {
		return false, nil
	}
	defer C.g_object_unref(C.gpointer(components))

	for i := C.guint(0); i < C.as_component_box_get_size(components); i++ {
		component := C.as_component_box_index_safe(components, i)
		if C.as_component_get_kind(component) == C.AS_COMPONENT_KIND_OPERATING_SYSTEM {
			return true, nil
		}
	}

	return false, nil
}

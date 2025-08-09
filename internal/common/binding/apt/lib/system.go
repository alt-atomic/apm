package lib

/*
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime"
)

// System represents APT system configuration
type System struct{ Ptr *C.AptSystem }

// NewSystem initializes APT system
func NewSystem() (*System, error) {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	if C.apt_init_config() != 0 {
		return nil, &AptError{Code: -1, Message: "Failed to initialize APT configuration"}
	}
	var ptr *C.AptSystem
	if res := C.apt_init_system(&ptr); res != 0 || ptr == nil {
		return nil, &AptError{Code: int(res), Message: "Failed to initialize APT system"}
	}
	s := &System{Ptr: ptr}
	runtime.SetFinalizer(s, (*System).Close)
	return s, nil
}

// Close frees the system resources
func (s *System) Close() {
	if s.Ptr != nil {
		C.apt_cleanup_system(s.Ptr)
		s.Ptr = nil
		runtime.SetFinalizer(s, nil)
	}
}

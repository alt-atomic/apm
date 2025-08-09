package lib

/*
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// Helper: safely convert C string to Go string
func cStringToGo(cstr *C.char) string {
	if cstr != nil {
		return C.GoString(cstr)
	}
	return ""
}

// Convert C AptPackageInfo to Go PackageInfo
func (p *PackageInfo) fromCStruct(c *C.AptPackageInfo) {
	p.Name = cStringToGo(c.name)
	p.Version = cStringToGo(c.version)
	p.Description = cStringToGo(c.description)
	p.ShortDescription = cStringToGo(c.short_description)
	p.Section = cStringToGo(c.section)
	p.Architecture = cStringToGo(c.architecture)
	p.Maintainer = cStringToGo(c.maintainer)
	p.Homepage = cStringToGo(c.homepage)
	p.Priority = cStringToGo(c.priority)
	p.MD5Hash = cStringToGo(c.md5_hash)
	p.Blake2bHash = cStringToGo(c.blake2b_hash)
	p.SourcePackage = cStringToGo(c.source_package)
	p.Changelog = cStringToGo(c.changelog)
	p.Filename = cStringToGo(c.filename)
	p.Depends = cStringToGo(c.depends)
	p.Provides = cStringToGo(c.provides)
	p.Conflicts = cStringToGo(c.conflicts)
	p.Obsoletes = cStringToGo(c.obsoletes)
	p.Recommends = cStringToGo(c.recommends)
	p.Suggests = cStringToGo(c.suggests)
	p.State = PackageState(c.state)
	p.AutoInstalled = bool(c.auto_installed)
	p.Essential = bool(c.essential)
	p.InstalledSize = uint64(c.installed_size)
	p.DownloadSize = uint64(c.download_size)
	p.PackageID = uint32(c.package_id)
}

// Cache represents package cache
type Cache struct {
	Ptr    *C.AptCache
	system *System
}

// OpenCache opens the package cache
func OpenCache(system *System, readOnly bool) (*Cache, error) {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	var ptr *C.AptCache
	withLock := C.bool(!readOnly)
	if res := C.apt_cache_open(system.Ptr, &ptr, withLock); res != 0 || ptr == nil {
		return nil, &AptError{Code: int(res), Message: "Failed to open package cache"}
	}
	c := &Cache{Ptr: ptr, system: system}
	runtime.SetFinalizer(c, (*Cache).Close)
	return c, nil
}

// Close frees the cache resources
func (c *Cache) Close() {
	if c.Ptr != nil {
		C.apt_cache_close(c.Ptr)
		c.Ptr = nil
		runtime.SetFinalizer(c, nil)
	}
}

func (c *Cache) Update() error {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	if res := C.apt_cache_update(c.Ptr); res != 0 {
		return &AptError{Code: int(res), Message: "Failed to update package cache"}
	}
	return nil
}

func (c *Cache) Refresh() error {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	if res := C.apt_cache_refresh(c.Ptr); res != 0 {
		return &AptError{Code: int(res), Message: "Failed to refresh package cache"}
	}
	return nil
}

func (c *Cache) BrokenCount() int {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	return int(C.apt_get_broken_count(c.Ptr))
}
func (c *Cache) HasBrokenPackages() bool {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	return bool(C.apt_has_broken_packages(c.Ptr))
}

func (c *Cache) MarkInstall(packageName string, autoInstall bool) error {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	cname := C.CString(packageName)
	defer C.free(unsafe.Pointer(cname))
	if res := C.apt_mark_install(c.Ptr, cname, C.bool(autoInstall)); res != 0 {
		return &AptError{Code: int(res), Message: fmt.Sprintf("Failed to mark package '%s' for installation", packageName)}
	}
	return nil
}

func (c *Cache) MarkRemove(packageName string, purge bool) error {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	cname := C.CString(packageName)
	defer C.free(unsafe.Pointer(cname))
	if res := C.apt_mark_remove(c.Ptr, cname, C.bool(purge)); res != 0 {
		return &AptError{Code: int(res), Message: fmt.Sprintf("Failed to mark package '%s' for removal", packageName)}
	}
	return nil
}

func (c *Cache) MarkKeep(packageName string) error {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	cname := C.CString(packageName)
	defer C.free(unsafe.Pointer(cname))
	if res := C.apt_mark_keep(c.Ptr, cname); res != 0 {
		return &AptError{Code: int(res), Message: fmt.Sprintf("Failed to mark package '%s' to keep", packageName)}
	}
	return nil
}

func (c *Cache) MarkAuto(packageName string, auto bool) error {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	cname := C.CString(packageName)
	defer C.free(unsafe.Pointer(cname))
	if res := C.apt_mark_auto(c.Ptr, cname, C.bool(auto)); res != 0 {
		return &AptError{Code: int(res), Message: fmt.Sprintf("Failed to mark package '%s' auto status", packageName)}
	}
	return nil
}

func (c *Cache) GetPackageInfo(packageName string) (*PackageInfo, error) {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	cname := C.CString(packageName)
	defer C.free(unsafe.Pointer(cname))
	var ci C.AptPackageInfo
	if res := C.apt_get_package_info(c.Ptr, cname, &ci); res != 0 {
		return nil, &AptError{Code: int(res), Message: fmt.Sprintf("Failed to get info for package '%s'", packageName)}
	}
	defer C.apt_free_package_info(&ci)
	info := &PackageInfo{}
	info.fromCStruct(&ci)
	return info, nil
}

func (c *Cache) SearchPackages(pattern string) ([]PackageInfo, error) {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	cPattern := C.CString(pattern)
	defer C.free(unsafe.Pointer(cPattern))
	var list C.AptPackageList
	if res := C.apt_search_packages(c.Ptr, cPattern, &list); res != 0 {
		return nil, &AptError{Code: int(res), Message: fmt.Sprintf("Failed to search packages for pattern '%s'", pattern)}
	}
	defer C.apt_free_package_list(&list)
	pkgs := make([]PackageInfo, int(list.count))
	if list.count > 0 {
		cp := unsafe.Slice(list.packages, int(list.count))
		for i, cpi := range cp {
			pkgs[i].fromCStruct(&cpi)
		}
	}
	return pkgs, nil
}

// Simulations
func (c *Cache) SimulateDistUpgrade() (*PackageChanges, error) {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	var cc C.AptPackageChanges
	res := C.apt_simulate_dist_upgrade(c.Ptr, &cc)
	defer C.apt_free_package_changes(&cc)
	if res != 0 {
		return nil, &AptError{Code: int(res), Message: "Failed to simulate dist-upgrade"}
	}
	changes := &PackageChanges{
		UpgradedCount:     int(cc.upgraded_count),
		NewInstalledCount: int(cc.new_installed_count),
		RemovedCount:      int(cc.removed_count),
		NotUpgradedCount:  int(cc.not_upgraded_count),
		DownloadSize:      uint64(cc.download_size),
		InstallSize:       uint64(cc.install_size),
	}
	if cc.upgraded_count > 0 {
		changes.UpgradedPackages = make([]string, int(cc.upgraded_count))
		for i := 0; i < int(cc.upgraded_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.upgraded_packages)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.UpgradedPackages[i] = C.GoString(*ptr)
		}
	}
	if cc.new_installed_count > 0 {
		changes.NewInstalledPackages = make([]string, int(cc.new_installed_count))
		for i := 0; i < int(cc.new_installed_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.new_installed_packages)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.NewInstalledPackages[i] = C.GoString(*ptr)
		}
	}
	if cc.removed_count > 0 {
		changes.RemovedPackages = make([]string, int(cc.removed_count))
		for i := 0; i < int(cc.removed_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.removed_packages)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.RemovedPackages[i] = C.GoString(*ptr)
		}
	}
	return changes, nil
}

func (c *Cache) SimulateInstall(packageNames []string) (*PackageChanges, error) {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	if len(packageNames) == 0 {
		return nil, &AptError{Code: APT_ERROR_INVALID_PARAMETERS, Message: "no packages specified"}
	}
	cNames := make([]*C.char, len(packageNames))
	for i, name := range packageNames {
		cNames[i] = C.CString(name)
		defer C.free(unsafe.Pointer(cNames[i]))
	}
	var cc C.AptPackageChanges
	res := C.apt_simulate_install(c.Ptr, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(packageNames)), &cc)
	defer C.apt_free_package_changes(&cc)
	if res != 0 {
		return nil, &AptError{Code: int(res), Message: fmt.Sprintf("Failed to simulate install for packages: %v", packageNames)}
	}
	changes := &PackageChanges{
		UpgradedCount:     int(cc.upgraded_count),
		NewInstalledCount: int(cc.new_installed_count),
		RemovedCount:      int(cc.removed_count),
		NotUpgradedCount:  int(cc.not_upgraded_count),
		DownloadSize:      uint64(cc.download_size),
		InstallSize:       uint64(cc.install_size),
	}
	if cc.extra_installed_count > 0 {
		changes.ExtraInstalled = make([]string, int(cc.extra_installed_count))
		for i := 0; i < int(cc.extra_installed_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.extra_installed)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.ExtraInstalled[i] = C.GoString(*ptr)
		}
	}
	if cc.upgraded_count > 0 {
		changes.UpgradedPackages = make([]string, int(cc.upgraded_count))
		for i := 0; i < int(cc.upgraded_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.upgraded_packages)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.UpgradedPackages[i] = C.GoString(*ptr)
		}
	}
	if cc.new_installed_count > 0 {
		changes.NewInstalledPackages = make([]string, int(cc.new_installed_count))
		for i := 0; i < int(cc.new_installed_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.new_installed_packages)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.NewInstalledPackages[i] = C.GoString(*ptr)
		}
	}
	if cc.removed_count > 0 {
		changes.RemovedPackages = make([]string, int(cc.removed_count))
		for i := 0; i < int(cc.removed_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.removed_packages)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.RemovedPackages[i] = C.GoString(*ptr)
		}
	}
	return changes, nil
}

func (c *Cache) SimulateRemove(packageNames []string) (*PackageChanges, error) {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	if len(packageNames) == 0 {
		return nil, &AptError{Code: APT_ERROR_INVALID_PARAMETERS, Message: "no packages specified"}
	}
	cNames := make([]*C.char, len(packageNames))
	for i, name := range packageNames {
		cNames[i] = C.CString(name)
		defer C.free(unsafe.Pointer(cNames[i]))
	}
	var cc C.AptPackageChanges
	res := C.apt_simulate_remove(c.Ptr, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(packageNames)), &cc)
	defer C.apt_free_package_changes(&cc)
	if res != 0 {
		return nil, &AptError{Code: int(res), Message: fmt.Sprintf("Failed to simulate remove for packages: %v", packageNames)}
	}
	changes := &PackageChanges{
		UpgradedCount:     int(cc.upgraded_count),
		NewInstalledCount: int(cc.new_installed_count),
		RemovedCount:      int(cc.removed_count),
		NotUpgradedCount:  int(cc.not_upgraded_count),
		DownloadSize:      uint64(cc.download_size),
		InstallSize:       uint64(cc.install_size),
	}
	if cc.extra_installed_count > 0 {
		changes.ExtraInstalled = make([]string, int(cc.extra_installed_count))
		for i := 0; i < int(cc.extra_installed_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.extra_installed)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.ExtraInstalled[i] = C.GoString(*ptr)
		}
	}
	if cc.upgraded_count > 0 {
		changes.UpgradedPackages = make([]string, int(cc.upgraded_count))
		for i := 0; i < int(cc.upgraded_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.upgraded_packages)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.UpgradedPackages[i] = C.GoString(*ptr)
		}
	}
	if cc.new_installed_count > 0 {
		changes.NewInstalledPackages = make([]string, int(cc.new_installed_count))
		for i := 0; i < int(cc.new_installed_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.new_installed_packages)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.NewInstalledPackages[i] = C.GoString(*ptr)
		}
	}
	if cc.removed_count > 0 {
		changes.RemovedPackages = make([]string, int(cc.removed_count))
		for i := 0; i < int(cc.removed_count); i++ {
			ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cc.removed_packages)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil))))
			changes.RemovedPackages[i] = C.GoString(*ptr)
		}
	}
	return changes, nil
}

// Sweep-тест: каждая DTO, которую отдаёт API v2, обязана сериализоваться.
// Новый ответ добавляется в список — неподдерживаемый тип поля валит CI.
package wire_test

import (
	"reflect"
	"testing"

	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/filter"
	"altlinux.space/alt-atomic/apm/internal/common/imagesvc"
	"altlinux.space/alt-atomic/apm/internal/common/sandbox"
	"altlinux.space/alt-atomic/apm/internal/common/swcat"
	"altlinux.space/alt-atomic/apm/internal/domain/distrobox"
	"altlinux.space/alt-atomic/apm/internal/domain/kernel"
	"altlinux.space/alt-atomic/apm/internal/domain/repository"
	"altlinux.space/alt-atomic/apm/internal/domain/system"
	"altlinux.space/alt-atomic/apm/internal/domain/system/appstream"

	"github.com/godbus/dbus/v5"
)

// v2DTOs все типы ответов, уходящие через StructDict.
var v2DTOs = []any{
	// Packages
	system.CheckResponse{},
	system.InstallRemoveResponse{},
	system.UpdateResponse{},
	system.UpgradeResponse{},
	system.InfoResponse{},
	system.MultiInfoResponse{},
	system.ListResponse{},
	system.SearchResponse{},
	system.SectionsResponse{},
	system.AptConfigResponse{},
	_package.Package{},
	filter.FieldInfo{},
	// Image
	system.ImageStatusResponse{},
	system.ImageUpdateResponse{},
	system.ImageApplyResponse{},
	system.ImageSwitchResponse{},
	system.ImageHistoryResponse{},
	system.ImageFixNssResponse{},
	system.ImageSyncGroupsResponse{},
	imagesvc.ImageHistory{},
	// Applications
	appstream.UpdateResponse{},
	appstream.InfoResponse{},
	appstream.ListResponse{},
	appstream.CategoriesResponse{},
	swcat.Component{},
	swcat.DBAppStream{},
	// Kernel
	kernel.ListKernelsResponse{},
	kernel.GetCurrentKernelResponse{},
	kernel.InstallUpdateKernelResponse{},
	kernel.CleanOldKernelsResponse{},
	kernel.ListKernelModulesResponse{},
	kernel.InstallKernelModulesResponse{},
	kernel.RemoveKernelModulesResponse{},
	// Repo
	repository.RepoListResponse{},
	repository.RepoAddRemoveResponse{},
	repository.RepoSetResponse{},
	repository.RepoSimulateResponse{},
	repository.BranchesResponse{},
	repository.TaskPackagesResponse{},
	repository.TestTaskResponse{},
	// Distrobox
	distrobox.UpdateResponse{},
	distrobox.InfoResponse{},
	distrobox.SearchResponse{},
	distrobox.ListResponse{},
	distrobox.InstallResponse{},
	distrobox.RemoveResponse{},
	distrobox.ContainerListResponse{},
	distrobox.ContainerAddResponse{},
	distrobox.ContainerRemoveResponse{},
}

func TestStructDictAllDTOs(t *testing.T) {
	for _, dto := range v2DTOs {
		rt := reflect.TypeOf(dto)
		t.Run(rt.String(), func(t *testing.T) {
			filled := reflect.New(rt)
			fillValue(filled.Elem(), 3)

			d, err := wire.StructDict(filled.Interface())
			if err != nil {
				t.Fatalf("StructDict: %v", err)
			}
			if len(d) == 0 {
				t.Fatal("empty dict for filled DTO")
			}
			if sig := dbus.MakeVariant(d).Signature().String(); sig != "a{sv}" {
				t.Fatalf("signature = %s", sig)
			}
		})
	}
}

// fillValue заполняет значение непустыми данными, чтобы пройти все ветки сериализации.
func fillValue(rv reflect.Value, depth int) {
	if depth == 0 || !rv.CanSet() {
		return
	}
	switch rv.Kind() {
	case reflect.String:
		rv.SetString("x")
	case reflect.Bool:
		rv.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		rv.SetInt(7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		rv.SetUint(7)
	case reflect.Float32, reflect.Float64:
		rv.SetFloat(0.5)
	case reflect.Pointer:
		rv.Set(reflect.New(rv.Type().Elem()))
		fillValue(rv.Elem(), depth-1)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Interface {
			return
		}
		item := reflect.New(rv.Type().Elem()).Elem()
		fillValue(item, depth-1)
		rv.Set(reflect.Append(rv, item))
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return
		}
		rv.Set(reflect.MakeMap(rv.Type()))
		value := reflect.New(rv.Type().Elem()).Elem()
		if value.Kind() == reflect.Interface {
			value.Set(reflect.ValueOf("x"))
		} else {
			fillValue(value, depth-1)
		}
		rv.SetMapIndex(reflect.ValueOf("k"), value)
	case reflect.Struct:
		for i := 0; i < rv.NumField(); i++ {
			fillValue(rv.Field(i), depth-1)
		}
	default:
	}
}

// TestStructDictFilterFields прогоняет реальные конфиги фильтров: их Extra
// содержит карты с нестроковыми ключами, которые ломали FilterFields на шине.
func TestStructDictFilterFields(t *testing.T) {
	configs := map[string]*filter.Config{
		"packages":     _package.SystemFilterConfig,
		"applications": swcat.FilterConfig,
		"distrobox":    sandbox.DistroFilterConfig,
	}
	for name, cfg := range configs {
		t.Run(name, func(t *testing.T) {
			rows, err := wire.StructDicts(cfg.FieldsInfo())
			if err != nil {
				t.Fatalf("StructDicts: %v", err)
			}
			if len(rows) == 0 {
				t.Fatal("no filter fields")
			}
			if sig := dbus.MakeVariant(rows).Signature().String(); sig != "aa{sv}" {
				t.Fatalf("signature = %s", sig)
			}
		})
	}
}

// Тесты StructDict на реальных DTO приложения; внешний тест-пакет,
// чтобы импортировать домен без цикла.
package wire_test

import (
	"testing"

	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/imagesvc"
	"altlinux.space/alt-atomic/apm/internal/common/swcat"
	"altlinux.space/alt-atomic/apm/internal/domain/system"
	aptlib "altlinux.space/alt-atomic/apm/pkg/apt/lib"
	pkgbuild "altlinux.space/alt-atomic/apm/pkg/build"

	"github.com/godbus/dbus/v5"
)

// TestStructDictRealPackage сверяет проводную форму пакета с JSON-выводом apm s info apt --full.
func TestStructDictRealPackage(t *testing.T) {
	pkg := _package.Package{
		Name:             "apt",
		Architecture:     "x86_64",
		Section:          "System/Configuration/Packaging",
		InstalledSize:    1481108,
		Maintainer:       "Ivan Zakharyaschev <imz@altlinux.org>",
		Version:          "0.5.15lorg2",
		VersionRaw:       "0.5.15lorg2-alt103:sisyphus+429241.100.1.1@1786527673",
		VersionInstalled: "0.5.15lorg2",
		Depends:          []string{"/etc/apt/pkgpriorities", "libapt", "rpm", "zstd"},
		Aliases:          nil,
		Provides:         []string{"/usr/bin/apt-cache", "/usr/bin/apt-get", "apt"},
		Size:             435209,
		Filename:         "RPMS.classic/apt-0.5.15lorg2-alt103.x86_64.rpm",
		Summary:          "Debian APT - Усовершенствованное средство управления пакетами с поддержкой RPM",
		Description:      "Перенесенные из Debian средства управления пакетами APT",
		Changelog:        "* Ср авг 12 2026 Ivan Zakharyaschev",
		Installed:        true,
		TypePackage:      0,
		HasAppStream:     true,
		Files:            []string{"/usr/bin/apt-cache", "/usr/bin/apt-get"},
	}

	d, err := wire.StructDict(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if sig := dbus.MakeVariant(d).Signature().String(); sig != "a{sv}" {
		t.Fatalf("signature = %s", sig)
	}

	wantStrings := map[string]string{
		"name":             "apt",
		"architecture":     "x86_64",
		"section":          "System/Configuration/Packaging",
		"maintainer":       "Ivan Zakharyaschev <imz@altlinux.org>",
		"version":          "0.5.15lorg2",
		"versionRaw":       "0.5.15lorg2-alt103:sisyphus+429241.100.1.1@1786527673",
		"versionInstalled": "0.5.15lorg2",
		"filename":         "RPMS.classic/apt-0.5.15lorg2-alt103.x86_64.rpm",
		"summary":          "Debian APT - Усовершенствованное средство управления пакетами с поддержкой RPM",
		"description":      "Перенесенные из Debian средства управления пакетами APT",
		"lastChangelog":    "* Ср авг 12 2026 Ivan Zakharyaschev",
	}
	for key, want := range wantStrings {
		got, ok := d[key]
		if !ok {
			t.Errorf("key %q missing", key)
			continue
		}
		if got.Value().(string) != want {
			t.Errorf("%s = %q, want %q", key, got.Value(), want)
		}
	}

	if got := d["installedSize"].Value().(int64); got != 1481108 {
		t.Errorf("installedSize = %v", got)
	}
	if got := d["size"].Value().(int64); got != 435209 {
		t.Errorf("size = %v", got)
	}
	if got := d["typePackage"].Value().(int64); got != 0 {
		t.Errorf("typePackage = %v", got)
	}
	if got := d["installed"].Value().(bool); !got {
		t.Error("installed = false")
	}
	if got := d["depends"].Value().([]string); len(got) != 4 || got[0] != "/etc/apt/pkgpriorities" {
		t.Errorf("depends = %v", got)
	}
	if got := d["provides"].Value().([]string); len(got) != 3 {
		t.Errorf("provides = %v", got)
	}
	if got := d["files"].Value().([]string); len(got) != 2 {
		t.Errorf("files = %v", got)
	}
	// aliases: null в JSON — на шине пустой as (нет omitempty).
	if got, ok := d["aliases"]; !ok || len(got.Value().([]string)) != 0 {
		t.Errorf("aliases = %v", got)
	}
	// json:"-" и пустой omitempty не публикуются.
	if _, ok := d["HasAppStream"]; ok {
		t.Error("HasAppStream must be skipped")
	}
	if _, ok := d["appStream"]; ok {
		t.Error("empty appStream must be omitted")
	}
}

// TestStructDictCheckResponse проверяет вложенный PackageChanges.
func TestStructDictCheckResponse(t *testing.T) {
	resp := system.CheckResponse{
		Message: "Inspection information",
		Info: aptlib.PackageChanges{
			NewInstalledCount:    2,
			NewInstalledPackages: []string{"vim", "vim-common"},
			DownloadSize:         12345,
			InstallSize:          -100,
			EssentialPackages:    []aptlib.EssentialPackage{{Name: "glibc", Reason: "essential"}},
		},
	}

	d, err := wire.StructDict(resp)
	if err != nil {
		t.Fatal(err)
	}

	info := d["info"].Value().(wire.Dict)
	if got := info["newInstalledCount"].Value().(int64); got != 2 {
		t.Errorf("newInstalledCount = %v", got)
	}
	if got := info["downloadSize"].Value().(uint64); got != 12345 {
		t.Errorf("downloadSize = %v", got)
	}
	if got := info["installSize"].Value().(int64); got != -100 {
		t.Errorf("installSize = %v (must keep sign)", got)
	}
	essential := info["essentialPackages"].Value().([]wire.Dict)
	if len(essential) != 1 || essential[0]["reason"].Value().(string) != "essential" {
		t.Errorf("essentialPackages = %v", essential)
	}
}

// TestStructDictComponent проверяет кастомные json.Marshaler swcat: LocalizedMap/KeywordList/URLMap.
func TestStructDictComponent(t *testing.T) {
	comp := swcat.Component{
		Type: "desktop-application",
		ID:   "org.vim.Vim",
		Name: swcat.LocalizedMap{
			{Lang: "", Value: "Vim"},
			{Lang: "ru", Value: "Вим"},
		},
		Keywords: swcat.KeywordList{{Value: "editor"}, {Value: "text"}},
		Urls:     swcat.URLMap{{Type: "homepage", Value: "https://vim.org"}},
	}

	d, err := wire.StructDict(comp)
	if err != nil {
		t.Fatal(err)
	}

	name := d["name"].Value().(wire.Dict)
	if name["C"].Value().(string) != "Vim" || name["ru"].Value().(string) != "Вим" {
		t.Errorf("name = %v (custom MarshalJSON form expected)", name)
	}
	keywords := d["keywords"].Value().([]string)
	if len(keywords) != 2 || keywords[0] != "editor" {
		t.Errorf("keywords = %v", keywords)
	}
	urls := d["urls"].Value().(wire.Dict)
	if urls["homepage"].Value().(string) != "https://vim.org" {
		t.Errorf("urls = %v", urls)
	}
	if _, ok := d["XMLName"]; ok {
		t.Error("xml.Name must be skipped via json tag")
	}
}

// TestStructDictImageStatus проверяет статус образа с конфигом и интерфейсным Body модуля.
func TestStructDictImageStatus(t *testing.T) {
	cfg, err := pkgbuild.ParseYamlConfigData([]byte(`
image: ghcr.io/alt-gnome/alt-atomic:latest
modules:
  - type: mkdir
    name: dirs
    body:
      targets:
        - /var/lib/test
      perm: "0755"
`))
	if err != nil {
		t.Fatal(err)
	}

	resp := system.ImageStatusResponse{
		Message: "Image status",
		BootedImage: system.ImageStatus{
			Status: "Cloud image without changes",
			Image: imagesvc.HostImage{
				Spec: struct {
					Image imagesvc.ImageInfo `json:"image"`
				}{Image: imagesvc.ImageInfo{Image: "ghcr.io/alt-gnome/alt-atomic:latest", Transport: "registry"}},
			},
			Config: cfg,
		},
	}

	d, err := wire.StructDict(resp)
	if err != nil {
		t.Fatal(err)
	}

	booted := d["bootedImage"].Value().(wire.Dict)
	image := booted["image"].Value().(wire.Dict)
	spec := image["spec"].Value().(wire.Dict)
	specImage := spec["image"].Value().(wire.Dict)
	if specImage["transport"].Value().(string) != "registry" {
		t.Errorf("spec.image = %v", specImage)
	}

	config := booted["config"].Value().(wire.Dict)
	modules := config["modules"].Value().([]wire.Dict)
	if len(modules) != 1 || modules[0]["type"].Value().(string) != "mkdir" {
		t.Fatalf("modules = %v", modules)
	}
	body := modules[0]["body"].Value().(wire.Dict)
	if targets := body["targets"].Value().([]string); len(targets) != 1 || targets[0] != "/var/lib/test" {
		t.Errorf("body.targets = %v (interface Body must unwrap)", body)
	}
}

//go:build e2e

// Эти тесты запускаются через scripts/test-dbus-e2e.sh: установка реального
// пакета должна завершиться до Repo.Clean, а сервис между группами перезапускается.
package dbus_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	apmpkg "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/domain/system"
	aptlib "altlinux.space/alt-atomic/apm/pkg/apt/lib"
	"altlinux.space/alt-atomic/apm/tests/e2e/dbustest"
)

const (
	packagesIface = "org.altlinux.APM2.Packages"
	jobsIface     = "org.altlinux.APM2.Jobs"
	testPackage   = "hello"
)

// packagesV2Contract намеренно дублирует introspection как независимый golden
// публичного API, который проверяется через реально запущенную D-Bus-службу.
var packagesV2Contract = dbustest.InterfaceContract{
	"Install":      "in packages:as, in options_json:s, out job:s",
	"Remove":       "in packages:as, in options_json:s, out job:s",
	"Reinstall":    "in packages:as, out job:s",
	"Upgrade":      "in options_json:s, out job:s",
	"Update":       "in options_json:s, out job:s",
	"CheckInstall": "in packages:as, out job:s",
	"CheckRemove":  "in packages:as, in options_json:s, out job:s",
	"CheckUpgrade": "out job:s",
	"List":         "in request_json:s, out json:s",
	"Info":         "in name:s, out json:s",
	"MultiInfo":    "in names:as, out json:s",
	"Search":       "in text:s, in installed:b, out json:s",
	"Sections":     "out sections:as",
	"FilterFields": "out json:s",
	"AptConfig":    "out json:s",
	"SetAptConfig": "in options_json:s",
}

func TestPackagesV2PolkitDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Packages v2 polkit denial test must run as an unprivileged user")
	}

	client := dbustest.NewSystemClient(t, serviceName, objectPath)
	err := client.Request(packagesIface, "CheckInstall").Args([]string{testPackage}).Call().Err
	dbustest.AssertError(t, err, "org.freedesktop.DBus.Error.AccessDenied", "org.altlinux.APM2.packages.manage")
}

func TestPackagesV2InstallRemove(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Packages v2 transaction test must run as root")
	}

	client := dbustest.NewSystemClient(t, serviceName, objectPath, dbustest.WithTimeout(5*time.Minute))
	client.AssertContract(t, packagesIface, packagesV2Contract)
	assertRPMInstalled(t, testPackage, false)

	if !t.Run("ListAndInfo", func(t *testing.T) {
		info := callPackageInfo(t, client, testPackage, false)
		response := callPackageList(t, client, testPackage)
		if response.TotalCount < len(response.Packages) {
			t.Fatalf("Packages.List total = %d, smaller than returned page of %d packages", response.TotalCount, len(response.Packages))
		}
		if response.TotalCount == 0 || len(response.Packages) == 0 {
			t.Fatalf("Packages.List(%q) returned total=%d packages=%d", testPackage, response.TotalCount, len(response.Packages))
		}

		listed := findPackage(response.Packages, testPackage)
		if listed == nil {
			t.Fatalf("Packages.List(%q) response has no exact package match: %#v", testPackage, response.Packages)
		}
		assertShortPackage(t, *listed, info)
	}) {
		t.FailNow()
	}

	if !t.Run("CheckInstall", func(t *testing.T) {
		response := callPackagesCheck(t, client, "CheckInstall", []string{testPackage})
		assertPackageChange(t, response.Info, response.Message, true, testPackage)
		assertRPMInstalled(t, testPackage, false)
	}) {
		t.FailNow()
	}

	if !t.Run("Install", func(t *testing.T) {
		options := dbustest.EncodeJSON(t, map[string]bool{"noUpdate": true})
		job := client.Request(packagesIface, "Install").
			Args([]string{testPackage}, options).
			WaitJob(t, jobsIface)
		assertSuccessfulPackageJob(t, job, true, testPackage)
		assertJobStillAvailable(t, client, job)
		assertRPMInstalled(t, testPackage, true)
		callPackageInfo(t, client, testPackage, true)
	}) {
		t.FailNow()
	}

	if !t.Run("CheckRemove", func(t *testing.T) {
		response := callPackagesCheck(t, client, "CheckRemove", []string{testPackage}, "{}")
		assertPackageChange(t, response.Info, response.Message, false, testPackage)
		assertRPMInstalled(t, testPackage, true)
	}) {
		t.FailNow()
	}

	if !t.Run("Remove", func(t *testing.T) {
		job := client.Request(packagesIface, "Remove").
			Args([]string{testPackage}, "{}").
			WaitJob(t, jobsIface)
		assertSuccessfulPackageJob(t, job, false, testPackage)
		assertRPMInstalled(t, testPackage, false)
		callPackageInfo(t, client, testPackage, false)
	}) {
		t.FailNow()
	}
}

// callPackagesCheck прогоняет симуляцию.
func callPackagesCheck(t *testing.T, client *dbustest.Client, method string, args ...any) system.CheckResponse {
	t.Helper()

	job := client.Request(packagesIface, method).Args(args...).WaitJob(t, jobsIface)
	if job.Status != "ok" {
		t.Fatalf("%s job %s status = %q, want ok; error=%q message=%q", method, job.ID, job.Status, job.ErrorType, job.Message)
	}

	response := dbustest.DecodeJob[system.CheckResponse](t, job)
	assertResponseMessage(t, response.Message, method)
	return response
}

func assertSuccessfulPackageJob(t *testing.T, job dbustest.JobResult, install bool, packageName string) {
	t.Helper()

	if job.Status != "ok" {
		t.Fatalf("job %s status = %q, want ok; error=%q message=%q json=%s", job.ID, job.Status, job.ErrorType, job.Message, job.JSON)
	}
	if job.Message != "" || job.ErrorType != "" {
		t.Fatalf("successful job %s reported error %q/%q", job.ID, job.ErrorType, job.Message)
	}
	response := dbustest.DecodeJob[system.InstallRemoveResponse](t, job)
	assertPackageChange(t, response.Info, response.Message, install, packageName)
}

// assertJobStillAvailable проверяет, что завершённая задача остаётся в реестре:
// клиент мог узнать id уже после JobFinished и обязан забрать результат.
func assertJobStillAvailable(t *testing.T, client *dbustest.Client, job dbustest.JobResult) {
	t.Helper()

	state := dbustest.CallJSON[struct {
		ID     string          `json:"id"`
		State  string          `json:"state"`
		Result json.RawMessage `json:"result"`
	}](t, client.Request(jobsIface, "Get").Args(job.ID))

	if state.ID != job.ID || state.State != "ok" {
		t.Fatalf("Jobs.Get(%s) = %+v, want finished job", job.ID, state)
	}
	if string(state.Result) != job.JSON {
		t.Fatalf("Jobs.Get(%s) result = %s, want %s", job.ID, state.Result, job.JSON)
	}
}

func assertPackageChange(t *testing.T, changes aptlib.PackageChanges, message string, install bool, packageName string) {
	t.Helper()
	assertResponseMessage(t, message, "package operation")

	packages := changes.RemovedPackages
	count := changes.RemovedCount
	if install {
		packages = changes.NewInstalledPackages
		count = changes.NewInstalledCount
	}
	if !slices.Contains(packages, packageName) {
		t.Fatalf("changed packages = %#v, want package %q", packages, packageName)
	}
	if count < 1 {
		t.Fatalf("changed package count = %d, want at least 1", count)
	}
}

func assertResponseMessage(t *testing.T, message, source string) {
	t.Helper()
	if strings.TrimSpace(message) == "" {
		t.Fatalf("%s returned an empty response message", source)
	}
}

func callPackageList(t *testing.T, client *dbustest.Client, packageName string) system.ListResponse {
	t.Helper()

	request := dbustest.EncodeJSON(t, map[string]any{
		"sort":  "name",
		"order": "asc",
		"limit": 10,
		"filters": []map[string]string{
			{"field": "name", "op": "eq", "value": packageName},
		},
	})
	response := dbustest.CallJSON[system.ListResponse](t, client.Request(packagesIface, "List").Args(request))
	assertResponseMessage(t, response.Message, "Packages.List")
	return response
}

func callPackageInfo(t *testing.T, client *dbustest.Client, packageName string, wantInstalled bool) apmpkg.Package {
	t.Helper()

	response := dbustest.CallJSON[system.InfoResponse](t, client.Request(packagesIface, "Info").Args(packageName))
	assertResponseMessage(t, response.Message, "Packages.Info")
	info := response.PackageInfo
	if info.Name != packageName {
		t.Fatalf("Packages.Info(%q) name = %q", packageName, info.Name)
	}
	if info.Installed != wantInstalled {
		t.Fatalf("Packages.Info(%q) installed = %t, want %t", packageName, info.Installed, wantInstalled)
	}

	fields := map[string]string{
		"architecture":  info.Architecture,
		"section":       info.Section,
		"maintainer":    info.Maintainer,
		"version":       info.Version,
		"versionRaw":    info.VersionRaw,
		"filename":      info.Filename,
		"summary":       info.Summary,
		"description":   info.Description,
		"lastChangelog": info.Changelog,
	}
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			t.Errorf("Packages.Info(%q) %s is empty", packageName, name)
		}
	}
	if wantInstalled && strings.TrimSpace(info.VersionInstalled) == "" {
		t.Errorf("Packages.Info(%q) versionInstalled is empty for an installed package", packageName)
	}
	if !wantInstalled && info.VersionInstalled != "" {
		t.Errorf("Packages.Info(%q) versionInstalled = %q for an uninstalled package", packageName, info.VersionInstalled)
	}
	if info.InstalledSize <= 0 || info.Size <= 0 {
		t.Errorf("Packages.Info(%q) sizes = installed:%d archive:%d, want positive values", packageName, info.InstalledSize, info.Size)
	}
	if info.TypePackage != 0 {
		t.Errorf("Packages.Info(%q) typePackage = %d, want system package type 0", packageName, info.TypePackage)
	}
	return info
}

func assertShortPackage(t *testing.T, listed, info apmpkg.Package) {
	t.Helper()

	if listed.Name != info.Name || listed.Summary != info.Summary || listed.Version != info.Version || listed.Maintainer != info.Maintainer {
		t.Errorf("Packages.List package differs from Packages.Info: list=%#v info=%#v", listed, info)
	}
	if listed.Installed != info.Installed {
		t.Errorf("Packages.List installed = %t, differs from Packages.Info", listed.Installed)
	}
}

func findPackage(packages []apmpkg.Package, name string) *apmpkg.Package {
	for i := range packages {
		if packages[i].Name == name {
			return &packages[i]
		}
	}
	return nil
}

func assertRPMInstalled(t *testing.T, packageName string, want bool) {
	t.Helper()

	err := exec.Command("rpm", "--quiet", "-q", packageName).Run()
	if want && err != nil {
		t.Fatalf("rpm reports %q is not installed: %v", packageName, err)
	}
	if !want && err == nil {
		t.Fatalf("rpm reports %q is unexpectedly installed", packageName)
	}
	if !want {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatalf("query rpm state of %q: %v", packageName, err)
		}
	}
}

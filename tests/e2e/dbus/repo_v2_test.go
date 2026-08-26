//go:build e2e

// Эти тесты запускаются через scripts/test-dbus-e2e.sh: порядок фикстур,
// остановка сервиса и отдельные процессы для методов имеют значение.
package dbus_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/domain/repository"
	"altlinux.space/alt-atomic/apm/pkg/aptrepo"
	"altlinux.space/alt-atomic/apm/tests/e2e/dbustest"

	"github.com/godbus/dbus/v5"
)

const (
	serviceName = "org.altlinux.APM"
	objectPath  = dbus.ObjectPath("/org/altlinux/APM2")
	repoIface   = "org.altlinux.APM2.Repo"

	activeRepoURL    = "https://example.invalid/apm-e2e/active"
	inactiveRepoURL  = "https://example.invalid/apm-e2e/inactive"
	mutatingRepoURL  = "https://example.invalid/apm-e2e/mutating"
	temporaryRepoURL = "https://example.invalid/apm-e2e/temporary"
)

// repoV2Contract намеренно дублирует introspection как независимый golden
// публичного API, который проверяется через реально запущенную D-Bus-службу.
var repoV2Contract = dbustest.InterfaceContract{
	"List":           "in all:b, out json:s",
	"Branches":       "out branches:as",
	"TaskPackages":   "in task:s, out job:s",
	"TestTask":       "in task:s, out job:s",
	"Add":            "in sources:as, in date:s, out json:s",
	"Remove":         "in sources:as, in date:s, out json:s",
	"SetBranch":      "in branch:s, in date:s, out json:s",
	"Clean":          "out json:s",
	"CheckAdd":       "in sources:as, in date:s, out json:s",
	"CheckRemove":    "in sources:as, in date:s, out json:s",
	"CheckSetBranch": "in branch:s, in date:s, out json:s",
	"CheckClean":     "out json:s",
}

func TestRepoV2ReadOnly(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("read-only activation test must run as root")
	}

	client := dbustest.NewSystemClient(t, serviceName, objectPath)
	expectActivation := os.Getenv("APM_E2E_EXPECT_ACTIVATION") == "1"

	if expectActivation && client.NameHasOwner(t) {
		t.Fatalf("%s already has an owner; D-Bus activation was not tested", serviceName)
	}

	t.Run("activation and introspection contract", func(t *testing.T) {
		client.AssertContract(t, repoIface, repoV2Contract)

		if expectActivation && !client.NameHasOwner(t) {
			t.Fatalf("%s has no owner after introspection call", serviceName)
		}
	})

	t.Run("Branches", func(t *testing.T) {
		var branches []string
		client.Request(repoIface, "Branches").Store(t, &branches)
		if len(branches) == 0 {
			t.Fatal("Branches returned an empty list")
		}

		seen := make(map[string]struct{}, len(branches))
		for _, branch := range branches {
			if strings.TrimSpace(branch) == "" {
				t.Fatal("Branches returned an empty branch name")
			}
			if _, exists := seen[branch]; exists {
				t.Fatalf("Branches returned duplicate %q", branch)
			}
			seen[branch] = struct{}{}
		}
	})

	t.Run("List", func(t *testing.T) {
		active := callRepoList(t, client, false)
		assertRepository(t, active, activeRepoURL, true, "classic")
		assertRepositoryMissing(t, active, inactiveRepoURL)

		all := callRepoList(t, client, true)
		assertRepository(t, all, activeRepoURL, true, "classic")
		assertRepository(t, all, inactiveRepoURL, false, "classic")
	})
}

func TestRepoV2PolkitDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("polkit denial test must run as an unprivileged user")
	}

	client := dbustest.NewSystemClient(t, serviceName, objectPath)
	err := client.Request(repoIface, "CheckClean").Call().Err
	dbustest.AssertError(t, err, "org.freedesktop.DBus.Error.AccessDenied", "org.altlinux.APM2.repo.manage")

	// фоновая задача тоже под polkit: иначе любой мог бы плодить походы в сеть
	err = client.Request(repoIface, "TaskPackages").Args("400000").Call().Err
	dbustest.AssertError(t, err, "org.freedesktop.DBus.Error.AccessDenied", "org.altlinux.APM2.repo.manage")
}

func TestRepoV2Privileged(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("privileged Repo v2 test must run as root")
	}

	client := dbustest.NewSystemClient(t, serviceName, objectPath)
	mutatingSource := []string{mutatingRepoURL, "x86_64", "classic"}

	t.Run("CheckAdd and Add", func(t *testing.T) {
		assertRepoResult(t, client, "CheckAdd", mutatingSource, "")
		assertRepositoryMissing(t, callRepoList(t, client, false), mutatingRepoURL)

		assertRepoResult(t, client, "Add", mutatingSource, "")
		assertRepository(t, callRepoList(t, client, false), mutatingRepoURL, true, "classic")
	})

	t.Run("CheckRemove and Remove", func(t *testing.T) {
		assertRepoResult(t, client, "CheckRemove", mutatingSource, "")
		assertRepoResult(t, client, "Remove", mutatingSource, "")
		assertRepositoryMissing(t, callRepoList(t, client, false), mutatingRepoURL)
	})

	t.Run("CheckClean and Clean", func(t *testing.T) {
		assertRepository(t, callRepoList(t, client, false), temporaryRepoURL, true, "task")
		assertRepoResult(t, client, "CheckClean")
		assertRepoResult(t, client, "Clean")
		assertRepositoryMissing(t, callRepoList(t, client, false), temporaryRepoURL)
		assertRepository(t, callRepoList(t, client, true), temporaryRepoURL, false, "task")
	})

	t.Run("CheckSetBranch and SetBranch", func(t *testing.T) {
		assertRepoResult(t, client, "CheckSetBranch", "sisyphus", "")
		result := dbustest.CallJSON[repository.RepoSetResponse](t,
			client.Request(repoIface, "SetBranch").Args("sisyphus", ""))
		assertRepoMessage(t, "SetBranch", result.Message)
		if result.Branch != "sisyphus" {
			t.Fatalf("SetBranch branch = %q, want sisyphus", result.Branch)
		}

		repositories := callRepoList(t, client, false)
		if repository := findRepositoryByBranch(repositories, "sisyphus"); repository == nil {
			t.Fatalf("SetBranch did not create a sisyphus repository: %#v", repositories)
		}
	})
}

func callRepoList(t *testing.T, client *dbustest.Client, all bool) []aptrepo.Repository {
	t.Helper()

	response := dbustest.CallJSON[repository.RepoListResponse](t, client.Request(repoIface, "List").Args(all))
	if strings.TrimSpace(response.Message) == "" {
		t.Fatal("Repo.List returned an empty message")
	}
	if response.Count != len(response.Repositories) {
		t.Fatalf("Repo.List count = %d, repositories = %d", response.Count, len(response.Repositories))
	}
	return response.Repositories
}

func assertRepoResult(t *testing.T, client *dbustest.Client, method string, args ...any) {
	t.Helper()

	request := client.Request(repoIface, method).Args(args...)
	switch method {
	case "Add", "Remove", "Clean":
		result := dbustest.CallJSON[repository.RepoAddRemoveResponse](t, request)
		assertRepoMessage(t, method, result.Message)
	case "CheckAdd", "CheckRemove", "CheckSetBranch", "CheckClean":
		result := dbustest.CallJSON[repository.RepoSimulateResponse](t, request)
		assertRepoMessage(t, method, result.Message)
	default:
		t.Fatalf("assertRepoResult does not support %s", method)
	}
}

func assertRepoMessage(t *testing.T, method, message string) {
	t.Helper()
	if strings.TrimSpace(message) == "" {
		t.Fatalf("%s returned an empty message", method)
	}
}

func assertRepository(
	t *testing.T,
	repositories []aptrepo.Repository,
	url string,
	active bool,
	wantComponents ...string,
) {
	t.Helper()

	repository := findRepositoryByURL(repositories, url)
	if repository == nil {
		t.Fatalf("repository %q not found in %#v", url, repositories)
	}

	if repository.Active != active {
		t.Fatalf("repository %q active = %t, want %t", url, repository.Active, active)
	}

	if !slices.Equal(repository.Components, wantComponents) {
		t.Fatalf("repository %q components = %#v, want %#v", url, repository.Components, wantComponents)
	}
}

func assertRepositoryMissing(t *testing.T, repositories []aptrepo.Repository, url string) {
	t.Helper()

	if repository := findRepositoryByURL(repositories, url); repository != nil {
		t.Fatalf("repository %q unexpectedly present: %#v", url, repository)
	}
}

func findRepositoryByURL(repositories []aptrepo.Repository, url string) *aptrepo.Repository {
	for i := range repositories {
		if repositories[i].URL == url {
			return &repositories[i]
		}
	}
	return nil
}

func findRepositoryByBranch(repositories []aptrepo.Repository, branch string) *aptrepo.Repository {
	for i := range repositories {
		if repositories[i].Branch == branch {
			return &repositories[i]
		}
	}
	return nil
}

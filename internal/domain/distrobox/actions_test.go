package distrobox

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/sandbox"
	"apm/internal/common/testutil"
	"context"
	"errors"
	"testing"
)

type mockPackageService struct {
	infoResult    sandbox.InfoPackageAnswer
	infoErr       error
	searchResult  sandbox.PackageQueryResult
	installErr    error
	removeErr     error
	installCalled bool
	removeCalled  bool
}

func (m *mockPackageService) UpdatePackages(_ context.Context, _ sandbox.ContainerInfo) ([]sandbox.PackageInfo, error) {
	return nil, nil
}

func (m *mockPackageService) GetInfoPackage(_ context.Context, _ sandbox.ContainerInfo, _ string) (sandbox.InfoPackageAnswer, error) {
	return m.infoResult, m.infoErr
}

func (m *mockPackageService) GetPackageByName(_ context.Context, _ sandbox.ContainerInfo, _ string) (sandbox.PackageQueryResult, error) {
	return m.searchResult, nil
}

func (m *mockPackageService) GetPackagesQuery(_ context.Context, _ sandbox.ContainerInfo, _ sandbox.PackageQueryBuilder) (sandbox.PackageQueryResult, error) {
	return sandbox.PackageQueryResult{}, nil
}

func (m *mockPackageService) InstallPackage(_ context.Context, _ sandbox.ContainerInfo, _ string) error {
	m.installCalled = true
	return m.installErr
}

func (m *mockPackageService) RemovePackage(_ context.Context, _ sandbox.ContainerInfo, _ string) error {
	m.removeCalled = true
	return m.removeErr
}

type mockDistroDBService struct {
	containerExistErr error
	deleteErr         error
	updatedFields     []updatedField
	deleteCalled      bool
}

type updatedField struct {
	container, name, field string
	value                  bool
}

func (m *mockDistroDBService) DatabaseExist(_ context.Context) error { return nil }

func (m *mockDistroDBService) ContainerDatabaseExist(_ context.Context, _ string) error {
	return m.containerExistErr
}

func (m *mockDistroDBService) DeletePackagesFromContainer(_ context.Context, _ string) error {
	m.deleteCalled = true
	return m.deleteErr
}

func (m *mockDistroDBService) UpdatePackageField(_ context.Context, containerName, name, fieldName string, value bool) {
	m.updatedFields = append(m.updatedFields, updatedField{containerName, name, fieldName, value})
}

type mockDistroAPIService struct {
	osInfo       sandbox.ContainerInfo
	osInfoErr    error
	removeResult sandbox.ContainerInfo
	removeErr    error
	exportCalled bool
	exportDelete bool
}

func (m *mockDistroAPIService) GetContainerList(_ context.Context, _ bool) ([]sandbox.ContainerInfo, error) {
	return nil, nil
}

func (m *mockDistroAPIService) GetContainerOsInfo(_ context.Context, _ string) (sandbox.ContainerInfo, error) {
	return m.osInfo, m.osInfoErr
}

func (m *mockDistroAPIService) CreateContainer(_ context.Context, _, _, _ string, _ string) (sandbox.ContainerInfo, error) {
	return sandbox.ContainerInfo{}, nil
}

func (m *mockDistroAPIService) RemoveContainer(_ context.Context, _ string) (sandbox.ContainerInfo, error) {
	return m.removeResult, m.removeErr
}

func (m *mockDistroAPIService) ExportingApp(_ context.Context, _ sandbox.ContainerInfo, _ string, _, _ []string, deleteApp bool) error {
	m.exportCalled = true
	m.exportDelete = deleteApp
	return nil
}

type mockIconService struct {
	iconData []byte
	iconErr  error
}

func (m *mockIconService) GetIcon(_, _ string) ([]byte, error) {
	return m.iconData, m.iconErr
}

func (m *mockIconService) ReloadIcons(_ context.Context) error {
	return nil
}

func newTestActions(pkg *mockPackageService, db *mockDistroDBService, api *mockDistroAPIService, ico *mockIconService) *Actions {
	return &Actions{
		servicePackage:        pkg,
		serviceDistroDatabase: db,
		serviceDistroAPI:      api,
		iconService:           ico,
	}
}

func defaultAPI() *mockDistroAPIService {
	return &mockDistroAPIService{
		osInfo: sandbox.ContainerInfo{ContainerName: "test-container", OS: "alt"},
	}
}

func defaultDB() *mockDistroDBService {
	return &mockDistroDBService{}
}

func hasDBField(fields []updatedField, field string, value bool) bool {
	for _, f := range fields {
		if f.field == field && f.value == value {
			return true
		}
	}
	return false
}

func TestInstall(t *testing.T) {
	tests := []struct {
		name         string
		pkg          *mockPackageService
		packageName  string
		export       bool
		wantErr      bool
		wantErrType  string
		wantInstall  bool
		wantExport   bool
		wantDBFields map[string]bool // field -> value
	}{
		{
			name: "already installed, no export",
			pkg: &mockPackageService{
				infoResult: sandbox.InfoPackageAnswer{
					Package: sandbox.PackageInfo{Name: "vim", Installed: true},
				},
			},
			packageName: "vim",
			wantInstall: false,
		},
		{
			name: "not installed, installs and updates DB",
			pkg: &mockPackageService{
				infoResult: sandbox.InfoPackageAnswer{
					Package: sandbox.PackageInfo{Name: "vim", Installed: false},
				},
			},
			packageName:  "vim",
			wantInstall:  true,
			wantDBFields: map[string]bool{"installed": true},
		},
		{
			name: "export with desktop paths sets exporting",
			pkg: &mockPackageService{
				infoResult: sandbox.InfoPackageAnswer{
					Package:      sandbox.PackageInfo{Name: "firefox", Installed: true},
					DesktopPaths: []string{"/usr/share/applications/firefox.desktop"},
				},
			},
			packageName:  "firefox",
			export:       true,
			wantExport:   true,
			wantDBFields: map[string]bool{"exporting": true},
		},
		{
			name: "export without paths does not set exporting",
			pkg: &mockPackageService{
				infoResult: sandbox.InfoPackageAnswer{
					Package: sandbox.PackageInfo{Name: "curl", Installed: true},
				},
			},
			packageName: "curl",
			export:      true,
			wantExport:  true,
		},
		{
			name:        "empty package name returns validation error",
			pkg:         &mockPackageService{},
			packageName: "  ",
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeValidation,
		},
		{
			name: "GetInfoPackage error returns not found",
			pkg: &mockPackageService{
				infoErr: errors.New("package not found"),
			},
			packageName: "nonexistent",
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeNotFound,
		},
		{
			name: "InstallPackage error returns container error",
			pkg: &mockPackageService{
				infoResult: sandbox.InfoPackageAnswer{
					Package: sandbox.PackageInfo{Name: "vim", Installed: false},
				},
				installErr: errors.New("permission denied"),
			},
			packageName: "vim",
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeContainer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := defaultDB()
			api := defaultAPI()
			actions := newTestActions(tt.pkg, db, api, nil)

			_, err := actions.Install(context.Background(), "test-container", tt.packageName, tt.export)

			if tt.wantErr {
				testutil.AssertAPMError(t, err, tt.wantErrType)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.pkg.installCalled != tt.wantInstall {
				t.Errorf("installCalled = %v, want %v", tt.pkg.installCalled, tt.wantInstall)
			}
			if api.exportCalled != tt.wantExport {
				t.Errorf("exportCalled = %v, want %v", api.exportCalled, tt.wantExport)
			}
			for field, value := range tt.wantDBFields {
				if !hasDBField(db.updatedFields, field, value) {
					t.Errorf("expected DB update %s=%v, got %v", field, value, db.updatedFields)
				}
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name         string
		pkg          *mockPackageService
		packageName  string
		onlyExport   bool
		wantErr      bool
		wantErrType  string
		wantRemove   bool
		wantExport   bool
		wantDBFields map[string]bool
	}{
		{
			name: "exporting package removes export then package",
			pkg: &mockPackageService{
				infoResult: sandbox.InfoPackageAnswer{
					Package:      sandbox.PackageInfo{Name: "firefox", Installed: true, Exporting: true},
					DesktopPaths: []string{"/usr/share/applications/firefox.desktop"},
				},
			},
			packageName:  "firefox",
			wantRemove:   true,
			wantExport:   true,
			wantDBFields: map[string]bool{"exporting": false, "installed": false},
		},
		{
			name: "only export keeps package installed",
			pkg: &mockPackageService{
				infoResult: sandbox.InfoPackageAnswer{
					Package: sandbox.PackageInfo{Name: "vim", Installed: true, Exporting: true},
				},
			},
			packageName:  "vim",
			onlyExport:   true,
			wantRemove:   false,
			wantExport:   true,
			wantDBFields: map[string]bool{"exporting": false},
		},
		{
			name: "not exporting skips export removal",
			pkg: &mockPackageService{
				infoResult: sandbox.InfoPackageAnswer{
					Package: sandbox.PackageInfo{Name: "curl", Installed: true},
				},
			},
			packageName:  "curl",
			wantRemove:   true,
			wantExport:   false,
			wantDBFields: map[string]bool{"installed": false},
		},
		{
			name:        "empty package name returns validation error",
			pkg:         &mockPackageService{},
			packageName: "",
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeValidation,
		},
		{
			name: "GetInfoPackage error returns not found",
			pkg: &mockPackageService{
				infoErr: errors.New("not found"),
			},
			packageName: "missing",
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeNotFound,
		},
		{
			name: "RemovePackage error returns container error",
			pkg: &mockPackageService{
				infoResult: sandbox.InfoPackageAnswer{
					Package: sandbox.PackageInfo{Name: "vim", Installed: true},
				},
				removeErr: errors.New("failed"),
			},
			packageName: "vim",
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeContainer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := defaultDB()
			api := defaultAPI()
			actions := newTestActions(tt.pkg, db, api, nil)

			_, err := actions.Remove(context.Background(), "test-container", tt.packageName, tt.onlyExport)

			if tt.wantErr {
				testutil.AssertAPMError(t, err, tt.wantErrType)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.pkg.removeCalled != tt.wantRemove {
				t.Errorf("removeCalled = %v, want %v", tt.pkg.removeCalled, tt.wantRemove)
			}
			if api.exportCalled != tt.wantExport {
				t.Errorf("exportCalled = %v, want %v", api.exportCalled, tt.wantExport)
			}
			for field, value := range tt.wantDBFields {
				if !hasDBField(db.updatedFields, field, value) {
					t.Errorf("expected DB update %s=%v, got %v", field, value, db.updatedFields)
				}
			}
		})
	}
}

func TestRemove_ExportingPackage_DBUpdateOrder(t *testing.T) {
	pkg := &mockPackageService{
		infoResult: sandbox.InfoPackageAnswer{
			Package:      sandbox.PackageInfo{Name: "firefox", Installed: true, Exporting: true},
			DesktopPaths: []string{"/usr/share/applications/firefox.desktop"},
		},
	}
	db := defaultDB()
	actions := newTestActions(pkg, db, defaultAPI(), nil)

	_, err := actions.Remove(context.Background(), "test-container", "firefox", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(db.updatedFields) < 2 {
		t.Fatalf("expected at least 2 DB updates, got %d", len(db.updatedFields))
	}
	if db.updatedFields[0].field != "exporting" || db.updatedFields[0].value != false {
		t.Error("first DB update should be exporting=false")
	}
	if db.updatedFields[1].field != "installed" || db.updatedFields[1].value != false {
		t.Error("second DB update should be installed=false")
	}
}

func TestInstall_ContainerNotFound_CleansDBAndReturnsError(t *testing.T) {
	api := &mockDistroAPIService{osInfoErr: errors.New("container not found")}
	db := &mockDistroDBService{containerExistErr: nil}
	actions := newTestActions(&mockPackageService{}, db, api, nil)

	_, err := actions.Install(context.Background(), "gone-container", "vim", false)
	testutil.AssertAPMError(t, err, apmerr.ErrorTypeNotFound)
	if !db.deleteCalled {
		t.Error("should clean DB records for container that no longer exists in distrobox")
	}
}

func TestRemove_ContainerNotFound_CleansDBAndReturnsError(t *testing.T) {
	api := &mockDistroAPIService{osInfoErr: errors.New("container not found")}
	db := &mockDistroDBService{containerExistErr: nil}
	actions := newTestActions(&mockPackageService{}, db, api, nil)

	_, err := actions.Remove(context.Background(), "gone-container", "vim", false)
	testutil.AssertAPMError(t, err, apmerr.ErrorTypeNotFound)
	if !db.deleteCalled {
		t.Error("should clean DB records for container that no longer exists in distrobox")
	}
}

func TestContainerRemove(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		api           *mockDistroAPIService
		db            *mockDistroDBService
		wantErr       bool
		wantErrType   string
		wantDBClean   bool
	}{
		{
			name:          "success deletes packages from DB",
			containerName: "mybox",
			api:           &mockDistroAPIService{removeResult: sandbox.ContainerInfo{ContainerName: "mybox"}},
			db:            defaultDB(),
			wantDBClean:   true,
		},
		{
			name:          "empty name returns validation error",
			containerName: "  ",
			api:           defaultAPI(),
			db:            defaultDB(),
			wantErr:       true,
			wantErrType:   apmerr.ErrorTypeValidation,
		},
		{
			name:          "API error returns container error",
			containerName: "mybox",
			api:           &mockDistroAPIService{removeErr: errors.New("failed")},
			db:            defaultDB(),
			wantErr:       true,
			wantErrType:   apmerr.ErrorTypeContainer,
		},
		{
			name:          "DB delete error returns database error",
			containerName: "mybox",
			api:           &mockDistroAPIService{removeResult: sandbox.ContainerInfo{ContainerName: "mybox"}},
			db:            &mockDistroDBService{deleteErr: errors.New("db error")},
			wantErr:       true,
			wantErrType:   apmerr.ErrorTypeDatabase,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := newTestActions(nil, tt.db, tt.api, nil)

			_, err := actions.ContainerRemove(context.Background(), tt.containerName)

			if tt.wantErr {
				testutil.AssertAPMError(t, err, tt.wantErrType)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantDBClean && !tt.db.deleteCalled {
				t.Error("should delete packages from DB after container removal")
			}
		})
	}
}

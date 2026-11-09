// Run only in the disposable container prepared by scripts/test-dbus-e2e.sh.
// These calls go through the public library, the system bus and the real APM
// service. Package transactions are confined to the hello fixture.

private bool contains (string[]? values, string value) {
    if (values == null) {
        return false;
    }
    foreach (var item in values) {
        if (item == value) {
            return true;
        }
    }
    return false;
}

private async void check_denied () throws Error {
    var client = yield Apm.Client.connect_bus (BusType.SYSTEM, false);
    bool denied = false;
    try {
        yield client.packages.check_install ({ "hello" });
    } catch (Error e) {
        denied = e is Apm.Error.PERMISSION;
        if (!denied) {
            throw e;
        }
    }
    assert (denied);
    denied = false;
    try {
        yield client.repo.check_clean ();
    } catch (Error e) {
        denied = e is Apm.Error.PERMISSION;
        if (!denied) {
            throw e;
        }
    }
    assert (denied);
}

private async void check_service () throws Error {
    var client = yield Apm.Client.connect_bus (BusType.SYSTEM, false);
    Test.message ("Properties and typed package queries");
    assert ((yield client.packages.get_version ()).length > 0);
    var info = yield client.packages.info ("hello");
    assert (info.package_info.name == "hello");
    assert (!info.package_info.installed);
    assert (info.package_info.version.length > 0);

    var query = new Apm.ModelsListRequest () { limit = 1 };
    var filter = new Apm.ModelsFilter () { field = "name", op = "eq", value = "hello" };
    query.filters.add (filter);
    var listed = yield client.packages.list (query);
    assert (listed.total_count == 1);
    assert (listed.packages.length == 1);
    assert (listed.packages[0].name == "hello");
    var multi = yield client.packages.multi_info ({ "hello" });
    assert (multi.packages.length == 1);
    assert (multi.packages[0].name == "hello");
    var search = yield client.packages.search ("hello", false);
    assert (search.packages.length > 0);
    assert ((yield client.packages.sections ()).length > 0);
    assert ((yield client.packages.filter_fields ()).length > 0);

    Test.message ("Jobs: simulate, install, reinstall, remove, typed results and RPM state");
    var check = yield client.packages.check_install ({ "hello" });
    assert (contains (check.info.new_installed_packages, "hello"));
    assert (!(yield client.packages.info ("hello")).package_info.installed);
    var installed = yield client.packages.install (
        { "hello" },
        new Apm.ModelsInstallOptions () { no_update = true }
    );
    assert (contains (installed.info.new_installed_packages, "hello"));
    assert ((yield client.packages.info ("hello")).package_info.installed);
    yield client.packages.reinstall ({ "hello" });
    assert ((yield client.packages.info ("hello")).package_info.installed);
    var removing = yield client.packages.check_remove ({ "hello" });
    assert (contains (removing.info.removed_packages, "hello"));
    assert ((yield client.packages.info ("hello")).package_info.installed);
    var removed = yield client.packages.remove ({ "hello" });
    assert (contains (removed.info.removed_packages, "hello"));
    assert (!(yield client.packages.info ("hello")).package_info.installed);

    Test.message ("Typed remote error");
    bool failed = false;
    try {
        yield client.packages.info ("apm-e2e-nonexistent-package-948153");
    } catch (Error e) {
        failed = e is Apm.Error.NOT_FOUND;
        if (!failed) {
            throw e;
        }
    }
    assert (failed);

    Test.message ("Repositories, arrays and simulation responses");
    assert ((yield client.repo.branches ()).length > 0);
    var repositories = yield client.repo.list (true);
    assert (repositories.repositories.length > 0);
    foreach (var repository in repositories.repositories) {
        assert (repository.url.length > 0);
        assert (repository.components.length > 0);
    }
    yield client.repo.check_add ({ "https://example.invalid/apm-libapm-e2e", "x86_64", "classic" }, "");

    Test.message ("Application catalog job, nested components and collections");
    yield client.applications.update ();
    var applications = yield client.applications.list (new Apm.ModelsListRequest () { limit = 2 });
    assert (applications.total_count > 0);
    assert (applications.components.length > 0);
    assert (applications.components.length <= 2);
    assert ((yield client.applications.filter_fields ()).length > 0);
    assert ((yield client.applications.categories ()).length > 0);
}

private void test_service () {
    var loop = new MainLoop ();
    bool success = false;
    if (Environment.get_variable ("APM_LIBAPM_E2E_DENIED") != null) {
        check_denied.begin ((obj, result) => {
            try {
                check_denied.end (result);
                success = true;
            } catch (Error e) {
                Test.message ("Permission E2E failed: %s", e.message);
            }
            loop.quit ();
        });
    } else {
        check_service.begin ((obj, result) => {
            try {
                check_service.end (result);
                success = true;
            } catch (Error e) {
                Test.message ("Library E2E failed: %s", e.message);
            }
            loop.quit ();
        });
    }
    loop.run ();
    assert (success);
}

int main (string[] args) {
    Test.init (ref args);
    if (Environment.get_variable ("APM_LIBAPM_E2E") == null) {
        Test.message ("Requires the disposable system-bus container; use scripts/test-dbus-e2e.sh");
        return 77;
    }
    Test.add_func ("/libapm/live/service", test_service);
    return Test.run ();
}

/* SPDX-License-Identifier: GPL-3.0-or-later */

// Public API roundtrips over a real private message bus. The responder checks
// signatures independently of the client, according to the Go APM2 handlers.
private class ApiFixture : Object {
    private const string NAME = "org.altlinux.APM";
    private const string PATH = "/org/altlinux/APM2";
    private const string PREFIX = "org.altlinux.APM2.";
    internal TestDBus bus;
    internal DBusConnection service;
    internal DBusConnection connection;
    internal Apm.Client client;
    private Mutex mutex;
    private uint filter;
    private string expected_interface = "";
    private string expected_member = "";
    private string expected_signature = "";
    private string payload = "{}";
    private string remote_error = "";
    private Variant? reply_body;
    private Variant? received;
    private bool job;
    private uint calls;
    internal uint total_calls;
    internal uint job_queries;
    private string problem = "";

    internal ApiFixture () throws Error {
        bus = new TestDBus (TestDBusFlags.NONE);
        bus.up ();
        var flags = DBusConnectionFlags.AUTHENTICATION_CLIENT |
                    DBusConnectionFlags.MESSAGE_BUS_CONNECTION;
        service = new DBusConnection.for_address_sync (bus.get_bus_address (), flags);
        connection = new DBusConnection.for_address_sync (bus.get_bus_address (), flags);
        service.call_sync (
            "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
            "RequestName", new Variant ("(su)", NAME, 0u), new VariantType ("(u)"),
            DBusCallFlags.NONE, -1
        );
        filter = service.add_filter ((connection, message, incoming) => {
            if (!incoming || message.get_message_type () != DBusMessageType.METHOD_CALL ||
                message.get_path () != PATH) {
                return message;
            }
            mutex.lock ();
            var response = new DBusMessage.method_reply (message);
            if (message.get_interface () == PREFIX + "Jobs" && message.get_member () == "Get") {
                job_queries++;
                response.set_body (new Variant ("(s)",
                    "{\"id\":\"api-job\",\"state\":\"ok\",\"result\":%s}".printf (payload)));
            } else {
                var body = message.get_body ();
                var signature = body == null ? "()" : body.get_type_string ();
                if (message.get_interface () != expected_interface ||
                    message.get_member () != expected_member || signature != expected_signature) {
                    problem = "Expected %s.%s %s, got %s.%s %s".printf (
                        expected_interface, expected_member, expected_signature,
                        message.get_interface (), message.get_member (), signature
                    );
                    response = new DBusMessage.method_error_literal (
                        message, "org.freedesktop.DBus.Error.InvalidArgs", problem
                    );
                } else {
                    calls++;
                    total_calls++;
                    received = body;
                    if (remote_error != "") {
                        response = new DBusMessage.method_error_literal (
                            message, "org.altlinux.APM.Error." + remote_error, "expected remote failure"
                        );
                    } else if (reply_body != null) {
                        response.set_body (reply_body);
                    } else {
                        response.set_body (new Variant ("(s)", job ? "api-job" : payload));
                    }
                }
            }
            mutex.unlock ();
            try {
                connection.send_message (response, DBusSendMessageFlags.NONE, null);
            } catch (Error e) {
                error ("Fake service send failed: %s", e.message);
            }
            return null;
        });
    }

    internal async void connect_client () throws Error {
        client = yield Apm.Client.from_connection (connection, false);
    }

    internal void expect (
        string iface,
        string member,
        string signature,
        string json = "{\"message\":\"decoded\"}",
        bool is_job = false,
        Variant? reply = null,
        string error_kind = ""
    ) {
        mutex.lock ();
        expected_interface = iface.has_prefix ("org.") ? iface : PREFIX + iface;
        expected_member = member;
        expected_signature = signature;
        payload = json;
        job = is_job;
        reply_body = reply;
        remote_error = error_kind;
        calls = 0;
        received = null;
        problem = "";
        mutex.unlock ();
    }

    internal Variant check () {
        mutex.lock ();
        if (problem != "") {
            Test.message ("%s", problem);
        }
        assert (problem == "");
        assert (calls == 1);
        var body = received ?? new Variant.tuple ({});
        mutex.unlock ();
        return body;
    }

    internal void json_argument (int index, string key, string value) throws Deser.Error {
        var codec = new Deser.DynamicValueCodec ();
        var json = check ().get_child_value (index).get_string ();
        var actual = new Deser.JsonWorker (json).deserialize_with_codec<Deser.DynamicValue> (codec);
        var expected = new Deser.JsonWorker (value).deserialize_with_codec<Deser.DynamicValue> (codec);
        assert (actual.kind == Deser.ValueKind.MAP);
        if (!actual.entries.contains (key)) {
            // Omitting a false/null default is equivalent to sending it.
            assert (expected.kind == Deser.ValueKind.NULL ||
                    (expected.kind == Deser.ValueKind.BOOL && !expected.boolean_value));
            return;
        }
        assert_dynamic_equal (actual.entries.lookup (key), expected);
    }

    private void assert_dynamic_equal (Deser.DynamicValue actual, Deser.DynamicValue expected) {
        assert (actual.kind == expected.kind);
        switch (expected.kind) {
            case Deser.ValueKind.NULL: break;
            case Deser.ValueKind.BOOL: assert (actual.boolean_value == expected.boolean_value); break;
            case Deser.ValueKind.INTEGER: assert (actual.integer_value == expected.integer_value); break;
            case Deser.ValueKind.STRING: assert (actual.text_value == expected.text_value); break;
            case Deser.ValueKind.ARRAY:
                assert (actual.items.length == expected.items.length);
                for (uint i = 0; i < expected.items.length; i++) {
                    assert_dynamic_equal (actual.items[i], expected.items[i]);
                }
                break;
            case Deser.ValueKind.MAP:
                assert (actual.entries.size () == expected.entries.size ());
                expected.entries.foreach ((key, child) => {
                    assert (actual.entries.contains (key));
                    assert_dynamic_equal (actual.entries.lookup (key), child);
                });
                break;
            default: assert_not_reached ();
        }
    }

    internal void close () {
        service.remove_filter (filter);
        try {
            connection.close_sync ();
            service.close_sync ();
        } catch (Error e) {
            error ("Closing test bus: %s", e.message);
        }
        bus.down ();
    }
}

private async void exercise_packages (ApiFixture f) throws Error {
    var c = f.client.packages;
    string[] packages = {"hello"};
    f.expect ("Packages", "Install", "(ass)", "{\"message\":\"decoded\",\"info\":{\"newInstalledCount\":1}}", true);
    var apt = new HashTable<string, string> (str_hash, str_equal);
    apt.insert ("APT::Get::Assume-Yes", "true");
    var installed = yield c.install (packages, new Apm.ModelsInstallOptions () {no_update = true, apt_config = apt});
    assert (installed.message == "decoded" && installed.info.new_installed_count == 1);
    f.json_argument (1, "noUpdate", "true");
    f.json_argument (1, "aptConfig", "{\"APT::Get::Assume-Yes\":\"true\"}");
    f.json_argument (1, "downloadOnly", "false");
    f.expect ("Packages", "Remove", "(ass)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.remove (packages, new Apm.ModelsRemoveOptions () {purge = true})).message == "decoded");
    f.json_argument (1, "purge", "true");
    f.expect ("Packages", "Reinstall", "(ass)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.reinstall (packages)).message == "decoded"); f.check ();
    f.expect ("Packages", "Upgrade", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.upgrade (new Apm.ModelsUpgradeOptions () {download_only = true})).message == "decoded");
    f.json_argument (0, "downloadOnly", "true");
    f.expect ("Packages", "Update", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.update (new Apm.ModelsUpdateOptions () {only_db = true})).message == "decoded");
    f.json_argument (0, "onlyDB", "true");
    f.expect ("Packages", "CheckInstall", "(ass)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.check_install (packages)).message == "decoded"); f.check ();
    f.expect ("Packages", "CheckRemove", "(ass)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.check_remove (packages)).message == "decoded"); f.check ();
    f.expect ("Packages", "CheckUpgrade", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.check_upgrade ()).message == "decoded"); f.check ();
    f.expect ("Packages", "List", "(s)", "{\"message\":\"decoded\",\"totalCount\":1,\"packages\":[{\"name\":\"hello\"}]}");
    var request = new Apm.ModelsListRequest () {limit = 3, offset = 1};
    request.filters.add (new Apm.ModelsFilter () {field = "name", op = "eq", value = "hello"});
    request.or_filters.add (new Apm.ModelsFilter () {field = "section", op = "eq", value = "Development"});
    var list = yield c.list (request);
    assert (list.total_count == 1 && list.packages.length == 1 && list.packages[0].name == "hello");
    f.json_argument (0, "limit", "3"); f.json_argument (0, "offset", "1");
    f.json_argument (0, "filters", "[{\"field\":\"name\",\"op\":\"eq\",\"value\":\"hello\"}]");
    f.json_argument (0, "orFilters", "[{\"field\":\"section\",\"op\":\"eq\",\"value\":\"Development\"}]");
    f.expect ("Packages", "Info", "(s)", "{\"message\":\"decoded\",\"packageInfo\":{\"name\":\"hello\",\"installed\":true}}");
    var info = yield c.info ("hello");
    assert (info.package_info.name == "hello" && info.package_info.installed); f.check ();
    f.expect ("Packages", "MultiInfo", "(as)", "{\"message\":\"decoded\",\"packages\":[{\"name\":\"hello\"}]}");
    assert ((yield c.multi_info (packages)).message == "decoded"); f.check ();
    f.expect ("Packages", "Search", "(sb)", "{\"message\":\"decoded\"}");
    assert ((yield c.search ("hello", true)).message == "decoded");
    assert (f.check ().get_child_value (1).get_boolean ());
    f.expect ("Packages", "Sections", "()", "{}", false, new Variant ("(^as)", new string[] {"Development"}));
    assert ((yield c.sections ())[0] == "Development"); f.check ();
    f.expect ("Packages", "FilterFields", "()", "[{\"name\":\"name\",\"defaultOp\":\"eq\",\"allowedOps\":[\"eq\"],\"extra\":{\"type\":\"STRING\"}}]");
    var fields = yield c.filter_fields ();
    assert (fields.length == 1 && fields[0].name == "name" && fields[0].default_op == "eq"); f.check ();
    f.expect ("org.freedesktop.DBus.Properties", "Get", "(ss)", "{}", false, new Variant ("(v)", new Variant.string ("0.7.1")));
    assert ((yield c.get_version ()) == "0.7.1");
    assert (f.check ().get_child_value (1).get_string () == "Version");
    f.expect ("org.freedesktop.DBus.Properties", "Get", "(ss)", "{}", false, new Variant ("(v)", new Variant.boolean (true)));
    assert (yield c.get_is_atomic ());
    assert (f.check ().get_child_value (1).get_string () == "IsAtomic");
    f.expect ("org.freedesktop.DBus.Properties", "Get", "(ss)", "{}", false, new Variant ("(v)", new Variant.boolean (false)));
    assert (!(yield c.get_kernel_supported ()));
    assert (f.check ().get_child_value (1).get_string () == "KernelSupported");
}

private async void exercise_applications (ApiFixture f) throws Error {
    var c = f.client.applications;
    f.expect ("Applications", "Update", "()", "{\"message\":\"decoded\"}", true);
    assert ((yield c.update ()).message == "decoded"); f.check ();
    f.expect ("Applications", "Info", "(s)", "{\"message\":\"decoded\",\"pkgname\":\"hello\",\"components\":[{\"id\":\"org.example.App\",\"name\":{\"C\":\"App\"}}]}");
    var info = yield c.info ("hello");
    assert (info.pkgname == "hello" && info.components.length == 1); f.check ();
    f.expect ("Applications", "List", "(s)", "{\"message\":\"decoded\",\"totalCount\":1,\"components\":[{\"id\":7,\"pkgname\":\"hello\",\"components\":[{\"id\":\"org.example.App\"}]}]}");
    var list = yield c.list (new Apm.ModelsListRequest () {limit = 2});
    assert (list.total_count == 1 && list.components.length == 1); f.json_argument (0, "limit", "2");
    f.expect ("Applications", "Categories", "()", "{}", false, new Variant ("(^as)", new string[] {"Utility"}));
    assert ((yield c.categories ())[0] == "Utility"); f.check ();
    f.expect ("Applications", "FilterFields", "()", "[{\"name\":\"id\"}]");
    assert ((yield c.filter_fields ())[0].name == "id"); f.check ();
}

private async void exercise_image (ApiFixture f) throws Error {
    var c = f.client.image;
    f.expect ("Image", "Status", "()");
    assert ((yield c.status ()).message == "decoded"); f.check ();
    f.expect ("Image", "Update", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.update (new Apm.ModelsImageUpdateOptions () {no_cache = true})).message == "decoded");
    f.json_argument (0, "noCache", "true");
    f.expect ("Image", "Apply", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.apply (new Apm.ModelsImageApplyOptions () {pull = true, config_path = "/tmp/config.yml"})).message == "decoded");
    f.json_argument (0, "configPath", "\"/tmp/config.yml\"");
    f.expect ("Image", "Switch", "(ss)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.switch ("registry.example/onyx", new Apm.ModelsImageSwitchOptions () {no_cache = true})).message == "decoded");
    assert (f.check ().get_child_value (0).get_string () == "registry.example/onyx"); f.json_argument (1, "noCache", "true");
    f.expect ("Image", "History", "(ss)", "{\"message\":\"decoded\",\"totalCount\":2,\"history\":[]}");
    var history = yield c.history ("onyx", new Apm.ModelsPageRequest () {limit = 5});
    assert (history.total_count == 2); f.json_argument (1, "limit", "5");
    f.expect ("Image", "GetConfig", "()", """
        {"config":{"image":"onyx","env":{"KEY":"VALUE"},"modules":[
        {"type":"branding","body":{"metainfo":{
        "name":{"C":"Onyx"},"summary":{"ru":"Система"},
        "variants":[{"image":"registry.example/onyx","name":{"C":"Desktop"}}],
        "latest-release":{"version":"0.8","description":{"C":"<p>Release</p>"}}}}}]}}
    """);
    var config = yield c.get_config ();
    assert (config.config.image == "onyx" && config.config.env.lookup ("KEY") == "VALUE"); f.check ();
    f.expect ("Image", "SaveConfig", "(s)", "{}", false, new Variant.tuple ({}));
    var branding = (Apm.ModelsConfigModuleBranding) config.config.modules[0];
    assert (branding.body.metainfo.name.lookup ("C") == "Onyx");
    branding.body.metainfo.summary["C"] = "Updated summary";
    yield c.save_config (config.config); f.json_argument (0, "image", "\"onyx\"");
    var saved = new Deser.JsonWorker (f.check ().get_child_value (0).get_string ())
        .deserialize_with_codec<Deser.DynamicValue> (new Deser.DynamicValueCodec ());
    var metadata = saved.lookup ("modules").items[0].lookup ("body").lookup ("metainfo");
    assert (metadata.lookup ("name").lookup ("C").text_value == "Onyx");
    assert (metadata.lookup ("summary").lookup ("ru").text_value == "Система");
    assert (metadata.lookup ("summary").lookup ("C").text_value == "Updated summary");
    var variant = metadata.lookup ("variants").items[0];
    assert (variant.lookup ("image").text_value == "registry.example/onyx");
    assert (variant.lookup ("name").lookup ("C").text_value == "Desktop");
    assert (metadata.lookup ("latest-release").lookup ("version").text_value == "0.8");
    assert (metadata.lookup ("latest-release").lookup ("description").lookup ("C").text_value == "<p>Release</p>");
    f.expect ("Image", "SyncGroups", "()", "{\"message\":\"decoded\",\"added\":2}");
    assert ((yield c.sync_groups ()).added == 2); f.check ();
    f.expect ("Image", "FixNss", "()", "{\"message\":\"decoded\",\"etcPasswdCount\":3}");
    assert ((yield c.fix_nss ()).etc_passwd_count == 3); f.check ();
}

private async void exercise_kernel (ApiFixture f) throws Error {
    var c = f.client.kernel;
    string[] modules = {"nvidia"};
    f.expect ("Kernel", "ListKernels", "(sb)", "{\"message\":\"decoded\",\"kernels\":[]}");
    assert ((yield c.list_kernels ("std-def", true)).message == "decoded"); f.check ();
    f.expect ("Kernel", "Current", "()", "{\"message\":\"decoded\",\"kernel\":{\"name\":\"kernel-image-std-def\"}}");
    assert ((yield c.current ()).message == "decoded"); f.check ();
    f.expect ("Kernel", "Install", "(sass)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.install ("std-def", modules, new Apm.ModelsKernelOptions () {headers = true})).message == "decoded");
    f.json_argument (2, "headers", "true");
    f.expect ("Kernel", "Update", "(sass)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.update ("std-def", modules)).message == "decoded"); f.check ();
    f.expect ("Kernel", "CheckInstall", "(sass)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.check_install ("std-def", modules)).message == "decoded"); f.check ();
    f.expect ("Kernel", "CheckUpdate", "(sass)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.check_update ("std-def", modules)).message == "decoded"); f.check ();
    f.expect ("Kernel", "CleanOld", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.clean_old (new Apm.ModelsKernelCleanOptions () {no_backup = true})).message == "decoded");
    f.json_argument (0, "noBackup", "true");
    f.expect ("Kernel", "CheckCleanOld", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.check_clean_old ()).message == "decoded"); f.check ();
    f.expect ("Kernel", "ListModules", "(s)", "{\"message\":\"decoded\",\"modules\":[]}");
    assert ((yield c.list_modules ("std-def")).message == "decoded"); f.check ();
    f.expect ("Kernel", "InstallModules", "(sas)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.install_modules ("std-def", modules)).message == "decoded"); f.check ();
    f.expect ("Kernel", "CheckInstallModules", "(sas)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.check_install_modules ("std-def", modules)).message == "decoded"); f.check ();
    f.expect ("Kernel", "RemoveModules", "(sas)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.remove_modules ("std-def", modules)).message == "decoded"); f.check ();
    f.expect ("Kernel", "CheckRemoveModules", "(sas)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.check_remove_modules ("std-def", modules)).message == "decoded"); f.check ();
}

private async void exercise_repo (ApiFixture f) throws Error {
    var c = f.client.repo;
    string[] sources = {"rpm http://example.invalid Sisyphus/x86_64 classic"};
    f.expect ("Repo", "List", "(b)", "{\"message\":\"decoded\",\"count\":1,\"repositories\":[{\"name\":\"test\"}]}");
    var list = yield c.list (true);
    assert (list.count == 1 && list.repositories.length == 1); f.check ();
    f.expect ("Repo", "Branches", "()", "{}", false, new Variant ("(^as)", new string[] {"Sisyphus"}));
    assert ((yield c.branches ())[0] == "Sisyphus"); f.check ();
    f.expect ("Repo", "TaskPackages", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.task_packages ("123")).message == "decoded"); f.check ();
    f.expect ("Repo", "TestTask", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.test_task ("123")).message == "decoded"); f.check ();
    f.expect ("Repo", "Add", "(ass)");
    assert ((yield c.add (sources, "20261107")).message == "decoded"); f.check ();
    f.expect ("Repo", "Remove", "(ass)");
    assert ((yield c.remove (sources, "20261107")).message == "decoded"); f.check ();
    f.expect ("Repo", "SetBranch", "(ss)");
    assert ((yield c.set_branch ("Sisyphus", "20261107")).message == "decoded"); f.check ();
    f.expect ("Repo", "Clean", "()");
    assert ((yield c.clean ()).message == "decoded"); f.check ();
    f.expect ("Repo", "CheckAdd", "(ass)", "{\"message\":\"decoded\",\"willAdd\":[],\"willRemove\":[]}");
    assert ((yield c.check_add (sources, "20261107")).message == "decoded"); f.check ();
    f.expect ("Repo", "CheckRemove", "(ass)");
    assert ((yield c.check_remove (sources, "20261107")).message == "decoded"); f.check ();
    f.expect ("Repo", "CheckSetBranch", "(ss)");
    assert ((yield c.check_set_branch ("Sisyphus", "20261107")).message == "decoded"); f.check ();
    f.expect ("Repo", "CheckClean", "()");
    assert ((yield c.check_clean ()).message == "decoded"); f.check ();
}

private async void exercise_distrobox (ApiFixture f) throws Error {
    var c = f.client.distrobox;
    f.expect ("Distrobox", "ContainerList", "()", "{\"containers\":[{\"name\":\"test\"}]}");
    assert ((yield c.container_list ()).containers.length == 1); f.check ();
    f.expect ("Distrobox", "ContainerAdd", "(sss)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.container_add ("image", "test", new Apm.ModelsContainerOptions () {
        additional_packages = "git", init_hooks = "echo ready"
    })).message == "decoded");
    f.json_argument (2, "additionalPackages", "\"git\""); f.json_argument (2, "initHooks", "\"echo ready\"");
    f.expect ("Distrobox", "ContainerRemove", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.container_remove ("test")).message == "decoded"); f.check ();
    f.expect ("Distrobox", "Update", "(s)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.update ("test")).message == "decoded"); f.check ();
    f.expect ("Distrobox", "Install", "(sss)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.install ("test", "hello", new Apm.ModelsDistroboxInstallOptions () {export = true})).message == "decoded");
    f.json_argument (2, "export", "true");
    f.expect ("Distrobox", "Remove", "(sss)", "{\"message\":\"decoded\"}", true);
    assert ((yield c.remove ("test", "hello", new Apm.ModelsDistroboxRemoveOptions () {only_export = true})).message == "decoded");
    f.json_argument (2, "onlyExport", "true");
    f.expect ("Distrobox", "Info", "(ss)", "{\"message\":\"decoded\",\"packageInfo\":{\"package\":{\"name\":\"hello\"}}}");
    assert ((yield c.info ("test", "hello")).package_info.package.name == "hello"); f.check ();
    f.expect ("Distrobox", "Search", "(ss)", "{\"message\":\"decoded\",\"packages\":[]}");
    assert ((yield c.search ("test", "hello")).message == "decoded"); f.check ();
    f.expect ("Distrobox", "List", "(ss)", "{\"message\":\"decoded\",\"totalCount\":1,\"packages\":[{\"name\":\"hello\"}]}");
    var list = yield c.list ("test", new Apm.ModelsListRequest () {limit = 2});
    assert (list.packages.length == 1); f.json_argument (1, "limit", "2");
    f.expect ("Distrobox", "FilterFields", "()", "[{\"name\":\"name\"}]");
    assert ((yield c.filter_fields ())[0].name == "name"); f.check ();
}

private async void exercise_icons (ApiFixture f) throws Error {
    var builder = new VariantBuilder (new VariantType ("ay"));
    builder.add ("y", (uint8) 0x89); builder.add ("y", (uint8) 0x50); builder.add ("y", (uint8) 0x4e);
    f.expect ("Icons", "Icon", "(ss)", "{}", false, new Variant.tuple ({builder.end ()}));
    var bytes = yield f.client.icons.icon ("org.example.App", "test");
    assert (bytes.get_size () == 3 && bytes.get_data ()[0] == 0x89);
    assert (f.check ().get_child_value (0).get_string () == "org.example.App");
    assert (f.check ().get_child_value (1).get_string () == "test");
}

private async void exercise_errors (ApiFixture f) throws Error {
    string[] kinds = {"Database", "Repository", "Apt", "Validation", "Permission", "Image",
                      "Kernel", "Container", "NoOperation", "NotFound", "Unexpected"};
    int[] codes = {Apm.Error.DATABASE, Apm.Error.REPOSITORY, Apm.Error.APT, Apm.Error.VALIDATION,
                   Apm.Error.PERMISSION, Apm.Error.IMAGE, Apm.Error.KERNEL, Apm.Error.CONTAINER,
                   Apm.Error.NO_OPERATION, Apm.Error.NOT_FOUND, Apm.Error.FAILED};
    for (var i = 0; i < kinds.length; i++) {
        f.expect ("Packages", "Info", "(s)", "{}", false, null, kinds[i]);
        try {
            yield f.client.packages.info ("hello");
            assert_not_reached ();
        } catch (Apm.Error e) {
            assert (e.code == codes[i]);
        }
        f.check ();
    }
    foreach (var payload in new string[] {"{]", "{\"packageInfo\":{\"installed\":\"wrong type\"}}"}) {
        f.expect ("Packages", "Info", "(s)", payload);
        try {
            yield f.client.packages.info ("hello");
            assert_not_reached ();
        } catch (Deser.Error e) {
            // Parse/type errors must propagate across the asynchronous codec.
        }
        f.check ();
    }
    f.expect ("Packages", "Info", "(s)", "{}", false, new Variant ("(b)", true));
    bool wrong_reply_rejected = false;
    try {
        yield f.client.packages.info ("hello");
    } catch (Error e) {
        wrong_reply_rejected = true;
    }
    assert (wrong_reply_rejected); f.check ();
}

private async void exercise_precancelled (ApiFixture f) throws Error {
    var cancelled = new Cancellable ();
    cancelled.cancel ();
    // Every public method exposing a Cancellable must reject before submitting
    // a server job. No expected D-Bus method is armed for these calls.
    try { yield f.client.applications.update (cancelled); assert_not_reached (); }
    catch (IOError.CANCELLED e) {}
    try { yield f.client.image.update (null, cancelled); assert_not_reached (); }
    catch (IOError.CANCELLED e) {}
    try { yield f.client.image.apply (null, cancelled); assert_not_reached (); }
    catch (IOError.CANCELLED e) {}
    try { yield f.client.image.switch ("image", null, cancelled); assert_not_reached (); }
    catch (IOError.CANCELLED e) {}
    try { yield f.client.repo.task_packages ("123", cancelled); assert_not_reached (); }
    catch (IOError.CANCELLED e) {}
    try { yield f.client.distrobox.container_add ("image", "test", null, cancelled); assert_not_reached (); }
    catch (IOError.CANCELLED e) {}
    try { yield f.client.distrobox.container_remove ("test", cancelled); assert_not_reached (); }
    catch (IOError.CANCELLED e) {}
    try { yield f.client.distrobox.update ("test", cancelled); assert_not_reached (); }
    catch (IOError.CANCELLED e) {}
}

private async void exercise_all (ApiFixture f) throws Error {
    yield f.connect_client ();
    yield exercise_packages (f);
    yield exercise_applications (f);
    yield exercise_image (f);
    yield exercise_kernel (f);
    yield exercise_repo (f);
    yield exercise_distrobox (f);
    yield exercise_icons (f);
    yield exercise_errors (f);
    yield exercise_precancelled (f);
    assert (f.total_calls == 81);
    assert (f.job_queries == 29);
    Test.message ("67 public methods, 81 API requests, 29 Jobs.Get roundtrips, 8 pre-cancel checks");
}

private void test_public_api () {
    ApiFixture? fixture = null;
    try {
        fixture = new ApiFixture ();
        var loop = new MainLoop ();
        Error? failure = null;
        exercise_all.begin (fixture, (object, result) => {
            try {
                exercise_all.end (result);
            } catch (Error e) {
                failure = e;
            }
            loop.quit ();
        });
        loop.run ();
        if (failure != null) {
            Test.message ("Public API roundtrip failed: %s", failure.message);
        }
        assert (failure == null);
    } catch (Error e) {
        Test.message ("Fixture failed: %s", e.message);
        assert_not_reached ();
    } finally {
        if (fixture != null) {
            fixture.close ();
        }
    }
}

public int main (string[] args) {
    Test.init (ref args);
    Test.add_func ("/libapm/api/public-methods", test_public_api);
    return Test.run ();
}

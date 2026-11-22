/*
 * Copyright (C) 2026 Vladimir Romanov <rirusha@altlinux.org>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see
 * <https://www.gnu.org/licenses/gpl-3.0-standalone.html>.
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

private string encode<T> (T model) throws Deser.Error {
    return Deser.JsonWorker.serialize (model, new Deser.ObjectCodec<T> (), new Deser.Settings ());
}

private Deser.DynamicValue document (string json) throws Deser.Error {
    return new Deser.JsonWorker (json).deserialize_with_codec<Deser.DynamicValue> (
        new Deser.DynamicValueCodec ()
    );
}

private void test_options_serialize () {
    try {
        var options = new Apm.ModelsInstallOptions () { download_only = true };
        var value = document (encode (options));
        assert (value.lookup ("downloadOnly").boolean_value);
        assert (!value.lookup ("noUpdate").boolean_value);
        assert (value.lookup ("aptConfig").kind == Deser.ValueKind.NULL);
        options.apt_config = new HashTable<string, string> (str_hash, str_equal);
        options.apt_config.insert ("APT::Install-Recommends", "false");
        value = document (encode (options));
        assert (value.lookup ("aptConfig").lookup ("APT::Install-Recommends").text_value == "false");
    } catch (Error e) {
        error ("Options serialization failed: %s", e.message);
    }
}

private void test_pagination_serialize () {
    try {
        var request = new Apm.ModelsListRequest () { limit = int.MAX, offset = 200 };
        request.filters.add (new Apm.ModelsFilter () { field = "name", op = "eq", value = "editor" });
        request.or_filters.add (new Apm.ModelsFilter () { field = "installed", op = "eq", value = "true" });
        var value = document (encode (request));
        assert (value.lookup ("limit").integer_value == int.MAX);
        assert (value.lookup ("offset").integer_value == 200);
        assert (value.lookup ("orFilters").items.length == 1);
        var page = new Apm.ModelsPageRequest () { limit = 25, offset = 50 };
        value = document (encode (page));
        assert (value.lookup ("limit").integer_value == 25);
        assert (value.lookup ("offset").integer_value == 50);
    } catch (Error e) {
        error ("Pagination serialization failed: %s", e.message);
    }
}

private void test_package_deserialize () {
    const string json = """
        {"message":"ok","packageInfo":{"name":"editor","installedSize":42,
        "depends":["libc"],"appStream":[{"type":"desktop-application","id":"org.example.Editor",
        "name":{"C":"Editor","ru":"Редактор"},"developer_name":{"C":"Team"},
        "screenshots":[{"type":"default","caption":{"C":"Screenshot"},
        "images":[{"type":"source","width":1280,"height":720,"url":"https://example.invalid/shot.png"}]}],
        "releases":[{"version":"1.0","description":{"C":"Release"},
        "issues":[{"type":"bug","value":"42"}],"artifacts":[{"type":"binary",
        "checksums":[{"type":"sha256","value":"abc"}],"sizes":[{"type":"download","value":128}]}]}]}]}}
    """;
    try {
        var response = new Deser.JsonWorker (json).deserialize_object<Apm.ModelsPackageResponse> ();
        assert (response.package_info.name == "editor");
        assert (response.package_info.installed_size == 42);
        assert (response.package_info.depends[0] == "libc");
        assert (response.package_info.app_stream.length == 1);
        var component = response.package_info.app_stream[0];
        assert (component.id == "org.example.Editor");
        assert (component.name.lookup ("ru") == "Редактор");
        assert (component.developer_name.lookup ("C") == "Team");
        assert (component.screenshots[0].images.length == 1);
        assert (component.releases[0].artifacts[0].sizes[0].value == 128);
        var roundtrip = new Deser.JsonWorker (encode (response)).deserialize_object<Apm.ModelsPackageResponse> ();
        assert (roundtrip.package_info.app_stream[0].releases[0].artifacts[0].checksums[0].value == "abc");
    } catch (Error e) {
        error ("Package deserialization failed: %s", e.message);
    }
}

private void test_config_roundtrip () {
    const string json = """
        {"env":{"CHANNEL":"stable"},"image":"registry.example/base:latest","modules":[
        {"name":"kernel setup","type":"kernel","body":{"kernel-info":{"flavour":"std-def",
        "modules":["nvidia"],"no-inherit-modules":true},"initrd":{"method":"auto","plymouth-theme":"bgrt"}}},
        {"type":"branding","body":{"name":"onyx","release-overrides":{"ID":"onyx"}}},
        {"type":"packages","body":{"install":["hello"],"options":{"foo":"bar"}}},
        {"type":"repos","body":{"branch":"sisyphus","no-update":true}},
        {"type":"git","body":{"url":"https://example.invalid/git","branch":"main"}},
        {"type":"future","body":{"enabled":true,"items":[1,null,{"nested":"value"}]}}]}
    """;
    try {
        var config = new Deser.JsonWorker (json).deserialize_object<Apm.ModelsConfig> ();
        assert (config.modules.length == 6);
        assert (config.modules[0] is Apm.ModelsConfigModuleKernel);
        assert (config.modules[1] is Apm.ModelsConfigModuleBranding);
        assert (config.modules[2] is Apm.ModelsConfigModulePackages);
        assert (config.modules[3] is Apm.ModelsConfigModuleRepos);
        assert (config.modules[4] is Apm.ModelsConfigModuleGit);
        assert (config.modules[5] is Apm.ModelsUnknownConfigModule);
        var kernel = (Apm.ModelsConfigModuleKernel) config.modules[0];
        assert (kernel.body.kernel_info.flavour == "std-def");
        assert (kernel.body.kernel_info.no_inherit_modules);
        var unknown = (Apm.ModelsUnknownConfigModule) config.modules[5];
        assert (unknown.body.lookup ("enabled").boolean_value);
        assert (unknown.body.lookup ("items").items[1].kind == Deser.ValueKind.NULL);
        var encoded = document (encode (config));
        var body = encoded.lookup ("modules").items[0].lookup ("body");
        assert (body.lookup ("kernel-info").lookup ("no-inherit-modules").boolean_value);
        var roundtrip = new Deser.JsonWorker (encode (config)).deserialize_object<Apm.ModelsConfig> ();
        unknown = (Apm.ModelsUnknownConfigModule) roundtrip.modules[5];
        assert (unknown.body.lookup ("items").items[2].lookup ("nested").text_value == "value");
    } catch (Error e) {
        error ("Config roundtrip failed: %s", e.message);
    }
}

private void test_branding_metainfo_roundtrip () {
    const string json = """
        {"image":"registry.example/base:latest","modules":[{"type":"branding","body":{
        "name":"onyx","build-type":"desktop","metainfo":{
        "name":{"C":"Onyx","ru":"Оникс"},"summary":{"C":"Operating system"},
        "description":{"C":"<p>System</p>","ru":"<p>Система</p>"},
        "variants":[{"image":"registry.example/desktop:latest","name":{"C":"Desktop"},
        "summary":{"ru":"Рабочий стол"},"description":{"C":"<p>Desktop</p>"}}],
        "latest-release":{"version":"0.6","date":"2026-11-07","description":{"C":"<p>Release</p>"}}}}}]}
    """;
    try {
        var config = new Deser.JsonWorker (json).deserialize_object<Apm.ModelsConfig> ();
        var branding = (Apm.ModelsConfigModuleBranding) config.modules[0];
        assert (branding.body.build_type == "desktop");
        var metadata = branding.body.metainfo;
        assert (metadata.name.lookup ("ru") == "Оникс");
        assert (metadata.summary.lookup ("C") == "Operating system");
        assert (metadata.description.lookup ("ru") == "<p>Система</p>");
        assert (metadata.variants.length == 1);
        assert (metadata.variants[0].image == "registry.example/desktop:latest");
        assert (metadata.variants[0].name.lookup ("C") == "Desktop");
        assert (metadata.variants[0].summary.lookup ("ru") == "Рабочий стол");
        assert (metadata.variants[0].description.lookup ("C") == "<p>Desktop</p>");
        assert (metadata.latest_release.version == "0.6");
        assert (metadata.latest_release.date == "2026-11-07");
        assert (metadata.latest_release.description.lookup ("C") == "<p>Release</p>");
        var original = document (json).lookup ("modules").items[0].lookup ("body").lookup ("metainfo");
        var serialized = encode (config);
        var restored = new Deser.JsonWorker (serialized).deserialize_object<Apm.ModelsConfig> ();
        var encoded = document (encode (restored)).lookup ("modules").items[0].lookup ("body");
        assert (encoded.lookup ("build-type").text_value == "desktop");
        assert (Deser.JsonWorker.serialize (original, new Deser.DynamicValueCodec ()) ==
            Deser.JsonWorker.serialize (encoded.lookup ("metainfo"), new Deser.DynamicValueCodec ()));
        foreach (var body in new string[] {
            "{}", "{\"metainfo\":null}", "{\"metainfo\":{}}",
            "{\"metainfo\":{\"latest-release\":{}}}",
            "{\"metainfo\":{\"name\":null,\"variants\":[],\"latest-release\":null}}"
        }) {
            var input = "{\"modules\":[{\"type\":\"branding\",\"body\":" + body + "}]}";
            config = new Deser.JsonWorker (input).deserialize_object<Apm.ModelsConfig> ();
            restored = new Deser.JsonWorker (encode (config)).deserialize_object<Apm.ModelsConfig> ();
            branding = (Apm.ModelsConfigModuleBranding) restored.modules[0];
            if (body == "{}" || body == "{\"metainfo\":null}") {
                assert (branding.body.metainfo == null);
            } else {
                assert (branding.body.metainfo != null);
                assert (branding.body.metainfo.name == null);
                if (body.contains ("variants")) {
                    assert (branding.body.metainfo.variants != null);
                    assert (branding.body.metainfo.variants.length == 0);
                } else {
                    assert (branding.body.metainfo.variants == null);
                }
                if (body.contains ("\"latest-release\":{}")) {
                    assert (branding.body.metainfo.latest_release != null);
                    assert (branding.body.metainfo.latest_release.version == null);
                } else {
                    assert (branding.body.metainfo.latest_release == null);
                }
            }
        }
    } catch (Error e) {
        error ("Branding metainfo roundtrip failed: %s", e.message);
    }
}

private void test_missing_null_unknown () {
    try {
        var missing = new Deser.JsonWorker ("{}").deserialize_object<Apm.ModelsConfig> ();
        assert (missing.modules != null && missing.modules.length == 0);
        assert (missing.env != null && missing.env.size () == 0);
        var explicit_null = new Deser.JsonWorker ("{\"env\":null,\"modules\":null,\"image\":null}")
            .deserialize_object<Apm.ModelsConfig> ();
        assert (explicit_null.env == null);
        assert (explicit_null.modules == null);
        assert (explicit_null.image == null);
        var extra = new Deser.JsonWorker ("{\"name\":\"editor\",\"future\":{\"nested\":true}}")
            .deserialize_object<Apm.ModelsPackageInfo> ();
        assert (extra.name == "editor");
    } catch (Error e) {
        error ("Null and missing fields failed: %s", e.message);
    }
}

private void test_invalid_documents () {
    foreach (var json in new string[] {
        "{", "[]", "{\"name\":7}", "{\"installedSize\":\"42\"}",
        "{\"appStream\":{}}", "{\"appStream\":[{\"name\":{\"C\":42}}]}",
        "{\"installedSize\":9223372036854775808}", "{\"name\":\"first\",\"name\":\"duplicate\"}"
    }) {
        bool failed = false;
        try {
            new Deser.JsonWorker (json).deserialize_object<Apm.ModelsPackageInfo> ();
        } catch (Deser.Error e) {
            failed = true;
        }
        assert (failed);
    }
}

private void test_numeric_boundaries () {
    try {
        var model = new Deser.JsonWorker ("{\"downloadSize\":9223372036854775807,\"installSize\":-9223372036854775808}")
            .deserialize_object<Apm.ModelsOperationInfo> ();
        assert (model.download_size == (uint64) int64.MAX);
        assert (model.install_size == int64.MIN);
        var restored = new Deser.JsonWorker (encode (model)).deserialize_object<Apm.ModelsOperationInfo> ();
        assert (restored.download_size == (uint64) int64.MAX);
        assert (restored.install_size == int64.MIN);
    } catch (Error e) {
        error ("Numeric boundaries failed: %s", e.message);
    }
    foreach (var json in new string[] {"{\"downloadSize\":-1}", "{\"downloadSize\":1.5}", "{\"downloadSize\":18446744073709551615}"}) {
        bool failed = false;
        try {
            new Deser.JsonWorker (json).deserialize_object<Apm.ModelsOperationInfo> ();
        } catch (Deser.Error e) {
            failed = true;
        }
        assert (failed);
    }
}

private void test_string_arrays () {
    try {
        var model = new Deser.JsonWorker ("{\"depends\":null,\"aliases\":[],\"provides\":[\"editor\"]}")
            .deserialize_object<Apm.ModelsPackageInfo> ();
        assert (model.depends == null);
        assert (model.aliases != null && model.aliases.length == 0);
        assert (model.provides.length == 1 && model.provides[0] == "editor");
        var restored = new Deser.JsonWorker (encode (model)).deserialize_object<Apm.ModelsPackageInfo> ();
        assert (restored.depends == null);
        assert (restored.aliases != null && restored.aliases.length == 0);
        assert (restored.provides[0] == "editor");
    } catch (Error e) {
        error ("Nullable string arrays failed: %s", e.message);
    }
}

private void test_unsigned_write_overflow () {
    var model = new Apm.ModelsOperationInfo () { download_size = uint64.MAX };
    bool rejected = false;
    try {
        encode (model);
    } catch (Deser.Error e) {
        rejected = e is Deser.Error.RANGE;
    }
    assert (rejected);
}

private void test_all_model_schemas () {
    foreach (var type in new Type[] {
        typeof (Apm.ModelsApplicationBrandingInfo),
        typeof (Apm.ModelsApplicationContentRatingInfo),
        typeof (Apm.ModelsApplicationDeveloperInfo),
        typeof (Apm.ModelsApplicationDisplayLengthInfo),
        typeof (Apm.ModelsApplicationIconInfo),
        typeof (Apm.ModelsApplicationImageInfo),
        typeof (Apm.ModelsApplicationInfo),
        typeof (Apm.ModelsApplicationInfoResponse),
        typeof (Apm.ModelsApplicationLanguageInfo),
        typeof (Apm.ModelsApplicationListComponentInfo),
        typeof (Apm.ModelsApplicationListResponse),
        typeof (Apm.ModelsApplicationPropertyInfo),
        typeof (Apm.ModelsApplicationProvidesDbusInfo),
        typeof (Apm.ModelsApplicationProvidesInfo),
        typeof (Apm.ModelsApplicationReleaseArtifactChecksumInfo),
        typeof (Apm.ModelsApplicationReleaseArtifactInfo),
        typeof (Apm.ModelsApplicationReleaseArtifactSizeInfo),
        typeof (Apm.ModelsApplicationReleaseInfo),
        typeof (Apm.ModelsApplicationReleaseIssueInfo),
        typeof (Apm.ModelsApplicationReleaseUrlInfo),
        typeof (Apm.ModelsApplicationScreenshotInfo),
        typeof (Apm.ModelsApplicationTranslationInfo),
        typeof (Apm.ModelsApplicationUpdateResponse),
        typeof (Apm.ModelsAptOptions),
        typeof (Apm.ModelsBootImageInfo),
        typeof (Apm.ModelsCheckEssentialPackageInfo),
        typeof (Apm.ModelsCheckResponse),
        typeof (Apm.ModelsCleanOldKernelsResponse),
        typeof (Apm.ModelsConfig),
        typeof (Apm.ModelsConfigModule),
        typeof (Apm.ModelsConfigModuleBranding),
        typeof (Apm.ModelsConfigModuleBranding.Body),
        typeof (Apm.ModelsConfigModuleGit),
        typeof (Apm.ModelsConfigModuleGit.Body),
        typeof (Apm.ModelsConfigModuleKernel),
        typeof (Apm.ModelsConfigModuleKernel.Body),
        typeof (Apm.ModelsConfigModuleKernel.Initrd),
        typeof (Apm.ModelsConfigModuleKernel.KernelInfo),
        typeof (Apm.ModelsConfigModulePackages),
        typeof (Apm.ModelsConfigModulePackages.Body),
        typeof (Apm.ModelsConfigModuleRepos),
        typeof (Apm.ModelsConfigModuleRepos.Body),
        typeof (Apm.ModelsContainerInfo),
        typeof (Apm.ModelsContainerOptions),
        typeof (Apm.ModelsDistroboxContainerListResponse),
        typeof (Apm.ModelsDistroboxContainerResponse),
        typeof (Apm.ModelsDistroboxInstallOptions),
        typeof (Apm.ModelsDistroboxListResponse),
        typeof (Apm.ModelsDistroboxPackage),
        typeof (Apm.ModelsDistroboxPackageInfo),
        typeof (Apm.ModelsDistroboxPackageResponse),
        typeof (Apm.ModelsDistroboxRemoveOptions),
        typeof (Apm.ModelsDistroboxSearchResponse),
        typeof (Apm.ModelsDistroboxUpdateResponse),
        typeof (Apm.ModelsFilter),
        typeof (Apm.ModelsFilterFieldsInfo),
        typeof (Apm.ModelsImageApplyOptions),
        typeof (Apm.ModelsImageApplyResponse),
        typeof (Apm.ModelsImageConfigResponse),
        typeof (Apm.ModelsImageFixNssResponse),
        typeof (Apm.ModelsImageHistoryRecord),
        typeof (Apm.ModelsImageHistoryResponse),
        typeof (Apm.ModelsImageInfo),
        typeof (Apm.ModelsImageSpecImageInfo),
        typeof (Apm.ModelsImageSpecInfo),
        typeof (Apm.ModelsImageStatusImageData),
        typeof (Apm.ModelsImageStatusImageInfo),
        typeof (Apm.ModelsImageStatusInfo),
        typeof (Apm.ModelsImageSwitchOptions),
        typeof (Apm.ModelsImageSyncGroupsResponse),
        typeof (Apm.ModelsImageUpdateOptions),
        typeof (Apm.ModelsInstallKernelModulesResponse),
        typeof (Apm.ModelsInstallOptions),
        typeof (Apm.ModelsInstallRemoveResponse),
        typeof (Apm.ModelsInstallUpdateKernelResponse),
        typeof (Apm.ModelsKernelCleanOptions),
        typeof (Apm.ModelsKernelCurrentResponse),
        typeof (Apm.ModelsKernelInfo),
        typeof (Apm.ModelsKernelListResponse),
        typeof (Apm.ModelsKernelModuleInfo),
        typeof (Apm.ModelsKernelOptions),
        typeof (Apm.ModelsKernelUpgradePreview),
        typeof (Apm.ModelsKernelWReasonsInfo),
        typeof (Apm.ModelsKernelWithModulesInfo),
        typeof (Apm.ModelsListKernelModulesResponse),
        typeof (Apm.ModelsListRequest),
        typeof (Apm.ModelsListResponse),
        typeof (Apm.ModelsMultiInfoResponse),
        typeof (Apm.ModelsOperationInfo),
        typeof (Apm.ModelsPackageInfo),
        typeof (Apm.ModelsPackageResponse),
        typeof (Apm.ModelsPageRequest),
        typeof (Apm.ModelsRemoveKernelModulesResponse),
        typeof (Apm.ModelsRemoveOptions),
        typeof (Apm.ModelsRepoAddRemoveResponse),
        typeof (Apm.ModelsRepoInfo),
        typeof (Apm.ModelsRepoListResponse),
        typeof (Apm.ModelsRepoSetResponse),
        typeof (Apm.ModelsRepoSimulateResponse),
        typeof (Apm.ModelsRepoTestTaskResponse),
        typeof (Apm.ModelsSearchResponse),
        typeof (Apm.ModelsTaskPackagesResponse),
        typeof (Apm.ModelsUnknownConfigModule),
        typeof (Apm.ModelsUpdateOptions),
        typeof (Apm.ModelsUpdateResponse),
        typeof (Apm.ModelsUpgradeOptions),
        typeof (Apm.ModelsUpgradeResponse)
    }) {
        try {
            var codec = new Deser.ObjectCodec<Object>.from_gtype (type);
            var model = Object.new (type);
            var json = Deser.JsonWorker.serialize (model, codec);
            var restored = new Deser.JsonWorker (json).deserialize_with_codec<Object> (codec);
            assert (restored.get_type () == type);
        } catch (Error e) {
            error ("Model %s failed: %s", type.name (), e.message);
        }
    }
}

public int main (string[] args) {
    Test.init (ref args);
    Test.add_func ("/libapm/options/serialize", test_options_serialize);
    Test.add_func ("/libapm/pagination/serialize", test_pagination_serialize);
    Test.add_func ("/libapm/package/deserialize", test_package_deserialize);
    Test.add_func ("/libapm/config/roundtrip", test_config_roundtrip);
    Test.add_func ("/libapm/config/branding-metainfo", test_branding_metainfo_roundtrip);
    Test.add_func ("/libapm/models/null-missing-unknown", test_missing_null_unknown);
    Test.add_func ("/libapm/models/invalid-documents", test_invalid_documents);
    Test.add_func ("/libapm/models/numeric-boundaries", test_numeric_boundaries);
    Test.add_func ("/libapm/models/string-arrays", test_string_arrays);
    Test.add_func ("/libapm/models/unsigned-write-overflow", test_unsigned_write_overflow);
    Test.add_func ("/libapm/models/all-schemas", test_all_model_schemas);
    return Test.run ();
}

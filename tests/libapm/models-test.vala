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

private Serialize.Settings wire_settings () {
    return new Serialize.Settings () {
        names_case = Serialize.Case.KEBAB,
        ignore_default = true
    };
}

private void test_options_serialize () {
    var options = new Apm.Models.InstallOptions () {
        download_only = true
    };
    var json = Serialize.JsonWorker.serialize (options, wire_settings ());
    var parser = new Json.Parser ();
    try {
        parser.load_from_data (json);
    } catch (Error e) {
        assert_not_reached ();
    }
    var object = parser.get_root ().get_object ();
    assert (object.get_boolean_member ("downloadOnly"));
    assert (!object.has_member ("noUpdate"));
}

private void test_package_deserialize () {
    const string json = """
        {
          "message":"ok",
          "packageInfo":{
            "name":"editor",
            "installedSize":42,
            "appStream":[{
              "type":"desktop-application",
              "id":"org.example.Editor",
              "name":{"C":"Editor"},
              "screenshots":[{
                "type":"default",
                "images":[{"type":"source","width":1280,"height":720,"url":"https://example.invalid/shot.png"}]
              }]
            }]
          }
        }
    """;
    try {
        var response = Serialize.JsonWorker.simple_from_json<Apm.Models.PackageResponse> (
            json, null, wire_settings ());
        assert (response.package_info.name == "editor");
        assert (response.package_info.installed_size == 42);
        assert (response.package_info.app_stream.size == 1);
        var component = response.package_info.app_stream[0];
        assert (component.id == "org.example.Editor");
        assert (component.name["C"] == "Editor");
        assert (component.screenshots[0].images.size == 1);
    } catch (Error e) {
        Test.message ("Deserialization failed: %s", e.message);
        assert_not_reached ();
    }
}

private void test_config_roundtrip () {
    const string json = """
        {
          "env":{"CHANNEL":"stable"},
          "image":"registry.example/base:latest",
          "modules":[{
            "name":"kernel setup",
            "type":"kernel",
            "body":{
              "kernel-info":{"flavour":"std-def","no-inherit-modules":true},
              "initrd":{"method":"auto","plymouth-theme":"bgrt"}
            }
          }]
        }
    """;
    try {
        var config = Serialize.JsonWorker.simple_from_json<Apm.Models.Config> (
            json, null, wire_settings ());
        assert (config.modules.size == 1);
        assert (config.modules[0] is Apm.Models.ConfigModuleKernel);
        var kernel = (Apm.Models.ConfigModuleKernel) config.modules[0];
        assert (kernel.body.kernel_info.flavour == "std-def");
        assert (kernel.body.kernel_info.no_inherit_modules);

        var encoded = Serialize.JsonWorker.serialize (config, wire_settings ());
        var parser = new Json.Parser ();
        parser.load_from_data (encoded);
        var body = parser.get_root ().get_object ()
            .get_array_member ("modules").get_object_element (0)
            .get_object_member ("body");
        assert (body.has_member ("kernel-info"));
        assert (body.get_object_member ("kernel-info").has_member ("no-inherit-modules"));
    } catch (Error e) {
        Test.message ("Roundtrip failed: %s", e.message);
        assert_not_reached ();
    }
}

public int main (string[] args) {
    Test.init (ref args);
    Test.add_func ("/libapm/options/serialize", test_options_serialize);
    Test.add_func ("/libapm/package/deserialize", test_package_deserialize);
    Test.add_func ("/libapm/config/roundtrip", test_config_roundtrip);
    return Test.run ();
}

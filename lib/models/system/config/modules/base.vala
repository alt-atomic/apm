/*
 * Copyright (C) 2025-2026 Vladimir Romanov <rirusha@altlinux.org>
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
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 * 
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

public class Apm.Models.ConfigModule : Serialize.DataObject, Serialize.TypeFamily {

    [Description (nick = "name")]
    public string name { get; set; }

    [Description (nick = "type")]
    public string type_ { get; set; }

    [Description (nick = "id")]
    public string id { get; set; }

    [Description (nick = "env")]
    public Serialize.Dict<string> env { get; set; default = new Serialize.Dict<string> (); }

    [Description (nick = "if")]
    public string @if { get; set; }

    [Description (nick = "output")]
    public Serialize.Dict<string> output { get; set; default = new Serialize.Dict<string> (); }

    public Type match_type (Json.Node node) {
        if (node.get_node_type () != Json.NodeType.OBJECT || !node.get_object ().has_member ("type")) {
            return typeof (ConfigModule);
        }

        var module_type = node.get_object ().get_string_member ("type");

        switch (module_type) {
            case "branding":
                return typeof (ConfigModuleBranding);
            case "git":
                return typeof (ConfigModuleGit);
            case "kernel":
                return typeof (ConfigModuleKernel);
            case "packages":
                return typeof (ConfigModulePackages);
            case "repos":
                return typeof (ConfigModuleRepos);
            default:
                warning ("Unknown config module type: %s", module_type);
                return typeof (UnknownConfigModule);
        }
    }
}

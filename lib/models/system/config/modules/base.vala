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

public class Apm.ModelsConfigModule : ModelsDataObject {

    public string name { get; set; }

    [Description (nick = "type")]
    public string type_ { get; set; }

    public string id { get; set; }

    public HashTable<string, string> env { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    public string @if { get; set; }

    public HashTable<string, string> output { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    internal static Deser.Codec<ModelsConfigModule> create_codec () throws Deser.Error {
        return new Deser.ObjectCodec<ModelsConfigModule> (
            new Deser.ObjectCodecOptions ().with_resolver (resolve_type)
        );
    }

    static Type resolve_type (
        Type declared_type,
        Deser.ValueView node,
        Deser.DecodeContext context
    ) throws Deser.Error {
        if (node.kind != Deser.ValueKind.MAP) {
            throw new Deser.Error.TYPE_MISMATCH ("%s: expected config module", context.path ());
        }
        var cursor = node.open_map ();
        string key;
        Deser.ValueView value;
        while (cursor.next (out key, out value)) {
            if (key != "type") {
                continue;
            }
            switch (value.get_text ()) {
                case "branding": return typeof (ModelsConfigModuleBranding);
                case "git": return typeof (ModelsConfigModuleGit);
                case "kernel": return typeof (ModelsConfigModuleKernel);
                case "packages": return typeof (ModelsConfigModulePackages);
                case "repos": return typeof (ModelsConfigModuleRepos);
                default: return typeof (ModelsUnknownConfigModule);
            }
        }
        return typeof (ModelsConfigModule);
    }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "env") {
            return Deser.create_codec_for_map<string> ();
        }
        if (property.name == "output") {
            return Deser.create_codec_for_map<string> ();
        }
        return base.get_property_codec (property);
    }
}

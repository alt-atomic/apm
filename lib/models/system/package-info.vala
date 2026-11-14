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

public sealed class Apm.ModelsPackageInfo : ModelsDataObject {

    public string name { get; set; }

    public string architecture { get; set; }

    public string section { get; set; }

    [Description (nick = "installedSize")]
    public int64 installed_size { get; set; }

    public string maintainer { get; set; }

    public string version { get; set; }

    [Description (nick = "versionRaw")]
    public string version_raw { get; set; }

    [Description (nick = "versionInstalled")]
    public string version_installed { get; set; }

    public string[] depends { get; set; }

    public string[] aliases { get; set; }

    public string[] provides { get; set; }

    public int64 size { get; set; }

    public string filename { get; set; }

    public string summary { get; set; }

    public string description { get; set; }

    [Description (nick = "appStream")]
    public GenericArray<ModelsApplicationInfo> app_stream { get; set; default = new GenericArray<ModelsApplicationInfo> (); }

    [Description (nick = "lastChangelog")]
    public string last_changelog { get; set; }

    public bool installed { get; set; }

    [Description (nick = "typePackage")]
    public int type_package { get; set; }

    public string[] files { get; set; }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "app-stream") {
            return Deser.create_codec_for_array<ModelsApplicationInfo> ();
        }
        return base.get_property_codec (property);
    }
}

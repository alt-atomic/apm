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

public sealed class Apm.Models.PackageInfo : Serialize.DataObject {

    [Description (nick = "name")]
    public string name { get; set; }

    [Description (nick = "architecture")]
    public string architecture { get; set; }

    [Description (nick = "section")]
    public string section { get; set; }

    [Description (nick = "installedSize")]
    public int64 installed_size { get; set; }

    [Description (nick = "maintainer")]
    public string maintainer { get; set; }

    [Description (nick = "version")]
    public string version { get; set; }

    [Description (nick = "versionRaw")]
    public string version_raw { get; set; }

    [Description (nick = "versionInstalled")]
    public string version_installed { get; set; }

    [Description (nick = "depends")]
    public string[] depends { get; set; }

    [Description (nick = "aliases")]
    public string[] aliases { get; set; }

    [Description (nick = "provides")]
    public string[] provides { get; set; }

    [Description (nick = "size")]
    public int64 size { get; set; }

    [Description (nick = "filename")]
    public string filename { get; set; }

    [Description (nick = "summary")]
    public string summary { get; set; }

    [Description (nick = "description")]
    public string description { get; set; }

    [Description (nick = "appStream")]
    public Serialize.Array<ApplicationInfo> app_stream { get; set; default = new Serialize.Array<ApplicationInfo> (); }

    [Description (nick = "lastChangelog")]
    public string last_changelog { get; set; }

    [Description (nick = "installed")]
    public bool installed { get; set; }

    [Description (nick = "typePackage")]
    public int type_package { get; set; }

    [Description (nick = "files")]
    public string[] files { get; set; }
}

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

public sealed class Apm.Models.OperationInfo : Serialize.DataObject {

    [Description (nick = "extraInstalled")]
    public string[] extra_installed { get; set; }

    [Description (nick = "upgradedPackages")]
    public string[] upgraded_packages { get; set; }

    [Description (nick = "newInstalledPackages")]
    public string[] new_installed_packages { get; set; }

    [Description (nick = "removedPackages")]
    public string[] removed_packages { get; set; }

    [Description (nick = "keptBackPackages")]
    public string[] kept_back_packages { get; set; }

    [Description (nick = "upgradedCount")]
    public int upgraded_count { get; set; }

    [Description (nick = "newInstalledCount")]
    public int new_installed_count { get; set; }

    [Description (nick = "removedCount")]
    public int removed_count { get; set; }

    [Description (nick = "keptBackCount")]
    public int kept_back_count { get; set; }

    [Description (nick = "notUpgradedCount")]
    public int not_upgraded_count { get; set; }

    [Description (nick = "downloadSize")]
    public uint64 download_size { get; set; }

    [Description (nick = "installSize")]
    public int64 install_size { get; set; }

    [Description (nick = "essentialPackages")]
    public Serialize.Array<CheckEssentialPackageInfo> essential_packages {
        get; set; default = new Serialize.Array<CheckEssentialPackageInfo> ();
    }
}

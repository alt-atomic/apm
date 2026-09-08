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

public class Apm.Models.KernelInfo : Serialize.DataObject {

    [Description (nick = "packageName")]
    public string package_name { get; set; }

    [Description (nick = "flavour")]
    public string flavour { get; set; }

    [Description (nick = "version")]
    public string version { get; set; }

    [Description (nick = "versionInstalled")]
    public string version_installed { get; set; }

    [Description (nick = "release")]
    public string release { get; set; }

    [Description (nick = "buildTime")]
    public string build_time { get; set; }

    [Description (nick = "isInstalled")]
    public bool is_installed { get; set; }

    [Description (nick = "isRunning")]
    public bool is_running { get; set; }

    [Description (nick = "fullVersion")]
    public string full_version { get; set; }

    [Description (nick = "ageInDays")]
    public int64 age_in_days { get; set; }
}

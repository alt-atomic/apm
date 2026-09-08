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

public sealed class Apm.Models.Distrobox.PackageInfo : Serialize.DataObject {

    [Description (nick = "package")]
    public Distrobox.Package package { get; set; }

    [Description (nick = "desktopPaths")]
    public string[] desktop_paths { get; set; }

    [Description (nick = "consolePaths")]
    public string[] console_paths { get; set; }

    [Description (nick = "isConsole")]
    public bool is_console { get; set; }
}

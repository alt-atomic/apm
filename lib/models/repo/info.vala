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

public sealed class Apm.Models.RepoInfo : Serialize.DataObject {

    [Description (nick = "url")]
    public string url { get; set; }

    [Description (nick = "arch")]
    public string arch { get; set; }

    [Description (nick = "components")]
    public string[] components { get; set; }

    [Description (nick = "active")]
    public bool active { get; set; }

    [Description (nick = "file")]
    public string file { get; set; }

    [Description (nick = "entry")]
    public string entry { get; set; }

    [Description (nick = "branch")]
    public string branch { get; set; }
}

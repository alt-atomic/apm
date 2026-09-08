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

public sealed class Apm.Models.ApplicationProvidesInfo : Serialize.DataObject {

    [Description (nick = "binaries")]
    public string[] binaries { get; set; }

    [Description (nick = "fonts")]
    public string[] fonts { get; set; }

    [Description (nick = "modalias")]
    public string[] modalias { get; set; }

    [Description (nick = "mediatypes")]
    public string[] mediatypes { get; set; }

    [Description (nick = "ids")]
    public string[] ids { get; set; }

    [Description (nick = "dbus")]
    public Serialize.Array<ApplicationProvidesDbusInfo> dbus {
        get; set; default = new Serialize.Array<ApplicationProvidesDbusInfo> ();
    }

    [Description (nick = "libraries")]
    public string[] libraries { get; set; }
}

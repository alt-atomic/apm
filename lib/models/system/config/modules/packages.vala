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

public sealed class Apm.Models.ConfigModulePackages : ConfigModule {

    public sealed class Body : Serialize.DataObject {

        [Description (nick = "install")]
        public string[] install { get; set; }

        [Description (nick = "remove")]
        public string[] remove { get; set; }

        [Description (nick = "update")]
        public bool update { get; set; }

        [Description (nick = "upgrade")]
        public bool upgrade { get; set; }

        [Description (nick = "depends")]
        public bool depends { get; set; }

        [Description (nick = "options")]
        public Serialize.Dict<string> options { get; set; default = new Serialize.Dict<string> (); }
    }

    [Description (nick = "body")]
    public Body body { get; set; }
}

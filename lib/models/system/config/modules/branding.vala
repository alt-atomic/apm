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

public sealed class Apm.Models.ConfigModuleBranding : ConfigModule {

    public sealed class Body : Serialize.DataObject {

        [Description (nick = "name")]
        public string name { get; set; }

        [Description (nick = "subpackages")]
        public string[] subpackages { get; set; }

        [Description (nick = "release-overrides")]
        public Serialize.Dict<string> release_overrides { get; set; default = new Serialize.Dict<string> (); }

        [Description (nick = "build-type")]
        public string build_type { get; set; }
    }

    [Description (nick = "body")]
    public Body body { get; set; }
}

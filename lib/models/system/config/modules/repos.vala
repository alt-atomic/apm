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

public sealed class Apm.Models.ConfigModuleRepos : ConfigModule {

    public sealed class Body : Serialize.DataObject {

        [Description (nick = "clean")]
        public bool clean { get; set; }

        [Description (nick = "custom")]
        public string[] custom { get; set; }

        [Description (nick = "branch")]
        public string branch { get; set; }

        [Description (nick = "date")]
        public string date { get; set; }

        [Description (nick = "tasks")]
        public string[] tasks { get; set; }

        [Description (nick = "name")]
        public string name { get; set; }

        [Description (nick = "no-update")]
        public bool no_update { get; set; }

        [Description (nick = "clean-temporary")]
        public bool clean_temporary { get; set; }
    }

    [Description (nick = "body")]
    public Body body { get; set; }
}

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

public sealed class Apm.ModelsConfigModulePackages : ModelsConfigModule {

    public sealed class Body : ModelsDataObject {

        public string[] install { get; set; }

        public string[] remove { get; set; }

        public bool update { get; set; }

        public bool upgrade { get; set; }

        public bool depends { get; set; }

        public HashTable<string, string> options { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

        public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
            if (property.name == "options") {
                return Deser.create_codec_for_map<string> ();
            }
            return base.get_property_codec (property);
        }
    }

    public Body body { get; set; }
}

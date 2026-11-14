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

public sealed class Apm.ModelsApplicationScreenshotInfo : ModelsDataObject {

    [Description (nick = "type")]
    public string type_ { get; set; }

    public string environment { get; set; }

    public HashTable<string, string> caption { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    public GenericArray<ModelsApplicationImageInfo> images {
        get; set; default = new GenericArray<ModelsApplicationImageInfo> ();
    }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "caption") {
            return Deser.create_codec_for_map<string> ();
        }
        if (property.name == "images") {
            return Deser.create_codec_for_array<ModelsApplicationImageInfo> ();
        }
        return base.get_property_codec (property);
    }
}

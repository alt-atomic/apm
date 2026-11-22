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

public sealed class Apm.ModelsConfigModuleBranding : ModelsConfigModule {

    public sealed class Metainfo : ModelsDataObject {

        public HashTable<string, string>? name { get; set; }

        public HashTable<string, string>? summary { get; set; }

        public HashTable<string, string>? description { get; set; }

        public GenericArray<Variant>? variants { get; set; }

        public Release? latest_release { get; set; }

        public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
            if (property.name == "name" || property.name == "summary" || property.name == "description") {
                return Deser.create_codec_for_map<string> ();
            }
            if (property.name == "variants") {
                return Deser.create_codec_for_array<Variant> ();
            }
            return base.get_property_codec (property);
        }
    }

    public sealed class Variant : ModelsDataObject {

        public string image { get; set; }

        public HashTable<string, string>? name { get; set; }

        public HashTable<string, string>? summary { get; set; }

        public HashTable<string, string>? description { get; set; }

        public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
            if (property.name == "name" || property.name == "summary" || property.name == "description") {
                return Deser.create_codec_for_map<string> ();
            }
            return base.get_property_codec (property);
        }
    }

    public sealed class Release : ModelsDataObject {

        public string? version { get; set; }

        public string? date { get; set; }

        public HashTable<string, string>? description { get; set; }

        public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
            if (property.name == "description") {
                return Deser.create_codec_for_map<string> ();
            }
            return base.get_property_codec (property);
        }
    }

    public sealed class Body : ModelsDataObject {

        public string name { get; set; }

        public string[] subpackages { get; set; }

        public HashTable<string, string> release_overrides { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

        public string build_type { get; set; }

        public Metainfo? metainfo { get; set; }

        public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
            if (property.name == "release-overrides") {
                return Deser.create_codec_for_map<string> ();
            }
            return base.get_property_codec (property);
        }
    }

    public Body body { get; set; }
}

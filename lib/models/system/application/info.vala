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

public sealed class Apm.ModelsApplicationInfo : ModelsDataObject {

    [Description (nick = "type")]
    public string type_ { get; set; }

    public string id { get; set; }

    [Description (nick = "metadata_license")]
    public string metadata_license { get; set; }

    [Description (nick = "project_license")]
    public string project_license { get; set; }

    public HashTable<string, string> name { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    public HashTable<string, string> summary { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    public HashTable<string, string> description { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    [Description (nick = "developer_name")]
    public HashTable<string, string> developer_name { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    public ModelsApplicationDeveloperInfo developer { get; set; }

    [Description (nick = "project_group")]
    public string project_group { get; set; }

    public string[] keywords { get; set; }

    public string[] categories { get; set; }

    public HashTable<string, string> urls { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    public GenericArray<ModelsApplicationScreenshotInfo> screenshots {
        get; set; default = new GenericArray<ModelsApplicationScreenshotInfo> ();
    }

    public GenericArray<ModelsApplicationReleaseInfo> releases {
        get; set; default = new GenericArray<ModelsApplicationReleaseInfo> ();
    }

    public GenericArray<ModelsApplicationIconInfo> icons {
        get; set; default = new GenericArray<ModelsApplicationIconInfo> ();
    }

    public HashTable<string, string> launchable { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    [Description (nick = "content_rating")]
    public ModelsApplicationContentRatingInfo content_rating { get; set; }

    public ModelsApplicationProvidesInfo provides { get; set; }

    public string[] extends { get; set; }

    public GenericArray<ModelsApplicationBrandingInfo> branding {
        get; set; default = new GenericArray<ModelsApplicationBrandingInfo> ();
    }

    public GenericArray<ModelsApplicationTranslationInfo> translations {
        get; set; default = new GenericArray<ModelsApplicationTranslationInfo> ();
    }

    public GenericArray<ModelsApplicationLanguageInfo> languages {
        get; set; default = new GenericArray<ModelsApplicationLanguageInfo> ();
    }

    public HashTable<string, string> custom { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    public string[] kudos { get; set; }

    public string[] mimetypes { get; set; }

    public ModelsApplicationPropertyInfo @requires { get; set; }

    public ModelsApplicationPropertyInfo recommends { get; set; }

    public ModelsApplicationPropertyInfo supports { get; set; }

    public string[] suggests { get; set; }

    public string[] replaces { get; set; }

    [Description (nick = "compulsory_for_desktop")]
    public string[] compulsory_for_desktop { get; set; }

    [Description (nick = "update_contact")]
    public string update_contact { get; set; }

    public string pkgname { get; set; }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "name") {
            return Deser.create_codec_for_map<string> ();
        }
        if (property.name == "summary") {
            return Deser.create_codec_for_map<string> ();
        }
        if (property.name == "description") {
            return Deser.create_codec_for_map<string> ();
        }
        if (property.name == "developer-name") {
            return Deser.create_codec_for_map<string> ();
        }
        if (property.name == "urls") {
            return Deser.create_codec_for_map<string> ();
        }
        if (property.name == "screenshots") {
            return Deser.create_codec_for_array<ModelsApplicationScreenshotInfo> ();
        }
        if (property.name == "releases") {
            return Deser.create_codec_for_array<ModelsApplicationReleaseInfo> ();
        }
        if (property.name == "icons") {
            return Deser.create_codec_for_array<ModelsApplicationIconInfo> ();
        }
        if (property.name == "launchable") {
            return Deser.create_codec_for_map<string> ();
        }
        if (property.name == "branding") {
            return Deser.create_codec_for_array<ModelsApplicationBrandingInfo> ();
        }
        if (property.name == "translations") {
            return Deser.create_codec_for_array<ModelsApplicationTranslationInfo> ();
        }
        if (property.name == "languages") {
            return Deser.create_codec_for_array<ModelsApplicationLanguageInfo> ();
        }
        if (property.name == "custom") {
            return Deser.create_codec_for_map<string> ();
        }
        return base.get_property_codec (property);
    }
}

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

public sealed class Apm.Models.ApplicationInfo : Serialize.DataObject {

    [Description (nick = "type")]
    public string type_ { get; set; }

    [Description (nick = "id")]
    public string id { get; set; }

    [Description (nick = "metadata_license")]
    public string metadata_license { get; set; }

    [Description (nick = "project_license")]
    public string project_license { get; set; }

    [Description (nick = "name")]
    public Serialize.Dict<string> name { get; set; default = new Serialize.Dict<string> (); }

    [Description (nick = "summary")]
    public Serialize.Dict<string> summary { get; set; default = new Serialize.Dict<string> (); }

    [Description (nick = "description")]
    public Serialize.Dict<string> description { get; set; default = new Serialize.Dict<string> (); }

    [Description (nick = "developer_name")]
    public Serialize.Dict<string> developer_name { get; set; default = new Serialize.Dict<string> (); }

    [Description (nick = "developer")]
    public ApplicationDeveloperInfo developer { get; set; }

    [Description (nick = "project_group")]
    public string project_group { get; set; }

    [Description (nick = "keywords")]
    public string[] keywords { get; set; }

    [Description (nick = "categories")]
    public string[] categories { get; set; }

    [Description (nick = "urls")]
    public Serialize.Dict<string> urls { get; set; default = new Serialize.Dict<string> (); }

    [Description (nick = "screenshots")]
    public Serialize.Array<ApplicationScreenshotInfo> screenshots {
        get; set; default = new Serialize.Array<ApplicationScreenshotInfo> ();
    }

    [Description (nick = "releases")]
    public Serialize.Array<ApplicationReleaseInfo> releases {
        get; set; default = new Serialize.Array<ApplicationReleaseInfo> ();
    }

    [Description (nick = "icons")]
    public Serialize.Array<ApplicationIconInfo> icons {
        get; set; default = new Serialize.Array<ApplicationIconInfo> ();
    }

    [Description (nick = "launchable")]
    public Serialize.Dict<string> launchable { get; set; default = new Serialize.Dict<string> (); }

    [Description (nick = "content_rating")]
    public ApplicationContentRatingInfo content_rating { get; set; }

    [Description (nick = "provides")]
    public ApplicationProvidesInfo provides { get; set; }

    [Description (nick = "extends")]
    public string[] extends { get; set; }

    [Description (nick = "branding")]
    public Serialize.Array<ApplicationBrandingInfo> branding {
        get; set; default = new Serialize.Array<ApplicationBrandingInfo> ();
    }

    [Description (nick = "translations")]
    public Serialize.Array<ApplicationTranslationInfo> translations {
        get; set; default = new Serialize.Array<ApplicationTranslationInfo> ();
    }

    [Description (nick = "languages")]
    public Serialize.Array<ApplicationLanguageInfo> languages {
        get; set; default = new Serialize.Array<ApplicationLanguageInfo> ();
    }

    [Description (nick = "custom")]
    public Serialize.Dict<string> custom { get; set; default = new Serialize.Dict<string> (); }

    [Description (nick = "kudos")]
    public string[] kudos { get; set; }

    [Description (nick = "mimetypes")]
    public string[] mimetypes { get; set; }

    [Description (nick = "requires")]
    public ApplicationPropertyInfo @requires { get; set; }

    [Description (nick = "recommends")]
    public ApplicationPropertyInfo recommends { get; set; }

    [Description (nick = "supports")]
    public ApplicationPropertyInfo supports { get; set; }

    [Description (nick = "suggests")]
    public string[] suggests { get; set; }

    [Description (nick = "replaces")]
    public string[] replaces { get; set; }

    [Description (nick = "compulsory_for_desktop")]
    public string[] compulsory_for_desktop { get; set; }

    [Description (nick = "update_contact")]
    public string update_contact { get; set; }

    [Description (nick = "pkgname")]
    public string pkgname { get; set; }
}

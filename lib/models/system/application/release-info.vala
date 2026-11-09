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

public sealed class Apm.ModelsApplicationReleaseInfo : ModelsDataObject {

    public string version { get; set; }

    public string date { get; set; }

    [Description (nick = "date_eol")]
    public string date_eol { get; set; }

    public int64 timestamp { get; set; }

    [Description (nick = "type")]
    public string type_ { get; set; }

    public string urgency { get; set; }

    public HashTable<string, string> description { get; set; default = new HashTable<string, string> (str_hash, str_equal); }

    public ModelsApplicationReleaseUrlInfo url { get; set; }

    public GenericArray<ModelsApplicationReleaseIssueInfo> issues {
        get; set; default = new GenericArray<ModelsApplicationReleaseIssueInfo> ();
    }

    public GenericArray<ModelsApplicationReleaseArtifactInfo> artifacts {
        get; set; default = new GenericArray<ModelsApplicationReleaseArtifactInfo> ();
    }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "description") {
            return Deser.create_codec_for_map<string> ();
        }
        if (property.name == "issues") {
            return Deser.create_codec_for_array<ModelsApplicationReleaseIssueInfo> ();
        }
        if (property.name == "artifacts") {
            return Deser.create_codec_for_array<ModelsApplicationReleaseArtifactInfo> ();
        }
        return base.get_property_codec (property);
    }
}

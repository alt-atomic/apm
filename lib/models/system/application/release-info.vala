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

public sealed class Apm.Models.ApplicationReleaseInfo : Serialize.DataObject {

    [Description (nick = "version")]
    public string version { get; set; }

    [Description (nick = "date")]
    public string date { get; set; }

    [Description (nick = "date_eol")]
    public string date_eol { get; set; }

    [Description (nick = "timestamp")]
    public int64 timestamp { get; set; }

    [Description (nick = "type")]
    public string type_ { get; set; }

    [Description (nick = "urgency")]
    public string urgency { get; set; }

    [Description (nick = "description")]
    public Serialize.Dict<string> description { get; set; default = new Serialize.Dict<string> (); }

    [Description (nick = "url")]
    public ApplicationReleaseUrlInfo url { get; set; }

    [Description (nick = "issues")]
    public Serialize.Array<ApplicationReleaseIssueInfo> issues {
        get; set; default = new Serialize.Array<ApplicationReleaseIssueInfo> ();
    }

    [Description (nick = "artifacts")]
    public Serialize.Array<ApplicationReleaseArtifactInfo> artifacts {
        get; set; default = new Serialize.Array<ApplicationReleaseArtifactInfo> ();
    }
}

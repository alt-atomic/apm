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

public sealed class Apm.Models.ApplicationReleaseArtifactInfo : Serialize.DataObject {

    [Description (nick = "type")]
    public string type_ { get; set; }

    [Description (nick = "platform")]
    public string platform { get; set; }

    [Description (nick = "locations")]
    public string[] locations { get; set; }

    [Description (nick = "checksums")]
    public Serialize.Array<ApplicationReleaseArtifactChecksumInfo> checksums {
        get; set; default = new Serialize.Array<ApplicationReleaseArtifactChecksumInfo> ();
    }

    [Description (nick = "sizes")]
    public Serialize.Array<ApplicationReleaseArtifactSizeInfo> sizes {
        get; set; default = new Serialize.Array<ApplicationReleaseArtifactSizeInfo> ();
    }

    [Description (nick = "filename")]
    public string filename { get; set; }
}

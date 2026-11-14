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

public sealed class Apm.ModelsApplicationReleaseArtifactInfo : ModelsDataObject {

    [Description (nick = "type")]
    public string type_ { get; set; }

    public string platform { get; set; }

    public string[] locations { get; set; }

    public GenericArray<ModelsApplicationReleaseArtifactChecksumInfo> checksums {
        get; set; default = new GenericArray<ModelsApplicationReleaseArtifactChecksumInfo> ();
    }

    public GenericArray<ModelsApplicationReleaseArtifactSizeInfo> sizes {
        get; set; default = new GenericArray<ModelsApplicationReleaseArtifactSizeInfo> ();
    }

    public string filename { get; set; }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "checksums") {
            return Deser.create_codec_for_array<ModelsApplicationReleaseArtifactChecksumInfo> ();
        }
        if (property.name == "sizes") {
            return Deser.create_codec_for_array<ModelsApplicationReleaseArtifactSizeInfo> ();
        }
        return base.get_property_codec (property);
    }
}

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

public sealed class Apm.ModelsCleanOldKernelsResponse : ModelsDataObject {

    public string message { get; set; }

    [Description (nick = "removeKernels")]
    public GenericArray<ModelsKernelInfo> remove_kernels {
        get; set; default = new GenericArray<ModelsKernelInfo> ();
    }

    [Description (nick = "keptKernels")]
    public GenericArray<ModelsKernelWReasonsInfo> kept_kernels {
        get; set; default = new GenericArray<ModelsKernelWReasonsInfo> ();
    }

    public ModelsOperationInfo preview { get; set; }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "remove-kernels") {
            return Deser.create_codec_for_array<ModelsKernelInfo> ();
        }
        if (property.name == "kept-kernels") {
            return Deser.create_codec_for_array<ModelsKernelWReasonsInfo> ();
        }
        return base.get_property_codec (property);
    }
}

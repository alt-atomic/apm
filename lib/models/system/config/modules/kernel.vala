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

public sealed class Apm.Models.ConfigModuleKernel : ConfigModule {

    public sealed class KernelInfo : Serialize.DataObject {

        [Description (nick = "flavour")]
        public string flavour { get; set; }

        [Description (nick = "modules")]
        public string[] modules { get; set; }

        [Description (nick = "no-inherit-modules")]
        public bool no_inherit_modules { get; set; }

        [Description (nick = "include-headers")]
        public bool include_headers { get; set; }
    }

    public sealed class Initrd : Serialize.DataObject {

        [Description (nick = "method")]
        public string method { get; set; }

        [Description (nick = "plymouth-theme")]
        public string plymouth_theme { get; set; }
    }

    public sealed class Body : Serialize.DataObject {

        [Description (nick = "kernel-info")]
        public KernelInfo kernel_info { get; set; }

        [Description (nick = "initrd")]
        public Initrd initrd { get; set; }
    }

    [Description (nick = "body")]
    public Body body { get; set; }
}

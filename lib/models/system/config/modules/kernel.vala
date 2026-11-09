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

public sealed class Apm.ModelsConfigModuleKernel : ModelsConfigModule {

    public sealed class KernelInfo : ModelsDataObject {

        public string flavour { get; set; }

        public string[] modules { get; set; }

        public bool no_inherit_modules { get; set; }

        public bool include_headers { get; set; }
    }

    public sealed class Initrd : ModelsDataObject {

        public string method { get; set; }

        public string plymouth_theme { get; set; }
    }

    public sealed class Body : ModelsDataObject {

        public KernelInfo kernel_info { get; set; }

        public Initrd initrd { get; set; }
    }

    public Body body { get; set; }
}

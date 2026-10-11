/*
 * Copyright (C) 2026 Vladimir Romanov <rirusha@altlinux.org>
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
 * along with this program. If not, see
 * <https://www.gnu.org/licenses/gpl-3.0-standalone.html>.
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */
/**
 * Client for kernel and kernel module management.
 *
 * Operations backed by APM2 jobs return only after completion. Kernel jobs
 * cannot be cancelled by the server, so these methods do not accept a
 * {@link Cancellable}.
 */
public sealed class Apm.Kernel : Module {

    internal Kernel (Transport transport) {
        base (transport, "Kernel");
    }

    public async ModelsKernelListResponse list_kernels (
        string flavour,
        bool installed_only
    ) throws GLib.Error {
        return yield request<ModelsKernelListResponse> ("ListKernels", new Variant ("(sb)", flavour, installed_only));
    }

    public async ModelsKernelCurrentResponse current () throws GLib.Error {
        return yield request<ModelsKernelCurrentResponse> ("Current", null);
    }

    public async ModelsInstallUpdateKernelResponse install (
        string flavour,
        string[] modules,
        ModelsKernelOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<ModelsInstallUpdateKernelResponse> (
            "Install",
            new Variant ("(s^ass)", flavour, modules, options_json),
            null,
            progress
        );
    }

    public async ModelsInstallUpdateKernelResponse update (
        string flavour,
        string[] modules,
        ModelsKernelOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<ModelsInstallUpdateKernelResponse> (
            "Update",
            new Variant ("(s^ass)", flavour, modules, options_json),
            null,
            progress
        );
    }

    public async ModelsInstallUpdateKernelResponse check_install (
        string flavour,
        string[] modules,
        ModelsKernelOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<ModelsInstallUpdateKernelResponse> (
            "CheckInstall",
            new Variant ("(s^ass)", flavour, modules, options_json),
            null,
            progress
        );
    }

    public async ModelsInstallUpdateKernelResponse check_update (
        string flavour,
        string[] modules,
        ModelsKernelOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<ModelsInstallUpdateKernelResponse> (
            "CheckUpdate",
            new Variant ("(s^ass)", flavour, modules, options_json),
            null,
            progress
        );
    }

    public async ModelsCleanOldKernelsResponse clean_old (
        ModelsKernelCleanOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<ModelsCleanOldKernelsResponse> (
            "CleanOld",
            new Variant ("(s)", options_json),
            null,
            progress
        );
    }

    public async ModelsCleanOldKernelsResponse check_clean_old (
        ModelsKernelCleanOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<ModelsCleanOldKernelsResponse> (
            "CheckCleanOld",
            new Variant ("(s)", options_json),
            null,
            progress
        );
    }

    public async ModelsListKernelModulesResponse list_modules (string flavour) throws GLib.Error {
        return yield request<ModelsListKernelModulesResponse> ("ListModules", new Variant ("(s)", flavour));
    }

    public async ModelsInstallKernelModulesResponse install_modules (
        string flavour,
        string[] modules,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<ModelsInstallKernelModulesResponse> (
            "InstallModules",
            new Variant ("(s^as)", flavour, modules),
            null,
            progress
        );
    }

    public async ModelsInstallKernelModulesResponse check_install_modules (
        string flavour,
        string[] modules,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<ModelsInstallKernelModulesResponse> (
            "CheckInstallModules",
            new Variant ("(s^as)", flavour, modules),
            null,
            progress
        );
    }

    public async ModelsRemoveKernelModulesResponse remove_modules (
        string flavour,
        string[] modules,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<ModelsRemoveKernelModulesResponse> (
            "RemoveModules",
            new Variant ("(s^as)", flavour, modules),
            null,
            progress
        );
    }

    public async ModelsRemoveKernelModulesResponse check_remove_modules (
        string flavour,
        string[] modules,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<ModelsRemoveKernelModulesResponse> (
            "CheckRemoveModules",
            new Variant ("(s^as)", flavour, modules),
            null,
            progress
        );
    }
}

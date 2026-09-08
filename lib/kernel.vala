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

    public async Models.KernelListResponse list_kernels (
        string flavour,
        bool installed_only
    ) throws GLib.Error {
        return yield request<Models.KernelListResponse> ("ListKernels", new Variant ("(sb)", flavour, installed_only));
    }

    public async Models.KernelCurrentResponse current () throws GLib.Error {
        return yield request<Models.KernelCurrentResponse> ("Current", null);
    }

    public async Models.InstallUpdateKernelResponse install (
        string flavour,
        string[] modules,
        Models.KernelOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.InstallUpdateKernelResponse> (
            "Install",
            new Variant ("(s^ass)", flavour, modules, options_json),
            null,
            progress
        );
    }

    public async Models.InstallUpdateKernelResponse update (
        string flavour,
        string[] modules,
        Models.KernelOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.InstallUpdateKernelResponse> (
            "Update",
            new Variant ("(s^ass)", flavour, modules, options_json),
            null,
            progress
        );
    }

    public async Models.InstallUpdateKernelResponse check_install (
        string flavour,
        string[] modules,
        Models.KernelOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.InstallUpdateKernelResponse> (
            "CheckInstall",
            new Variant ("(s^ass)", flavour, modules, options_json),
            null,
            progress
        );
    }

    public async Models.InstallUpdateKernelResponse check_update (
        string flavour,
        string[] modules,
        Models.KernelOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.InstallUpdateKernelResponse> (
            "CheckUpdate",
            new Variant ("(s^ass)", flavour, modules, options_json),
            null,
            progress
        );
    }

    public async Models.CleanOldKernelsResponse clean_old (
        Models.KernelCleanOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.CleanOldKernelsResponse> (
            "CleanOld",
            new Variant ("(s)", options_json),
            null,
            progress
        );
    }

    public async Models.CleanOldKernelsResponse check_clean_old (
        Models.KernelCleanOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.CleanOldKernelsResponse> (
            "CheckCleanOld",
            new Variant ("(s)", options_json),
            null,
            progress
        );
    }

    public async Models.ListKernelModulesResponse list_modules (string flavour) throws GLib.Error {
        return yield request<Models.ListKernelModulesResponse> ("ListModules", new Variant ("(s)", flavour));
    }

    public async Models.InstallKernelModulesResponse install_modules (
        string flavour,
        string[] modules,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.InstallKernelModulesResponse> (
            "InstallModules",
            new Variant ("(s^as)", flavour, modules),
            null,
            progress
        );
    }

    public async Models.InstallKernelModulesResponse check_install_modules (
        string flavour,
        string[] modules,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.InstallKernelModulesResponse> (
            "CheckInstallModules",
            new Variant ("(s^as)", flavour, modules),
            null,
            progress
        );
    }

    public async Models.RemoveKernelModulesResponse remove_modules (
        string flavour,
        string[] modules,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.RemoveKernelModulesResponse> (
            "RemoveModules",
            new Variant ("(s^as)", flavour, modules),
            null,
            progress
        );
    }

    public async Models.RemoveKernelModulesResponse check_remove_modules (
        string flavour,
        string[] modules,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.RemoveKernelModulesResponse> (
            "CheckRemoveModules",
            new Variant ("(s^as)", flavour, modules),
            null,
            progress
        );
    }
}

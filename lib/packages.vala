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
 * Client for package management and package queries.
 *
 * Operations backed by APM2 jobs return only after the job reaches a terminal
 * state. Package jobs cannot be cancelled by the server, so these methods do
 * not accept a {@link Cancellable}.
 */
public sealed class Apm.Packages : Module {

    internal Packages (Transport transport) {
        base (transport, "Packages");
    }

    public async Models.InstallRemoveResponse install (
        string[] packages,
        Models.InstallOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.InstallRemoveResponse> (
            "Install",
            new Variant ("(^ass)", packages, options_json),
            null,
            progress
        );
    }

    public async Models.InstallRemoveResponse remove (
        string[] packages,
        Models.RemoveOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.InstallRemoveResponse> (
            "Remove",
            new Variant ("(^ass)", packages, options_json),
            null,
            progress
        );
    }

    public async Models.InstallRemoveResponse reinstall (
        string[] packages,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.InstallRemoveResponse> ("Reinstall", new Variant ("(^as)", packages), null, progress);
    }

    public async Models.UpgradeResponse upgrade (
        Models.UpgradeOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.UpgradeResponse> ("Upgrade", new Variant ("(s)", options_json), null, progress);
    }

    public async Models.UpdateResponse update (
        Models.UpdateOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.UpdateResponse> ("Update", new Variant ("(s)", options_json), null, progress);
    }

    public async Models.CheckResponse check_install (
        string[] packages,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.CheckResponse> ("CheckInstall", new Variant ("(^as)", packages), null, progress);
    }

    public async Models.CheckResponse check_remove (
        string[] packages,
        Models.RemoveOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.CheckResponse> (
            "CheckRemove",
            new Variant ("(^ass)", packages, options_json),
            null,
            progress
        );
    }

    public async Models.CheckResponse check_upgrade (
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.CheckResponse> ("CheckUpgrade", null, null, progress);
    }

    public async Models.ListResponse list (
        Models.ListRequest? request = null
    ) throws GLib.Error {
        var request_json = yield Codec.encode (request);
        return yield this.request<Models.ListResponse> ("List", new Variant ("(s)", request_json));
    }

    public async Models.PackageResponse info (string name) throws GLib.Error {
        return yield request<Models.PackageResponse> ("Info", new Variant ("(s)", name));
    }

    public async Models.MultiInfoResponse multi_info (string[] names) throws GLib.Error {
        return yield request<Models.MultiInfoResponse> ("MultiInfo", new Variant ("(^as)", names));
    }

    public async Models.SearchResponse search (string text, bool installed) throws GLib.Error {
        return yield request<Models.SearchResponse> ("Search", new Variant ("(sb)", text, installed));
    }

    public async string[] sections () throws GLib.Error {
        var reply = yield transport.call (
            Transport.PREFIX + iface,
            "Sections",
            null,
            "(as)"
        );
        return reply.get_child_value (0).dup_strv ();
    }

    public async Serialize.Array<Models.FilterFieldsInfo> filter_fields () throws GLib.Error {
        return yield request_array<Models.FilterFieldsInfo> ("FilterFields", null);
    }

    public async Models.AptConfigResponse apt_config () throws GLib.Error {
        return yield request<Models.AptConfigResponse> ("AptConfig", null);
    }

    public async void set_apt_config (Serialize.Dict<string>? options = null) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        yield transport.call (
            Transport.PREFIX + iface,
            "SetAptConfig",
            new Variant ("(s)", options_json),
            "()"
        );
    }

    public async string get_version () throws GLib.Error {
        var reply = yield transport.call (
            "org.freedesktop.DBus.Properties",
            "Get",
            new Variant ("(ss)", Transport.PREFIX + iface, "Version"),
            "(v)"
        );
        return reply.get_child_value (0).get_variant ().dup_string ();
    }

    public async bool get_is_atomic () throws GLib.Error {
        var reply = yield transport.call (
            "org.freedesktop.DBus.Properties",
            "Get",
            new Variant ("(ss)", Transport.PREFIX + iface, "IsAtomic"),
            "(v)"
        );
        return reply.get_child_value (0).get_variant ().get_boolean ();
    }

    public async bool get_kernel_supported () throws GLib.Error {
        var reply = yield transport.call (
            "org.freedesktop.DBus.Properties",
            "Get",
            new Variant ("(ss)", Transport.PREFIX + iface, "KernelSupported"),
            "(v)"
        );
        return reply.get_child_value (0).get_variant ().get_boolean ();
    }
}

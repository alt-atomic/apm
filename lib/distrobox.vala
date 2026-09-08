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
 * Client for the session-side Distrobox service.
 *
 * Container creation, removal and update support server-side cancellation.
 * Package install and removal wait for non-cancellable APM2 jobs.
 */
public sealed class Apm.Distrobox : Module {

    internal Distrobox (Transport transport) {
        base (transport, "Distrobox");
    }

    public async Models.Distrobox.ContainerListResponse container_list () throws GLib.Error {
        return yield request<Models.Distrobox.ContainerListResponse> ("ContainerList", null);
    }

    public async Models.Distrobox.ContainerResponse container_add (
        string image,
        string name,
        Models.ContainerOptions? options = null,
        Cancellable? cancellable = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.Distrobox.ContainerResponse> (
            "ContainerAdd",
            new Variant ("(sss)", image, name, options_json),
            cancellable,
            progress
        );
    }

    public async Models.Distrobox.ContainerResponse container_remove (
        string name,
        Cancellable? cancellable = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.Distrobox.ContainerResponse> (
            "ContainerRemove",
            new Variant ("(s)", name),
            cancellable,
            progress
        );
    }

    public async Models.Distrobox.UpdateResponse update (
        string container,
        Cancellable? cancellable = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.Distrobox.UpdateResponse> (
            "Update",
            new Variant ("(s)", container),
            cancellable,
            progress
        );
    }

    public async Models.Distrobox.PackageResponse install (
        string container,
        string name,
        Models.DistroboxInstallOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.Distrobox.PackageResponse> (
            "Install",
            new Variant ("(sss)", container, name, options_json),
            null,
            progress
        );
    }

    public async Models.Distrobox.PackageResponse remove (
        string container,
        string name,
        Models.DistroboxRemoveOptions? options = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<Models.Distrobox.PackageResponse> (
            "Remove",
            new Variant ("(sss)", container, name, options_json),
            null,
            progress
        );
    }

    public async Models.Distrobox.PackageResponse info (
        string container,
        string name
    ) throws GLib.Error {
        return yield request<Models.Distrobox.PackageResponse> ("Info", new Variant ("(ss)", container, name));
    }

    public async Models.Distrobox.SearchResponse search (
        string container,
        string text
    ) throws GLib.Error {
        return yield request<Models.Distrobox.SearchResponse> ("Search", new Variant ("(ss)", container, text));
    }

    public async Models.Distrobox.ListResponse list (
        string container,
        Models.ListRequest? request = null
    ) throws GLib.Error {
        var request_json = yield Codec.encode (request);
        return yield this.request<Models.Distrobox.ListResponse> (
            "List",
            new Variant ("(ss)", container, request_json)
        );
    }

    public async Serialize.Array<Models.FilterFieldsInfo> filter_fields () throws GLib.Error {
        return yield request_array<Models.FilterFieldsInfo> ("FilterFields", null);
    }
}

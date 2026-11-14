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
 * Client for atomic image status, configuration and updates.
 *
 * Update, apply and switch calls wait for their APM2 jobs and support
 * server-side cancellation.
 */
public sealed class Apm.Image : Module {

    internal Image (Transport transport) {
        base (transport, "Image");
    }

    public async ModelsImageApplyResponse status () throws GLib.Error {
        return yield request<ModelsImageApplyResponse> ("Status", null);
    }

    public async ModelsImageApplyResponse update (
        ModelsImageUpdateOptions? options = null,
        Cancellable? cancellable = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<ModelsImageApplyResponse> (
            "Update",
            new Variant ("(s)", options_json),
            cancellable,
            progress
        );
    }

    public async ModelsImageApplyResponse apply (
        ModelsImageApplyOptions? options = null,
        Cancellable? cancellable = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<ModelsImageApplyResponse> ("Apply", new Variant ("(s)", options_json), cancellable, progress);
    }

    public async ModelsImageApplyResponse switch (
        string image,
        ModelsImageSwitchOptions? options = null,
        Cancellable? cancellable = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        var options_json = yield Codec.encode (options);
        return yield run<ModelsImageApplyResponse> (
            "Switch",
            new Variant ("(ss)", image, options_json),
            cancellable,
            progress
        );
    }

    public async ModelsImageHistoryResponse history (
        string image,
        ModelsPageRequest? request = null
    ) throws GLib.Error {
        var request_json = yield Codec.encode (request);
        return yield this.request<ModelsImageHistoryResponse> ("History", new Variant ("(ss)", image, request_json));
    }

    public async ModelsImageConfigResponse get_config () throws GLib.Error {
        return yield request<ModelsImageConfigResponse> ("GetConfig", null);
    }

    public async void save_config (ModelsConfig? config = null) throws GLib.Error {
        var config_json = yield Codec.encode (config);
        yield transport.call (
            Transport.PREFIX + iface,
            "SaveConfig",
            new Variant ("(s)", config_json),
            "()"
        );
    }

    public async ModelsImageSyncGroupsResponse sync_groups () throws GLib.Error {
        return yield request<ModelsImageSyncGroupsResponse> ("SyncGroups", null);
    }

    public async ModelsImageFixNssResponse fix_nss () throws GLib.Error {
        return yield request<ModelsImageFixNssResponse> ("FixNss", null);
    }
}

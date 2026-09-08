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
 * Client for the AppStream application catalogue.
 *
 * {@link update} waits for its APM2 job and supports server-side cancellation.
 */
public sealed class Apm.Applications : Module {

    internal Applications (Transport transport) {
        base (transport, "Applications");
    }

    public async Models.ApplicationUpdateResponse update (
        Cancellable? cancellable = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.ApplicationUpdateResponse> ("Update", null, cancellable, progress);
    }

    public async Models.ApplicationInfoResponse info (string name) throws GLib.Error {
        return yield request<Models.ApplicationInfoResponse> ("Info", new Variant ("(s)", name));
    }

    public async Models.ApplicationListResponse list (
        Models.ListRequest? request = null
    ) throws GLib.Error {
        var request_json = yield Codec.encode (request);
        return yield this.request<Models.ApplicationListResponse> ("List", new Variant ("(s)", request_json));
    }

    public async string[] categories () throws GLib.Error {
        var reply = yield transport.call (
            Transport.PREFIX + iface,
            "Categories",
            null,
            "(as)"
        );
        return reply.get_child_value (0).dup_strv ();
    }

    public async Serialize.Array<Models.FilterFieldsInfo> filter_fields () throws GLib.Error {
        return yield request_array<Models.FilterFieldsInfo> ("FilterFields", null);
    }
}

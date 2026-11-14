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

namespace Apm.Codec {

    internal async T decode<T> (string json) throws GLib.Error {
        var reader = new Deser.JsonWorker (json, new Deser.Settings ());
        return yield reader.deserialize_async (new Deser.ObjectCodec<T> ());
    }

    internal async GenericArray<T> decode_array<T> (string json) throws GLib.Error {
        var reader = new Deser.JsonWorker (json, new Deser.Settings ());
        return yield reader.deserialize_async (Deser.create_codec_for_array<T> ());
    }

    internal async string encode (Object? value) throws GLib.Error {
        if (value == null) {
            return "{}";
        }
        var context = MainContext.ref_thread_default ();
        GLib.Error? failure = null;
        var worker = new Thread<string?> ("apm-json", () => {
            string? result = null;
            try {
                result = Deser.JsonWorker.serialize (
                    value,
                    new Deser.ObjectCodec<Object>.from_gtype (value.get_type ()),
                    new Deser.Settings ()
                );
            } catch (GLib.Error e) {
                failure = e;
            }
            var source = new IdleSource ();
            source.set_callback (encode.callback);
            source.attach (context);
            return result;
        });
        yield;
        var result = worker.join ();
        if (failure != null) {
            throw failure;
        }
        return result;
    }
}

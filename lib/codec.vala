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

    // Explicit wire nicks allow the mixed naming conventions in APM payloads.
    internal Serialize.Settings settings () {
        return new Serialize.Settings () {
            names_case = Serialize.Case.KEBAB,
            ignore_default = true
        };
    }

    internal async Object decode<T> (string json, bool array = false) throws GLib.Error {
        var context = MainContext.ref_thread_default ();
        GLib.Error? failure = null;
        var worker = new Thread<Object?> ("apm-json", () => {
            Object? result = null;
            try {
                var reader = new Serialize.JsonWorker (json, null, settings ());
                if (array) {
                    var items = new Serialize.Array<T> ();
                    reader.deserialize_array_into (items);
                    result = items;
                } else {
                    result = reader.deserialize_object_by_type (typeof (T));
                }
            } catch (GLib.Error e) {
                failure = e;
            }
            var source = new IdleSource ();
            source.set_callback (decode.callback);
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

    internal async string encode (Object? value) {
        if (value == null) {
            return "{}";
        }

        var context = MainContext.ref_thread_default ();
        var worker = new Thread<string> ("apm-json", () => {
            var result = Serialize.JsonWorker.serialize (value, settings ());
            var source = new IdleSource ();
            source.set_callback (encode.callback);
            source.attach (context);
            return result;
        });
        yield;
        return worker.join ();
    }
}

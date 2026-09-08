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
 * Errors reported by APM for immediate calls and completed operations.
 */
public errordomain Apm.Error {

    DATABASE,
    REPOSITORY,
    APT,
    VALIDATION,
    PERMISSION,
    IMAGE,
    KERNEL,
    CONTAINER,
    NO_OPERATION,
    NOT_FOUND,
    FAILED,
    DISCONNECTED,
    PROTOCOL;
}

/**
 * Progress of the current operation without exposing a server job identifier.
 */
public sealed class Apm.Progress : Object {

    public string event { get; internal set; }
    public string kind { get; internal set; }
    public string state { get; internal set; }
    public string message { get; internal set; }
    public double percent { get; internal set; }
    public string done { get; internal set; }
}

public delegate void Apm.ProgressCallback (Progress progress);

internal sealed class Apm.Transport : Object {

    internal const string BUS_NAME = "org.altlinux.APM";
    internal const string PATH = "/org/altlinux/APM2";
    internal const string PREFIX = "org.altlinux.APM2.";
    internal const string ERROR_PREFIX = "org.altlinux.APM.Error.";
    internal const string JOBS = "org.altlinux.APM2.Jobs";
    DBusConnection connection;

    internal bool interactive;

    internal Transport (DBusConnection connection, bool interactive) {
        this.connection = connection;
        this.interactive = interactive;
    }

    internal static GLib.Error remote_error (string kind, string message) {
        switch (kind) {
            case "DATABASE":
            case "Database":
                return new Error.DATABASE ("%s", message);

            case "REPOSITORY":
            case "Repository":
                return new Error.REPOSITORY ("%s", message);

            case "APT":
            case "Apt":
                return new Error.APT ("%s", message);

            case "VALIDATION":
            case "Validation":
                return new Error.VALIDATION ("%s", message);

            case "PERMISSION":
            case "Permission":
            case "AccessDenied":
                return new Error.PERMISSION ("%s", message);

            case "CANCELED":
            case "Canceled":
                return new IOError.CANCELLED ("%s", message);

            case "IMAGE":
            case "Image":
                return new Error.IMAGE ("%s", message);

            case "KERNEL":
            case "Kernel":
                return new Error.KERNEL ("%s", message);

            case "CONTAINER":
            case "Container":
                return new Error.CONTAINER ("%s", message);

            case "NO_OPERATION":
            case "NoOperation":
                return new Error.NO_OPERATION ("%s", message);

            case "NOT_FOUND":
            case "NotFound":
                return new Error.NOT_FOUND ("%s", message);

            default:
                return new Error.FAILED ("%s: %s", kind, message);
        }
    }

    internal async Variant call (
        string iface,
        string method,
        Variant? args,
        string reply = "(s)",
        string destination = BUS_NAME,
        Cancellable? cancellable = null
    ) throws GLib.Error {
        try {
            return yield connection.call (
                destination,
                PATH,
                iface,
                method,
                args,
                new VariantType (reply),
                interactive ? DBusCallFlags.ALLOW_INTERACTIVE_AUTHORIZATION : DBusCallFlags.NONE,
                -1,
                cancellable
            );
        } catch (GLib.Error e) {
            var remote = DBusError.get_remote_error (e);

            if (remote != null && (remote.has_prefix (ERROR_PREFIX) || remote.has_suffix (".AccessDenied"))) {
                DBusError.strip_remote_error (e);
                throw remote_error (remote.substring (remote.last_index_of (".") + 1), e.message);
            }
            throw e;
        }
    }

    internal async string request (string iface, string method, Variant? args) throws GLib.Error {
        var result = yield call (PREFIX + iface, method, args);
        return result.get_child_value (0).dup_string ();
    }

    // Resolve/activate once and pin every operation to this unique owner. Never
    // retry a mutating method after an uncertain reply or a service restart.
    internal async DBusProxy jobs_proxy () throws GLib.Error {
        var proxy = yield new DBusProxy (
            connection,
            DBusProxyFlags.DO_NOT_LOAD_PROPERTIES,
            null,
            BUS_NAME,
            PATH,
            JOBS
        );

        if (proxy.get_name_owner () == null) {
            throw new Error.DISCONNECTED ("APM2 is unavailable");
        }

        return proxy;
    }

    internal async string run (
        string iface,
        string method,
        Variant? args,
        Cancellable? cancellable,
        ProgressCallback? progress
    ) throws GLib.Error {
        if (cancellable != null) {
            cancellable.set_error_if_cancelled ();
        }

        var proxy = yield jobs_proxy ();

        if (cancellable != null) {
            cancellable.set_error_if_cancelled ();
        }

        var owner = proxy.get_name_owner ();
        var pending = new Pending ();

        ulong signals = proxy.g_signal.connect ((sender, member, parameters) => {
            if (sender != owner) {
                return;
            }

            var id = parameters.get_child_value (0).get_string ();

            if (member == "JobFinished") {
                if (pending.id == null || pending.id == id) {
                    pending.finished.insert (id, parameters);
                    pending.wake ();
                }
            } else if (member == "JobProgress" && pending.id == id && progress != null) {
                progress (new Progress () {
                    event = parameters.get_child_value (1).dup_string (),
                    kind = parameters.get_child_value (2).dup_string (),
                    state = parameters.get_child_value (3).dup_string (),
                    message = parameters.get_child_value (4).dup_string (),
                    percent = parameters.get_child_value (5).get_double (),
                    done = parameters.get_child_value (6).dup_string ()
                });
            }
        });

        ulong changed = proxy.notify["g-name-owner"].connect (() => {
            if (proxy.get_name_owner () != owner) {
                pending.disconnected = true;
                pending.wake ();
            }
        });
        // Cancellation of the initiating DBus call would lose the task ID.
        // Accept cancellation only after obtaining it, then await server terminal state.
        try {
            var response = yield call (PREFIX + iface, method, args, "(s)", owner);
            pending.id = response.get_child_value (0).dup_string ();
            bool cancel_sent = false;

            while (true) {
                if (pending.disconnected || connection.is_closed ()) {
                    throw new Error.DISCONNECTED ("APM2 disconnected during %s.%s", iface, method);
                }

                var final_signal = pending.finished.lookup (pending.id);

                if (final_signal != null) {
                    return finish (
                        final_signal.get_child_value (1).get_string (),
                        final_signal.get_child_value (2).get_string (),
                        final_signal.get_child_value (3).get_string (),
                        final_signal.get_child_value (4).get_string ()
                    );
                }

                var snapshot = yield call (JOBS, "Get", new Variant ("(s)", pending.id), "(s)", owner);
                var parser = new Json.Parser ();
                parser.load_from_data (snapshot.get_child_value (0).get_string ());
                var root = parser.get_root ();

                if (root.get_node_type () != Json.NodeType.OBJECT) {
                    throw new Error.PROTOCOL ("Invalid operation state");
                }

                var state = root.get_object ();

                if (!state.has_member ("state")) {
                    throw new Error.PROTOCOL ("Missing operation state");
                }

                var status = state.get_string_member ("state");

                if (status != "running") {
                    return finish (
                        status,
                        state.has_member ("errorType") ? state.get_string_member ("errorType") : "",
                        state.has_member ("message") ? state.get_string_member ("message") : "",
                        state.has_member ("result") ? Json.to_string (state.get_member ("result"), false) : "{}"
                    );
                }

                if (!cancel_sent && cancellable != null && cancellable.is_cancelled ()) {
                    cancel_sent = true;

                    try {
                        yield call (JOBS, "Cancel", new Variant ("(s)", pending.id), "()", owner);
                    } catch (Error.VALIDATION e) {
                        // Completion may race with Cancel. Re-read the state, but
                        // do not silently ignore a server refusal for a running task.
                        var check = yield call (JOBS, "Get", new Variant ("(s)", pending.id), "(s)", owner);
                        parser.load_from_data (check.get_child_value (0).get_string ());

                        if (parser.get_root ().get_object ().get_string_member ("state") == "running") {
                            throw e;
                        }
                    }

                    continue;
                }

                // Signals wake immediately; periodic snapshots also recover missed
                // completion signals and observe Cancellable from any thread.
                yield pending.pause ();
            }
        } finally {
            proxy.disconnect (signals);
            proxy.disconnect (changed);
        }
    }

    static string finish (string status, string kind, string message, string result) throws GLib.Error {
        switch (status) {
            case "ok":
                return result;

            case "canceled":
                throw new IOError.CANCELLED ("%s", message);

            case "error":
                throw remote_error (kind, message);

            default:
                throw new Error.PROTOCOL ("Unknown operation state: %s", status);
        }
    }
}

internal sealed class Apm.Pending : Object {

    internal string? id;
    internal bool disconnected;
    internal HashTable<string, Variant> finished = new HashTable<string, Variant> (str_hash, str_equal);

    Source? timer;

    internal void wake () {
        if (timer != null) {
            timer.set_ready_time (0);
        }
    }

    internal async void pause () {
        timer = new TimeoutSource (200);
        timer.set_callback (pause.callback);
        timer.attach (MainContext.get_thread_default ());
        yield;
        timer.destroy ();
        timer = null;
    }
}

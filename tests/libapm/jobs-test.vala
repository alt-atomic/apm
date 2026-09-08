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

private const string BUS_NAME = "org.altlinux.APM";
private const string PATH = "/org/altlinux/APM2";

private class FakeState : Object {
    internal FakeJobs jobs;
    internal HashTable<string, string> states = new HashTable<string, string> (str_hash, str_equal);
}

[DBus (name = "org.altlinux.APM2.Jobs")]
private class FakeJobs : Object {
    private FakeState state;

    public signal void job_started (string job, string domain, string kind);
    public signal void job_progress (string job, string event, string type, string state,
                                     string message, double progress, string progress_done);
    public signal void job_finished (string job, string status, string error_type,
                                     string message, string json);

    internal FakeJobs (FakeState state) {
        this.state = state;
    }

    public string list () throws GLib.Error {
        return "[]";
    }

    public new string get (string job) throws GLib.Error {
        var value = state.states.lookup (job);

        if (value == null) {
            throw new DBusError.FAILED ("unknown job");
        }

        return value;
    }

    public void cancel (string job) throws GLib.Error {
        if (job != "job-cancel") {
            throw new DBusError.FAILED ("unknown job");
        }
        const string payload = "{}";
        state.states[job] = "{\"id\":\"job-cancel\",\"state\":\"canceled\",\"errorType\":\"CANCELED\",\"message\":\"canceled\"}";
        job_finished (job, "canceled", "CANCELED", "canceled", payload);
    }
}

[DBus (name = "org.altlinux.APM2.Packages")]
private class FakePackages : Object {
    private FakeState state;

    internal FakePackages (FakeState state) {
        this.state = state;
    }

    public string check_upgrade () throws GLib.Error {
        const string payload = "{\"message\":\"ready\",\"info\":{\"upgradedCount\":2}}";
        state.states["job-fast"] = "{\"id\":\"job-fast\",\"state\":\"ok\",\"result\":%s}".printf (payload);
        // Deliberately finish before returning the job identifier.
        state.jobs.job_finished ("job-fast", "ok", "", "", payload);
        return "job-fast";
    }
}

[DBus (name = "org.altlinux.APM2.Image")]
private class FakeImage : Object {
    private FakeState state;

    internal FakeImage (FakeState state) {
        this.state = state;
    }

    public string update (string options_json) throws GLib.Error {
        state.states["job-cancel"] = "{\"id\":\"job-cancel\",\"state\":\"running\",\"cancellable\":true}";
        Timeout.add (5, () => {
            state.jobs.job_progress (
                "job-cancel",
                "download",
                "Image",
                "running",
                "Downloading",
                50.0,
                "50/100"
            );
            return Source.REMOVE;
        });
        return "job-cancel";
    }
}

private class Fixture : Object {
    internal TestDBus bus;
    internal DBusConnection service;
    internal DBusConnection client_connection;

    internal Fixture () throws Error {
        bus = new TestDBus (TestDBusFlags.NONE);
        bus.up ();
        var flags = DBusConnectionFlags.AUTHENTICATION_CLIENT |
                    DBusConnectionFlags.MESSAGE_BUS_CONNECTION;
        service = new DBusConnection.for_address_sync (bus.get_bus_address (), flags);
        client_connection = new DBusConnection.for_address_sync (bus.get_bus_address (), flags);

        var state = new FakeState ();
        state.jobs = new FakeJobs (state);
        service.register_object<FakeJobs> (PATH, state.jobs);
        service.register_object<FakePackages> (PATH, new FakePackages (state));
        service.register_object<FakeImage> (PATH, new FakeImage (state));
        service.call_sync ("org.freedesktop.DBus", "/org/freedesktop/DBus",
            "org.freedesktop.DBus", "RequestName", new Variant ("(su)", BUS_NAME, 0u),
            new VariantType ("(u)"), DBusCallFlags.NONE, -1);
    }

    internal void close () {
        try {
            client_connection.close_sync ();
            service.close_sync ();
        } catch (Error e) {
            Test.message ("Failed to close test bus connections: %s", e.message);
        }
        bus.down ();
    }
}

private void test_finished_before_reply () {
    Fixture fixture;
    try {
        fixture = new Fixture ();
    } catch (Error e) {
        Test.message ("Fixture failed: %s", e.message);
        assert_not_reached ();
    }
    var loop = new MainLoop ();
    Error? failure = null;
    Apm.Models.CheckResponse? response = null;
    Apm.Client.from_connection.begin (fixture.client_connection, false, (object, result) => {
        var client = Apm.Client.from_connection.end (result);
        client.packages.check_upgrade.begin (null, (object, result) => {
            try {
                response = client.packages.check_upgrade.end (result);
            } catch (Error e) {
                failure = e;
            }
            loop.quit ();
        });
    });
    loop.run ();
    assert (failure == null);
    assert (response != null);
    assert (response.message == "ready");
    assert (response.info.upgraded_count == 2);
    fixture.close ();
}

private void test_cancellable_cancels_server_job () {
    Fixture fixture;
    try {
        fixture = new Fixture ();
    } catch (Error e) {
        Test.message ("Fixture failed: %s", e.message);
        assert_not_reached ();
    }
    var loop = new MainLoop ();
    var cancellable = new Cancellable ();
    Error? failure = null;
    bool progress_seen = false;
    Apm.Client.from_connection.begin (fixture.client_connection, false, (object, result) => {
        var client = Apm.Client.from_connection.end (result);
        client.image.update.begin (null, cancellable, (progress) => {
            assert (progress.event == "download");
            assert (progress.kind == "Image");
            assert (progress.state == "running");
            assert (progress.message == "Downloading");
            assert (progress.percent == 50.0);
            assert (progress.done == "50/100");
            progress_seen = true;
        }, (object, result) => {
            try {
                client.image.update.end (result);
            } catch (Error e) {
                failure = e;
            }
            loop.quit ();
        });
        Timeout.add (20, () => {
            cancellable.cancel ();
            return Source.REMOVE;
        });
    });
    loop.run ();
    assert (failure is IOError.CANCELLED);
    assert (progress_seen);
    fixture.close ();
}

public int main (string[] args) {
    Test.init (ref args);
    Test.add_func ("/libapm/jobs/finished-before-reply", test_finished_before_reply);
    Test.add_func ("/libapm/jobs/cancellable-cancels-server-job", test_cancellable_cancels_server_job);
    return Test.run ();
}

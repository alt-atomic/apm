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
 * APM2 client for one bus. Use SYSTEM for Packages, Applications, Image,
 * Kernel and Repo; SESSION for Distrobox and Icons. Connections are lazy
 * with respect to the service: unavailable interfaces fail on use.
 */
public sealed class Apm.Client : Object {

    public Packages packages { get; private set; }
    public Applications applications { get; private set; }
    public Image image { get; private set; }
    public Kernel kernel { get; private set; }
    public Repo repo { get; private set; }
    public Distrobox distrobox { get; private set; }
    public Icons icons { get; private set; }

    private Client (DBusConnection connection, bool interactive) {
        var transport = new Transport (connection, interactive);
        packages = new Packages (transport);
        applications = new Applications (transport);
        image = new Image (transport);
        kernel = new Kernel (transport);
        repo = new Repo (transport);
        distrobox = new Distrobox (transport);
        icons = new Icons (transport);
    }

    public static async Client connect_bus (
        BusType bus_type = BusType.SYSTEM,
        bool interactive = true
    ) throws GLib.Error {
        var connection = yield Bus.get (bus_type);
        return new Client (connection, interactive);
    }

    /**
     * Creates a client using an existing connection without taking ownership.
     */
    public static async Client from_connection (DBusConnection connection, bool interactive = true) {
        return new Client (connection, interactive);
    }
}

public abstract class Apm.Module : Object {

    internal Transport transport;
    internal string iface;

    internal Module (Transport transport, string iface) {
        this.transport = transport;
        this.iface = iface;
    }

    internal async T request<T> (string method, Variant? args) throws GLib.Error {
        var json = yield transport.request (iface, method, args);
        return (T) yield Codec.decode<T> (json);
    }

    internal async Serialize.Array<T> request_array<T> (string method, Variant? args) throws GLib.Error {
        var json = yield transport.request (iface, method, args);
        return (Serialize.Array<T>) yield Codec.decode<T> (json, true);
    }

    internal async T run<T> (
        string method,
        Variant? args,
        Cancellable? cancellable,
        ProgressCallback? progress
    ) throws GLib.Error {
        var json = yield transport.run (iface, method, args, cancellable, progress);
        return (T) yield Codec.decode<T> (json);
    }
}

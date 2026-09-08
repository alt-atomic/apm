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
 * Client for repository configuration and task testing.
 *
 * {@link task_packages} supports server-side cancellation. {@link test_task}
 * waits for its non-cancellable APM2 job.
 */
public sealed class Apm.Repo : Module {

    internal Repo (Transport transport) {
        base (transport, "Repo");
    }

    public async Models.RepoListResponse list (bool all) throws GLib.Error {
        return yield request<Models.RepoListResponse> ("List", new Variant ("(b)", all));
    }

    public async string[] branches () throws GLib.Error {
        var reply = yield transport.call (Transport.PREFIX + iface, "Branches", null, "(as)");
        return reply.get_child_value (0).dup_strv ();
    }

    public async Models.TaskPackagesResponse task_packages (
        string task,
        Cancellable? cancellable = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.TaskPackagesResponse> (
            "TaskPackages",
            new Variant ("(s)", task),
            cancellable,
            progress
        );
    }

    public async Models.RepoTestTaskResponse test_task (
        string task,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<Models.RepoTestTaskResponse> ("TestTask", new Variant ("(s)", task), null, progress);
    }

    public async Models.RepoAddRemoveResponse add (string[] sources, string date) throws GLib.Error {
        return yield request<Models.RepoAddRemoveResponse> ("Add", new Variant ("(^ass)", sources, date));
    }

    public async Models.RepoAddRemoveResponse remove (string[] sources, string date) throws GLib.Error {
        return yield request<Models.RepoAddRemoveResponse> ("Remove", new Variant ("(^ass)", sources, date));
    }

    public async Models.RepoSetResponse set_branch (string branch, string date) throws GLib.Error {
        return yield request<Models.RepoSetResponse> ("SetBranch", new Variant ("(ss)", branch, date));
    }

    public async Models.RepoAddRemoveResponse clean () throws GLib.Error {
        return yield request<Models.RepoAddRemoveResponse> ("Clean", null);
    }

    public async Models.RepoSimulateResponse check_add (string[] sources, string date) throws GLib.Error {
        return yield request<Models.RepoSimulateResponse> ("CheckAdd", new Variant ("(^ass)", sources, date));
    }

    public async Models.RepoSimulateResponse check_remove (string[] sources, string date) throws GLib.Error {
        return yield request<Models.RepoSimulateResponse> ("CheckRemove", new Variant ("(^ass)", sources, date));
    }

    public async Models.RepoSimulateResponse check_set_branch (string branch, string date) throws GLib.Error {
        return yield request<Models.RepoSimulateResponse> ("CheckSetBranch", new Variant ("(ss)", branch, date));
    }

    public async Models.RepoSimulateResponse check_clean () throws GLib.Error {
        return yield request<Models.RepoSimulateResponse> ("CheckClean", null);
    }
}

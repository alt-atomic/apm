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

    public async ModelsRepoListResponse list (bool all) throws GLib.Error {
        return yield request<ModelsRepoListResponse> ("List", new Variant ("(b)", all));
    }

    public async string[] branches () throws GLib.Error {
        var reply = yield transport.call (Transport.PREFIX + iface, "Branches", null, "(as)");
        return reply.get_child_value (0).dup_strv ();
    }

    public async ModelsTaskPackagesResponse task_packages (
        string task,
        Cancellable? cancellable = null,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<ModelsTaskPackagesResponse> (
            "TaskPackages",
            new Variant ("(s)", task),
            cancellable,
            progress
        );
    }

    public async ModelsRepoTestTaskResponse test_task (
        string task,
        owned ProgressCallback? progress = null
    ) throws GLib.Error {
        return yield run<ModelsRepoTestTaskResponse> ("TestTask", new Variant ("(s)", task), null, progress);
    }

    public async ModelsRepoAddRemoveResponse add (string[] sources, string date) throws GLib.Error {
        return yield request<ModelsRepoAddRemoveResponse> ("Add", new Variant ("(^ass)", sources, date));
    }

    public async ModelsRepoAddRemoveResponse remove (string[] sources, string date) throws GLib.Error {
        return yield request<ModelsRepoAddRemoveResponse> ("Remove", new Variant ("(^ass)", sources, date));
    }

    public async ModelsRepoSetResponse set_branch (string branch, string date) throws GLib.Error {
        return yield request<ModelsRepoSetResponse> ("SetBranch", new Variant ("(ss)", branch, date));
    }

    public async ModelsRepoAddRemoveResponse clean () throws GLib.Error {
        return yield request<ModelsRepoAddRemoveResponse> ("Clean", null);
    }

    public async ModelsRepoSimulateResponse check_add (string[] sources, string date) throws GLib.Error {
        return yield request<ModelsRepoSimulateResponse> ("CheckAdd", new Variant ("(^ass)", sources, date));
    }

    public async ModelsRepoSimulateResponse check_remove (string[] sources, string date) throws GLib.Error {
        return yield request<ModelsRepoSimulateResponse> ("CheckRemove", new Variant ("(^ass)", sources, date));
    }

    public async ModelsRepoSimulateResponse check_set_branch (string branch, string date) throws GLib.Error {
        return yield request<ModelsRepoSimulateResponse> ("CheckSetBranch", new Variant ("(ss)", branch, date));
    }

    public async ModelsRepoSimulateResponse check_clean () throws GLib.Error {
        return yield request<ModelsRepoSimulateResponse> ("CheckClean", null);
    }
}

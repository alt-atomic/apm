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

/* APM2 wire models shared by several public modules. */

public sealed class Apm.ModelsPackageResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "packageInfo")]
    public ModelsPackageInfo package_info { get; set; }

}

public sealed class Apm.ModelsInstallRemoveResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "info")]
    public ModelsOperationInfo info { get; set; }
}

public sealed class Apm.ModelsKernelUpgradePreview : Serialize.DataObject {

    [Description (nick = "changes")]
    public ModelsOperationInfo changes { get; set; }

    [Description (nick = "selectedModules")]
    public string[] selected_modules { get; set; }

    [Description (nick = "missingModules")]
    public string[] missing_modules { get; set; }

}

public sealed class Apm.ModelsKernelListResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "kernels")]
    public Serialize.Array<ModelsKernelWithModulesInfo> kernels {
        get; set; default = new Serialize.Array<ModelsKernelWithModulesInfo> ();
    }

}

public sealed class Apm.ModelsKernelCurrentResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "kernel")]
    public ModelsKernelWithModulesInfo kernel { get; set; }

}

public sealed class Apm.ModelsImageHistoryResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "history")]
    public Serialize.Array<ModelsImageHistoryRecord> history {
        get; set; default = new Serialize.Array<ModelsImageHistoryRecord> ();
    }

    [Description (nick = "totalCount")]
    public int total_count { get; set; }

}

public sealed class Apm.ModelsImageConfigResponse : Serialize.DataObject {

    [Description (nick = "config")]
    public ModelsConfig config { get; set; }

}

public sealed class Apm.ModelsImageSyncGroupsResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "added")]
    public int added { get; set; }

    [Description (nick = "fixed")]
    public int fixed { get; set; }

    [Description (nick = "removed")]
    public int removed { get; set; }

    [Description (nick = "skipped")]
    public int skipped { get; set; }

}

public sealed class Apm.ModelsImageFixNssResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "etcPasswdCount")]
    public int etc_passwd_count { get; set; }

    [Description (nick = "libPasswdCount")]
    public int lib_passwd_count { get; set; }

    [Description (nick = "etcGroupCount")]
    public int etc_group_count { get; set; }

    [Description (nick = "libGroupCount")]
    public int lib_group_count { get; set; }

    [Description (nick = "removedUidConflicts")]
    public int removed_uid_conflicts { get; set; }

    [Description (nick = "removedGidConflicts")]
    public int removed_gid_conflicts { get; set; }

    [Description (nick = "normalizedGids")]
    public int normalized_gids { get; set; }

    [Description (nick = "normalizedGroups")]
    public string[] normalized_groups { get; set; }

}

public sealed class Apm.ModelsRepoTestTaskResponse : Serialize.DataObject {

    [Description (nick = "taskNum")]
    public string task_num { get; set; }

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "info")]
    public ModelsOperationInfo info { get; set; }

}

public sealed class Apm.ModelsDistroboxPackageResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "packageInfo")]
    public ModelsDistroboxPackageInfo package_info { get; set; }

}

public sealed class Apm.ModelsDistroboxSearchResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "packages")]
    public Serialize.Array<ModelsDistroboxPackage> packages { get; set; default = new Serialize.Array<ModelsDistroboxPackage> (); }

}

public sealed class Apm.ModelsDistroboxContainerListResponse : Serialize.DataObject {

    [Description (nick = "containers")]
    public Serialize.Array<ModelsContainerInfo> containers { get; set; default = new Serialize.Array<ModelsContainerInfo> (); }

}

public sealed class Apm.ModelsDistroboxContainerResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "containerInfo")]
    public ModelsContainerInfo container_info { get; set; }

}

public sealed class Apm.ModelsFilter : Serialize.DataObject {

    [Description (nick = "field")]
    public string field { get; set; }

    [Description (nick = "op")]
    public string op { get; set; }

    [Description (nick = "value")]
    public string value { get; set; }

}

public sealed class Apm.ModelsPageRequest : Serialize.DataObject {

    [Description (nick = "limit")]
    public int limit { get; set; }

    [Description (nick = "offset")]
    public int offset { get; set; }

}

public sealed class Apm.ModelsListRequest : Serialize.DataObject {

    [Description (nick = "sort")]
    public string sort { get; set; }

    [Description (nick = "order")]
    public string order { get; set; }

    [Description (nick = "limit")]
    public int limit { get; set; }

    [Description (nick = "offset")]
    public int offset { get; set; }

    [Description (nick = "forceUpdate")]
    public bool force_update { get; set; }

    [Description (nick = "filters")]
    public Serialize.Array<ModelsFilter> filters { get; set; default = new Serialize.Array<ModelsFilter> (); }

    [Description (nick = "orFilters")]
    public Serialize.Array<ModelsFilter> or_filters { get; set; default = new Serialize.Array<ModelsFilter> (); }

}

public sealed class Apm.ModelsInstallOptions : Serialize.DataObject {

    [Description (nick = "downloadOnly")]
    public bool download_only { get; set; }

    [Description (nick = "noUpdate")]
    public bool no_update { get; set; }

}

public sealed class Apm.ModelsRemoveOptions : Serialize.DataObject {

    [Description (nick = "purge")]
    public bool purge { get; set; }

    [Description (nick = "depends")]
    public bool depends { get; set; }

}

public sealed class Apm.ModelsUpgradeOptions : Serialize.DataObject {

    [Description (nick = "downloadOnly")]
    public bool download_only { get; set; }

}

public sealed class Apm.ModelsUpdateOptions : Serialize.DataObject {

    [Description (nick = "onlyDB")]
    public bool only_db { get; set; }

}

public sealed class Apm.ModelsImageUpdateOptions : Serialize.DataObject {

    [Description (nick = "noCache")]
    public bool no_cache { get; set; }

}

public sealed class Apm.ModelsImageApplyOptions : Serialize.DataObject {

    [Description (nick = "pull")]
    public bool pull { get; set; }

    [Description (nick = "noCache")]
    public bool no_cache { get; set; }

    [Description (nick = "configPath")]
    public string config_path { get; set; }

    [Description (nick = "workdir")]
    public string workdir { get; set; }

}

public sealed class Apm.ModelsImageSwitchOptions : Serialize.DataObject {

    [Description (nick = "pull")]
    public bool pull { get; set; }

    [Description (nick = "noCache")]
    public bool no_cache { get; set; }

}

public sealed class Apm.ModelsKernelOptions : Serialize.DataObject {

    [Description (nick = "headers")]
    public bool headers { get; set; }

}

public sealed class Apm.ModelsKernelCleanOptions : Serialize.DataObject {

    [Description (nick = "noBackup")]
    public bool no_backup { get; set; }

}

public sealed class Apm.ModelsContainerOptions : Serialize.DataObject {

    [Description (nick = "additionalPackages")]
    public string additional_packages { get; set; }

    [Description (nick = "initHooks")]
    public string init_hooks { get; set; }

}

public sealed class Apm.ModelsDistroboxInstallOptions : Serialize.DataObject {

    [Description (nick = "export")]
    public bool export { get; set; }

}

public sealed class Apm.ModelsDistroboxRemoveOptions : Serialize.DataObject {

    [Description (nick = "onlyExport")]
    public bool only_export { get; set; }

}

public sealed class Apm.ModelsUnknownConfigModule : ModelsConfigModule {

    [Description (nick = "body")]
    public Serialize.Dict<GLib.Value?> body { get; set; default = new Serialize.Dict<GLib.Value?> (); }

}

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

public sealed class Apm.Models.PackageResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "packageInfo")]
    public PackageInfo package_info { get; set; }

}

public sealed class Apm.Models.InstallRemoveResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "info")]
    public OperationInfo info { get; set; }
}

public sealed class Apm.Models.KernelUpgradePreview : Serialize.DataObject {

    [Description (nick = "changes")]
    public OperationInfo changes { get; set; }

    [Description (nick = "selectedModules")]
    public string[] selected_modules { get; set; }

    [Description (nick = "missingModules")]
    public string[] missing_modules { get; set; }

}

public sealed class Apm.Models.KernelListResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "kernels")]
    public Serialize.Array<KernelWithModulesInfo> kernels {
        get; set; default = new Serialize.Array<KernelWithModulesInfo> ();
    }

}

public sealed class Apm.Models.KernelCurrentResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "kernel")]
    public KernelWithModulesInfo kernel { get; set; }

}

public sealed class Apm.Models.ImageHistoryResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "history")]
    public Serialize.Array<ImageHistoryRecord> history {
        get; set; default = new Serialize.Array<ImageHistoryRecord> ();
    }

    [Description (nick = "totalCount")]
    public int total_count { get; set; }

}

public sealed class Apm.Models.ImageConfigResponse : Serialize.DataObject {

    [Description (nick = "config")]
    public Config config { get; set; }

}

public sealed class Apm.Models.ImageSyncGroupsResponse : Serialize.DataObject {

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

public sealed class Apm.Models.ImageFixNssResponse : Serialize.DataObject {

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

public sealed class Apm.Models.RepoTestTaskResponse : Serialize.DataObject {

    [Description (nick = "taskNum")]
    public string task_num { get; set; }

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "info")]
    public OperationInfo info { get; set; }

}

public sealed class Apm.Models.Distrobox.PackageResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "packageInfo")]
    public PackageInfo package_info { get; set; }

}

public sealed class Apm.Models.Distrobox.SearchResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "packages")]
    public Serialize.Array<Package> packages { get; set; default = new Serialize.Array<Package> (); }

}

public sealed class Apm.Models.Distrobox.ContainerListResponse : Serialize.DataObject {

    [Description (nick = "containers")]
    public Serialize.Array<ContainerInfo> containers { get; set; default = new Serialize.Array<ContainerInfo> (); }

}

public sealed class Apm.Models.Distrobox.ContainerResponse : Serialize.DataObject {

    [Description (nick = "message")]
    public string message { get; set; }

    [Description (nick = "containerInfo")]
    public ContainerInfo container_info { get; set; }

}

public sealed class Apm.Models.Filter : Serialize.DataObject {

    [Description (nick = "field")]
    public string field { get; set; }

    [Description (nick = "op")]
    public string op { get; set; }

    [Description (nick = "value")]
    public string value { get; set; }

}

public sealed class Apm.Models.PageRequest : Serialize.DataObject {

    [Description (nick = "limit")]
    public uint limit { get; set; }

    [Description (nick = "offset")]
    public uint offset { get; set; }

}

public sealed class Apm.Models.ListRequest : Serialize.DataObject {

    [Description (nick = "sort")]
    public string sort { get; set; }

    [Description (nick = "order")]
    public string order { get; set; }

    [Description (nick = "limit")]
    public uint limit { get; set; }

    [Description (nick = "offset")]
    public uint offset { get; set; }

    [Description (nick = "forceUpdate")]
    public bool force_update { get; set; }

    [Description (nick = "filters")]
    public Serialize.Array<Filter> filters { get; set; default = new Serialize.Array<Filter> (); }

    [Description (nick = "orFilters")]
    public Serialize.Array<Filter> or_filters { get; set; default = new Serialize.Array<Filter> (); }

}

public sealed class Apm.Models.InstallOptions : Serialize.DataObject {

    [Description (nick = "downloadOnly")]
    public bool download_only { get; set; }

    [Description (nick = "noUpdate")]
    public bool no_update { get; set; }

}

public sealed class Apm.Models.RemoveOptions : Serialize.DataObject {

    [Description (nick = "purge")]
    public bool purge { get; set; }

    [Description (nick = "depends")]
    public bool depends { get; set; }

}

public sealed class Apm.Models.UpgradeOptions : Serialize.DataObject {

    [Description (nick = "downloadOnly")]
    public bool download_only { get; set; }

}

public sealed class Apm.Models.UpdateOptions : Serialize.DataObject {

    [Description (nick = "onlyDB")]
    public bool only_db { get; set; }

}

public sealed class Apm.Models.ImageUpdateOptions : Serialize.DataObject {

    [Description (nick = "noCache")]
    public bool no_cache { get; set; }

}

public sealed class Apm.Models.ImageApplyOptions : Serialize.DataObject {

    [Description (nick = "pull")]
    public bool pull { get; set; }

    [Description (nick = "noCache")]
    public bool no_cache { get; set; }

    [Description (nick = "configPath")]
    public string config_path { get; set; }

    [Description (nick = "workdir")]
    public string workdir { get; set; }

}

public sealed class Apm.Models.ImageSwitchOptions : Serialize.DataObject {

    [Description (nick = "pull")]
    public bool pull { get; set; }

    [Description (nick = "noCache")]
    public bool no_cache { get; set; }

}

public sealed class Apm.Models.KernelOptions : Serialize.DataObject {

    [Description (nick = "headers")]
    public bool headers { get; set; }

}

public sealed class Apm.Models.KernelCleanOptions : Serialize.DataObject {

    [Description (nick = "noBackup")]
    public bool no_backup { get; set; }

}

public sealed class Apm.Models.ContainerOptions : Serialize.DataObject {

    [Description (nick = "additionalPackages")]
    public string additional_packages { get; set; }

    [Description (nick = "initHooks")]
    public string init_hooks { get; set; }

}

public sealed class Apm.Models.DistroboxInstallOptions : Serialize.DataObject {

    [Description (nick = "export")]
    public bool export { get; set; }

}

public sealed class Apm.Models.DistroboxRemoveOptions : Serialize.DataObject {

    [Description (nick = "onlyExport")]
    public bool only_export { get; set; }

}

public sealed class Apm.Models.UnknownConfigModule : ConfigModule {

    [Description (nick = "body")]
    public Serialize.Dict<GLib.Value?> body { get; set; default = new Serialize.Dict<GLib.Value?> (); }

}

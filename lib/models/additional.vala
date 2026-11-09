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

// Go slices may be encoded as null; libdeser's standard string-list codec
// accepts arrays only. Keep native string[] properties nullable on the wire.
internal sealed class Apm.NullableStringListCodec : Deser.ValueCodec {
    Deser.StringListCodec inner = new Deser.StringListCodec ();

    public override Type value_type { get { return typeof (string[]); } }
    public override bool accepts_null { get { return true; } }

    public override Deser.NullableValue decode_value (
        Deser.ValueView node,
        Deser.DecodeContext context
    ) throws Deser.Error {
        if (node.kind == Deser.ValueKind.NULL) {
            Deser.NullableValue result = Deser.NullableValue ();
            result.is_null = true;
            result.value = Value (typeof (string[]));
            return result;
        }
        return inner.decode_value (node, context);
    }

    public override void encode_value (
        Deser.NullableValue value,
        Deser.ValueWriter writer,
        Deser.EncodeContext context
    ) throws Deser.Error {
        if (value.is_null) {
            writer.write_null ();
        } else {
            inner.encode_value (value, writer, context);
        }
    }
}

public abstract class Apm.ModelsDataObject : Deser.DataObject {
    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.value_type == typeof (string[])) {
            return new NullableStringListCodec ();
        }
        return base.get_property_codec (property);
    }
}

public sealed class Apm.ModelsPackageResponse : ModelsDataObject {

    public string message { get; set; }

    [Description (nick = "packageInfo")]
    public ModelsPackageInfo package_info { get; set; }

}

public sealed class Apm.ModelsInstallRemoveResponse : ModelsDataObject {

    public string message { get; set; }

    public ModelsOperationInfo info { get; set; }
}

public sealed class Apm.ModelsKernelUpgradePreview : ModelsDataObject {

    public ModelsOperationInfo changes { get; set; }

    [Description (nick = "selectedModules")]
    public string[] selected_modules { get; set; }

    [Description (nick = "missingModules")]
    public string[] missing_modules { get; set; }

}

public sealed class Apm.ModelsKernelListResponse : ModelsDataObject {

    public string message { get; set; }

    public GenericArray<ModelsKernelWithModulesInfo> kernels {
        get; set; default = new GenericArray<ModelsKernelWithModulesInfo> ();
    }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "kernels") {
            return Deser.create_codec_for_array<ModelsKernelWithModulesInfo> ();
        }
        return base.get_property_codec (property);
    }
}

public sealed class Apm.ModelsKernelCurrentResponse : ModelsDataObject {

    public string message { get; set; }

    public ModelsKernelWithModulesInfo kernel { get; set; }

}

public sealed class Apm.ModelsImageHistoryResponse : ModelsDataObject {

    public string message { get; set; }

    public GenericArray<ModelsImageHistoryRecord> history {
        get; set; default = new GenericArray<ModelsImageHistoryRecord> ();
    }

    [Description (nick = "totalCount")]
    public int total_count { get; set; }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "history") {
            return Deser.create_codec_for_array<ModelsImageHistoryRecord> ();
        }
        return base.get_property_codec (property);
    }
}

public sealed class Apm.ModelsImageConfigResponse : ModelsDataObject {

    public ModelsConfig config { get; set; }

}

public sealed class Apm.ModelsImageSyncGroupsResponse : ModelsDataObject {

    public string message { get; set; }

    public int added { get; set; }

    public int fixed { get; set; }

    public int removed { get; set; }

    public int skipped { get; set; }

}

public sealed class Apm.ModelsImageFixNssResponse : ModelsDataObject {

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

public sealed class Apm.ModelsRepoTestTaskResponse : ModelsDataObject {

    [Description (nick = "taskNum")]
    public string task_num { get; set; }

    public string message { get; set; }

    public ModelsOperationInfo info { get; set; }

}

public sealed class Apm.ModelsDistroboxPackageResponse : ModelsDataObject {

    public string message { get; set; }

    [Description (nick = "packageInfo")]
    public ModelsDistroboxPackageInfo package_info { get; set; }

}

public sealed class Apm.ModelsDistroboxSearchResponse : ModelsDataObject {

    public string message { get; set; }

    public GenericArray<ModelsDistroboxPackage> packages { get; set; default = new GenericArray<ModelsDistroboxPackage> (); }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "packages") {
            return Deser.create_codec_for_array<ModelsDistroboxPackage> ();
        }
        return base.get_property_codec (property);
    }
}

public sealed class Apm.ModelsDistroboxContainerListResponse : ModelsDataObject {

    public GenericArray<ModelsContainerInfo> containers { get; set; default = new GenericArray<ModelsContainerInfo> (); }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "containers") {
            return Deser.create_codec_for_array<ModelsContainerInfo> ();
        }
        return base.get_property_codec (property);
    }
}

public sealed class Apm.ModelsDistroboxContainerResponse : ModelsDataObject {

    public string message { get; set; }

    [Description (nick = "containerInfo")]
    public ModelsContainerInfo container_info { get; set; }

}

public sealed class Apm.ModelsFilter : ModelsDataObject {

    public string field { get; set; }

    public string op { get; set; }

    public string value { get; set; }

}

public sealed class Apm.ModelsPageRequest : ModelsDataObject {

    public int limit { get; set; }

    public int offset { get; set; }

}

public sealed class Apm.ModelsListRequest : ModelsDataObject {

    public string sort { get; set; }

    public string order { get; set; }

    public int limit { get; set; }

    public int offset { get; set; }

    [Description (nick = "forceUpdate")]
    public bool force_update { get; set; }

    public GenericArray<ModelsFilter> filters { get; set; default = new GenericArray<ModelsFilter> (); }

    [Description (nick = "orFilters")]
    public GenericArray<ModelsFilter> or_filters { get; set; default = new GenericArray<ModelsFilter> (); }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "filters") {
            return Deser.create_codec_for_array<ModelsFilter> ();
        }
        if (property.name == "or-filters") {
            return Deser.create_codec_for_array<ModelsFilter> ();
        }
        return base.get_property_codec (property);
    }
}

public class Apm.ModelsAptOptions : ModelsDataObject {

    [Description (nick = "aptConfig")]
    public HashTable<string, string>? apt_config { get; set; }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "apt-config") {
            return Deser.create_codec_for_map<string> ();
        }
        return base.get_property_codec (property);
    }
}

public sealed class Apm.ModelsInstallOptions : ModelsAptOptions {

    [Description (nick = "downloadOnly")]
    public bool download_only { get; set; }

    [Description (nick = "noUpdate")]
    public bool no_update { get; set; }

}

public sealed class Apm.ModelsRemoveOptions : ModelsAptOptions {

    public bool purge { get; set; }

    public bool depends { get; set; }

}

public sealed class Apm.ModelsUpgradeOptions : ModelsAptOptions {

    [Description (nick = "downloadOnly")]
    public bool download_only { get; set; }

}

public sealed class Apm.ModelsUpdateOptions : ModelsAptOptions {

    [Description (nick = "onlyDB")]
    public bool only_db { get; set; }

}

public sealed class Apm.ModelsImageUpdateOptions : ModelsDataObject {

    [Description (nick = "noCache")]
    public bool no_cache { get; set; }

}

public sealed class Apm.ModelsImageApplyOptions : ModelsDataObject {

    public bool pull { get; set; }

    [Description (nick = "noCache")]
    public bool no_cache { get; set; }

    [Description (nick = "configPath")]
    public string config_path { get; set; }

    public string workdir { get; set; }

}

public sealed class Apm.ModelsImageSwitchOptions : ModelsDataObject {

    public bool pull { get; set; }

    [Description (nick = "noCache")]
    public bool no_cache { get; set; }

}

public sealed class Apm.ModelsKernelOptions : ModelsDataObject {

    public bool headers { get; set; }

}

public sealed class Apm.ModelsKernelCleanOptions : ModelsDataObject {

    [Description (nick = "noBackup")]
    public bool no_backup { get; set; }

}

public sealed class Apm.ModelsContainerOptions : ModelsDataObject {

    [Description (nick = "additionalPackages")]
    public string additional_packages { get; set; }

    [Description (nick = "initHooks")]
    public string init_hooks { get; set; }

}

public sealed class Apm.ModelsDistroboxInstallOptions : ModelsDataObject {

    public bool export { get; set; }

}

public sealed class Apm.ModelsDistroboxRemoveOptions : ModelsDataObject {

    [Description (nick = "onlyExport")]
    public bool only_export { get; set; }

}

public sealed class Apm.ModelsUnknownConfigModule : ModelsConfigModule {

    public HashTable<string, Deser.DynamicValue> body { get; set; default = new HashTable<string, Deser.DynamicValue> (str_hash, str_equal); }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "body") {
            return new Deser.HashTableCodec<Deser.DynamicValue> (new Deser.DynamicValueCodec ());
        }
        return base.get_property_codec (property);
    }
}

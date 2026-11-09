/*
 * Copyright (C) 2025-2026 Vladimir Romanov <rirusha@altlinux.org>
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
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

public sealed class Apm.ModelsFilterFieldsInfo : ModelsDataObject {

    public string name { get; set; }

    [Description (nick = "defaultOp")]
    public string default_op { get; set; }

    [Description (nick = "allowedOps")]
    public string[] allowed_ops { get; set; }

    public HashTable<string, Deser.DynamicValue> extra { get; set; default = new HashTable<string, Deser.DynamicValue> (str_hash, str_equal); }

    public bool sortable { get; set; }

    public override Deser.ValueCodec get_property_codec (ParamSpec property) throws Deser.Error {
        if (property.name == "extra") {
            return new Deser.HashTableCodec<Deser.DynamicValue> (new Deser.DynamicValueCodec ());
        }
        return base.get_property_codec (property);
    }
}

/*
 * Copyright (C) 2025 Vladimir Romanov <rirusha@altlinux.org>
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

package models

import (
	"context"
	"fmt"
	"os"
)

var EtcHostname = "/etc/hostname"
var EtcHosts = "/etc/hosts"

type NetworkBody struct {
	// Hostname in image
	Hostname string `yaml:"hostname,omitempty" json:"hostname,omitempty" required:""`
}

func (b *NetworkBody) Execute(ctx context.Context, svc Service) (any, error) {
	if err := os.WriteFile(EtcHostname, fmt.Appendf(nil, "%s\n", b.Hostname), 0644); err != nil {
		return nil, err
	}
	hosts := fmt.Sprintf(
		"127.0.0.1 localhost %s\n::1 localhost6 %s6\n",
		b.Hostname,
		b.Hostname,
	)
	if err := os.WriteFile(EtcHosts, []byte(hosts), 0644); err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *NetworkBody) Hash(_ string, env map[string]string) string {
	return hashWithEnv(b, env)
}

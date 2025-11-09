// Atomic Package Manager
// Copyright (C) 2025 Vladimir Romanov <rirusha@altlinux.org>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package version

import (
	"fmt"
	"strconv"
	"strings"
)

type Version struct {
	Major   int
	Minor   int
	Patch   int
	Commits int
}

func (v *Version) ToString() string {
	postfix := ""
	if v.Commits != 0 {
		postfix = fmt.Sprintf("+%d", v.Commits)
	}
	return fmt.Sprintf("%d.%d.%d%s", v.Major, v.Minor, v.Patch, postfix)
}

func ParseVersion(version string) Version {
	var ver Version
	var err error

	parts := strings.Split(version, "+")
	if len(parts) == 2 {
		ver.Commits, err = strconv.Atoi(parts[1])
		if err != nil {
			panic("Wrong version format")
		}
	}
	version = strings.TrimPrefix(parts[0], "v")

	parts = strings.Split(version, ".")
	if len(parts) != 3 {
		panic("Wrong version format")
	}

	ver.Major, err = strconv.Atoi(parts[0])
	if err != nil {
		panic("Wrong version format")
	}

	ver.Minor, err = strconv.Atoi(parts[1])
	if err != nil {
		panic("Wrong version format")
	}

	ver.Patch, err = strconv.Atoi(parts[2])
	if err != nil {
		panic("Wrong version format")
	}

	return ver
}

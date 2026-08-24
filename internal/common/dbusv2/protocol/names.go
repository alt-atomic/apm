// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
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

// Package protocol содержит имена шины, интерфейсов и polkit-действий API v2.
package protocol

import (
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	// Prefix общий префикс API v2: интерфейсы, ошибки, путь.
	Prefix = "org.altlinux.APM2"

	PackagesIface     = Prefix + ".Packages"
	ImageIface        = Prefix + ".Image"
	ApplicationsIface = Prefix + ".Applications"
	KernelIface       = Prefix + ".Kernel"
	RepoIface         = Prefix + ".Repo"
	DistroboxIface    = Prefix + ".Distrobox"
	IconsIface        = Prefix + ".Icons"
	JobsIface         = Prefix + ".Jobs"

	ErrorPrefix = Prefix + ".Error."
)

// Path объектный путь API v2, один на все интерфейсы.
const Path = dbus.ObjectPath("/org/altlinux/APM2")

// Polkit-действия v2, гранулярные по доменам.
const (
	ActionPackagesManage     = "org.altlinux.apm2.packages.manage"
	ActionImageManage        = "org.altlinux.apm2.image.manage"
	ActionApplicationsManage = "org.altlinux.apm2.applications.manage"
	ActionKernelManage       = "org.altlinux.apm2.kernel.manage"
	ActionRepoManage         = "org.altlinux.apm2.repo.manage"
)

// In описывает входной аргумент метода.
func In(name, sig string) introspect.Arg {
	return introspect.Arg{Name: name, Type: sig, Direction: "in"}
}

// Out описывает выходной аргумент метода.
func Out(name, sig string) introspect.Arg {
	return introspect.Arg{Name: name, Type: sig, Direction: "out"}
}

// SignalArg описывает аргумент сигнала.
func SignalArg(name, sig string) introspect.Arg {
	return introspect.Arg{Name: name, Type: sig}
}

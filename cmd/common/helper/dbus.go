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

package helper

import "github.com/godbus/dbus/v5/introspect"

const UserIntrospectXML = `
<node>
  <interface name="org.altlinux.APM">
    <signal name="Notification">
      <arg type="s" name="message" direction="out"/>
    </signal>
  </interface>

  <interface name="org.altlinux.APM.distrobox">
    <method name="Update">
      <arg direction="in" type="s" name="container"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="GetIconByPackage">
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="s" name="container"/>
      <arg direction="out" type="ay" name="result"/>
    </method>

    <method name="GetFilterFields">
      <arg direction="in" type="s" name="container"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="Info">
      <arg direction="in" type="s" name="container"/>
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="Search">
      <arg direction="in" type="s" name="container"/>
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="List">
      <arg direction="in" type="s" name="paramsJSON"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="Install">
      <arg direction="in" type="s" name="container"/>
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="b" name="export"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="Remove">
      <arg direction="in" type="s" name="container"/>
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="b" name="onlyExport"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="ContainerList">
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="ContainerAdd">
      <arg direction="in" type="s" name="image"/>
      <arg direction="in" type="s" name="name"/>
      <arg direction="in" type="s" name="additionalPackages"/>
      <arg direction="in" type="s" name="initHooks"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="ContainerRemove">
      <arg direction="in" type="s" name="name"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
  </interface>
` + introspect.IntrospectDataString + `</node>`

const SystemIntrospectXML = `
<node>
  <interface name="org.altlinux.APM">
    <signal name="Notification">
      <arg type="s" name="message" direction="out"/>
    </signal>
  </interface>

  <interface name="org.altlinux.APM.system">

    <method name="CheckUpgrade">
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="UpdateKernel">
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="CheckUpdateKernel">
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="Upgrade">
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="Install">
      <arg direction="in" type="as" name="packages"/>
      <arg direction="in" type="b" name="applyAtomic"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    
    <method name="Remove">
      <arg direction="in" type="as" name="packages"/>
      <arg direction="in" type="b" name="applyAtomic"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="Update">
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    
    <method name="List">
      <arg direction="in" type="s" name="paramsJSON"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    
    <method name="Info">
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    
    <method name="CheckInstall">
      <arg direction="in" type="as" name="packages"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>

    <method name="CheckRemove">
      <arg direction="in" type="as" name="packages"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    
    <method name="Search">
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="in" type="b" name="installed"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    
    <method name="ImageApply">
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    
    <method name="ImageHistory">
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="in" type="s" name="imageName"/>
      <arg direction="in" type="x" name="limit"/>
      <arg direction="in" type="x" name="offset"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    
    <method name="ImageUpdate">
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    
    <method name="ImageStatus">
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
  </interface>
` + introspect.IntrospectDataString + `</node>`

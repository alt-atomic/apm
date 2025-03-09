package helper

import "github.com/godbus/dbus/v5/introspect"

const UserIntrospectXML = `
<node>
  <interface name="com.application.APM">
    <signal name="Notification">
      <arg type="s" name="message" direction="out"/>
    </signal>
  </interface>
  <interface name="com.application.distrobox">
    <method name="Update">
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
  <interface name="com.application.APM">
    <signal name="Notification">
      <arg type="s" name="message" direction="out"/>
    </signal>
  </interface>
  <interface name="com.application.system">
    <method name="Install">
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    <method name="Update">
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="s" name="transaction"/>	
      <arg direction="out" type="s" name="result"/>
    </method>
    <method name="Info">
      <arg direction="in" type="s" name="packageName"/>
	  <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    <method name="Search">
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="s" name="transaction"/>
      <arg direction="out" type="s" name="result"/>
    </method>
    <method name="Remove">
      <arg direction="in" type="s" name="packageName"/>
      <arg direction="in" type="s" name="transaction"/>
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

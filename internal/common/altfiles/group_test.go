package altfiles

import (
	"testing"
)

var realGroup = []byte(`adm:x:4:dm,root
apache2:x:957:
apache:x:96:
asterisk:x:34:
audio:x:81:dm
auth:x:27:
_avahi:x:984:
bin:x:1:root
camera:x:963:dm
cdrom:x:22:dm
cdwriter:x:80:dm
chkpwd:x:24:
_chrony:x:977:
colord:x:982:
console:x:17:
crontab:x:980:
cuse:x:986:dm
cvsadmin:x:53:
cvs:x:52:
daemon:x:2:root
dialout:x:992:
dip:x:40:
disk:x:6:root
dm:x:1000:
_dnsmasq:x:995:
docker:x:948:dm
exim:x:79:
firewall:x:11:
flatpak:x:960:
floppy:x:71:
ftpadmin:x:51:
ftp:x:50:
fuse:x:987:dm
fwupd-refresh:x:951:
games:x:20:
gdm:x:47:
geoclue:x:971:
gnome-remote-desktop:x:950:
_gnupg:x:999:
gopher:x:30:
input:x:991:
iputils:x:974:
_keytab:x:998:
kmem:x:9:
kqemu:x:35:
ldap:x:55:
libvirt:x:972:dm
loop:x:70:
lp:x:7:
lxd:x:975:dm
mailman:x:41:
mail:x:12:
man:x:15:
mem:x:8:
messagebus:x:994:
mysql:x:45:
named:x:25:
netadmin:x:973:dm
netwatch:x:31:
news:x:13:
nfsuser:x:968:
nm-openconnect:x:955:
_nobody99:x:99:
nobody:x:65534:
nscd:x:28:
openvpn:x:956:
pcscd:x:949:
pipewire:x:964:
plugdev:x:962:
polkitd:x:983:
popa3d:x:43:
postdrop:x:54:
postfix:x:42:
postgres:x:46:
postman:x:48:
printadmin:x:970:
proc:x:19:root
radio:x:83:
remote:x:961:
render:x:989:dm
root:x:0:
rpcuser:x:29:
rpc:x:32:
rpminst:x:33:
rpm:x:16:
rtkit:x:965:
sasl:x:997:
sgx:x:988:
shadow:x:26:
slocate:x:21:
squid:x:23:
sshagent:x:996:
sshd:x:954:
stapler-builder:x:952:
systemd-journal:x:979:
systemd-oom:x:978:
sys:x:3:adm,bin,root
tape:x:993:
_teamd:x:981:
tss:x:985:
tty:x:5:
usbmux:x:969:
usershares:x:953:
users:x:100:dm
utempter:x:947:
utmp:x:72:
uucp:x:14:dm
video:x:990:dm
vmusers:x:36:
webmaster:x:958:
_webserver:x:959:apache2
wheel:x:10:dm,root
wnn:x:18:
_wsdd:x:967:
x10:x:82:
_xfsscrub:x:976:
xfs:x:44:
xgrp:x:966:dm
`)

func TestSplitGroupReal(t *testing.T) {
	entries, err := ParseGroup(realGroup)
	if err != nil {
		t.Fatalf("ParseGroup: %v", err)
	}

	if len(entries) != 118 {
		t.Fatalf("expected 118 entries, got %d", len(entries))
	}

	etc, lib := SplitGroup(entries)

	etcNames := names(etc)
	expectNames(t, etcNames, []string{"dm", "root", "wheel"}, "/etc")

	if len(lib) != 115 {
		t.Errorf("/usr/lib: expected 115 entries, got %d", len(lib))
	}

	for _, e := range lib {
		if e.Name == "wheel" {
			t.Error("wheel should not be in /usr/lib")
		}
		if e.Name == "root" {
			t.Error("root should not be in /usr/lib")
		}
		if e.Name == "dm" {
			t.Error("dm should not be in /usr/lib")
		}
	}

	foundNobody := false
	for _, e := range lib {
		if e.Name == "nobody" && e.GID == 65534 {
			foundNobody = true
		}
	}
	if !foundNobody {
		t.Error("nobody should be in /usr/lib")
	}

	// Проверяем что members сохраняются
	for _, e := range etc {
		if e.Name == "wheel" {
			if len(e.Members) != 2 || e.Members[0] != "dm" || e.Members[1] != "root" {
				t.Errorf("wheel members: got %v, want [dm root]", e.Members)
			}
		}
	}

	// Roundtrip
	for _, e := range entries {
		reparsed, err := ParseGroup([]byte(e.String()))
		if err != nil {
			t.Errorf("roundtrip failed for %s: %v", e.Name, err)
		}
		if len(reparsed) != 1 || reparsed[0].String() != e.String() {
			t.Errorf("roundtrip mismatch for %s", e.Name)
		}
	}
}

func TestSplitGroupIdempotentReal(t *testing.T) {
	entries, _ := ParseGroup(realGroup)
	etc1, lib1 := SplitGroup(entries)

	merged := MergeGroup(etc1, lib1)
	etc2, lib2 := SplitGroup(merged)

	expectNames(t, names(etc2), names(etc1), "idempotent /etc")
	if len(lib2) != len(lib1) {
		t.Errorf("idempotent /usr/lib: got %d entries, want %d", len(lib2), len(lib1))
	}
}

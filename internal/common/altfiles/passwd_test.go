package altfiles

import (
	"testing"
)

var realPasswd = []byte(`adm:x:3:4:adm:/var/adm:/dev/null
apache2:x:979:957:Apache2 WWW server:/var/www:/dev/null
apache:x:96:96:Apache web server:/var/www:/dev/null
_avahi:x:996:984:Avahi service:/var/run/avahi-daemon:/dev/null
bin:x:1:1:bin:/:/dev/null
_chrony:x:990:977:Chrony User:/var/lib/chrony:/dev/null
colord:x:994:982:User for colord:/var/colord:/dev/null
daemon:x:2:2:daemon:/:/dev/null
dm:x:1000:1000::/var/home/dm:/bin/bash
_dnsmasq:x:999:995::/dev/null:/dev/null
exim:x:79:79:Exim Mail Transport Agent:/var/spool/exim:/dev/null
flatpak:x:980:960:User for flatpak system helper:/:/sbin/nologin
ftp:x:14:50:FTP User:/var/ftp:/dev/null
fwupd-refresh:x:951:951:Firmware update daemon:/var/lib/fwupd:/sbin/nologin
games:x:12:100:games:/usr/games:/dev/null
gdm:x:47:47:GDM:/var/lib/gdm:/dev/null
geoclue:x:986:971:User for GeoClue service:/var/lib/geoclue:/dev/null
gnome-remote-desktop:x:950:950:GNOME Remote Desktop:/var/lib/gnome-remote-desktop:/sbin/nologin
iputils:x:987:974::/dev/null:/dev/null
ldap:x:55:55:LDAP User:/var/lib/ldap:/dev/null
_libvirt:x:991:36:libvirt user:/var/lib/libvirt:/bin/false
lp:x:4:7:lp:/var/spool/lpd:/dev/null
lxd:x:988:975:LXD daemon:/dev/null:/dev/null
mailman:x:41:41:GNU Mailing List Manager:/usr/share/mailman:/dev/null
mail:x:8:12:mail:/var/spool/mail:/dev/null
messagebus:x:998:994:D-Bus System User:/run/dbus:/dev/null
mysql:x:45:45:MySQL server:/var/lib/mysql:/dev/null
named:x:25:25:Bind User:/var/lib/named:/dev/null
news:x:9:13:news:/var/spool/news:/dev/null
nfsuser:x:984:968:NFS Service User:/dev/null:/dev/null
nm-openconnect:x:977:955:NetworkManager user for OpenConnect:/var/lib/nm-openconnect:/sbin/nologin
_nobody99:x:99:99:Nobody:/dev/null:/dev/null
nobody:x:65534:65534:Linux Kernel overflowuid:/dev/null:/dev/null
nscd:x:28:28:NSCD Daemon:/:/dev/null
openvpn:x:978:956:OpenVPN daemon:/dev/null:/dev/null
pcscd:x:949:949:PC/SC Smart Card Daemon:/:/sbin/nologin
pipewire:x:981:964:PipeWire System Daemon:/:/dev/null
polkitd:x:995:983:User for polkitd:/:/dev/null
popa3d:x:43:43:POP3 daemon:/dev/null:/dev/null
postfix:x:42:42:Postfix Mail Transport Agent:/var/spool/postfix:/dev/null
postgres:x:46:46:PostgreSQL Server:/var/lib/pgsql:/dev/null
root:x:0:0:System Administrator:/root:/bin/bash
rpcuser:x:29:29:RPC Service User:/var/lib/nfs:/dev/null
rpc:x:32:32:Portmapper RPC user:/:/dev/null
rtkit:x:982:965:RealtimeKit:/proc:/sbin/nologin
squid:x:23:23:Squid User:/var/spool/squid:/dev/null
sshd:x:976:954::/var/empty:/dev/null
stapler-builder:x:975:952:Stapler Builder:/var/cache/stplr:/sbin/nologin
systemd-oom:x:992:978:systemd Userspace OOM Killer:/var/empty:/dev/null
_teamd:x:993:981:teamd user:/dev/null:/dev/null
tss:x:997:985:TPM2 Software Stack User:/var/empty:/dev/null
usbmux:x:985:969:USB Multiplex Daemon:/var/empty:/dev/null
uucp:x:10:14:uucp:/var/spool/uucp:/dev/null
_wsdd:x:983:967:Web Service Discovery host daemon:/:/sbin/nologin
_xfsscrub:x:989:976:Special User for the XFS Scrub service:/:/bin/false
xfs:x:44:44:X Font Server:/etc/X11/fs:/dev/null
`)

func TestSplitPasswdReal(t *testing.T) {
	entries, err := ParsePasswd(realPasswd)
	if err != nil {
		t.Fatalf("ParsePasswd: %v", err)
	}

	if len(entries) != 56 {
		t.Fatalf("expected 56 entries, got %d", len(entries))
	}

	etc, lib := SplitPasswd(entries)

	etcNames := names(etc)
	expectNames(t, etcNames, []string{"dm", "root"}, "/etc")

	if len(lib) != 54 {
		t.Errorf("/usr/lib: expected 54 entries, got %d", len(lib))
	}

	foundNobody := false
	for _, e := range lib {
		if e.Name == "nobody" {
			foundNobody = true
			if e.UID != 65534 {
				t.Errorf("nobody UID: got %d, want 65534", e.UID)
			}
		}
	}
	if !foundNobody {
		t.Error("nobody should be in /usr/lib")
	}

	for _, e := range lib {
		if e.Name == "root" || e.Name == "dm" {
			t.Errorf("%s should not be in /usr/lib", e.Name)
		}
	}

	for _, e := range entries {
		reparsed, err := ParsePasswd([]byte(e.String()))
		if err != nil {
			t.Errorf("roundtrip failed for %s: %v", e.Name, err)
		}
		if len(reparsed) != 1 || reparsed[0] != e {
			t.Errorf("roundtrip mismatch for %s", e.Name)
		}
	}
}

func TestSplitPasswdIdempotentReal(t *testing.T) {
	entries, _ := ParsePasswd(realPasswd)
	etc1, lib1 := SplitPasswd(entries)

	// Имитация повторного запуска: объединяем и разделяем снова
	merged := MergePasswd(etc1, lib1)
	etc2, lib2 := SplitPasswd(merged)

	expectNames(t, names(etc2), names(etc1), "idempotent /etc")
	if len(lib2) != len(lib1) {
		t.Errorf("idempotent /usr/lib: got %d entries, want %d", len(lib2), len(lib1))
	}
}

func names[T interface{ getName() string }](entries []T) []string {
	var result []string
	for _, e := range entries {
		result = append(result, e.getName())
	}
	return result
}

func (e PasswdEntry) getName() string { return e.Name }
func (e GroupEntry) getName() string  { return e.Name }

func expectNames(t *testing.T, got, want []string, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %v, want %v", label, got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", label, i, got[i], want[i])
		}
	}
}

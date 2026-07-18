package modules

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetworkBody_WritesFiles(t *testing.T) {
	dir := t.TempDir()
	hostnamePath := filepath.Join(dir, "hostname")
	hostsPath := filepath.Join(dir, "hosts")

	// redirect target files, restore afterwards
	origHostname, origHosts := EtcHostname, EtcHosts
	EtcHostname, EtcHosts = hostnamePath, hostsPath
	t.Cleanup(func() { EtcHostname, EtcHosts = origHostname, origHosts })

	b := &NetworkBody{Hostname: "myhost"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}

	if got := mustReadFile(t, hostnamePath); got != "myhost\n" {
		t.Errorf("hostname = %q, want %q", got, "myhost\n")
	}
	hosts := mustReadFile(t, hostsPath)
	if !strings.Contains(hosts, "127.0.0.1 localhost myhost") {
		t.Errorf("hosts missing ipv4 line: %q", hosts)
	}
	if !strings.Contains(hosts, "::1 localhost6 myhost6") {
		t.Errorf("hosts missing ipv6 line: %q", hosts)
	}
}

package lint

import "testing"

func TestPolicyFor(t *testing.T) {
	tests := []struct {
		path     string
		expected pathPolicy
	}{
		// default — консервация только в /var
		{"/var/lib/myapp/state.db", policyDefault},
		{"/var/games/scores", policyDefault},

		// skip-content
		{"/var/cache/apt", policySkipContent},
		{"/var/cache/apt/archives", policySkipContent},
		{"/var/tmp", policySkipContent},
		{"/etc/skel", policySkipContent},
		{"/etc/skel.ru_RU.KOI8-R", policySkipContent}, // сырой префикс skel*
		{"/etc/skel/.bashrc", policySkipContent},

		// skip-files
		{"/var/lib/rpm", policySkipFiles},
		{"/var/lib/rpm/Packages", policySkipFiles},
		{"/var/lib/apt/lists/lock", policySkipFiles},
		{"/var/lib/apm/apm.db", policySkipFiles},
		{"/etc/ld.so.cache", policySkipFiles},
		{"/etc/.pwd.lock", policySkipFiles},

		// no-factory
		{"/var/log/lastlog", policyNoFactory},
		{"/var/spool/mail/root", policyNoFactory},
		{"/var/cache/ldconfig/aux-cache", policyNoFactory},
		// весь /etc — z без консервации, заводское состояние держит ostree
		{"/etc/hosts", policyNoFactory},
		{"/etc/pam.d/login", policyNoFactory},
		{"/etc/machine-id", policyNoFactory},
		{"/etc/passwd", policyNoFactory},
		{"/etc/shadow", policyNoFactory},

		// самое длинное совпадение: /var/cache/apt (skip-content) против /var/cache (no-factory)
		{"/var/cache/apt/cache.bin", policySkipContent},

		// граница сегмента: /var/lib/rpmnew не должен матчиться на /var/lib/rpm
		{"/var/lib/rpmnew", policyDefault},

		// машинное состояние и chroot-резолвер — без записей для файлов
		{"/var/lib/systemd/random-seed", policySkipFiles},
		{"/var/lib/systemd/catalog/database", policySkipFiles},
		{"/var/lib/logrotate/status", policySkipFiles},
		{"/var/resolv/lib64/libnss_files.so.2", policySkipFiles},
		{"/etc/udev/hwdb.bin", policySkipFiles},

		// суффиксы: остатки rpm ловятся и в /var
		{"/var/lib/myapp/state.db.rpmsave", policyNoFactory},
	}

	for _, tt := range tests {
		if got := policyFor(tt.path); got != tt.expected {
			t.Errorf("policyFor(%q) = %d, want %d", tt.path, got, tt.expected)
		}
	}
}
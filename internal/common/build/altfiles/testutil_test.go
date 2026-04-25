package altfiles

import "path/filepath"

func newTestService(dir string) *Service {
	return New(Config{
		EtcPasswd:   filepath.Join(dir, "etc_passwd"),
		EtcGroup:    filepath.Join(dir, "etc_group"),
		EtcNsswitch: filepath.Join(dir, "nsswitch.conf"),
		LibPasswd:   filepath.Join(dir, "lib_passwd"),
		LibGroup:    filepath.Join(dir, "lib_group"),
	})
}

package etcfiles

const (
	// systemUIDMax граница системных UID/GID (обычные пользователи >= 1000)
	systemUIDMax = 999
	// nobodyUID специальный nobody/nogroup ID
	nobodyUID = 65534
)

// IsSystemID проверяет что UID/GID является системным (1-999 или nobody 65534), но не root (0)
func IsSystemID(id int) bool {
	return (id > 0 && id <= systemUIDMax) || id == nobodyUID
}

// IsRegularUser возвращает true для root (UID 0) и реальных пользователей (UID 1000-60000)
func IsRegularUser(uid int) bool {
	return uid == 0 || (uid >= 1000 && uid <= 60000)
}

// IsRegularGroup возвращает true для root (GID 0), wheel и реальных групп (GID 1000-60000)
func IsRegularGroup(name string, gid int) bool {
	if gid == 0 || name == "wheel" {
		return true
	}
	return gid >= 1000 && gid <= 60000
}

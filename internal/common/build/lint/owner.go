package lint

import (
	"apm/internal/common/app"
	"fmt"
	"io/fs"
	"os/user"
	"strconv"
	"sync"
	"syscall"
)

// formatMode возвращает права в восьмеричном виде для tmpfiles.d, включая спецбиты setuid/setgid/sticky.
func formatMode(m fs.FileMode) string {
	mode := uint32(m.Perm())
	if m&fs.ModeSetuid != 0 {
		mode |= 0o4000
	}
	if m&fs.ModeSetgid != 0 {
		mode |= 0o2000
	}
	if m&fs.ModeSticky != 0 {
		mode |= 0o1000
	}
	return fmt.Sprintf("%04o", mode)
}

var ownerCache = struct {
	sync.RWMutex
	users  map[uint32]string
	groups map[uint32]string
}{
	users:  make(map[uint32]string),
	groups: make(map[uint32]string),
}

func lookupOwner(info fs.FileInfo) (string, string) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		app.Log.Fatal("unexpected: Stat_t unavailable")
	}
	return cachedUser(stat.Uid), cachedGroup(stat.Gid)
}

func cachedUser(uid uint32) string {
	ownerCache.RLock()
	if name, ok := ownerCache.users[uid]; ok {
		ownerCache.RUnlock()
		return name
	}
	ownerCache.RUnlock()

	idStr := strconv.FormatUint(uint64(uid), 10)
	name := idStr
	if u, err := user.LookupId(idStr); err == nil {
		name = u.Username
	}

	ownerCache.Lock()
	ownerCache.users[uid] = name
	ownerCache.Unlock()
	return name
}

func cachedGroup(gid uint32) string {
	ownerCache.RLock()
	if name, ok := ownerCache.groups[gid]; ok {
		ownerCache.RUnlock()
		return name
	}
	ownerCache.RUnlock()

	idStr := strconv.FormatUint(uint64(gid), 10)
	name := idStr
	if g, err := user.LookupGroupId(idStr); err == nil {
		name = g.Name
	}

	ownerCache.Lock()
	ownerCache.groups[gid] = name
	ownerCache.Unlock()
	return name
}

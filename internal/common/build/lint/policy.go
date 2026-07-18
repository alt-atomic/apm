package lint

import "strings"

// pathPolicy политика обработки пути при генерации tmpfiles.d.
type pathPolicy int

const (
	policyDefault     pathPolicy = iota // полная обработка: d/L, файлы — C + factory
	policySkipContent                   // Запись для самого каталога, содержимое не обходится
	policySkipFiles                     // Структура (d/L) остаётся, файлы без записей
	policyNoFactory                     // Файлы остаются z, без копии в factory
)

// pathRules правила обработки путей: '*' в конце — сырой префикс,
// '*' в начале — суффикс, иначе граница сегмента; выигрывает самое длинное совпадение.
var pathRules = []struct {
	prefix string
	policy pathPolicy
}{
	// содержимое управляется другими механизмами
	{"/var/home", policySkipContent},
	{"/var/root", policySkipContent},
	{"/var/cache/man", policySkipContent},
	{"/var/cache/fontconfig", policySkipContent},
	{"/var/cache/apt", policySkipContent},
	{"/var/tmp", policySkipContent},
	{"/etc/rc.d", policySkipContent},
	{"/etc/tcb", policySkipContent},
	{"/etc/alternatives/links", policySkipContent},
	{"/etc/skel*", policySkipContent},

	// базы и кэши
	{"/var/lib/rpm", policySkipFiles},
	{"/var/lib/apt", policySkipFiles},
	{"/var/lib/apm", policySkipFiles},
	{"/etc/ld.so.cache", policySkipFiles},
	{"/etc/.pwd.lock", policySkipFiles},
	{"/etc/udev/hwdb.bin", policySkipFiles},

	// машинное состояние
	{"/var/lib/systemd", policySkipFiles},
	{"/var/lib/logrotate", policySkipFiles},
	{"/var/resolv", policySkipFiles},

	// весь /etc — без консервации: в атомарных системах 3-way merge закрыты ostree/bootc composefs
	{"/etc", policyNoFactory},

	// права поддерживаем, контент не консервируем
	{"/var/log", policyNoFactory},
	{"/var/cache", policyNoFactory},
	{"/var/spool", policyNoFactory},

	// остатки rpm при обновлениях
	{"*.rpmorig", policyNoFactory},
	{"*.rpmnew", policyNoFactory},
	{"*.rpmsave", policyNoFactory},
}

// policyFor возвращает политику пути по самому длинному совпавшему правилу.
func policyFor(absPath string) pathPolicy {
	policy, bestLen := policyDefault, -1
	for _, r := range pathRules {
		if suffix, ok := strings.CutPrefix(r.prefix, "*"); ok {
			if strings.HasSuffix(absPath, suffix) && len(suffix) > bestLen {
				policy, bestLen = r.policy, len(suffix)
			}
			continue
		}
		p, raw := strings.CutSuffix(r.prefix, "*")
		matched := absPath == p || strings.HasPrefix(absPath, p) && (raw || absPath[len(p)] == '/')
		if matched && len(p) > bestLen {
			policy, bestLen = r.policy, len(p)
		}
	}
	return policy
}

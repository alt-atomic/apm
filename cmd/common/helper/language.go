package helper

import (
	"os"
	"strings"
)

func GetSystemLocale() string {
	if v := os.Getenv("LC_ALL"); v != "" {
		return stripAfterDot(v)
	}
	if v := os.Getenv("LC_MESSAGES"); v != "" {
		return stripAfterDot(v)
	}
	return stripAfterDot(os.Getenv("LANG"))
}

// stripAfterDot возвращает строку до точки, если точка есть.
// Пример: "ru_RU.UTF-8" -> "ru_RU".
func stripAfterDot(locale string) string {
	if idx := strings.Index(locale, "."); idx != -1 {
		return locale[:idx]
	}
	return locale
}

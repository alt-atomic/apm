package helper

import (
	"golang.org/x/text/language"
	"os"
	"strings"
)

// GetSystemLocale возвращает базовый язык системы в виде language.Tag.
func GetSystemLocale() language.Tag {
	var locale string
	if v := os.Getenv("LC_ALL"); v != "" {
		locale = stripAfterDot(v)
	} else if v := os.Getenv("LC_MESSAGES"); v != "" {
		locale = stripAfterDot(v)
	} else {
		locale = stripAfterDot(os.Getenv("LANG"))
	}

	// Приводим строку к формату BCP 47 (заменяем "_" на "-").
	locale = strings.Replace(locale, "_", "-", 1)
	tag, err := language.Parse(locale)
	if err != nil {
		return language.English
	}

	base, _ := tag.Base()
	return language.Make(base.String())
}

func stripAfterDot(locale string) string {
	if idx := strings.Index(locale, "."); idx != -1 {
		return locale[:idx]
	}
	return locale
}

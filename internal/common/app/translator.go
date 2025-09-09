// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package app

import (
	"os"
	"strings"

	"golang.org/x/text/language"

	"github.com/leonelquinteros/gotext"
)

// translatorImpl реализация Translator
type translatorImpl struct {
	localesPath string
	initialized bool
}

// NewTranslator создает новый переводчик
func NewTranslator(localesPath string) Translator {
	return &translatorImpl{
		localesPath: localesPath,
	}
}

// initLocales инициализирует систему переводов
func (t *translatorImpl) initLocales() {
	if t.initialized {
		return
	}

	if _, err := os.Stat(t.localesPath); os.IsNotExist(err) {
		Log.Warning("Translations folder not found at path: " + t.localesPath)
	}

	gotext.Configure(t.localesPath, GetSystemLocale().String(), "apm")
	t.initialized = true
}

// T_ возвращает переведенную строку
func (t *translatorImpl) T_(messageID string) string {
	t.initLocales()
	return gotext.Get(messageID)
}

// TN_ возвращает переведенную строку с поддержкой множественного числа
func (t *translatorImpl) TN_(messageID string, pluralMessageID string, count int) string {
	t.initLocales()
	return gotext.GetN(messageID, pluralMessageID, count)
}

// GetSystemLocale возвращает базовый язык системы в виде language.Tag.
func GetSystemLocale() language.Tag {
	var localeStr string
	if v := os.Getenv("LC_ALL"); v != "" {
		localeStr = stripAfterDot(v)
	} else if v := os.Getenv("LC_MESSAGES"); v != "" {
		localeStr = stripAfterDot(v)
	} else {
		localeStr = stripAfterDot(os.Getenv("LANG"))
	}

	// Приводим строку к формату BCP 47 (заменяем "_" на "-").
	localeStr = strings.Replace(localeStr, "_", "-", 1)
	tag, err := language.Parse(localeStr)
	if err != nil {
		return language.English
	}

	base, _ := tag.Base()
	return language.Make(base.String())
}

func stripAfterDot(localeStr string) string {
	if idx := strings.Index(localeStr, "."); idx != -1 {
		return localeStr[:idx]
	}
	return localeStr
}

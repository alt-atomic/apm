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

package lib

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/language"

	"github.com/leonelquinteros/gotext"
)

// InitLocales инициализирует локаль с доменом "apm".
func InitLocales() {
	if _, err := os.Stat(Env.PathLocales); os.IsNotExist(err) {
		textError := fmt.Sprintf(T_("Translations folder not found at path: %s"), Env.PathLocales)
		Log.Warning(textError)
	}

	gotext.Configure(Env.PathLocales, GetSystemLocale().String(), "apm")
}

// T_ T возвращает переведенную строку для заданного messageID.
func T_(messageID string) string {
	return gotext.Get(messageID)
}

func TN_(messageID string, pluralMessageID string, count int) string {
	return gotext.GetN(messageID, pluralMessageID, count)
}

//func TC_(messageID string, context string) string {
//	return gotext.GetC(messageID, context)
//}
//
//func TD_(domain string, messageID string) string {
//	return gotext.GetD(domain, messageID)
//}

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

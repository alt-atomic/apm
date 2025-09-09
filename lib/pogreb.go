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

package lib1

import (
	"os"
	"sync"

	"github.com/akrylysov/pogreb"
)

var (
	dbInstanceKv *pogreb.DB
	onceKeyValue sync.Once
)

func InitKeyValue() {
	onceKeyValue.Do(func() {
		dbFile := Env.PathDBKV
		if _, err := os.Stat(dbFile); os.IsNotExist(err) {
			Log.Warning(T_("Database file not found. It will be created automatically."))
		}

		var err error
		dbInstanceKv, err = pogreb.Open(dbFile, nil)
		if err != nil {
			Log.Error(T_("Error opening database: %v"), err)
		}
	})
}

func CheckDBKv() *pogreb.DB {
	return dbInstanceKv
}

func GetDBKv() *pogreb.DB {
	if dbInstanceKv == nil {
		InitKeyValue()
	}
	return dbInstanceKv
}

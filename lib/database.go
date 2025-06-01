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
	"database/sql"
	"os"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

var (
	dbInstance *sql.DB
	once       sync.Once
)

// InitDatabase инициализирует базу данных один раз
func InitDatabase(isSystemBD bool) {
	once.Do(func() {
		dbFile := Env.PathDBSQLUser
		if isSystemBD {
			dbFile = Env.PathDBSQLSystem
		}

		if _, err := os.Stat(dbFile); os.IsNotExist(err) {
			Log.Warning(T_("Database file not found. It will be created automatically."))
		}

		var err error
		dbInstance, err = sql.Open("sqlite3", dbFile)
		if err != nil {
			Log.Fatal(T_("Error opening database: %v"), err)
		}

		if err = dbInstance.Ping(); err != nil {
			Log.Fatal(T_("Error connecting to database: %v"), err)
		}
	})
}

func CheckDB() *sql.DB {
	return dbInstance
}

// GetDB возвращает экземпляр базы данных
func GetDB(isSystemBD bool) *sql.DB {
	if dbInstance == nil {
		InitDatabase(isSystemBD)
	}
	return dbInstance
}

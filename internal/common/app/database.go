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
	"database/sql"
	"fmt"
	"os"
	"sync"

	"github.com/akrylysov/pogreb"
	_ "github.com/mattn/go-sqlite3"
)

// databaseManagerImpl реализация DatabaseManager
type databaseManagerImpl struct {
	systemDB   *sql.DB
	userDB     *sql.DB
	keyValueDB *pogreb.DB

	systemPath string
	userPath   string
	kvPath     string

	mutex sync.RWMutex

	systemOnce sync.Once
	userOnce   sync.Once
	kvOnce     sync.Once
}

// NewDatabaseManager создает новый менеджер баз данных
func NewDatabaseManager(systemPath, userPath, kvPath string) DatabaseManager {
	return &databaseManagerImpl{
		systemPath: systemPath,
		userPath:   userPath,
		kvPath:     kvPath,
	}
}

// GetSystemDB возвращает системную базу данных с ленивой инициализацией
func (dm *databaseManagerImpl) GetSystemDB() *sql.DB {
	dm.systemOnce.Do(func() {
		if err := dm.initSystemDB(); err != nil {
			Log.Fatal("Failed to initialize system DB: ", err)
		}
	})
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	return dm.systemDB
}

// GetUserDB возвращает пользовательскую базу данных с ленивой инициализацией
func (dm *databaseManagerImpl) GetUserDB() *sql.DB {
	dm.userOnce.Do(func() {
		if err := dm.initUserDB(); err != nil {
			Log.Fatal("Failed to initialize user DB: ", err)
		}
	})
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	return dm.userDB
}

// GetKeyValueDB возвращает ключ-значение базу данных с ленивой инициализацией
func (dm *databaseManagerImpl) GetKeyValueDB() *pogreb.DB {
	dm.kvOnce.Do(func() {
		if err := dm.initKeyValueDB(); err != nil {
			Log.Fatal("Failed to initialize key-value DB: ", err)
		}
	})
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	return dm.keyValueDB
}

// initSystemDB инициализирует системную базу данных
func (dm *databaseManagerImpl) initSystemDB() error {
	if _, err := os.Stat(dm.systemPath); os.IsNotExist(err) {
		Log.Warning("System database file not found. It will be created automatically.")
	}

	db, err := sql.Open("sqlite3", dm.systemPath)
	if err != nil {
		return fmt.Errorf(T_("error opening system database: %w"), err)
	}

	if err = db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf(T_("error connecting to system database: %w"), err)
	}

	dm.systemDB = db
	return nil
}

// initUserDB инициализирует пользовательскую базу данных
func (dm *databaseManagerImpl) initUserDB() error {
	if _, err := os.Stat(dm.userPath); os.IsNotExist(err) {
		Log.Warning("User database file not found. It will be created automatically.")
	}

	db, err := sql.Open("sqlite3", dm.userPath)
	if err != nil {
		return fmt.Errorf(T_("error opening user database: %w"), err)
	}

	if err = db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf(T_("error connecting to user database: %w"), err)
	}

	dm.userDB = db
	return nil
}

// initKeyValueDB инициализирует ключ-значение базу данных
func (dm *databaseManagerImpl) initKeyValueDB() error {
	if _, err := os.Stat(dm.kvPath); os.IsNotExist(err) {
		Log.Warning("Key-value database file not found. It will be created automatically.")
	}

	db, err := pogreb.Open(dm.kvPath, nil)
	if err != nil {
		return fmt.Errorf(T_("error opening key-value database: %w"), err)
	}

	dm.keyValueDB = db
	return nil
}

// Close закрывает все подключения к базам данных
func (dm *databaseManagerImpl) Close() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if dm.keyValueDB != nil {
		if err := dm.keyValueDB.Close(); err != nil {
			Log.Error(T_("Error closing KV database: "), err)
		}
		dm.keyValueDB = nil
	}

	if dm.systemDB != nil {
		if err := dm.systemDB.Close(); err != nil {
			Log.Error(T_("Error closing SQL database: "), err)
		}
		dm.systemDB = nil
	}

	if dm.userDB != nil {
		if err := dm.userDB.Close(); err != nil {
			Log.Error(T_("Error closing SQL database: "), err)
		}
		dm.userDB = nil
	}

	return nil
}

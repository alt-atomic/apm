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
func InitDatabase() {
	once.Do(func() {
		dbFile := Env.PathDBFile

		if _, err := os.Stat(dbFile); os.IsNotExist(err) {
			Log.Warning("Файл базы данных не найден. Он будет создан автоматически.")
		}

		var err error
		dbInstance, err = sql.Open("sqlite3", dbFile)
		if err != nil {
			Log.Fatal("Ошибка при открытии базы данных: %v", err)
		}

		if err = dbInstance.Ping(); err != nil {
			Log.Fatal("Ошибка при подключении к базе данных: %v", err)
		}
	})
}

// GetDB возвращает экземпляр базы данных
func GetDB() *sql.DB {
	if dbInstance == nil {
		InitDatabase()
	}
	return dbInstance
}

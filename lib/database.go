package lib

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDatabase() {
	dbFile := Env.PathDBFile

	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		Log.Warning("Файл базы данных не найден. Он будет создан автоматически.")
	}

	var err error
	DB, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		Log.Fatal("Ошибка при открытии базы данных: %v", err)
	}

	if err = DB.Ping(); err != nil {
		Log.Fatal("Ошибка при подключении к базе данных: %v", err)
	}
}

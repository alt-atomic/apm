package lib

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
			Log.Warning("Файл базы данных не найден. Он будет создан автоматически.")
		}

		var err error
		dbInstanceKv, err = pogreb.Open(dbFile, nil)
		if err != nil {
			Log.Fatal("Ошибка при открытии базы данных: %v", err)
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

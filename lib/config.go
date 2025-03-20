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
	"github.com/ilyakaznacheev/cleanenv"
	"os"
	"path/filepath"
)

type Environment struct {
	CommandPrefix string `yaml:"commandPrefix"`
	Environment   string `yaml:"environment"`
	PathLocales   string `yaml:"pathLocales"`
	PathLogFile   string `yaml:"pathLogFile"`
	PathDBSQL     string `yaml:"pathDBSQL"`
	PathDBKV      string `yaml:"pathDBKV"`
	PathImageFile string `yaml:"pathImageFile"`
	IsAtomic      bool   // Внутреннее свойство
	Format        string // Внутреннее свойство
}

var Env Environment
var DevMode bool

func InitConfig() {
	var configPath string
	if _, err := os.Stat("config.yml"); err == nil {
		configPath = "config.yml"
	} else if _, err := os.Stat("/etc/apm/config.yml"); err == nil {
		configPath = "/etc/apm/config.yml"
	} else {
		panic("Конфигурационный файл не найден ни в /etc/apm/config.yml, ни в локальной директории")
	}

	err := cleanenv.ReadConfig(configPath, &Env)
	if err != nil {
		panic(err)
	}

	DevMode = Env.Environment != "prod"

	// Проверяем и создаём путь для лог-файла
	if err = EnsurePath(Env.PathLogFile); err != nil {
		panic(err)
	}

	// Проверяем и создаём путь для db-файла sql
	if err = EnsurePath(Env.PathDBSQL); err != nil {
		panic(err)
	}

	// Проверяем и создаём путь для db-директории key-value
	if err = EnsureDir(Env.PathDBKV); err != nil {
		panic(err)
	}

	if _, errAtomic := os.Stat("/usr/bin/bootc"); os.IsNotExist(errAtomic) {
		Env.IsAtomic = false
	} else {
		Env.IsAtomic = true
	}
}

// EnsurePath проверяет, существует ли файл и создает его при необходимости.
func EnsurePath(path string) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		var file *os.File
		file, err = os.Create(path)
		if err != nil {
			return err
		}
		err = file.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// EnsureDir проверяет, существует ли директория по указанному пути, и создает её при необходимости.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

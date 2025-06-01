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
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ilyakaznacheev/cleanenv"
)

type Environment struct {
	CommandPrefix   string `yaml:"commandPrefix"`
	Environment     string `yaml:"environment"`
	PathLocales     string `yaml:"pathLocales"`
	PathLogFile     string `yaml:"pathLogFile"`
	PathDBSQLSystem string `yaml:"pathDBSQLSystem"`
	PathDBSQLUser   string `yaml:"pathDBSQLUser"`
	PathDBKV        string `yaml:"pathDBKV"`
	PathImageFile   string `yaml:"pathImageFile"`
	ExistAlr        bool   // Внутреннее свойство
	IsAtomic        bool   // Внутреннее свойство
	Format          string // Внутреннее свойство
}

var Env Environment
var DevMode bool

// Глобальные переменные для возможности переопределения значений при сборке

var BuildCommandPrefix string
var BuildEnvironment string
var BuildPathLocales string
var BuildPathLogFile string
var BuildPathDBSQLUser string
var BuildPathDBSQLSystem string
var BuildPathDBKV string
var BuildPathImageFile string

func InitConfig() {
	var configPath string

	// Переопределяем значения из ldflags, если они заданы
	if BuildCommandPrefix != "" {
		Env.CommandPrefix = BuildCommandPrefix
	}
	if BuildEnvironment != "" {
		Env.Environment = BuildEnvironment
	}
	if BuildPathLocales != "" {
		Env.PathLocales = BuildPathLocales
	}
	if BuildPathLogFile != "" {
		Env.PathLogFile = BuildPathLogFile
	}
	Env.PathDBSQLUser = "${XDG_DATA_HOME:-$HOME/.cache}/apm/apm.db"
	if BuildPathDBSQLSystem != "" {
		Env.PathDBSQLSystem = BuildPathDBSQLSystem
	}
	Env.PathDBKV = "${XDG_DATA_HOME:-$HOME/.cache}/apm/pogreb"
	if BuildPathImageFile != "" {
		Env.PathImageFile = BuildPathImageFile
	}

	// Ищем конфигурационный файл в текущей директории
	if _, err := os.Stat("config.yml"); err == nil {
		configPath = "config.yml"
	} else if _, err = os.Stat("/etc/apm/config.yml"); err == nil {
		configPath = "/etc/apm/config.yml"
	}

	DevMode = Env.Environment != "prod"

	// Если найден конфигурационный файл, читаем его
	if configPath != "" {
		err := cleanenv.ReadConfig(configPath, &Env)
		if err != nil {
			Log.Fatal(err)
		}
	}

	// расширяем анализ строк, что бы парсить переменные в путях
	Env.PathDBSQLUser = filepath.Clean(expandShellVars(Env.PathDBSQLUser))
	Env.PathDBSQLSystem = filepath.Clean(expandShellVars(Env.PathDBSQLSystem))
	Env.PathDBKV = filepath.Clean(expandShellVars(Env.PathDBKV))
	Env.PathLogFile = filepath.Clean(expandShellVars(Env.PathLogFile))

	// Проверяем и создаём путь для лог-файла
	if err := EnsurePath(Env.PathLogFile); err != nil {
		Log.Fatal(err)
	}

	// Проверяем путь к базам данных, либо для юзера, либо системная директория
	if syscall.Geteuid() != 0 {
		// Проверяем и создаём путь для db-директории key-value
		if err := EnsureDir(Env.PathDBKV); err != nil {
			Log.Fatal(err)
		}

		// Проверяем и создаём путь для db-директории SQL
		if err := EnsurePath(Env.PathDBSQLUser); err != nil {
			Log.Fatal(err)
		}
	} else {
		if err := EnsurePath(Env.PathDBSQLSystem); err != nil {
			Log.Fatal(err)
		}
	}

	if _, errAtomic := os.Stat("/usr/bin/bootc"); os.IsNotExist(errAtomic) {
		Env.IsAtomic = false
	} else {
		Env.IsAtomic = true
	}

	if _, errAlr := os.Stat("/usr/bin/alr"); os.IsNotExist(errAlr) {
		Env.ExistAlr = false
	} else {
		Env.ExistAlr = true
	}
}

// EnsurePath проверяет, существует ли файл и создает его при необходимости.
func EnsurePath(path string) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0777); err != nil {
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
	return os.MkdirAll(path, 0777)
}

func expandShellVars(s string) string {
	return os.Expand(s, func(v string) string {
		if strings.Contains(v, ":-") {
			parts := strings.SplitN(v, ":-", 2)
			name, def := parts[0], parts[1]

			if val := os.Getenv(name); val != "" {
				return val
			}
			return expandShellVars(def)
		}
		return os.Getenv(v)
	})
}

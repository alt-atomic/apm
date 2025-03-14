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
	PathDBFile    string `yaml:"pathDBFile"`
	PathImageFile string `yaml:"pathImageFile"`
	IsAtomic      bool   // Внутреннее свойство
	Language      string // Внутреннее свойство
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

	// Если environment не равен "prod", то DevMode будет true, иначе false
	DevMode = Env.Environment != "prod"

	// Проверяем и создаём путь для лог-файла
	if err := EnsurePath(Env.PathLogFile); err != nil {
		panic(err)
	}

	// Проверяем и создаём путь для db-файла
	if err := EnsurePath(Env.PathDBFile); err != nil {
		panic(err)
	}

	if _, errAtomic := os.Stat("/usr/bin/bootc"); os.IsNotExist(errAtomic) {
		Env.IsAtomic = false
	} else {
		Env.IsAtomic = true
	}
}

func EnsurePath(path string) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.Create(path)
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

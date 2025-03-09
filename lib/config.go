package lib

import (
	"github.com/ilyakaznacheev/cleanenv"
	"os"
	"path/filepath"
)

type Environment struct {
	CommandPrefix string `yaml:"commandPrefix" env:"apm.prefix"`
	Environment   string `yaml:"environment" env:"prod"`
	PathLogFile   string `yaml:"pathLogFile" env:"apm.log"`
	PathDBFile    string `yaml:"pathDBFile" env:"apm.db"`
	PathImageFile string `yaml:"pathImageFile" env:"apm.image"`
	Format        string // Внутреннее свойство
}

var Env Environment
var DevMode bool

func InitConfig() {
	err := cleanenv.ReadConfig("config.yml", &Env)
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

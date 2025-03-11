package lib

import (
	"fmt"
	"github.com/qor/i18n"
	"github.com/qor/i18n/backends/yaml"
	"os"
)

var I18n *i18n.I18n

// InitLocales инициализация переводов
func InitLocales() {
	if _, err := os.Stat(Env.PathLocales); os.IsNotExist(err) {
		textError := fmt.Sprintf("Folder i18n not found in path: %s.", Env.PathLocales)
		Log.Error(textError)
		panic(err)
	}

	yamlBackend := yaml.New(Env.PathLocales)
	I18n = i18n.New(yamlBackend)
}

// T – вспомогательная функция перевода
func T(key string, defaultValue string, args ...interface{}) string {
	if I18n == nil {
		return key
	}
	return string(I18n.Default(defaultValue).T(Env.Language, key, args...))
}

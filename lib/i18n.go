package lib

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"os"
	"path/filepath"
)

var Bundle *i18n.Bundle

// InitLocales инициализация переводов
func InitLocales() {
	if _, err := os.Stat(Env.PathLocales); os.IsNotExist(err) {
		textError := fmt.Sprintf("Папка переводов не найдена по пути: %s.", Env.PathLocales)
		Log.Error(textError)
		panic(err)
	}

	// Создание bundle с базовым языком
	Bundle = i18n.NewBundle(language.Russian)
	Bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	// Чтение списка файлов из каталога с переводами
	entries, err := os.ReadDir(Env.PathLocales)
	if err != nil {
		Log.Error(fmt.Sprintf("Ошибка чтения директории переводов: %v", err))
		panic(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(Env.PathLocales, entry.Name())
			if _, err = Bundle.LoadMessageFile(filePath); err != nil {
				Log.Warn(fmt.Sprintf("Не удалось загрузить файл переводов %s: %v", filePath, err))
			} else {
				Log.Info(fmt.Sprintf("Файл переводов загружен: %s", filePath))
			}
		}
	}
}

// T возвращает переведённую строку по идентификатору сообщения с возможностью передачи данных для шаблона.
// Если шаблонные данные не переданы, используется значение по умолчанию.
// пример: lib.T("response.package", "package", map[string]interface{}{"Count": 5})
// пример: lib.T("response.package", "package")
func T(messageID string, defaultValue string, templateData ...map[string]interface{}) string {
	var data map[string]interface{}
	if len(templateData) > 0 {
		data = templateData[0]
	} else {
		data = make(map[string]interface{})
	}

	localize := i18n.NewLocalizer(Bundle, Env.Language)

	config := &i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    messageID,
			Other: defaultValue,
		},
		TemplateData: data,
	}

	if c, ok := data["Count"]; ok {
		var countValue = 1
		switch v := c.(type) {
		case int:
			countValue = v
		case float64:
			countValue = int(v)
		default:
			countValue = 1
		}
		config.PluralCount = countValue
	}

	translation, err := localize.Localize(config)
	if err != nil {
		Log.Error(err)
		return defaultValue
	}

	return translation
}

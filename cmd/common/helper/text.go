package helper

import (
	"fmt"
	"strconv"
	"strings"
)

// DeclOfNum отвечает за склонение слов
func DeclOfNum(number int, titles []string) string {
	if number < 0 {
		number *= -1
	}

	cases := []int{2, 0, 1, 1, 1, 2}
	var currentCase int
	if number%100 > 4 && number%100 < 20 {
		currentCase = 2
	} else if number%10 < 5 {
		currentCase = cases[number%10]
	} else {
		currentCase = cases[5]
	}
	return titles[currentCase]
}

// AutoSize возвращает размер данных для int
func AutoSize(value int) string {
	mb := float64(value) / (1024 * 1024)
	return fmt.Sprintf("%.2f MB", mb)
}

// ParseBool пытается преобразовать значение к bool.
func ParseBool(val interface{}) (bool, bool) {
	switch x := val.(type) {
	case bool:
		return x, true
	case int:
		return x != 0, true
	case string:
		lower := strings.ToLower(x)
		if lower == "true" {
			return true, true
		} else if lower == "false" {
			return false, true
		}
		if iv, err := strconv.Atoi(x); err == nil {
			return iv != 0, true
		}
	}
	return false, false
}

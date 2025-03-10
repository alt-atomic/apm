package helper

import (
	"fmt"
)

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

func AutoSize(value int) string {
	mb := float64(value) / (1024 * 1024)
	return fmt.Sprintf("%.2f MB", mb)
}

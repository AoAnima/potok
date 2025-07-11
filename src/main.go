package main

import (
	"fmt"
	"github.com/rodrigocfd/windigo"
)

func main() {
	// Пример использования функции предсказания слова
	коды := []int{0x44, 0x46, 0x51, 0x57, 0x45, 0x47}
	слово, err := предсказатьСлово(коды)
	if err != nil {
		fmt.Println("Ошибка:", err)
		return
	}
	fmt.Println("

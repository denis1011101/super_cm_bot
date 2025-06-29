package messagegenerators

import (
	"fmt"
	"math/rand"
	"strings"
)

func firstUnhandsomeSet(username string, diffSize int, newSize int) string {
	messages := []string{
		"Разворачиваю сервис по поиску пидорасов ✈️",
		"ping global.pidoras.com...",
		"pong 64 bytes from \"zaebal pingovat\"...",
		"Делаю запрос на поиск 🔎",
		"О, что-то нашлось...",
		fmt.Sprintf("Ага, пидор дня @%s! Твой хуй стал короче на %d см. Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

func secondUnhandsomeSet(username string, diffSize int, newSize int) string {
	messages := []string{
		"Начинаю расследование️ 🕵️‍♂️",
		"Отправляю запрос в антипидорскую службу 📩",
		"Уточняю координаты объекта 📍",
		"Избавляюсь от свидетелей 🥷",
		fmt.Sprintf("Попался, пидор. Мой попу, @%s. Твой хуй стал короче на %d см. Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

func thirdUnhandsomeSet(username string, diffSize int, newSize int) string {
	messages := []string{
		"Сча поищу.",
		"Первым делом зайду в бар 🍺",
		"Теперь погнал в клуб 🎉",
		"Ооо тут ещё казино есть 🎰",
		"Ёбаный рот этого казино... А? Что? Пидора надо найти? Сча.",
		fmt.Sprintf("Пусть пидором будет @%s. Твой хуй стал короче на %d см. Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

var unhandsomeMessageSets []func(username string, diffSize int, newSize int) string = unhandsomeSetsFabric()

func unhandsomeSetsFabric() []func(username string, diffSize int, newSize int) string {
	return []func(username string, diffSize int, newSize int) string{
		firstUnhandsomeSet,
		secondUnhandsomeSet,
		thirdUnhandsomeSet,
	}
}

func GetRandomUnhandsomeMessage(username string, diffSize int, newSize int) string {
	spin := rand.Intn(len(unhandsomeMessageSets))
	message := unhandsomeMessageSets[spin](username, diffSize, newSize)
	return message
}

func GetSkipUnhandsomeMessage() string {
	messages := []string{
		"Бляяя опять работать...",
		"Ну давай посмотрим, что у нас тут есть.",
		"Иди нахуй, сегодня все пидоры.",
	}
	text := strings.Join(messages, "\n")
	return text
}

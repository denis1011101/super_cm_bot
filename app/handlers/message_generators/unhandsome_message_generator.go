package messagegenerators

import (
	"fmt"
	"math/rand"
	"strings"
)

func firstUnhandsomeSetHoliday(username string, diffSize int, newSize int) string {
	messages := []string{
		"Разворачиваю сервис по поиску новогодних пидорасов ✈️",
		"ping global.pidoras.com...",
		"pong 64 bytes from \"zaebal pingovat\"...",
		"Делаю запрос на поиск 🔎",
		"О, что-то нашлось...",
		fmt.Sprintf("Ага, новогодний пидор дня @%s! Твой хуй стал короче на %d см. Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

func secondUnhandsomeSetHoliday(username string, diffSize int, newSize int) string {
	messages := []string{
		"Начинаю расследование️ 🕵️‍♂️",
		"Отправляю запрос в антипидорскую службу 📩",
		"Уточняю координаты объекта 📍",
		"Избавляюсь от свидетелей 🥷",
		fmt.Sprintf("Попался, новогодний пидор. Мой попу, @%s. Твой хуй стал короче на %d см. Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

func thirdUnhandsomeSetHoliday(username string, diffSize int, newSize int) string {
	messages := []string{
		"Сча поищу.",
		"Первым делом зайду в бар 🍺",
		"Теперь погнал в клуб 🎉",
		"Ооо тут ещё казино есть 🎰",
		"Ёбаный рот этого казино... А? Что? Пидора надо найти? Сча.",
		fmt.Sprintf("Пусть новогодним пидором будет @%s. Твой хуй стал короче на %d см. Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

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
var unhandsomeMessageSetsHoliday []func(username string, diffSize int, newSize int) string = unhandsomeSetsFabricHoliday()

func unhandsomeSetsFabric() []func(username string, diffSize int, newSize int) string {
	return []func(username string, diffSize int, newSize int) string{
		firstUnhandsomeSet,
		secondUnhandsomeSet,
		thirdUnhandsomeSet,
	}
}

func unhandsomeSetsFabricHoliday() []func(username string, diffSize int, newSize int) string {
	return []func(username string, diffSize int, newSize int) string{
		firstUnhandsomeSetHoliday,
		secondUnhandsomeSetHoliday,
		thirdUnhandsomeSetHoliday,
	}
}

func GetRandomUnhandsomeMessage(username string, diffSize int, newSize int, isHoliday bool) string {
	messageSets := unhandsomeMessageSets
	if isHoliday {
		messageSets = unhandsomeMessageSetsHoliday
	}
	spin := rand.Intn(len(messageSets))
	message := messageSets[spin](username, diffSize, newSize)
	return message
}

func GetSkipUnhandsomeMessage(isHoliday bool) string {
	if isHoliday {
		messages := []string{
			"Бляяя опять работать...",
			"Ну давай посмотрим, что у нас тут есть.",
			"Иди нахуй, сегодня все пидоры. С новым годом!",
		}
		text := strings.Join(messages, "\n")
		return text
	}
	messages := []string{
		"Бляяя опять работать...",
		"Ну давай посмотрим, что у нас тут есть.",
		"Иди нахуй, сегодня все пидоры.",
	}
	text := strings.Join(messages, "\n")
	return text
}

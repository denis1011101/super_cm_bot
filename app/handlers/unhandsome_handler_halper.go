package handlers

import (
	"fmt"
	"strings"
	"math/rand"
)

func firstSet(username string, diffSize int, newSize int) string {
    messages := []string{
        "Разворачиваю сервис по поиску пидорасов ",
        "ping global.pidoras.com...",
        "pong 64 bytes from zebal pingovat\"...",
        "Делаю запрос на поиск",
        "О, что-то нашлось...",
        fmt.Sprintf("Ага пидор дня @%s! Твой хуй стал короче на %b см. Теперь он %b см.", username, diffSize, newSize),
    }
    text := strings.Join(messages, "\n")
    return text
}

func secondSet(username string, diffSize int, newSize int) string {
    messages := []string{
        "Начинаю расследование️ 🕵️‍♂️",
        "Отправляю запрос в антипидорскую службу 📩",
        "Уточняю координаты объекта 📍",
        "Избавляюсь от свидетелей 🥷",
        fmt.Sprintf("Попавший пидор. Мой попу @%s. Твой хуй стал короче на %b см. Теперь он %b см.", username, diffSize, newSize),
    }
    text := strings.Join(messages, "\n")
	return text
}

func thirdSet(username string, diffSize int, newSize int) string {
    messages := []string{
        "Сча поищу.",
        "Первым делом зайду в бар ",
        "Теперь погнал в клуб ",
        "Ооо тут ещё казино есть ",
        "Ёбаный рот этого казино... А? Что? Пидора надо найти? Сча.",
        fmt.Sprintf("Пусть пидором будет @%s. Твой хуй стал короче на %b см. Теперь он %b см.", username, diffSize, newSize),
    }
    text := strings.Join(messages, "\n")
    return text
}

var setsFabric []func(username string, diffSize int, newSize int) string = createSetsFabric()

func createSetsFabric() []func(username string, diffSize int, newSize int) string {
    return []func(username string, diffSize int, newSize int) string {
        firstSet,
        secondSet,
        thirdSet,
    }
}

func getRandomUnhandsomeMessage(username string, diffSize int, newSize int) string {
	spin := rand.Intn(4);
	message := setsFabric[spin](username, diffSize, newSize)
	return message
}
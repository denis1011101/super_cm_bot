package messagegenerators

import (
	"fmt"
	"strings"
	"math/rand"
)

func firstUnhandsomeSet(username string, diffSize int, newSize int) string {
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

func secondUnhandsomeSet(username string, diffSize int, newSize int) string {
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

func thirdUnhandsomeSet(username string, diffSize int, newSize int) string {
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

var unhandsomeMessageSets []func(username string, diffSize int, newSize int) string = unhandsomeSetsFabric()

func unhandsomeSetsFabric() []func(username string, diffSize int, newSize int) string {
    return []func(username string, diffSize int, newSize int) string {
        firstUnhandsomeSet,
        secondUnhandsomeSet,
        thirdUnhandsomeSet,
    }
}

func GetRandomUnhandsomeMessage(username string, diffSize int, newSize int) string {
	spin := rand.Intn(2);
	message := unhandsomeMessageSets[spin](username, diffSize, newSize)
	return message
}
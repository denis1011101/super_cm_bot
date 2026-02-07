package messagegenerators

import (
	"fmt"
	"math/rand"
	"strings"
)

func firstGigaSetHoliday(username string, diffSize int, newSize int) string {
	messages := []string{
		"Жи есть! Сэйчас поищем новогоднего кразавчика ☝️",
		"Эу! У кого камри 3.5? 🏎",
		"Может хотябы приора есть? 🚗",
		"Похуй. Сэйчас у пацанов поспрашиваю кто? что? как? 🤷‍♂️",
		fmt.Sprintf("Воу воу воу паприветсвуйте новогоднего хасанчика @%s!🔥 Твой хуй стал длиннее на %d см Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

func secondGigaSetHoliday(username string, diffSize int, newSize int) string {
	messages := []string{
		"Хочешь узнать кто сегодня новогодний альфа самец? 🤨",
		"Этот в цирке выступает... 🎪",
		"Тот запомнить не может. Тупой ссука.",
		"А у этого хуя даже нет 🔫",
		fmt.Sprintf("А вот и он наш волчара новогодний альфа самец @%s! 🐺🔥 Твой хуй стал длиннее на %d см. Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

func thirdGigaSetHoliday(username string, diffSize int, newSize int) string {
	messages := []string{
		"Хмм... Кто же сегодня новогодний гигачад?🎄",
		"Провожу фотосессию 📸",
		"Обрабатываю снимки 📀",
		"Анализирую фотографии 🔬",
		"Синтезирую ДНК 🧬",
		fmt.Sprintf("@%s бля реально новогодний гигачад. 🎅 Твой хуй стал длиннее на %d см. Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

func firstGigaSet(username string, diffSize int, newSize int) string {
	messages := []string{
		"Жи есть! Сэйчас поищем кразавчика ☝️",
		"Эу! У кого камри 3.5? 🏎",
		"Может хотябы приора есть? 🚗",
		"Похуй. Сэйчас у пацанов поспрашиваю кто? что? как? 🤷‍♂️",
		fmt.Sprintf("Воу воу воу паприветсвуйте хасанчика @%s!🔥 Твой хуй стал длиннее на %d см Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

func secondGigaSet(username string, diffSize int, newSize int) string {
	messages := []string{
		"Хочешь узнать кто сегодня альфа самец? 🤨",
		"Этот в цирке выступает... 🎪",
		"Тот запомнить не может. Тупой ссука.",
		"А у этого хуя даже нет 🔫",
		fmt.Sprintf("А вот и он наш волчара альфа самец @%s! 🐺🔥 Твой хуй стал длиннее на %d см. Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

func thirdGigaSet(username string, diffSize int, newSize int) string {
	messages := []string{
		"Хмм... Кто же сегодня гигачад?",
		"Провожу фотосессию 📸",
		"Обрабатываю снимки 📀",
		"Анализирую фотографии 🔬",
		"Синтезирую ДНК 🧬",
		fmt.Sprintf("@%s бля реально гигачад. Твой хуй стал длиннее на %d см. Теперь он %d см.", username, diffSize, newSize),
	}
	text := strings.Join(messages, "\n")
	return text
}

var gigaMesasgeSets []func(username string, diffSize int, newSize int) string = gigaSetsFabric()
var gigaMesasgeSetsHoliday []func(username string, diffSize int, newSize int) string = gigaSetsFabricHoliday()

func gigaSetsFabric() []func(username string, diffSize int, newSize int) string {
	return []func(username string, diffSize int, newSize int) string{
		firstGigaSet,
		secondGigaSet,
		thirdGigaSet,
	}
}

func gigaSetsFabricHoliday() []func(username string, diffSize int, newSize int) string {
	return []func(username string, diffSize int, newSize int) string{
		firstGigaSetHoliday,
		secondGigaSetHoliday,
		thirdGigaSetHoliday,
	}
}

func GetRandomGigaMessage(username string, diffSize int, newSize int, isHoliday bool) string {
	messageSets := gigaMesasgeSets
	if isHoliday {
		messageSets = gigaMesasgeSetsHoliday
	}
	spin := rand.Intn(len(messageSets))
	message := messageSets[spin](username, diffSize, newSize)
	return message
}

func GetSkipGigaMessage(isHoliday bool) string {
	if isHoliday {
		return "Я блять тут альфа! +10 000 к моему хую! Так что пошли нахуй 👿 С новым годом!"
	}
	return "Я блять тут альфа! +10 000 к моему хую! Так что пошли нахуй 👿"
}

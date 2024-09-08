package messagegenerators

import (
	"fmt"
	"math/rand"
	"strings"
)

func firstGigaSet(username string, diffSize int, newSize int) string {
    messages := []string{
        "Жи есть! Сэйчас поищем кразавчика ",
        "Эу! У кого камри 3.5? ",
        "Может хотябы приора есть? ",
        "Похуй. Сэйчас у пацанов поспрашиваю кто? что? как?",
        fmt.Sprintf("Воу воу воу паприветсвуйте хасанчика @%s! Теперь он %d см.", username, newSize),
    }
    text := strings.Join(messages, "\n")
    return text
}

func secondGigaSet(username string, diffSize int, newSize int) string {
    messages := []string{
        "Хочешь узнать кто сегодня альфа самец? ",
        "Этот в цирке выступает... ",
        "Тот запомнить не может. Тупой ссука.",
        "А у этого хуя даже нет ",
        fmt.Sprintf("А вот и он наш волчара альфа самец @%s! Теперь он %d см.", username, newSize),
    }
    text := strings.Join(messages, "\n")
    return text
}

func thirdGigaSet(username string, diffSize int, newSize int) string {
    messages := []string{
        "Хмм... Кто же сегодня гигачад?",
        "Провожу фотосессию ",
        "Обрабатываю снимки ",
        "Анализирую фотографии ",
        "Синтезирую ДНК ",
        fmt.Sprintf("@%s бля реально гигачад. Теперь он %d см.", username, newSize),
    }
    text := strings.Join(messages, "\n")
    return text
}

var gigaMesasgeSets []func(username string, diffSize int, newSize int) string = gigaSetsFabric()

func gigaSetsFabric() []func(username string, diffSize int, newSize int) string {
    return []func(username string, diffSize int, newSize int) string {
        firstGigaSet,
        secondGigaSet,
        thirdGigaSet,
    }
}

func GetRandomGigaMessage(username string, diffSize int, newSize int) string {
	spin := rand.Intn(4);
	message := gigaMesasgeSets[spin](username, diffSize, newSize)
	return message
}
package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/denis1011101/super_cm_bot/app"
	"github.com/denis1011101/super_cm_bot/app/handlers"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/josestg/lazy"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/natefinch/lumberjack.v2"
)

// main создаёт бота и слушает обновления
func main() {
	// Указываем путь к папке для логов
	const logDir = "logs"
	logFilePath := filepath.Join(logDir, "bot.log")

	// Создаем папку, если она не существует
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	// Настройка ротации логов
	log.SetOutput(&lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10,   // Максимальный размер файла в мегабайтах
		MaxBackups: 3,    // Максимальное количество старых файлов
		MaxAge:     28,   // Максимальное количество дней хранения старых файлов
		Compress:   true, // Сжатие старых файлов
	})

	// Настройка логирования SQLite
	os.Setenv("SQLITE_TRACE", "1")
	os.Setenv("SQLITE_TRACE_FILE", logFilePath)

	// Открываем файл для записи логов
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	// Настраиваем логгер для записи в файл
	log.SetOutput(logFile)

	// Пример логирования
	log.Println("This is a log message")

	err = godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatalf("BOT_TOKEN is not set in .env file")
	}

	bot, updatesChannel := app.ConfigureBot(botToken)

	db, err := app.InitDB()
	if err != nil {
		log.Fatal("Ошибка инициализации базы данных: ", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatal("Ошибка закрытия базы данных: ", err)
		}
	}()

	// Создаем мьютекс для блокировки базы данных
	mutex := &sync.Mutex{}

	// Вызов функции резервного копирования в отдельной горутине
	app.StartBackupRoutine(db, mutex)

	// Вызов функции проверки не обнулилась ли база в отдельной горутине
	app.CheckPenLength(db)

	// Обработчики команд
	commandHandlers := map[string]func(tgbotapi.Update, *tgbotapi.BotAPI, *sql.DB){
		"/pen@super_cum_lovers_bot":           handlers.HandleSpin,
		"/pen":                                handlers.HandleSpin,
		"/giga@super_cum_lovers_bot":          handlers.ChooseGiga,
		"/giga":                               handlers.ChooseGiga,
		"/unhandsome@super_cum_lovers_bot":    handlers.ChooseUnhandsome,
		"/unh":                                handlers.ChooseUnhandsome,
		"/topLength@super_cum_lovers_bot":     handlers.TopLength,
		"/topLen":                             handlers.TopLength,
		"/topGiga@super_cum_lovers_bot":       handlers.TopGiga,
		"/topGiga":                            handlers.TopGiga,
		"/topUnhandsome@super_cum_lovers_bot": handlers.TopUnhandsome,
		"/topUnh":                             handlers.TopUnhandsome,
	}

	// Обработка обновлений
	for update := range updatesChannel {
		needHandleCommand, commandHandler := isNeedHandleCommand(update, commandHandlers)
		if needHandleCommand {
			commandHandler(update, bot, db)
			continue
		}

		if isNeedHandleReaction(update) {
			handlers.HandleReaction(update.MessageReaction, bot, db)
			continue
		}

		if isNeedHandleBotAddition(update) {
			handlers.HandleBotAddition(update, bot)
			continue
		}

		if isHandleOrdinaryMessage(update) {
			handlers.HandleOrdinaryMessage(update, bot, db)
			continue
		}
	}
}

var specificChatIDFactory = lazy.New(getSpecificChatId)

func isNeedHandleCommand(update tgbotapi.Update, commandHandlers map[string]func(tgbotapi.Update, *tgbotapi.BotAPI, *sql.DB)) (bool, func(tgbotapi.Update, *tgbotapi.BotAPI, *sql.DB)) {
	if update.Message != nil && update.Message.Chat.ID == specificChatIDFactory.Value() {
		if handler, exists := commandHandlers[update.Message.Text]; exists {
			return true, handler
		}
	}
	return false, nil
}

func isNeedHandleReaction(update tgbotapi.Update) bool {
	return update.MessageReaction != nil && update.MessageReaction.Chat.ID == specificChatIDFactory.Value()
}

func isNeedHandleBotAddition(update tgbotapi.Update) bool {
	return update.MyChatMember != nil && update.Message != nil
}

func isHandleOrdinaryMessage(update tgbotapi.Update) bool {
	return update.Message != nil
}

func getSpecificChatId() (int64, error) {
	specificChatIDStr := os.Getenv("SPECIFIC_CHAT_ID")
	if specificChatIDStr == "" {
		log.Fatalf("SPECIFIC_CHAT_ID is not set in .env file")
		return 0, nil
	}

	specificChatID, err := strconv.ParseInt(specificChatIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid SPECIFIC_CHAT_ID: %v", err)
		return 0, err
	}
	return specificChatID, nil
}

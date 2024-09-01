package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/denis1011101/super_cum_bot/app"
	"github.com/denis1011101/super_cum_bot/app/handlers"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/natefinch/lumberjack.v2"
)

// main создаёт бота и слушает обновления
func main() {
	// Указываем путь к папке для логов
	logDir := "logs"
	logFilePath := filepath.Join(logDir, "bot.log")

	// Создаем папку, если она не существует
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	// Открываем файл для записи логов
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	// Настраиваем логгер для записи в файл
	log.SetOutput(logFile)

	// Настройка ротации логов
	log.SetOutput(&lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10, // Максимальный размер файла в мегабайтах
		MaxBackups: 3,  // Максимальное количество старых файлов
		MaxAge:     28, // Максимальное количество дней хранения старых файлов
		Compress:   true, // Сжатие старых файлов
	})

	err = godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatalf("BOT_TOKEN is not set in .env file")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	specificChatIDStr := os.Getenv("SPECIFIC_CHAT_ID")
    if specificChatIDStr == "" {
        log.Fatalf("SPECIFIC_CHAT_ID is not set in .env file")
    }

	specificChatID, err := strconv.ParseInt(specificChatIDStr, 10, 64)
    if err != nil {
        log.Fatalf("Invalid SPECIFIC_CHAT_ID: %v", err)
    }

	// Пример логирования
	log.Println("This is a log message")

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60

    updates := bot.GetUpdatesChan(u)

	db, err := app.InitDB()
    if err != nil {
        log.Fatal("Ошибка инициализации базы данных: ", err)
    }
    defer func() {
        if err := db.Close(); err != nil {
            log.Fatal("Ошибка закрытия базы данных: ", err)
        }
    }()

    // Запуск резервного копирования в отдельной горутине
    go func() {
        // Настройка таймера для выполнения раз в день
        ticker := time.NewTicker(24 * time.Hour)
        defer ticker.Stop()

        for range ticker.C {
            app.BackupDatabase(db)
        }
    }()

	// Обработчики команд
	commandHandlers := map[string]func(tgbotapi.Update, *tgbotapi.BotAPI, *sql.DB){
		"/pen":           handlers.HandleSpin,
		"/giga":          handlers.ChooseGiga,
		"/unhandsome":    handlers.ChooseUnhandsome,
		"/topLength":     handlers.TopLength,
		"/topGiga":       handlers.TopGiga,
		"/topUnhandsome": handlers.TopUnhandsome,
	}

	// Обработка обновлений
	for update := range updates {
		if update.Message != nil {
			chatID := update.Message.Chat.ID
	        if chatID == specificChatID {
				// Обработка команд
				if handler, exists := commandHandlers[update.Message.Text]; exists {
					handler(update, bot, db)
				} else { // Обработка обычных сообщений
					handlers.HandlePenCommand(update, bot, db)
				}
			} else if update.MyChatMember != nil { // Обработка добавления бота в чат
				handlers.HandleBotAddition(update, bot)
			}
		}
	}
}

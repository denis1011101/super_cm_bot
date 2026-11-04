package main

import (
	"database/sql"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	"github.com/denis1011101/super_cm_bot/app/agentbus"
	"github.com/denis1011101/super_cm_bot/app/handlers"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/natefinch/lumberjack.v2"
)

// agentBusTTL is how long a published status stays current. It is a few times
// the bot's own reporting cadence, so a peer is marked stale only after it has
// genuinely stopped talking, not after one slow call.
const agentBusTTL = 15 * time.Minute

// klewoRe матчит паттерн "@юзер клево/клёво" с необязательными знаками препинания в конце
var klewoRe = regexp.MustCompile(`@\S+\s+кл[её]во[!?]?`)

// sosalRe матчит короткие утвердительные слова/фразы
var sosalRe = regexp.MustCompile(`(?i)^(да|da|угу|ага|ну да|конечно|естественно|разумеется|точно|реально|всё так|все так|так|именно|верно|йеп|еп|ок|окей|yes|yep|yeah|yea|sure|ладно|допустим)\s*\?\s*$`)

// main создаёт бота и слушает обновления
func main() {
	// Указываем путь к папке для логов
	logDir := "logs"
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
	if err := os.Setenv("SQLITE_TRACE", "1"); err != nil {
		log.Printf("Failed to set SQLITE_TRACE env: %v", err)
	}
	if err := os.Setenv("SQLITE_TRACE_FILE", logFilePath); err != nil {
		log.Printf("Failed to set SQLITE_TRACE_FILE env: %v", err)
	}

	// Открываем файл для записи логов
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			log.Printf("Error closing log file: %v", err)
		}
	}()

	// Настраиваем логгер для записи в файл
	log.SetOutput(logFile)

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

	// Свои реплики бот должен видеть в контексте Gemini, хоть и не отвечает на них
	app.EnableOutgoingMemory(db)

	// Создаем мьютекс для блокировки базы данных
	mutex := &sync.Mutex{}

	// Вызов функции резервного копирования в отдельной горутине
	app.StartBackupRoutine(db, mutex)

	// Вызов функции проверки не обнулилась ли база в отдельной горутине
	app.CheckPenLength(db)

	// Запуск еженедельного актуализирования pen_name
	app.StartPenNameUpdateRoutine(db, bot)

	// Запуск еженедельного архивирования неактивных пользователей
	app.StartArchiveRoutine(db)

	// Запуск ежедневных команд в случайное время
	app.StartDailyCommandsRoutine(db, bot, specificChatID, handlers.ChooseGiga, handlers.ChooseUnhandsome)

	// Ночная очистка памяти Gemini
	app.StartGeminiMemoryCleanupRoutine(db, nil)

	geminiAgent := app.NewGeminiAgent(db, bot)
	startAgentBus(geminiAgent, bot.Self.UserName)
	geminiAgent.SetAutoCommandHandler(func(source tgbotapi.Message, cmd string) {
		synthetic := source
		synthetic.Text = cmd
		update := tgbotapi.Update{Message: &synthetic}

		switch cmd {
		case "/giga":
			handlers.ChooseGiga(update, bot, db)
		case "/unh":
			handlers.ChooseUnhandsome(update, bot, db)
		}
	})

	// Обработчики команд
	commandHandlers := map[string]func(tgbotapi.Update, *tgbotapi.BotAPI, *sql.DB){
		"/pen@super_cum_lovers_bot":           handlers.HandleSpin,
		"/pen":                                handlers.HandleSpin,
		"/giga@super_cum_lovers_bot":          handlers.ChooseGiga,
		"/giga":                               handlers.ChooseGiga,
		"/unhandsome@super_cum_lovers_bot":    handlers.ChooseUnhandsome,
		"/unh":                                handlers.ChooseUnhandsome,
		"/toppens":                            handlers.TopLength,
		"/toppens@super_cum_lovers_bot":       handlers.TopLength,
		"/toplength@super_cum_lovers_bot":     handlers.TopLength,
		"/toplen":                             handlers.TopLength,
		"/topgiga@super_cum_lovers_bot":       handlers.TopGiga,
		"/topgiga":                            handlers.TopGiga,
		"/topunhandsome@super_cum_lovers_bot": handlers.TopUnhandsome,
		"/topunh":                             handlers.TopUnhandsome,
		"/memory@super_cum_lovers_bot":        handlers.ShowMyGeminiFacts,
		"/memory":                             handlers.ShowMyGeminiFacts,
		"/myfacts@super_cum_lovers_bot":       handlers.ShowMyGeminiFacts,
		"/myfacts":                            handlers.ShowMyGeminiFacts,
		"/forgetme@super_cum_lovers_bot":      handlers.ForgetMyGeminiFacts,
		"/forgetme":                           handlers.ForgetMyGeminiFacts,
	}

	// Обработка обновлений
	for update := range updates {
		if update.Message != nil {
			chatID := update.Message.Chat.ID
			if chatID == specificChatID {
				// Актуализируем список участников: по нему факты ИИ
				// связываются с людьми, даже если о них пишут в третьем лице
				app.RememberChatMember(db, chatID, update.Message.From)

				// Обработка команд
				if handler, exists := commandHandlers[update.Message.Text]; exists {
					app.RecordCommandMemory(db, chatID, update.Message.From, update.Message.Text)
					handler(update, bot, db)
				} else { // Обработка обычных сообщений
					handlers.HandlePenCommand(update, bot, db)
				}

				// "сосал?" — реагируем на утвердительные ответы (1 из 5)
				if sosalRe.MatchString(strings.TrimSpace(update.Message.Text)) && rand.Intn(5) == 0 {
					go func(chatID int64) {
						time.Sleep(time.Duration(2+rand.Intn(4)) * time.Second)
						app.SendMessage(chatID, "сосал?", bot, 0)
					}(chatID)
				}

				// эхо: @юзер клево/клёво ? — повторить сообщение
				if klewoRe.MatchString(strings.ToLower(update.Message.Text)) {
					go func(chatID int64, text string) {
						time.Sleep(time.Duration(2+rand.Intn(4)) * time.Second)
						app.SendMessage(chatID, text, bot, 0)
					}(chatID, update.Message.Text)
				} else {
					// если нас тегают или отвечают на наше сообщение -> immediate Gemini
					isMention := false
					if update.Message.Text != "" && bot != nil && bot.Self.UserName != "" {
						isMention = strings.Contains(update.Message.Text, "@"+bot.Self.UserName)
					}
					isReplyToBot := update.Message.ReplyToMessage != nil &&
						update.Message.ReplyToMessage.From != nil &&
						bot != nil &&
						update.Message.ReplyToMessage.From.ID == bot.Self.ID

					if isMention || isReplyToBot {
						_ = geminiAgent.TryRespondImmediate(*update.Message)
					} else {
						_ = geminiAgent.TryRespond(update, specificChatID)
					}
				}

			} else if update.MyChatMember != nil { // Обработка добавления бота в чат
				handlers.HandleBotAddition(update, bot)
			}
		}
	}
}

// startAgentBus exposes the shared status board over MCP and points this bot's
// own reporting at it. It is opt-in: with MCP_ADDR unset the bot behaves
// exactly as before.
func startAgentBus(agent *app.GeminiAgent, defaultName string) {
	addr := strings.TrimSpace(os.Getenv("MCP_ADDR"))
	if addr == "" {
		return
	}

	auth, err := agentbus.ParseAuth(os.Getenv("MCP_TOKENS"))
	if err != nil {
		// Refusing to serve is the safe failure: an unauthenticated board is
		// one any caller could poison with false quota reports.
		log.Printf("agentbus: not starting, MCP_TOKENS is unusable: %v", err)
		return
	}

	selfName := strings.TrimSpace(os.Getenv("MCP_SELF"))
	if selfName == "" {
		selfName = defaultName
	}

	reg := agentbus.NewRegistry(agentBusTTL)
	agent.SetStatusBus(reg, selfName)

	if host, _, splitErr := net.SplitHostPort(addr); splitErr == nil && !isLoopbackHost(host) {
		log.Printf("agentbus: %s is not loopback - bearer tokens will cross the network in clear "+
			"unless a TLS terminating proxy sits in front of it", addr)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           agentbus.Handler(reg, auth),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("agentbus: serving MCP on %s as %q for %v", addr, selfName, auth.Bots())
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("agentbus: server stopped: %v", err)
		}
	}()
}

// isLoopbackHost reports whether an MCP_ADDR host part keeps traffic on the machine.
func isLoopbackHost(host string) bool {
	if host == "" {
		// A bare ":8765" listens on every interface.
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

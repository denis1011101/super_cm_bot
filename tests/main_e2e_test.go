package tests

import (
	"bytes"
	"database/sql"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	"github.com/denis1011101/super_cm_bot/app/handlers"
	"github.com/denis1011101/super_cm_bot/tests/testutils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestMain(m *testing.M) {
	// Установка фиктивного токена
	if err := os.Setenv("BOT_TOKEN", "fake-token"); err != nil {
		log.Fatalf("Failed to set BOT_TOKEN: %v", err)
	}
	log.Println("Используется фиктивный токен.")

	// Запуск тестов
	code := m.Run()
	os.Exit(code)
}

func TestBotE2E(t *testing.T) {
	klewoRe := regexp.MustCompile(`@\S+\s+кл[её]во[!?]?`)

	// Настройка тестовой среды
	_, restoreFunc := testutils.SetupTestEnvironment(t, false)
	defer restoreFunc()

	// Инициализация базы данных
	db, err := app.InitDB()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Error closing database: %v", err)
		}
	}()

	// Проверка, что таблица создана
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='pens';").Scan(&tableName)
	if err != nil {
		t.Fatalf("Table 'pens' does not exist: %v", err)
	}
	if tableName != "pens" {
		t.Fatalf("Expected table name 'pens', but got %s", tableName)
	}

	// Вставляем тестовые данные.
	// 33333333 добавляем с давней датой — после фикса registerBot ставит CURRENT_TIMESTAMP,
	// поэтому свежезарегистрированный пользователь немедленно попадает под кулдаун /pen.
	insertDataQuery := `
    INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length, pen_last_update_at, handsome_count, handsome_last_update_at, unhandsome_count, unhandsome_last_update_at)
    VALUES ('testuser1', 11111111, -987654321, 5, '2024-09-13 08:53:49+00:00', 10, '2024-09-18 21:04:00+00:00', 8, '2024-09-18 21:04:21+00:00'),
           ('testuser2', 22222222, -987654321, 3, '2024-09-13 08:53:49+00:00', 5, '2024-09-18 21:04:00+00:00', 6, '2024-09-18 21:04:21+00:00'),
           ('testuser3', 33333333, -987654321, 5, '2024-09-13 08:53:49+00:00', 0, NULL, 0, NULL);`
	_, err = db.Exec(insertDataQuery)
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	type sentMessage struct {
		Text             string
		ReplyToMessageID string
	}

	var (
		sentMessagesMu sync.Mutex
		sentMessages   []sentMessage
	)

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("mock transport failed to parse form: %v", err)
			}
			if strings.Contains(r.URL.Path, "/sendMessage") {
				sentMessagesMu.Lock()
				sentMessages = append(sentMessages, sentMessage{
					Text:             r.Form.Get("text"),
					ReplyToMessageID: r.Form.Get("reply_to_message_id"),
				})
				sentMessagesMu.Unlock()
			}
			body := `{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"TestBot","username":"test_bot"}}`
			if strings.Contains(r.URL.Path, "/sendMessage") {
				body = `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":-987654321,"type":"group"},"text":"ok"}}`
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    r,
			}, nil
		}),
	}

	// Создаем мок-объект бота с фиктивным токеном и кастомным transport без реального listener
	apiURL := "https://telegram.test/bot%s/%s"
	bot, err := tgbotapi.NewBotAPIWithClient("fake-token", apiURL, client)
	if err != nil {
		t.Fatalf("Error creating bot: %v", err)
	}

	// Перенаправляем логи в буфер
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)

	// Создаем канал обновлений
	updates := make(chan tgbotapi.Update, 1)

	// Обработчики команд
	commandHandlers := map[string]func(tgbotapi.Update, *tgbotapi.BotAPI, *sql.DB){
		"/pen":     handlers.HandleSpin,
		"/giga":    handlers.ChooseGiga,
		"/unh":     handlers.ChooseUnhandsome,
		"/topLen":  handlers.TopLength,
		"/topGiga": handlers.TopGiga,
		"/topUnh":  handlers.TopUnhandsome,
	}

	specificChatID := int64(-987654321)

	// Запускаем бота в отдельной горутине
	go func() {
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

					if klewoRe.MatchString(strings.ToLower(update.Message.Text)) {
						echo := tgbotapi.NewMessage(chatID, update.Message.Text)
						_, _ = bot.Send(echo)
					}
				} else if update.MyChatMember != nil { // Обработка добавления бота в чат
					handlers.HandleBotAddition(update, bot)
				}
			}
		}
	}()

	// Тестирование регистрации нового пользователя
	t.Run("Registration", func(t *testing.T) {
		t.Log("Testing registration of a new user...")
		// Отправляем произвольное сообщение для регистрации нового пользователя
		testutils.SendMessage(t, updates, specificChatID, 44444444, "Hello")

		// Даем время боту обработать команду
		time.Sleep(1 * time.Second)

		// Проверяем, что новый пользователь зарегистрирован с длиной 5 см
		testutils.CheckPenLength(t, db, 44444444, nil, 5)
	})

	t.Run("KlewoEchoWithoutReply", func(t *testing.T) {
		sentMessagesMu.Lock()
		start := len(sentMessages)
		sentMessagesMu.Unlock()

		testText := "@vasya клёво"
		testutils.SendMessage(t, updates, specificChatID, 33333333, testText)
		time.Sleep(1 * time.Second)

		sentMessagesMu.Lock()
		defer sentMessagesMu.Unlock()

		for _, msg := range sentMessages[start:] {
			if msg.Text != testText {
				continue
			}
			if msg.ReplyToMessageID != "" {
				t.Fatalf("expected echo without reply_to_message_id, got %q", msg.ReplyToMessageID)
			}
			return
		}

		t.Fatalf("expected echo message %q to be sent", testText)
	})

	// Тестирование команды /pen и регистрации нового пользователя
	t.Run("PenCommandAndRegistration", func(t *testing.T) {
		t.Log("Testing /pen command and registration...")
		// Отправляем команду /pen
		testutils.SendMessage(t, updates, specificChatID, 33333333, "/pen")

		// Даем время боту обработать команду
		time.Sleep(1 * time.Second)

		// Извлекаем длину из логов
		penInfo, err := testutils.ExtractPenInfoFromLogs(t, &logBuffer, "/pen")
		if err != nil {
			t.Fatalf("Error extracting pen info: %v", err)
		}
		penLength := penInfo.NewSize

		// Проверяем результат в базе данных
		testutils.CheckPenLength(t, db, 33333333, nil, penLength)

		// Проверяем результат остальных пользователей в базе данных
		testutils.CheckPenLength(t, db, 11111111, nil, 5)
		testutils.CheckPenLength(t, db, 22222222, nil, 3)
	})

	// Тестирование команды /pen в течение 4 часов
	t.Run("PenCommandWithin4Hours", func(t *testing.T) {
		t.Log("Testing /pen command within 4 hours...")
		// Отправляем команду /pen
		testutils.SendMessage(t, updates, specificChatID, 33333333, "/pen")

		// Даем время боту обработать команду
		time.Sleep(1 * time.Second)

		// Извлекаем длину из логов
		penInfo, err := testutils.ExtractPenInfoFromLogs(t, &logBuffer, "/pen")
		if err != nil {
			t.Fatalf("Error extracting pen info: %v", err)
		}
		penLength := penInfo.NewSize

		// Проверяем результат в базе данных
		testutils.CheckPenLength(t, db, 33333333, nil, penLength)
	})

	// Тестирование команды /pen после 4 часов
	t.Run("PenCommandAfter4Hours", func(t *testing.T) {
		t.Log("Testing /pen command after 4 hours...")
		// Обновляем время последнего обновления
		testutils.ChangeData(t, db, 33333333, specificChatID, map[string]interface{}{"pen_last_update_at": time.Now().Add(-4 * time.Hour)})

		// Отправляем команду /pen
		testutils.SendMessage(t, updates, specificChatID, 33333333, "/pen")

		// Даем время боту обработать команду
		time.Sleep(1 * time.Second)

		// Извлекаем длину из логов
		penInfo, err := testutils.ExtractPenInfoFromLogs(t, &logBuffer, "/pen")
		if err != nil {
			t.Fatalf("Error extracting pen info: %v", err)
		}
		penLength := penInfo.NewSize

		// Проверяем результат в базе данных
		testutils.CheckPenLength(t, db, 33333333, nil, penLength)
	})

	// Тестирование команды /giga
	t.Run("GigaCommand", func(t *testing.T) {
		t.Log("Testing /giga command...")
		// Отправляем команду /giga
		testutils.SendMessage(t, updates, specificChatID, 33333333, "/giga")

		// Даем время боту обработать команду
		time.Sleep(1 * time.Second)

		// Извлекаем длину из логов
		penInfo, err := testutils.ExtractPenInfoFromLogs(t, &logBuffer, "/giga")
		if err != nil {
			t.Fatalf("Error extracting pen info: %v", err)
		}
		penLength := penInfo.NewSize
		penUser := penInfo.UserID
		penChat := penInfo.ChatID

		// Проверяем результат в базе данных
		testutils.CheckPenLength(t, db, penUser, &penChat, penLength)
	})

	// Тестирование команды /unh
	t.Run("UnhCommand", func(t *testing.T) {
		t.Log("Testing /unh command...")
		// Отправляем команду /unh
		testutils.SendMessage(t, updates, specificChatID, 33333333, "/unh")

		// Даем время боту обработать команду
		time.Sleep(1 * time.Second)

		// Извлекаем длину из логов
		penInfo, err := testutils.ExtractPenInfoFromLogs(t, &logBuffer, "/unh")
		if err != nil {
			t.Fatalf("Error extracting pen info: %v", err)
		}
		penLength := penInfo.NewSize
		penUser := penInfo.UserID
		penChat := penInfo.ChatID

		// Проверяем результат в базе данных
		testutils.CheckPenLength(t, db, penUser, &penChat, penLength)
	})

	// Тестирование команды /topLen
	t.Run("TopLenCommand", func(t *testing.T) {
		t.Log("Testing /topLen command...")
		// Отправляем команду /topLen
		testutils.SendMessage(t, updates, specificChatID, 33333333, "/topLen")

		// Даем время боту обработать команду
		time.Sleep(1 * time.Second)

		// Проверяем, что топ-лист был отправлен
		_, err := testutils.ExtractPenInfoFromLogs(t, &logBuffer, "/topLen")
		if err != nil {
			t.Fatalf("Error extracting top list: %v", err)
		}
	})

	// Тестирование команды /topGiga
	t.Run("TopGigaCommand", func(t *testing.T) {
		t.Log("Testing /topGiga command...")
		// Отправляем команду /topGiga
		testutils.SendMessage(t, updates, specificChatID, 33333333, "/topGiga")

		// Даем время боту обработать команду
		time.Sleep(1 * time.Second)

		// Проверяем, что топ-лист был отправлен
		_, err := testutils.ExtractPenInfoFromLogs(t, &logBuffer, "/topGiga")
		if err != nil {
			t.Fatalf("Error extracting top list: %v", err)
		}
	})

	// Тестирование команды /topUnh
	t.Run("TopUnhCommand", func(t *testing.T) {
		t.Log("Testing /topUnh command...")
		// Отправляем команду /topUnh
		testutils.SendMessage(t, updates, specificChatID, 33333333, "/topUnh")

		// Даем время боту обработать команду
		time.Sleep(1 * time.Second)

		// Проверяем, что топ-лист был отправлен
		_, err := testutils.ExtractPenInfoFromLogs(t, &logBuffer, "/topUnh")
		if err != nil {
			t.Fatalf("Error extracting top list: %v", err)
		}
	})

	// Тестирование регистрации пользователя при вступлении в чат
	t.Run("UserJoinEventRegistration", func(t *testing.T) {
		t.Log("Testing user registration on join event...")
		update := tgbotapi.Update{
			UpdateID: 1,
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID: specificChatID,
				},
				From: &tgbotapi.User{
					ID:       55555555,
					UserName: "testuser",
				},
				NewChatMembers: []tgbotapi.User{
					{
						ID:       55555555,
						UserName: "testuser",
					},
				},
			},
		}

		updates <- update
		time.Sleep(1 * time.Second)

		testutils.CheckPenLength(t, db, 55555555, &specificChatID, 5)
	})

	// Тестирование отсутствия регистрации при покидании чата
	t.Run("UserLeaveEventNoRegistration", func(t *testing.T) {
		t.Log("Testing no registration on leave event...")
		update := tgbotapi.Update{
			UpdateID: 2,
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID: specificChatID,
				},
				From: &tgbotapi.User{
					ID:       66666666,
					UserName: "testuser",
				},
				LeftChatMember: &tgbotapi.User{
					ID:       66666666,
					UserName: "testuser",
				},
			},
		}

		updates <- update
		time.Sleep(1 * time.Second)

		var penLength int
		err := db.QueryRow(
			"SELECT pen_length FROM pens WHERE tg_pen_id = ? AND tg_chat_id = ?",
			66666666,
			specificChatID,
		).Scan(&penLength)

		if err == nil {
			t.Fatalf("User should not be registered on leave event")
		}
	})
}

func TestSosalRegex(t *testing.T) {
	sosalRe := regexp.MustCompile(`(?i)^(да|угу|ага|ну да|конечно|естественно|разумеется|точно|реально|всё так|все так|так|именно|верно|йеп|еп|ок|окей|yes|yep|yeah|yea|sure|ладно|допустим)\s*\?\s*$`)

	shouldMatch := []string{
		"да?", "Да?", "ДА?", "да ?", "угу?", "ага?",
		"конечно?", "ну да?", "yes?", "yep?", "yeah?",
		"ок?", "окей?", "точно?", "реально?", "так?",
		"всё так?", "все так?", "именно?", "верно?",
	}
	shouldNotMatch := []string{
		"да", "угу", "ага", "конечно", "yes",
		"да!", "да.", "привет?", "нет?", "может?",
		"да конечно?", "ну привет?", "",
	}

	for _, s := range shouldMatch {
		if !sosalRe.MatchString(strings.TrimSpace(s)) {
			t.Errorf("expected match for %q", s)
		}
	}
	for _, s := range shouldNotMatch {
		if sosalRe.MatchString(strings.TrimSpace(s)) {
			t.Errorf("expected no match for %q", s)
		}
	}
}

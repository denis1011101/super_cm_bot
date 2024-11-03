package tests

import (
	"bytes"
	"database/sql"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	"github.com/denis1011101/super_cm_bot/app/handlers"
	"github.com/denis1011101/super_cm_bot/tests/testutils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

func TestMain(m *testing.M) {
    // Установка фиктивного токена
    os.Setenv("BOT_TOKEN", "fake-token")
    log.Println("Используется фиктивный токен.")

	// Запуск тестов
	code := m.Run()
	os.Exit(code)
}

func TestBotE2E(t *testing.T) {
	// Настройка тестовой среды
	_, restoreFunc := testutils.SetupTestEnvironment(t, false)
	defer restoreFunc()

	// Инициализация базы данных
	db, err := app.InitDB()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Проверка, что таблица создана
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='pens';").Scan(&tableName)
	if err != nil {
		t.Fatalf("Table 'pens' does not exist: %v", err)
	}
	if tableName != "pens" {
		t.Fatalf("Expected table name 'pens', but got %s", tableName)
	}

	// Вставляем тестовые данные
	insertDataQuery := `
    INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length, pen_last_update_at, handsome_count, handsome_last_update_at, unhandsome_count, unhandsome_last_update_at)
    VALUES ('testuser1', 11111111, -987654321, 5, '2024-09-13 08:53:49.959836013+05:00', 10, '2024-09-18 21:04:00.758573552+05:00', 8, '2024-09-18 21:04:21.388000393+05:00'),
           ('testuser2', 22222222, -987654321, 3, '2024-09-13 08:53:49.959836013+05:00', 5, '2024-09-18 21:04:00.758573552+05:00', 6, '2024-09-18 21:04:21.388000393+05:00');`
	_, err = db.Exec(insertDataQuery)
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	// Создаем мок-сервер для API Telegram
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"TestBot","username":"test_bot"}}`))
	}))
	defer mockServer.Close()

	// Создаем мок-объект бота с фиктивным токеном и перенаправляем запросы на мок-сервер
	apiURL := mockServer.URL + "/bot%s/%s"
	bot, err := tgbotapi.NewBotAPIWithClient("fake-token", apiURL, mockServer.Client())
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
}

package tests

import (
	"bytes"
	"database/sql"
	"fmt"
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
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

func TestMain(m *testing.M) {
	// Загрузка переменных окружения
	err := godotenv.Load()
	if err != nil {
		log.Printf("Ошибка загрузки переменных окружения: %v. Используется фиктивный токен.", err)
		os.Setenv("BOT_TOKEN", "fake-token")
	}

	// Запуск тестов
	code := m.Run()
	os.Exit(code)
}

func TestBotIntegration(t *testing.T) {
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
    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60
    updates := make(chan tgbotapi.Update, 1)

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

    // Отправляем произвольное сообщение для регистрации нового пользователя
    update := tgbotapi.Update{
        UpdateID: 1,
        Message: &tgbotapi.Message{
            MessageID: 1,
            From: &tgbotapi.User{
                ID: 44444444,
            },
            Chat: &tgbotapi.Chat{
                ID: specificChatID,
            },
            Text: "Hello",
            Date: int(time.Now().Unix()),
        },
    }

    // Отправляем обновление в канал
    updates <- update

    // Даем время боту обработать команду
    time.Sleep(1 * time.Second)

    // Проверяем, что новый пользователь зарегистрирован с длиной 5 см
    var penLength int
    err = db.QueryRow("SELECT pen_length FROM pens WHERE tg_pen_id = ? AND tg_chat_id = ?", 44444444, specificChatID).Scan(&penLength)
    if err != nil {
        t.Fatalf("Failed to query new user: %v", err)
    }
    if penLength != 5 {
        t.Errorf("expected pen_length for new user to be 5, but got %d", penLength)
    }

    // Отправляем команду /pen
    update = tgbotapi.Update{
        UpdateID: 2,
        Message: &tgbotapi.Message{
            MessageID: 2,
            From: &tgbotapi.User{
                ID: 33333333,
            },
            Chat: &tgbotapi.Chat{
                ID: specificChatID,
            },
            Text: "/pen",
            Date: int(time.Now().Unix()),
        },
    }

    // Отправляем обновление в канал
    updates <- update

    // Даем время боту обработать команду
    time.Sleep(1 * time.Second)

    // Проверяем результат в базе данных
    checkPenLength(t, db, &logBuffer)
}

func checkPenLength(t *testing.T, db *sql.DB, logBuffer *bytes.Buffer) {
    // Извлекаем логи
    logs := logBuffer.String()
    t.Logf("Logs: %s", logs) // Добавим отладочное сообщение для логов

    // Проверяем логи на наличие обновленного размера
    expectedLog := "Updated pen size: "
    if !bytes.Contains([]byte(logs), []byte(expectedLog)) {
        t.Errorf("expected log to contain %q, but got %q", expectedLog, logs)
    }

    // Извлекаем новый размер из логов
	var newSize int
	logLines := bytes.Split([]byte(logs), []byte("\n"))
	for _, line := range logLines {
		if bytes.Contains(line, []byte(expectedLog)) {
			parts := bytes.SplitN(line, []byte("Updated pen size: "), 2)
			if len(parts) == 2 {
				_, err := fmt.Sscanf(string(parts[1]), "%d", &newSize)
				if err != nil {
					t.Fatalf("Failed to parse new size from logs: %v", err)
				}
				break
			}
		}
	}
    t.Logf("Parsed new size: %d", newSize) // Добавим отладочное сообщение для нового размера

    // Проверяем размер в базе данных
    rows, err := db.Query("SELECT tg_pen_id, pen_length FROM pens WHERE tg_chat_id = ?", -987654321)
    if err != nil {
        t.Fatalf("Error querying database: %v", err)
    }
    defer rows.Close()

	foundUser := false
    for rows.Next() {
        var tgUserID, penLength int
        if err := rows.Scan(&tgUserID, &penLength); err != nil {
            t.Fatalf("Error scanning row: %v", err)
        }
        switch tgUserID {
        case 11111111:
            if penLength != 5 {
                t.Errorf("expected pen_length for user 11111111 to be 5, but got %d", penLength)
            }
        case 22222222:
            if penLength != 3 {
                t.Errorf("expected pen_length for user 22222222 to be 3, but got %d", penLength)
            }
        case 33333333:
            if penLength != newSize {
                t.Errorf("expected pen_length for user 33333333 to be %d, but got %d", newSize, penLength)
            }
        case 44444444:
            if penLength != 5 {
                t.Errorf("expected pen_length for user 44444444 to be 5, but got %d", penLength)
            }
			foundUser = true
        default:
            t.Errorf("unexpected user ID %d", tgUserID)
        }
    }

	if !foundUser {
        t.Errorf("User 33333333 not found in database")
    }
}
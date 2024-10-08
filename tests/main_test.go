package tests

import (
	"bytes"
	"database/sql"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
    "regexp"
    "strconv"
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

// sendMessage отправляет сообщение в канал обновлений
func sendMessage(t *testing.T, updates chan tgbotapi.Update, chatID int64, userID int64, text string) {
    update := tgbotapi.Update{
        UpdateID: 1,
        Message: &tgbotapi.Message{
            MessageID: 1,
            From: &tgbotapi.User{
                ID: userID,
            },
            Chat: &tgbotapi.Chat{
                ID: chatID,
            },
            Text: text,
            Date: int(time.Now().Unix()),
        },
    }

    // Отправляем обновление в канал
    updates <- update
}

// checkPenLength проверяет длину pen в базе данных
func checkPenLength(t *testing.T, db *sql.DB, userID int64, expectedLength int) {
    var penLength int
    err := db.QueryRow("SELECT pen_length FROM pens WHERE tg_pen_id = ? AND tg_chat_id = ?", userID, -987654321).Scan(&penLength)
    if err != nil {
        t.Fatalf("Failed to query user %d: %v", userID, err)
    }
    if penLength != expectedLength {
        t.Errorf("expected pen_length for user %d to be %d, but got %d", userID, expectedLength, penLength)
    }
}

// checkLogs проверяет логи на наличие определенного сообщения
func checkLogs(t *testing.T, logBuffer *bytes.Buffer, expectedLog string) {
    logs := logBuffer.String()
    if !bytes.Contains([]byte(logs), []byte(expectedLog)) {
        t.Errorf("expected log to contain %q, but got %q", expectedLog, logs)
    }
}

// extractPenLengthFromLogs извлекает длину pen из логов
func extractPenLengthFromLogs(t *testing.T, logBuffer *bytes.Buffer) int {
    logs := logBuffer.String()
    re := regexp.MustCompile(`Updated pen size: (\d+)`)
    matches := re.FindStringSubmatch(logs)
    if len(matches) < 2 {
        t.Fatalf("Failed to extract pen_length from logs: %v", logs)
    }
    penLength, err := strconv.Atoi(matches[1])
    if err != nil {
        t.Fatalf("Failed to convert pen_length to int: %v", err)
    }
    return penLength
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
    updates := make(chan tgbotapi.Update, 1)

    // Обработчики команд
    commandHandlers := map[string]func(tgbotapi.Update, *tgbotapi.BotAPI, *sql.DB){
        "/pen":                                handlers.HandleSpin,
        "/giga":                               handlers.ChooseGiga,
        "/unh":                                handlers.ChooseUnhandsome,
        "/topLen":                             handlers.TopLength,
        "/topGiga":                            handlers.TopGiga,
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

    // Проверяем регуистрацию нового пользователя
    // Отправляем произвольное сообщение для регистрации нового пользователя
    sendMessage(t, updates, specificChatID, 44444444, "Hello")

    // Даем время боту обработать команду
    time.Sleep(1 * time.Second)

    // Проверяем, что новый пользователь зарегистрирован с длиной 5 см
    checkPenLength(t, db, 44444444, 5)

    // Проверяем обработку команды /pen
    // Отправляем команду /pen
    sendMessage(t, updates, specificChatID, 33333333, "/pen")

    // Даем время боту обработать команду
    time.Sleep(1 * time.Second)

    // Извлекаем длину из логов
    penLength := extractPenLengthFromLogs(t, &logBuffer)

    // Проверяем результат в базе данных
    checkPenLength(t, db, 33333333, penLength)

    // Проверяем результат остальных пользователей в базе данных
    checkPenLength(t, db, 11111111, 5)
    checkPenLength(t, db, 22222222, 3)

    // Проверяем что повторный вызов /pen не увеличит длину так как прошло меньше 24 часов
    // Отправляем команду /pen
    sendMessage(t, updates, specificChatID, 33333333, "/pen")

    // Проверяем результат в базе данных
    checkPenLength(t, db, 33333333, penLength)
}

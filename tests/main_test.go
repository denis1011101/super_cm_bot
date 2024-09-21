package tests

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/denis1011101/super_cum_bot/app/handlers"
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
	// Создаем базу данных SQLite в памяти
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Создаем таблицу pens
	createTableQuery := `
    CREATE TABLE IF NOT EXISTS pens (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        pen_name TEXT,
        tg_pen_id INTEGER UNIQUE,
        tg_chat_id INTEGER,
        pen_length INTEGER,
        pen_last_update_at TIMESTAMP,
        handsome_count INTEGER,
        handsome_last_update_at TIMESTAMP,
        unhandsome_count INTEGER,
        unhandsome_last_update_at TIMESTAMP
    );`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
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

	// Таблица тестов для команд и их обработчиков
	tests := []struct {
		command string
		handler func(tgbotapi.Update, *tgbotapi.BotAPI, *sql.DB)
	}{
		{"/pen@super_cum_lovers_bot", handlers.HandleSpin},
		{"/pen", handlers.HandleSpin},
		{"/giga@super_cum_lovers_bot", handlers.ChooseGiga},
		{"/giga", handlers.ChooseGiga},
		{"/unhandsome@super_cum_lovers_bot", handlers.ChooseUnhandsome},
		{"/unh", handlers.ChooseUnhandsome},
		{"/topLength@super_cum_lovers_bot", handlers.TopLength},
		{"/topLen", handlers.TopLength},
		{"/topGiga@super_cum_lovers_bot", handlers.TopGiga},
		{"/topGiga", handlers.TopGiga},
		{"/topUnhandsome@super_cum_lovers_bot", handlers.TopUnhandsome},
		{"/topUnh", handlers.TopUnhandsome},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			// Создаем обновление с сообщением
			update := tgbotapi.Update{
				Message: &tgbotapi.Message{
					Text: tt.command,
					Chat: &tgbotapi.Chat{
						ID: -987654321,
					},
					From: &tgbotapi.User{
						ID: 33333333,
					},
				},
			}

			// Перехватываем вывод логов
			var logOutput strings.Builder
			log.SetOutput(&logOutput)

			// Вызываем тестируемую функцию
			tt.handler(update, bot, db)

			// Формируем ожидаемое сообщение на основе данных из базы данных
			rows, err := db.Query("SELECT pen_name, unhandsome_count FROM pens WHERE tg_chat_id = ? ORDER BY unhandsome_count DESC LIMIT 10", -987654321)
			if err != nil {
				t.Fatalf("Error querying database: %v", err)
			}
			defer rows.Close()

			var expectedMessage strings.Builder
			expectedMessage.WriteString("Топ 10 пидоров:\n")
			for rows.Next() {
				var penName string
				var unhandsomeCount int
				if err := rows.Scan(&penName, &unhandsomeCount); err != nil {
					t.Fatalf("Error scanning row: %v", err)
				}
				expectedMessage.WriteString(fmt.Sprintf("%s: %d раз\n", penName, unhandsomeCount))
			}

			// Проверяем, что сообщение было отправлено
			if tt.command == "/topUnhandsome@super_cum_lovers_bot" || tt.command == "/topUnh" {
				if !strings.Contains(logOutput.String(), expectedMessage.String()) {
					t.Errorf("expected message to contain %q, but got %q", expectedMessage.String(), logOutput.String())
				}
			}
		})
	}
}

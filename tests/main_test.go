package tests

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/denis1011101/super_cum_bot/app/handlers"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
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
	// Создаем мок-объект базы данных
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// Настраиваем ожидания для мок-объекта
	rows := sqlmock.NewRows([]string{"pen_name", "unhandsome_count"}).
		AddRow("User1", 5).
		AddRow("User2", 3)
	mock.ExpectQuery("^SELECT pen_name, unhandsome_count FROM pens WHERE tg_chat_id = \\? ORDER BY unhandsome_count DESC LIMIT 10$").
		WithArgs(12345).
		WillReturnRows(rows)

	// Создаем мок-объект бота с фиктивным токеном
	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		t.Fatalf("Error creating bot: %v", err)
	}

	// Создаем мок-сервер для API Telegram
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"result":[]}`))
	}))
	defer mockServer.Close()

	// Перенаправляем запросы бота на мок-сервер
	bot.Client = mockServer.Client()

	// Создаем обновление с сообщением
	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: "/topunhandsome",
			Chat: &tgbotapi.Chat{
				ID: 12345,
			},
		},
	}

	// Перехватываем вывод логов
	var logOutput strings.Builder
	log.SetOutput(&logOutput)

	// Вызываем тестируемую функцию
	handlers.TopUnhandsome(update, bot, db)

	// Проверяем, что все ожидания были выполнены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}

	// Проверяем, что сообщение было отправлено
	expectedMessage := "Топ 10 пидоров:\nUser1: 5 раз\nUser2: 3 раз\n"
	if !strings.Contains(logOutput.String(), expectedMessage) {
		t.Errorf("expected message to contain %q, but got %q", expectedMessage, logOutput.String())
	}
}

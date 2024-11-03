package testutils

import (
	"os"
    "testing"
    "regexp"
    "strconv"
    "time"
    "database/sql"
    "bytes"

    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SetupTestEnvironment создает временную директорию для теста и меняет текущий рабочий каталог на эту директорию.
// Если returnTempDir равно true, возвращает путь к временной директории и функцию для восстановления оригинального рабочего каталога.
// Если returnTempDir равно false, возвращает только функцию для восстановления оригинального рабочего каталога.
func SetupTestEnvironment(t *testing.T, returnTempDir bool) (string, func()) {
	// Создаём временную директорию для теста, которая будет автоматически удалена после завершения теста
	tempDir := t.TempDir()

	// Сохраняем текущий рабочий каталог
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	// Меняем текущий рабочий каталог на временную директорию
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change working directory: %v", err)
	}

	// Функция для восстановления оригинального рабочего каталога
	restoreFunc := func() {
		err := os.Chdir(originalDir)
		if err != nil {
			t.Fatalf("Failed to restore original working directory: %v", err)
		}
	}

	if returnTempDir {
		return tempDir, restoreFunc
	}
	return "", restoreFunc
}

// SendMessage отправляет сообщение в канал обновлений
func SendMessage(t *testing.T, updates chan tgbotapi.Update, chatID int64, userID int64, text string) {
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

// CheckPenLength проверяет длину pen в базе данных
func CheckPenLength(t *testing.T, db *sql.DB, userID int64, tgChatID *int64, expectedLength int64) {
    var penLength int
    defaultChatID := int64(-987654321)
    if tgChatID == nil {
        tgChatID = &defaultChatID
    }
    err := db.QueryRow("SELECT pen_length FROM pens WHERE tg_pen_id = ? AND tg_chat_id = ?", userID, *tgChatID).Scan(&penLength)
    if err != nil {
        t.Fatalf("Failed to query user %d: %v", userID, err)
    }
    if int64(penLength) != expectedLength {
        t.Errorf("expected pen_length for user %d to be %d, but got %d", userID, expectedLength, penLength)
    } else {
        t.Logf("pen_length for user %d is as expected: %d", userID, penLength)
    }
}

type PenInfo struct {
    UserID  int64
    ChatID  int64
    NewSize int64
}

// ExtractPenInfoFromLogs извлекает длину pen из логов
func ExtractPenInfoFromLogs(t *testing.T, logBuffer *bytes.Buffer, mode string) (PenInfo, error) {
    logs := logBuffer.String()
    t.Log("Logs content:", logs)

    var re *regexp.Regexp
    var penInfo PenInfo

    switch mode {
    case "/pen":
        re = regexp.MustCompile(`Updated pen size: (\d+)`)
        matches := re.FindAllStringSubmatch(logs, -1)
        if len(matches) == 0 {
            t.Fatalf("Failed to extract pen_length from logs: %v", logs)
        }
        lastMatch := matches[len(matches)-1]
        penLength, err := strconv.ParseInt(lastMatch[1], 10, 64)
        if err != nil {
            t.Fatalf("Failed to convert pen_length to int64: %v", err)
        }
        penInfo.NewSize = penLength
    case "/giga", "/unh":
        re = regexp.MustCompile(`userID: (\d+), chatID: (-\d+), newSize: (-?\d+)`)
        matches := re.FindAllStringSubmatch(logs, -1)
        if len(matches) == 0 {
            t.Fatalf("Failed to extract parameters from logs: %v", logs)
        }
        lastMatch := matches[len(matches)-1]
        userID, err := strconv.ParseInt(lastMatch[1], 10, 64)
        if err != nil {
            t.Fatalf("Failed to convert userID to int64: %v", err)
        }
        chatID, err := strconv.ParseInt(lastMatch[2], 10, 64)
        if err != nil {
            t.Fatalf("Failed to convert chatID to int64: %v", err)
        }
        newSize, err := strconv.ParseInt(lastMatch[3], 10, 64)
        if err != nil {
            t.Fatalf("Failed to convert newSize to int64: %v", err)
        } else {
            t.Logf("Extracted parameters: userID=%d, chatID=%d, newSize=%d", userID, chatID, newSize)
        }
        penInfo = PenInfo{
            UserID:  userID,
            ChatID:  chatID,
            NewSize: newSize,
        }
    case "/topLen", "/topGiga", "/topUnh":
        var messagePattern string
        switch mode {
        case "/topLen":
            messagePattern = `Message sent to chat ID [-\d]+: Топ \d+ по длине пениса:`
        case "/topGiga":
            messagePattern = `Message sent to chat ID [-\d]+: Топ \d+ гигачадов:`
        case "/topUnh":
            messagePattern = `Message sent to chat ID [-\d]+: Топ \d+ пидоров:`
        }
        
        re = regexp.MustCompile(messagePattern)
        matches := re.FindStringSubmatch(logs)
        if len(matches) == 0 {
            t.Fatalf("Failed to extract top list from logs: %v", logs)
        }
        t.Logf("Found top list message: %v", matches[0])
    default:
        t.Fatalf("Invalid mode: %v", mode)
    }

    return penInfo, nil
}

// ChangeData изменяет данные в базе данных
func ChangeData(t *testing.T, db *sql.DB, userID int64, chatID int64, updates map[string]interface{}) {
    if len(updates) == 0 {
        t.Fatalf("No updates provided")
    }

    // Формируем SQL-запрос динамически
    query := "UPDATE pens SET "
    args := []interface{}{}
    for column, value := range updates {
        query += column + " = ?, "
        args = append(args, value)
    }
    query = query[:len(query)-2] // Удаляем последнюю запятую и пробел
    query += " WHERE tg_pen_id = ? AND tg_chat_id = ?"
    args = append(args, userID, chatID)

    // Логируем сформированный запрос и аргументы
    t.Logf("Executing query: %s with args: %v", query, args)

    // Выполняем запрос
    _, err := db.Exec(query, args...)
    if err != nil {
        t.Fatalf("Failed to update data: %v", err)
    }

    // Логируем успешное выполнение запроса
    t.Log("Data updated successfully")
}
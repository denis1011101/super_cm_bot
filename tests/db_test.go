package tests

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	"github.com/denis1011101/super_cm_bot/tests/testutils"
	_ "github.com/mattn/go-sqlite3"
)

func TestInitDB(t *testing.T) {
	// Настраиваем тестовую среду
	_, teardown := testutils.SetupTestEnvironment(t, false)
	defer teardown()

	// Инициализируем базу данных с использованием пути к файлу базы данных во временной директории
	db, err := app.InitDB()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Вставляем данные в таблицу
	_, err = db.Exec("INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length) VALUES ('test_pen', 12345, 67890, 10)")
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	// Проверяем данные с помощью SELECT
	var penName string
	err = db.QueryRow("SELECT pen_name FROM pens WHERE tg_pen_id = 12345").Scan(&penName)
	if err != nil {
		t.Fatalf("Failed to select data: %v", err)
	}

	if penName != "test_pen" {
		t.Fatalf("Expected 'test_pen', got %v", penName)
	}
}

func TestStartBackupRoutine(t *testing.T) {
	// Настраиваем тестовую среду
	tempDir, teardown := testutils.SetupTestEnvironment(t, true)
	defer teardown()

	// Инициализируем базу данных с использованием пути к файлу базы данных во временной директории
	db, err := app.InitDB()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	var mutex sync.Mutex

	// Выполняем несколько запросов к базе данных до запуска горутины
	for i := 0; i < 3; i++ {
		mutex.Lock()
		_, err := db.Exec("INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length) VALUES (?, ?, ?, ?)", "test_pen_before", i, -1, 10)
		mutex.Unlock()
		if err != nil {
			t.Errorf("Failed to insert data before goroutine: %v", err)
		}
	}

	// Запускаем рутину резервного копирования
	app.StartBackupRoutine(db, &mutex)

	// Канал для синхронизации завершения горутины
	done := make(chan bool)

	// Выполняем несколько запросов к базе данных
	go func() {
		for i := 0; i < 5; i++ {
			mutex.Lock()
			_, err := db.Exec("INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length) VALUES (?, ?, ?, ?)", "test_pen_during", i+100, -1, 10)
			mutex.Unlock()
			if err != nil {
				t.Errorf("Failed to insert data: %v", err)
			}
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Ожидаем завершения горутины
	<-done

	// Проверяем, что резервная копия была создана
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read backup directory: %v", err)
	}

	if len(files) == 0 {
		t.Fatalf("No backup files found")
	}

	// Проверяем, что запросы к базе данных были выполнены успешно
	var countBefore, countDuring int
	err = db.QueryRow("SELECT COUNT(*) FROM pens WHERE pen_name = ?", "test_pen_before").Scan(&countBefore)
	if err != nil {
		t.Fatalf("Failed to count inserted rows before goroutine: %v", err)
	}

	err = db.QueryRow("SELECT COUNT(*) FROM pens WHERE pen_name = ?", "test_pen_during").Scan(&countDuring)
	if err != nil {
		t.Fatalf("Failed to count inserted rows during goroutine: %v", err)
	}

	if countBefore != 3 {
		t.Fatalf("Expected 5 rows to be inserted before goroutine, but got %d", countBefore)
	}

	if countDuring != 5 {
		t.Fatalf("Expected 10 rows to be inserted during goroutine, but got %d", countDuring)
	}
}

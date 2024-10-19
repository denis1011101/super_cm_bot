package tests

import (
	"io"
	"log"
	"os"
	filepath "path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestEnvironment создает временную директорию для теста и меняет текущий рабочий каталог на эту директорию.
// Если returnTempDir равно true, возвращает путь к временной директории и функцию для восстановления оригинального рабочего каталога.
// Если returnTempDir равно false, возвращает только функцию для восстановления оригинального рабочего каталога.
func setupTestEnvironment(t *testing.T, returnTempDir bool) (string, func()) {
	// Создаём временную директорию для теста, которая будет автоматически удалена после завершения теста
	tempDir := t.TempDir()

	log.Println("temp dir: " + tempDir)

	// Сохраняем текущий рабочий каталог
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Меняем текущий рабочий каталог на временную директорию
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Копируем файлы миграции во временную директорию
    projectBaseDir := getBaseDir(originalDir)
	migrationsDir := filepath.Join(projectBaseDir, "app", "db", "migrations")
	copyDir(migrationsDir, "./app/db/migrations")
	log.Println("===== Base project tree =====")
	printDirTree(projectBaseDir)
	log.Println("===== Tests tree =====")
	printDirTree(tempDir)

	// Функция для восстановления оригинального рабочего каталога
	teardown := func() {
		err := os.Chdir(originalDir)
		if err != nil {
			t.Fatalf("Failed to restore original directory: %v", err)
		}
	}

	if returnTempDir {
		return tempDir, teardown
	}
	return "", teardown
}

func TestInitDB(t *testing.T) {
	// Настраиваем тестовую среду
	_, teardown := setupTestEnvironment(t, false)
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
	tempDir, teardown := setupTestEnvironment(t, true)
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

func copyDir(src, dst string) error {
	// Create the destination directory if it doesn't exist
	err := os.MkdirAll(dst, 0755)
	if err != nil {
		return err
	}

	// Walk through the source directory
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate the destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		// If it's a directory, create it
		if info.IsDir() {
			err := os.MkdirAll(dstPath, 0755)
			if err != nil {
				return err
			}
		} else {
			// If it's a file, copy it
			srcFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			dstFile, err := os.Create(dstPath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func printDirTree(path string) {
	filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		log.Println(name)
		return nil
	})
}

// Получение базовой дириктории, поиск по имени репозитория
// В ci в пути дублируется папка с репозиторем, поэтому ищется максимально длинная дириктория
// Пример /home/runner/work/super_cm_bot/super_cm_bot/tests -> /home/runner/work/super_cm_bot/super_cm_bot
// Пример c:\Users\Alexander\RiderProjects\super_cm_bot\tests -> c:\Users\Alexander\RiderProjects\super_cm_bot
func getBaseDir(path string) string {
	splitedDir := strings.SplitAfter(path, "super_cm_bot")
	
	if (len(splitedDir) > 1) {
		baseDir := splitedDir[0]
		index := 1

		for index < len(splitedDir) {
			if (strings.Contains(splitedDir[index], "super_cm_bot")) {
				baseDir = baseDir + splitedDir[index]
			} else {
				break
			}
			index++
		}

		return baseDir
	} else if len(splitedDir) == 1 {
		return splitedDir[0]
	} else {
		return ""
	}
}
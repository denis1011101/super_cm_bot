package testutils

import (
	"os"
	"testing"
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

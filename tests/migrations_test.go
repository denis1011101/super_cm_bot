package tests

import (
    "database/sql"
    "testing"
    "time"

    "github.com/denis1011101/super_cm_bot/app"
    "github.com/denis1011101/super_cm_bot/tests/testutils"
    _ "github.com/mattn/go-sqlite3"
)

func TestRunMigrations(t *testing.T) {
    // Настраиваем тестовую среду
    _, teardown := testutils.SetupTestEnvironment(t, false)
    defer teardown()

    // Инициализируем базу данных
    db, err := app.InitDB()
    if err != nil {
        t.Fatalf("Failed to initialize database: %v", err)
    }
    defer db.Close()

    // Проверяем, что таблица migrations создана
    var tableName string
    err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='migrations';").Scan(&tableName)
    if err != nil {
        t.Fatalf("Table 'migrations' does not exist: %v", err)
    }
    if tableName != "migrations" {
        t.Fatalf("Expected table name 'migrations', but got %s", tableName)
    }

    // Проверяем, что миграция была применена
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM migrations WHERE id = 1").Scan(&count)
    if err != nil {
        t.Fatalf("Failed to count migrations: %v", err)
    }
    if count != 1 {
        t.Fatalf("Expected migration 1 to be applied, but count is %d", count)
    }

    // Проверяем, что колонка is_active добавлена в таблицу pens
    rows, err := db.Query("PRAGMA table_info(pens)")
    if err != nil {
        t.Fatalf("Failed to get table info: %v", err)
    }
    defer rows.Close()

    var hasIsActiveColumn bool
    for rows.Next() {
        var cid int
        var name, dataType string
        var notNull, defaultValue, pk interface{}
        
        err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
        if err != nil {
            t.Fatalf("Failed to scan column info: %v", err)
        }
        
        if name == "is_active" {
            hasIsActiveColumn = true
            if dataType != "BOOLEAN" {
                t.Errorf("Expected is_active column type to be BOOLEAN, but got %s", dataType)
            }
            break
        }
    }

    if !hasIsActiveColumn {
        t.Fatalf("Column 'is_active' was not added to pens table")
    }
}

func TestMigrationsIdempotency(t *testing.T) {
    // Настраиваем тестовую среду
    _, teardown := testutils.SetupTestEnvironment(t, false)
    defer teardown()

    // Инициализируем базу данных
    db, err := app.InitDB()
    if err != nil {
        t.Fatalf("Failed to initialize database: %v", err)
    }
    defer db.Close()

    // Запускаем миграции первый раз
    err = app.RunMigrations(db)
    if err != nil {
        t.Fatalf("First migration run failed: %v", err)
    }

    // Проверяем количество записей в таблице migrations
    var countAfterFirst int
    err = db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&countAfterFirst)
    if err != nil {
        t.Fatalf("Failed to count migrations after first run: %v", err)
    }

    // Запускаем миграции второй раз
    err = app.RunMigrations(db)
    if err != nil {
        t.Fatalf("Second migration run failed: %v", err)
    }

    // Проверяем, что количество записей не изменилось
    var countAfterSecond int
    err = db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&countAfterSecond)
    if err != nil {
        t.Fatalf("Failed to count migrations after second run: %v", err)
    }

    if countAfterFirst != countAfterSecond {
        t.Fatalf("Migration was applied twice. Count after first: %d, count after second: %d", countAfterFirst, countAfterSecond)
    }
}

func TestMigrationWithError(t *testing.T) {
    // Настраиваем тестовую среду
    _, teardown := testutils.SetupTestEnvironment(t, false)
    defer teardown()

    // Создаем базу данных без использования InitDB (чтобы не создавать таблицу pens)
    db, err := sql.Open("sqlite3", "test.db")
    if err != nil {
        t.Fatalf("Failed to open database: %v", err)
    }
    defer db.Close()

    // Пытаемся запустить миграции на несуществующей таблице
    err = app.RunMigrations(db)
    if err == nil {
        t.Fatalf("Expected migration to fail, but it succeeded")
    }

    // Проверяем, что запись о миграции не была добавлена
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM migrations WHERE id = 1").Scan(&count)
    if err != nil {
        // Это ожидаемо, так как миграция должна была провалиться
        t.Logf("Expected error querying migrations table: %v", err)
        return
    }

    if count != 0 {
        t.Fatalf("Expected no migration records, but found %d", count)
    }
}

func TestMigrationTimestamp(t *testing.T) {
    // Настраиваем тестовую среду
    _, teardown := testutils.SetupTestEnvironment(t, false)
    defer teardown()

    // Инициализируем базу данных
    db, err := app.InitDB()
    if err != nil {
        t.Fatalf("Failed to initialize database: %v", err)
    }
    defer db.Close()

    // Получаем timestamp миграции
    var appliedAt string
    err = db.QueryRow("SELECT applied_at FROM migrations WHERE id = 1").Scan(&appliedAt)
    if err != nil {
        t.Fatalf("Failed to get migration timestamp: %v", err)
    }

    // Парсим timestamp
	parsedTime, err := time.Parse(time.RFC3339, appliedAt)
    if err != nil {
        t.Fatalf("Failed to parse migration timestamp: %v", err)
    }

    // Проверяем, что timestamp не старше 1 минуты
    if time.Since(parsedTime) > time.Minute {
        t.Fatalf("Migration timestamp is too old: %v", parsedTime)
    }
}

func TestMigrationTableStructure(t *testing.T) {
    // Настраиваем тестовую среду
    _, teardown := testutils.SetupTestEnvironment(t, false)
    defer teardown()

    // Инициализируем базу данных
    db, err := app.InitDB()
    if err != nil {
        t.Fatalf("Failed to initialize database: %v", err)
    }
    defer db.Close()

    // Проверяем структуру таблицы migrations
    rows, err := db.Query("PRAGMA table_info(migrations)")
    if err != nil {
        t.Fatalf("Failed to get migrations table info: %v", err)
    }
    defer rows.Close()

    expectedColumns := map[string]string{
        "id":         "INTEGER",
        "name":       "TEXT",
        "applied_at": "TIMESTAMP",
    }

    foundColumns := make(map[string]string)
    for rows.Next() {
        var cid int
        var name, dataType string
        var notNull, defaultValue, pk interface{}
        
        err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
        if err != nil {
            t.Fatalf("Failed to scan column info: %v", err)
        }
        
        foundColumns[name] = dataType
    }

    for expectedName, expectedType := range expectedColumns {
        if foundType, exists := foundColumns[expectedName]; !exists {
            t.Errorf("Expected column '%s' not found in migrations table", expectedName)
        } else if foundType != expectedType {
            t.Errorf("Expected column '%s' type to be '%s', but got '%s'", expectedName, expectedType, foundType)
        }
    }
}

func TestIsActiveColumnDefault(t *testing.T) {
    // Настраиваем тестовую среду
    _, teardown := testutils.SetupTestEnvironment(t, false)
    defer teardown()

    // Инициализируем базу данных
    db, err := app.InitDB()
    if err != nil {
        t.Fatalf("Failed to initialize database: %v", err)
    }
    defer db.Close()

    // Вставляем тестового пользователя без указания is_active
    _, err = db.Exec("INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length) VALUES ('testuser', 12345, 67890, 10)")
    if err != nil {
        t.Fatalf("Failed to insert test user: %v", err)
    }

    // Проверяем, что is_active по умолчанию TRUE
    var isActive bool
    err = db.QueryRow("SELECT is_active FROM pens WHERE tg_pen_id = 12345").Scan(&isActive)
    if err != nil {
        t.Fatalf("Failed to query is_active: %v", err)
    }

    if !isActive {
        t.Fatalf("Expected is_active to be TRUE by default, but got FALSE")
    }
}
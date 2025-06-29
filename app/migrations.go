package app

import (
    "database/sql"
    "log"
)

// Migration представляет собой миграцию базы данных
type Migration struct {
    ID      int
    Name    string
    SQL     string
}

// Список всех миграций в порядке их применения
var migrations = []Migration{
	{
		ID:   1,
		Name: "add_is_active_column",
		SQL:  "ALTER TABLE pens ADD COLUMN is_active BOOLEAN DEFAULT TRUE",
	},
}

// RunMigrations выполняет миграции, которые еще не были применены
func RunMigrations(db *sql.DB) error {
    // Создать таблицу migrations, если она не существует
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS migrations (
            id INTEGER PRIMARY KEY,
            name TEXT,
            applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    `)
    if err != nil {
        log.Printf("Error creating migrations table: %v", err)
        return err
    }

    // Проверяем каждую миграцию
    for _, migration := range migrations {
        // Проверяем, была ли миграция уже применена
        var count int
        err := db.QueryRow("SELECT COUNT(*) FROM migrations WHERE id = ?", migration.ID).Scan(&count)
        if err != nil {
            log.Printf("Error checking migration %d: %v", migration.ID, err)
            return err
        }

        // Если миграция еще не была применена
        if count == 0 {
            log.Printf("Applying migration %d: %s", migration.ID, migration.Name)
            
            // Начинаем транзакцию для атомарности
            tx, err := db.Begin()
            if err != nil {
                log.Printf("Error beginning transaction for migration %d: %v", migration.ID, err)
                return err
            }
            
            // Выполняем SQL миграции
            _, err = tx.Exec(migration.SQL)
            if err != nil {
                if rbErr := tx.Rollback(); rbErr != nil {
                    log.Printf("Error on transaction rollback: %v", rbErr)
                }
                log.Printf("Error executing migration %d: %v", migration.ID, err)
                return err
            }
            
            // Записываем информацию о применённой миграции
            _, err = tx.Exec("INSERT INTO migrations (id, name) VALUES (?, ?)", migration.ID, migration.Name)
            if err != nil {
                if rbErr := tx.Rollback(); rbErr != nil {
                    log.Printf("Error on transaction rollback: %v", rbErr)
                }
                log.Printf("Error recording migration %d: %v", migration.ID, err)
                return err
            }
            
            // Подтверждаем транзакцию
            err = tx.Commit()
            if err != nil {
                log.Printf("Error committing migration %d: %v", migration.ID, err)
                return err
            }
            
            log.Printf("Successfully applied migration %d: %s", migration.ID, migration.Name)
        }
    }
    
    return nil
}

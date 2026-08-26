package app

import (
	"database/sql"
	"log"
)

// Migration представляет собой миграцию базы данных
type Migration struct {
	ID   int
	Name string
	SQL  string
}

// Список всех миграций в порядке их применения
var migrations = []Migration{
	{
		ID:   1,
		Name: "add_is_active_column",
		SQL:  "ALTER TABLE pens ADD COLUMN is_active BOOLEAN DEFAULT TRUE",
	},
	{
		ID:   2,
		Name: "create_gemini_memories_table",
		SQL: `
		CREATE TABLE IF NOT EXISTS gemini_memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
			CREATE INDEX IF NOT EXISTS idx_gemini_memories_chat_created_at
				ON gemini_memories(chat_id, created_at);
			`,
	},
	{
		ID:   3,
		Name: "create_gemini_user_facts_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS gemini_user_facts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				chat_id INTEGER NOT NULL,
				user_name TEXT NOT NULL,
				fact TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_gemini_user_facts_chat_created_at
				ON gemini_user_facts(chat_id, created_at);
			CREATE UNIQUE INDEX IF NOT EXISTS idx_gemini_user_facts_unique
				ON gemini_user_facts(chat_id, user_name, fact);
			`,
	},
	{
		ID:   4,
		Name: "add_user_id_to_gemini_user_facts",
		SQL: `
			ALTER TABLE gemini_user_facts ADD COLUMN user_id INTEGER NOT NULL DEFAULT 0;
			CREATE INDEX IF NOT EXISTS idx_gemini_user_facts_chat_user_id
				ON gemini_user_facts(chat_id, user_id);
			UPDATE gemini_user_facts
			SET user_id = COALESCE((
				SELECT pens.tg_pen_id
				FROM pens
				WHERE pens.tg_chat_id = gemini_user_facts.chat_id
					AND pens.pen_name IS NOT NULL
					AND lower(trim(pens.pen_name, ' @')) = lower(trim(gemini_user_facts.user_name, ' @'))
			), 0);
			`,
	},
	{
		ID:   5,
		Name: "create_chat_members_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS chat_members (
				chat_id INTEGER NOT NULL,
				user_id INTEGER NOT NULL,
				first_name TEXT NOT NULL DEFAULT '',
				last_name TEXT NOT NULL DEFAULT '',
				user_name TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (chat_id, user_id)
			);
			INSERT OR IGNORE INTO chat_members (chat_id, user_id, user_name)
			SELECT tg_chat_id, tg_pen_id, COALESCE(pen_name, '')
			FROM pens
			WHERE tg_chat_id IS NOT NULL AND tg_pen_id IS NOT NULL;
			`,
	},
	{
		ID:   6,
		Name: "add_user_id_to_gemini_memories",
		// Роль в памяти — отображаемое имя, а по нему тёзки неотличимы:
		// /forgetme одного стирал бы реплики другого. Старые строки остаются
		// с user_id = 0, но память живёт сутки, так что бэкфилл не нужен.
		SQL: `
			ALTER TABLE gemini_memories ADD COLUMN user_id INTEGER NOT NULL DEFAULT 0;
			CREATE INDEX IF NOT EXISTS idx_gemini_memories_chat_user_id
				ON gemini_memories(chat_id, user_id);
			`,
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

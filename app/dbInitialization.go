package app

import (
	"database/sql"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrations "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
)

// InitDB инициализирует базу данных
func InitDB() (*sql.DB, error) {
	dbDir := "./data"

	// Проверка, существует ли директория базы данных
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		// Директория не существует, создаём её
		err = os.MkdirAll(dbDir, os.ModePerm)
		if err != nil {
			log.Printf("Error creating directory: %v", err)
			return nil, err
		}
	}

	dbPath := "./data/pens.db"

	// Проверка, существует ли директория базы данных
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		// Директория не существует, создаём её
		err = os.MkdirAll(dbDir, os.ModePerm)
		if err != nil {
			log.Printf("Error creating directory: %v", err)
			return nil, err
		}
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("Error opening database: %v", err)
		return nil, err
	}

	config := sqlite3migrations.Config{
		MigrationsTable: "migrations",
		DatabaseName:    dbPath,
	}

	driver, err := sqlite3migrations.WithInstance(db, &config)
	if err != nil {
		log.Printf("%v", err)
		return nil, err
	}

	m, err := migrate.NewWithDatabaseInstance("file://./app/db/migrations", "sqlite3", driver)
	if err != nil {
		log.Printf("%v", err)
		return nil, err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Printf("%v", err)
		return nil, err
	}

	// Установка режима журнала WAL
	_, err = db.Exec("PRAGMA journal_mode = WAL;")
	if err != nil {
		log.Printf("Error setting journal_mode: %v", err)
		return nil, err
	}

	log.Println("Database opened successfully")
	return db, nil
}

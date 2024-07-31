package app

import (
    "database/sql"
    "log"
    "fmt"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// RegisterBot регистрирует пользователя в боте
func RegisterBot(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB, sendWelcomeMessage bool) {
    // Логика регистрации в боте
    userID := update.Message.From.ID
    chatID := update.Message.Chat.ID
    userName := update.Message.From.UserName

    // Вставка пользователя в базу данных
    insertQuery := `
    INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length, handsome_count, unhandsome_count)
    VALUES (?, ?, ?, ?, 0, 0)
    `
    _, err := db.Exec(insertQuery, userName, userID, chatID, 5)
    if err != nil {
        log.Printf("Error inserting user into database: %v", err)
        return
    }

    // Обновление размера пениса
    err = UpdatepenSize(db, chatID, 5)
    if err != nil {
        log.Printf("Error updating pen size: %v", err)
        return
    }

    // Отправка ответного сообщения, если флаг установлен
    if sendWelcomeMessage {
        sendMessage(chatID, "Велком ту зе клаб, бади 😎🤝😎", bot, update.Message.MessageID)
    }

    fmt.Println("User registered in bot")
}
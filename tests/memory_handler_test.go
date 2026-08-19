package tests

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
	_ "unsafe"

	"github.com/denis1011101/super_cm_bot/app"
	_ "github.com/denis1011101/super_cm_bot/app/handlers"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

//go:linkname buildMyFactsMessage github.com/denis1011101/super_cm_bot/app/handlers.buildMyFactsMessage
func buildMyFactsMessage(db *sql.DB, chatID int64, user *tgbotapi.User) (string, error)

//go:linkname deleteMyFacts github.com/denis1011101/super_cm_bot/app/handlers.deleteMyFacts
func deleteMyFacts(db *sql.DB, chatID int64, user *tgbotapi.User) (int64, error)

func TestLoadGeminiUserFactsByNames(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(444)
	now := time.Now()

	testFacts := []struct {
		chatID         int64
		userName, fact string
		at             time.Time
	}{
		{chatID, "Денис", "старый факт", now.Add(-3 * time.Hour)},
		{chatID, "denis1011101", "новый факт", now.Add(-time.Hour)},
		{chatID, "ДЕНИС", "новый факт", now},
		{chatID, "Денис", "третий факт", now},
		{chatID, "denis1011101", "четвёртый факт", now},
		{chatID, "Дима", "чужой пользователь", now},
		{chatID + 1, "Денис", "чужой чат", now},
	}
	for _, testFact := range testFacts {
		if err := app.SaveGeminiUserFact(db, testFact.chatID, testFact.userName, testFact.fact, testFact.at); err != nil {
			t.Fatalf("save fact: %v", err)
		}
	}

	facts, err := app.LoadGeminiUserFactsByNames(db, chatID, []string{"денис", "@Denis1011101"}, 3)
	if err != nil {
		t.Fatalf("load facts by names: %v", err)
	}
	if len(facts) != 3 {
		t.Fatalf("expected 3 own unique facts, got %d: %+v", len(facts), facts)
	}
	for _, fact := range facts {
		if fact.Fact == "чужой пользователь" || fact.Fact == "чужой чат" {
			t.Fatalf("unrelated fact leaked into results: %+v", facts)
		}
	}
}

func TestSaveGeminiUserFactKeepsThirtyNewestPerName(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(445)
	start := time.Now().Add(-time.Hour)

	for i := 0; i < 35; i++ {
		if err := app.SaveGeminiUserFact(
			db,
			chatID,
			"Денис",
			fmt.Sprintf("факт-%02d", i),
			start.Add(time.Duration(i)*time.Minute),
		); err != nil {
			t.Fatalf("save fact %d: %v", i, err)
		}
	}

	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM gemini_user_facts WHERE chat_id = ? AND user_name = ?",
		chatID,
		"Денис",
	).Scan(&count); err != nil {
		t.Fatalf("count facts: %v", err)
	}
	if count != 30 {
		t.Fatalf("expected 30 facts after pruning, got %d", count)
	}

	var oldestRemaining string
	if err := db.QueryRow(
		`SELECT fact FROM gemini_user_facts
		WHERE chat_id = ? AND user_name = ?
		ORDER BY created_at, id
		LIMIT 1`,
		chatID,
		"Денис",
	).Scan(&oldestRemaining); err != nil {
		t.Fatalf("load oldest remaining fact: %v", err)
	}
	if oldestRemaining != "факт-05" {
		t.Fatalf("expected oldest facts to be pruned, oldest remaining is %q", oldestRemaining)
	}
}

func TestDeleteMyFactsOnlyDeletesCurrentUserAndChat(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(446)
	user := &tgbotapi.User{FirstName: "Денис", LastName: "Иванов", UserName: "denis1011101"}
	now := time.Now()

	testFacts := []struct {
		chatID   int64
		userName string
	}{
		{chatID, "Денис"},
		{chatID, "DENIS1011101"},
		{chatID, "Денис Иванов"},
		{chatID, "Дима"},
		{chatID + 1, "Денис"},
	}
	for i, testFact := range testFacts {
		if err := app.SaveGeminiUserFact(db, testFact.chatID, testFact.userName, fmt.Sprintf("факт-%d", i), now); err != nil {
			t.Fatalf("save fact: %v", err)
		}
	}

	deleted, err := deleteMyFacts(db, chatID, user)
	if err != nil {
		t.Fatalf("delete own facts: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("expected 3 deleted facts, got %d", deleted)
	}

	var remaining int
	if err := db.QueryRow("SELECT COUNT(*) FROM gemini_user_facts").Scan(&remaining); err != nil {
		t.Fatalf("count remaining facts: %v", err)
	}
	if remaining != 2 {
		t.Fatalf("expected other user and chat facts to remain, got %d rows", remaining)
	}
}

func TestBuildMyFactsMessage(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(555)
	user := &tgbotapi.User{FirstName: "Денис", LastName: "Иванов", UserName: "denis1011101"}

	for _, fact := range []app.GeminiUserFact{
		{UserName: "Денис", Fact: "любит Go"},
		{UserName: "denis1011101", Fact: "катается на сноуборде"},
		{UserName: "Дима", Fact: "это чужой факт"},
	} {
		if err := app.SaveGeminiUserFact(db, chatID, fact.UserName, fact.Fact, time.Now()); err != nil {
			t.Fatalf("save fact: %v", err)
		}
	}

	message, err := buildMyFactsMessage(db, chatID, user)
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	if !strings.Contains(message, "любит Go") || !strings.Contains(message, "катается на сноуборде") {
		t.Fatalf("own facts are missing: %q", message)
	}
	if strings.Contains(message, "чужой факт") {
		t.Fatalf("another user's fact leaked into message: %q", message)
	}
}

func TestBuildMyFactsMessageWhenEmpty(t *testing.T) {
	db := setupGeminiDB(t)

	message, err := buildMyFactsMessage(db, 666, &tgbotapi.User{FirstName: "Никто"})
	if err != nil {
		t.Fatalf("build empty message: %v", err)
	}
	if message != "ИИ пока ничего о тебе не запомнил." {
		t.Fatalf("unexpected empty message: %q", message)
	}
}

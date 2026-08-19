package tests

import (
	"database/sql"
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
		{chatID, "Дима", "чужой пользователь", now},
		{chatID + 1, "Денис", "чужой чат", now},
	}
	for _, testFact := range testFacts {
		if err := app.SaveGeminiUserFact(db, testFact.chatID, testFact.userName, testFact.fact, testFact.at); err != nil {
			t.Fatalf("save fact: %v", err)
		}
	}

	facts, err := app.LoadGeminiUserFactsByNames(db, chatID, []string{"денис", "@Denis1011101"}, 10)
	if err != nil {
		t.Fatalf("load facts by names: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 own unique facts, got %d: %+v", len(facts), facts)
	}
	if facts[0].Fact != "новый факт" || facts[1].Fact != "старый факт" {
		t.Fatalf("facts are missing or not newest-first: %+v", facts)
	}

	limited, err := app.LoadGeminiUserFactsByNames(db, chatID, []string{"Денис", "denis1011101"}, 1)
	if err != nil || len(limited) != 1 || limited[0].Fact != "новый факт" {
		t.Fatalf("limit was not respected: %+v, err: %v", limited, err)
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

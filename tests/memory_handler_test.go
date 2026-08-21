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

func TestLoadGeminiUserFactsForUser(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(444)
	now := time.Now()

	testFacts := []struct {
		chatID         int64
		userID         int64
		userName, fact string
		at             time.Time
	}{
		{chatID, 1, "Денис", "старый факт", now.Add(-3 * time.Hour)},
		{chatID, 1, "denis1011101", "новый факт", now.Add(-time.Hour)},
		{chatID, 1, "Денис Иванов", "третий факт", now},
		{chatID, 2, "Денис", "факт тёзки", now},
		{chatID, 2, "Дима", "чужой пользователь", now},
		{chatID + 1, 1, "Денис", "чужой чат", now},
	}
	for _, testFact := range testFacts {
		if err := app.SaveGeminiUserFact(db, testFact.chatID, testFact.userID, testFact.userName, testFact.fact, testFact.at); err != nil {
			t.Fatalf("save fact: %v", err)
		}
	}

	facts, err := app.LoadGeminiUserFactsForUser(db, chatID, 1, 3)
	if err != nil {
		t.Fatalf("load facts for user: %v", err)
	}
	if len(facts) != 3 {
		t.Fatalf("expected 3 own facts, got %d: %+v", len(facts), facts)
	}
	for _, fact := range facts {
		switch fact.Fact {
		case "факт тёзки", "чужой пользователь", "чужой чат":
			t.Fatalf("unrelated fact leaked into results: %+v", facts)
		}
	}
}

func TestLoadGeminiUserFactsForUserSkipsOwnerlessFacts(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(447)

	if err := app.SaveGeminiUserFact(db, chatID, 0, "Денис", "ничей факт", time.Now()); err != nil {
		t.Fatalf("save ownerless fact: %v", err)
	}

	facts, err := app.LoadGeminiUserFactsForUser(db, chatID, 1, 3)
	if err != nil {
		t.Fatalf("load facts for user: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("expected no facts for a user without owned facts, got %+v", facts)
	}
}

func TestSaveGeminiUserFactClaimsOwnerlessFact(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(448)
	now := time.Now()

	if err := app.SaveGeminiUserFact(db, chatID, 0, "Денис", "любит Go", now); err != nil {
		t.Fatalf("save ownerless fact: %v", err)
	}
	if err := app.SaveGeminiUserFact(db, chatID, 7, "Денис", "любит Go", now); err != nil {
		t.Fatalf("save owned fact: %v", err)
	}

	var (
		rows   int
		userID int64
	)
	if err := db.QueryRow("SELECT COUNT(*), MAX(user_id) FROM gemini_user_facts WHERE chat_id = ?", chatID).Scan(&rows, &userID); err != nil {
		t.Fatalf("count facts: %v", err)
	}
	if rows != 1 || userID != 7 {
		t.Fatalf("expected the stored fact to be claimed by user 7, got %d rows with user_id %d", rows, userID)
	}
}

func TestSaveGeminiUserFactKeepsThirtyNewestPerOwner(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(445)
	start := time.Now().Add(-time.Hour)

	for i := 0; i < 35; i++ {
		if err := app.SaveGeminiUserFact(
			db,
			chatID,
			1,
			"Денис",
			fmt.Sprintf("факт-%02d", i),
			start.Add(time.Duration(i)*time.Minute),
		); err != nil {
			t.Fatalf("save fact %d: %v", i, err)
		}
	}

	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM gemini_user_facts WHERE chat_id = ? AND user_id = ?",
		chatID,
		1,
	).Scan(&count); err != nil {
		t.Fatalf("count facts: %v", err)
	}
	if count != 30 {
		t.Fatalf("expected 30 facts after pruning, got %d", count)
	}

	var oldestRemaining string
	if err := db.QueryRow(
		`SELECT fact FROM gemini_user_facts
		WHERE chat_id = ? AND user_id = ?
		ORDER BY created_at, id
		LIMIT 1`,
		chatID,
		1,
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
	user := &tgbotapi.User{ID: 1, FirstName: "Денис", LastName: "Иванов", UserName: "denis1011101"}
	now := time.Now()

	testFacts := []struct {
		chatID   int64
		userID   int64
		userName string
	}{
		{chatID, 1, "Денис"},
		{chatID, 1, "DENIS1011101"},
		{chatID, 1, "Денис Иванов"},
		{chatID, 2, "Денис"},
		{chatID, 2, "Дима"},
		{chatID + 1, 1, "Денис"},
	}
	for i, testFact := range testFacts {
		if err := app.SaveGeminiUserFact(db, testFact.chatID, testFact.userID, testFact.userName, fmt.Sprintf("факт-%d", i), now); err != nil {
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
	if remaining != 3 {
		t.Fatalf("expected facts of the namesake and of the other chat to remain, got %d rows", remaining)
	}
}

func TestBuildMyFactsMessage(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(555)
	user := &tgbotapi.User{ID: 1, FirstName: "Денис", LastName: "Иванов", UserName: "denis1011101"}

	testFacts := []struct {
		userID   int64
		userName string
		fact     string
	}{
		{1, "Денис", "любит Go"},
		{1, "denis1011101", "катается на сноуборде"},
		{2, "Денис", "это факт тёзки"},
		{2, "Дима", "это чужой факт"},
	}
	for _, testFact := range testFacts {
		if err := app.SaveGeminiUserFact(db, chatID, testFact.userID, testFact.userName, testFact.fact, time.Now()); err != nil {
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
	if strings.Contains(message, "чужой факт") || strings.Contains(message, "факт тёзки") {
		t.Fatalf("another user's fact leaked into message: %q", message)
	}
}

func TestBuildMyFactsMessageWhenEmpty(t *testing.T) {
	db := setupGeminiDB(t)

	message, err := buildMyFactsMessage(db, 666, &tgbotapi.User{ID: 9, FirstName: "Никто"})
	if err != nil {
		t.Fatalf("build empty message: %v", err)
	}
	if message != "ИИ пока ничего о тебе не запомнил." {
		t.Fatalf("unexpected empty message: %q", message)
	}
}

func TestResolveGeminiFactUserID(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(777)
	author := &tgbotapi.User{ID: 1, FirstName: "Денис", LastName: "Иванов", UserName: "denis1011101"}
	dmitri := &tgbotapi.User{ID: 2, FirstName: "Дмитрий", UserName: "dima_t"}

	app.RememberChatMember(db, chatID, author)
	app.RememberChatMember(db, chatID, dmitri)

	cases := []struct {
		name string
		want int64
	}{
		{"Денис", 1},
		{"Denis", 1},
		{"ДЕНИС ИВАНОВ", 1},
		{"@denis1011101", 1},
		{"Дмитрий", 2},
		{"Дима", 2},
		{"Димон", 2},
		{"Андрей", 0},
		{"", 0},
	}
	for _, testCase := range cases {
		if got := app.ResolveGeminiFactUserID(db, chatID, testCase.name, author); got != testCase.want {
			t.Errorf("resolve %q: expected %d, got %d", testCase.name, testCase.want, got)
		}
	}
}

func TestResolveGeminiFactUserIDLeavesNamesakesUnresolved(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(778)
	author := &tgbotapi.User{ID: 5, FirstName: "Юрий", UserName: "yrcnbt"}

	app.RememberChatMember(db, chatID, &tgbotapi.User{ID: 1, FirstName: "Денис", UserName: "denis1011101"})
	app.RememberChatMember(db, chatID, &tgbotapi.User{ID: 2, FirstName: "Denis", UserName: "denis_two"})
	app.RememberChatMember(db, chatID, author)

	if got := app.ResolveGeminiFactUserID(db, chatID, "Денис", author); got != 0 {
		t.Fatalf("a name shared by two members must stay unresolved, got %d", got)
	}
	if got := app.ResolveGeminiFactUserID(db, chatID, "Юра", author); got != 5 {
		t.Fatalf("the author must still be resolved by a diminutive, got %d", got)
	}
}

func TestClaimOwnerlessFactsOnMyFacts(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(779)
	user := &tgbotapi.User{ID: 1, FirstName: "Denis", UserName: "denis1011101"}
	app.RememberChatMember(db, chatID, user)

	for i, userName := range []string{"Denis", "Денис", "Дима"} {
		if err := app.SaveGeminiUserFact(db, chatID, 0, userName, fmt.Sprintf("старый факт-%d", i), time.Now()); err != nil {
			t.Fatalf("save legacy fact: %v", err)
		}
	}

	message, err := buildMyFactsMessage(db, chatID, user)
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	if !strings.Contains(message, "старый факт-0") || !strings.Contains(message, "старый факт-1") {
		t.Fatalf("legacy facts of the user were not claimed: %q", message)
	}
	if strings.Contains(message, "старый факт-2") {
		t.Fatalf("a fact about somebody else was claimed: %q", message)
	}

	var claimed int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM gemini_user_facts WHERE chat_id = ? AND user_id = ?",
		chatID, user.ID,
	).Scan(&claimed); err != nil {
		t.Fatalf("count claimed facts: %v", err)
	}
	if claimed != 2 {
		t.Fatalf("expected 2 claimed facts, got %d", claimed)
	}
}

func TestClaimOwnerlessFactsSkipsNamesakes(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(780)
	user := &tgbotapi.User{ID: 1, FirstName: "Денис", UserName: "denis1011101"}
	app.RememberChatMember(db, chatID, user)
	app.RememberChatMember(db, chatID, &tgbotapi.User{ID: 2, FirstName: "Denis", UserName: "denis_two"})

	if err := app.SaveGeminiUserFact(db, chatID, 0, "Денис", "чей-то факт", time.Now()); err != nil {
		t.Fatalf("save legacy fact: %v", err)
	}

	claimed, err := app.ClaimOwnerlessGeminiUserFacts(db, chatID, user)
	if err != nil {
		t.Fatalf("claim facts: %v", err)
	}
	if claimed != 0 {
		t.Fatalf("a fact under a shared name must stay ownerless, claimed %d", claimed)
	}

	message, err := buildMyFactsMessage(db, chatID, user)
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	if message != "ИИ пока ничего о тебе не запомнил." {
		t.Fatalf("unexpected message: %q", message)
	}
}

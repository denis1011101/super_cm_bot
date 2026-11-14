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
	if message != "Пенис-ИИ пока ничего о тебе не запомнил." {
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
	if message != "Пенис-ИИ пока ничего о тебе не запомнил." {
		t.Fatalf("unexpected message: %q", message)
	}
}

func TestBuildMyFactsMessageMentionsSampling(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(781)
	user := &tgbotapi.User{ID: 1, FirstName: "Денис", UserName: "denis1011101"}

	for i := 0; i < 7; i++ {
		if err := app.SaveGeminiUserFact(db, chatID, user.ID, "Денис", fmt.Sprintf("факт-%d", i), time.Now()); err != nil {
			t.Fatalf("save fact: %v", err)
		}
	}

	message, err := buildMyFactsMessage(db, chatID, user)
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	if !strings.HasPrefix(message, "Вот 3 случайных факта из 7, что пенис-ИИ о тебе помнит:") {
		t.Fatalf("the message must say the facts are a random sample: %q", message)
	}
	if strings.Count(message, "\n• ") != 3 {
		t.Fatalf("expected 3 facts in the message: %q", message)
	}
}

func TestBuildMyFactsMessageWithoutSampling(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(782)
	user := &tgbotapi.User{ID: 1, FirstName: "Денис", UserName: "denis1011101"}

	for i := 0; i < 2; i++ {
		if err := app.SaveGeminiUserFact(db, chatID, user.ID, "Денис", fmt.Sprintf("факт-%d", i), time.Now()); err != nil {
			t.Fatalf("save fact: %v", err)
		}
	}

	message, err := buildMyFactsMessage(db, chatID, user)
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	if !strings.HasPrefix(message, "Вот что пенис-ИИ запомнил о тебе:") {
		t.Fatalf("all facts fit, sampling must not be mentioned: %q", message)
	}
}

// TestForgetGeminiUserClearsFactsAndOwnLines — /forgetme обязан вычистить и
// краткосрочную память автора: иначе бот говорит "забыл", а ИИ сутки видит
// реплики, из которых те же факты выводятся заново
func TestForgetGeminiUserClearsFactsAndOwnLines(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(447)
	user := &tgbotapi.User{ID: 1, FirstName: "Денис", UserName: "denis1011101"}
	now := time.Now()

	if err := app.SaveGeminiUserFact(db, chatID, user.ID, "Денис", "не спит сутки", now); err != nil {
		t.Fatalf("save fact: %v", err)
	}
	if err := app.SaveGeminiUserFact(db, chatID, 2, "Дима", "икона стиля", now); err != nil {
		t.Fatalf("save fact: %v", err)
	}

	// у второго участника то же отображаемое имя: чистка по имени вынесла бы
	// и его реплики, поэтому адресуемся по tg id
	lines := []struct {
		chatID int64
		userID int64
		role   string
	}{
		{chatID, user.ID, "Денис"},
		{chatID, user.ID, "Денис"},
		{chatID, 2, "Денис"},
		{chatID, 3, "Дима"},
		{chatID, 0, "bot"},
		{chatID + 1, user.ID, "Денис"},
	}
	for i, line := range lines {
		if err := app.SaveGeminiMemory(db, line.chatID, line.userID, line.role, fmt.Sprintf("реплика-%d", i), now); err != nil {
			t.Fatalf("save memory: %v", err)
		}
	}

	deleted, err := app.ForgetGeminiUser(db, chatID, user)
	if err != nil {
		t.Fatalf("forget user: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted fact, got %d", deleted)
	}

	var facts int
	if err := db.QueryRow("SELECT COUNT(*) FROM gemini_user_facts").Scan(&facts); err != nil {
		t.Fatalf("count remaining facts: %v", err)
	}
	if facts != 1 {
		t.Fatalf("expected the fact of the other user to remain, got %d rows", facts)
	}

	var memories int
	if err := db.QueryRow("SELECT COUNT(*) FROM gemini_memories").Scan(&memories); err != nil {
		t.Fatalf("count remaining memories: %v", err)
	}
	if memories != 4 {
		t.Fatalf("expected the lines of the namesake, of the other user, of the bot and of the other chat to remain, got %d rows", memories)
	}

	context, err := app.LoadGeminiMemoryContext(db, chatID, 10, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("LoadGeminiMemoryContext: %v", err)
	}
	if strings.Contains(context, "реплика-0") || strings.Contains(context, "реплика-1") {
		t.Fatalf("author lines must be gone from the context, got %q", context)
	}
	if !strings.Contains(context, "реплика-2") {
		t.Fatalf("the namesake keeps their lines, got %q", context)
	}
}

// TestForgetGeminiUserRollsBackOnFailure — очистки идут одной транзакцией,
// поэтому упавшая вторая не оставляет пользователя с половиной удалённого
func TestForgetGeminiUserRollsBackOnFailure(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(448)
	user := &tgbotapi.User{ID: 1, FirstName: "Денис", UserName: "denis1011101"}
	now := time.Now()

	if err := app.SaveGeminiUserFact(db, chatID, user.ID, "Денис", "не спит сутки", now); err != nil {
		t.Fatalf("save fact: %v", err)
	}
	if _, err := db.Exec("ALTER TABLE gemini_memories RENAME TO gemini_memories_hidden"); err != nil {
		t.Fatalf("hide memories table: %v", err)
	}

	if _, err := app.ForgetGeminiUser(db, chatID, user); err == nil {
		t.Fatal("expected an error when the memories table is unavailable")
	}

	var facts int
	if err := db.QueryRow("SELECT COUNT(*) FROM gemini_user_facts").Scan(&facts); err != nil {
		t.Fatalf("count remaining facts: %v", err)
	}
	if facts != 1 {
		t.Fatalf("the fact must survive a rolled back cleanup, got %d rows", facts)
	}
}

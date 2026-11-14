package tests

import (
	"testing"

	"github.com/denis1011101/super_cm_bot/app"
)

func TestNormalizePersonNameMatchesSpellings(t *testing.T) {
	groups := [][]string{
		{"Денис", "Denis", "𝘿𝙚𝙣𝙞𝙨", "  денис  ", "Дениска"},
		{"Дмитрий", "Dmitriy", "Дима", "Димас", "Димон", "Митя"},
		{"Юрий", "Yuriy", "Yurii", "Юра", "Юрик"},
		{"Михаил", "Mikhail", "Миша", "Мишка"},
		{"Евгений", "Evgeniy", "Женя", "Жека"},
		{"Александр", "Aleksandr", "Саша", "Саня", "Шурик"},
		{"Артём", "Артем", "Тёма", "Artyom"},
	}

	for _, group := range groups {
		want := app.NormalizePersonName(group[0])
		if want == "" {
			t.Fatalf("%q normalized to an empty key", group[0])
		}
		for _, name := range group[1:] {
			if got := app.NormalizePersonName(name); got != want {
				t.Errorf("%q and %q must share a key, got %q and %q", group[0], name, want, got)
			}
		}
	}
}

func TestNormalizePersonNameKeepsDifferentPeopleApart(t *testing.T) {
	names := []string{"Денис", "Дмитрий", "Андрей", "Юрий", "Евгений", "denis1011101"}

	seen := make(map[string]string, len(names))
	for _, name := range names {
		key := app.NormalizePersonName(name)
		if other, exists := seen[key]; exists {
			t.Fatalf("%q and %q collapsed into the same key %q", other, name, key)
		}
		seen[key] = name
	}

	if app.NormalizePersonName("") != "" {
		t.Fatalf("an empty name must give an empty key")
	}
	if app.NormalizePersonName("@Denis1011101") != app.NormalizePersonName("denis1011101") {
		t.Fatalf("the @ prefix must not change the key")
	}
}

func TestPersonNameKeys(t *testing.T) {
	keys := app.PersonNameKeys("Денис", "Иванов", "@denis1011101")

	for _, wanted := range []string{"denis", "denis1011101", "denisivanov"} {
		found := false
		for _, key := range keys {
			if key == wanted {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected key %q among %v", wanted, keys)
		}
	}
}

package app

import (
	"strings"
	"unicode"
)

// В чате одного человека зовут по-разному: "Денис", "Denis", "𝘿𝙚𝙣𝙞𝙨", а в
// третьем лице — ещё и уменьшительным ("Дима" про Дмитрия). NormalizePersonName
// сводит все эти написания к одному ключу, по которому участник и опознаётся.
func NormalizePersonName(name string) string {
	var cleaned strings.Builder
	for _, r := range name {
		r = foldStyledRune(r)
		r = unicode.ToLower(r)
		if r == 'ё' {
			r = 'е'
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cleaned.WriteRune(r)
		}
	}

	key := transliterate(cleaned.String())
	key = reduceNameSpelling(key)
	if full, exists := nameDiminutives[key]; exists {
		return full
	}
	return key
}

// PersonNameKeys собирает ключи, по которым участник может быть назван:
// имя, юзернейм и имя с фамилией.
func PersonNameKeys(firstName, lastName, userName string) []string {
	raw := []string{firstName, userName, strings.TrimSpace(firstName + " " + lastName)}

	keys := make([]string, 0, len(raw))
	for _, value := range raw {
		key := NormalizePersonName(value)
		if key == "" {
			continue
		}
		if !containsString(keys, key) {
			keys = append(keys, key)
		}
	}
	return keys
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// foldStyledRune приводит математические алфавитные символы (𝘿, 𝕞 и прочие
// украшенные варианты латиницы) к обычной букве.
func foldStyledRune(r rune) rune {
	if letter, exists := letterlikeSymbols[r]; exists {
		return letter
	}
	if r < 0x1D400 || r > 0x1D7CB {
		return r
	}

	// каждый стиль — это 26 заглавных и 26 строчных букв подряд
	styleBases := []rune{
		0x1D400, 0x1D434, 0x1D468, 0x1D49C, 0x1D4D0, 0x1D504, 0x1D538,
		0x1D56C, 0x1D5A0, 0x1D5D4, 0x1D608, 0x1D63C, 0x1D670,
	}
	for _, base := range styleBases {
		offset := r - base
		if offset < 0 || offset > 51 {
			continue
		}
		if offset < 26 {
			return 'A' + offset
		}
		return 'a' + offset - 26
	}
	return r
}

// буквы, вырезанные из математических блоков и живущие в Letterlike Symbols
var letterlikeSymbols = map[rune]rune{
	'\u2102': 'C', '\u210B': 'H', '\u210C': 'H', '\u210D': 'H', '\u210E': 'h',
	'\u2110': 'I', '\u2111': 'I', '\u2112': 'L', '\u2113': 'l', '\u2115': 'N',
	'\u2119': 'P', '\u211A': 'Q', '\u211B': 'R', '\u211C': 'R', '\u211D': 'R',
	'\u2124': 'Z', '\u2128': 'Z', '\u212C': 'B', '\u212F': 'e', '\u2130': 'E',
	'\u2131': 'F', '\u2133': 'M', '\u2134': 'o', '\u210A': 'g', '\u212D': 'C',
}

var cyrillicToLatin = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ж': "zh",
	'з': "z", 'и': "i", 'й': "i", 'к': "k", 'л': "l", 'м': "m", 'н': "n",
	'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f",
	'х': "h", 'ц': "c", 'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y",
	'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

func transliterate(value string) string {
	var out strings.Builder
	for _, r := range value {
		if latin, exists := cyrillicToLatin[r]; exists {
			out.WriteString(latin)
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// reduceNameSpelling убирает разницу между вариантами латинской записи одного
// имени: Yuriy/Yurii/Юрий, Mikhail/Михаил, Alyona/Алёна.
func reduceNameSpelling(key string) string {
	replacer := strings.NewReplacer("kh", "h", "ts", "c", "yo", "e", "j", "i")
	key = replacer.Replace(key)

	for _, ending := range []string{"iy", "ii", "yi", "yy"} {
		if strings.HasSuffix(key, ending) {
			return strings.TrimSuffix(key, ending) + "i"
		}
	}
	if strings.HasSuffix(key, "y") {
		return strings.TrimSuffix(key, "y") + "i"
	}
	return key
}

// nameDiminutives переводит уменьшительные в полное имя: в чате про человека
// чаще пишут "Дима", а представляется он Дмитрием. Ключи и значения записаны
// уже в нормализованном виде (см. NormalizePersonName).
var nameDiminutives = map[string]string{
	"alesha":    "aleksei",
	"andryusha": "andrei",
	"andryuha":  "andrei",
	"artemka":   "artem",
	"borya":     "boris",
	"dan":       "daniil",
	"danya":     "daniil",
	"danil":     "daniil",
	"danila":    "daniil",
	"dasha":     "darya",
	"den":       "denis",
	"denya":     "denis",
	"deniska":   "denis",
	"dima":      "dmitri",
	"dimas":     "dmitri",
	"dimon":     "dmitri",
	"evgen":     "evgeni",
	"fedya":     "fedor",
	"gosha":     "georgi",
	"grisha":    "grigori",
	"igorek":    "igor",
	"ira":       "irina",
	"katya":     "ekaterina",
	"kirya":     "kirill",
	"kolya":     "nikolai",
	"kolyan":    "nikolai",
	"kostya":    "konstantin",
	"ksyusha":   "kseniya",
	"leha":      "aleksei",
	"lena":      "elena",
	"lenya":     "leonid",
	"lesha":     "aleksei",
	"lyuda":     "lyudmila",
	"maks":      "maksim",
	"maksimka":  "maksim",
	"masha":     "mariya",
	"misha":     "mihail",
	"mishka":    "mihail",
	"mitya":     "dmitri",
	"nastya":    "anastasiya",
	"nataha":    "natalya",
	"nataliya":  "natalya",
	"natasha":   "natalya",
	"nikitka":   "nikita",
	"nikitos":   "nikita",
	"olya":      "olga",
	"pasha":     "pavel",
	"petya":     "petr",
	"roma":      "roman",
	"sanek":     "aleksandr",
	"sanya":     "aleksandr",
	"sasha":     "aleksandr",
	"serega":    "sergei",
	"serezha":   "sergei",
	"shurik":    "aleksandr",
	"slava":     "vyacheslav",
	"sonya":     "sofiya",
	"stas":      "stanislav",
	"stepa":     "stepan",
	"sveta":     "svetlana",
	"tanya":     "tatyana",
	"tema":      "artem",
	"tolya":     "anatoli",
	"vanya":     "ivan",
	"vika":      "viktoriya",
	"vitya":     "viktor",
	"vova":      "vladimir",
	"vovan":     "vladimir",
	"yasha":     "yakov",
	"yulya":     "yuliya",
	"yura":      "yuri",
	"yurik":     "yuri",
	"zheka":     "evgeni",
	"zhenek":    "evgeni",
	"zhenya":    "evgeni",
	"zhora":     "georgi",
}

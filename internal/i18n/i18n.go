// Package i18n holds the installer's UI catalogues.
package i18n

import (
	"embed"
	"encoding/json"
	"sort"
	"strings"

	"golang.org/x/text/language"
)

//go:embed catalog/*.json
var catalogs embed.FS

// Locale is one installer UI language.
type Locale string

const (
	EN Locale = "en"
	ZH Locale = "zh-Hans"
	DE Locale = "de"
	FR Locale = "fr"
	ES Locale = "es"
	PT Locale = "pt"
	JA Locale = "ja"
	KO Locale = "ko"
	RU Locale = "ru"
)

var order = []Locale{EN, ZH, DE, FR, ES, PT, JA, KO, RU}

var messages = map[Locale]map[string]string{}

func init() {
	for _, loc := range order {
		data, err := catalogs.ReadFile("catalog/" + string(loc) + ".json")
		if err != nil {
			panic(err)
		}
		m := map[string]string{}
		if err := json.Unmarshal(data, &m); err != nil {
			panic(err)
		}
		messages[loc] = m
	}
}

// Match maps a LANG/LC_MESSAGES value to an installer locale.
func Match(lang string) Locale {
	lang = strings.TrimSpace(lang)
	if i := strings.IndexAny(lang, ".@"); i >= 0 {
		lang = lang[:i]
	}
	lang = strings.ReplaceAll(lang, "_", "-")
	if lang == "" {
		return EN
	}
	tag, err := language.Parse(lang)
	if err != nil {
		return EN
	}
	base, _ := tag.Base()
	switch base.String() {
	case "zh":
		return ZH
	case "de":
		return DE
	case "fr":
		return FR
	case "es":
		return ES
	case "pt":
		return PT
	case "ja":
		return JA
	case "ko":
		return KO
	case "ru":
		return RU
	default:
		return EN
	}
}

// T returns the string for key in loc, falling back to English.
func T(loc Locale, key string) string {
	if v, ok := messages[loc][key]; ok {
		return v
	}
	return messages[EN][key]
}

// Locales returns the supported locales in F9 cycle order.
func Locales() []Locale { return append([]Locale{}, order...) }

// Next returns the next locale in the F9 cycle.
func Next(loc Locale) Locale {
	for i, l := range order {
		if l == loc {
			return order[(i+1)%len(order)]
		}
	}
	return EN
}

// Keys returns the sorted keys of a locale catalogue.
func Keys(loc Locale) []string {
	keys := make([]string, 0, len(messages[loc]))
	for k := range messages[loc] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

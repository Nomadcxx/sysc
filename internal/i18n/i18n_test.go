package i18n

import (
	"strings"
	"testing"
)

func TestEveryKeyExistsInAllCatalogs(t *testing.T) {
	union := map[string]bool{}
	for _, loc := range order {
		for _, k := range Keys(loc) {
			union[k] = true
		}
	}
	for _, loc := range order {
		have := map[string]bool{}
		for _, k := range Keys(loc) {
			have[k] = true
			if strings.TrimSpace(messages[loc][k]) == "" {
				t.Errorf("locale %s key %q is empty", loc, k)
			}
		}
		for k := range union {
			if !have[k] {
				t.Errorf("locale %s is missing key %q", loc, k)
			}
		}
	}
}

func TestWarnNiriSessionNamesCommand(t *testing.T) {
	for _, loc := range []Locale{EN, ZH, DE, FR} {
		msg := T(loc, "warn.niri_session")
		if !strings.Contains(msg, "niri-session") {
			t.Errorf("%s warning %q does not name niri-session", loc, msg)
		}
	}
}

func TestMatchLANG(t *testing.T) {
	cases := map[string]Locale{
		"zh_CN.UTF-8": ZH,
		"zh-Hans":     ZH,
		"de_DE":       DE,
		"fr_FR":       FR,
		"en_US":       EN,
		"ja_JP":       EN,
		"":            EN,
	}
	for in, want := range cases {
		if got := Match(in); got != want {
			t.Errorf("Match(%q) = %q, want %q", in, got, want)
		}
	}
}

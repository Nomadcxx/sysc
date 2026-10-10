package i18n

import (
	"fmt"
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
	for _, loc := range order {
		msg := T(loc, "warn.niri_session")
		if !strings.Contains(msg, "niri-session") {
			t.Errorf("%s warning %q does not name niri-session", loc, msg)
		}
	}
}

func TestPackageGuidanceAndPlannedDistroSupport(t *testing.T) {
	for _, loc := range order {
		if !strings.Contains(T(loc, "help.confirm"), "sysc-shell") {
			t.Errorf("%s confirmation guidance does not name the desktop", loc)
		}
		for _, text := range []string{"Debian", "Fedora", "https://nomadcxx.github.io/sysc/docs/"} {
			if !strings.Contains(T(loc, "refuse.distro"), text) {
				t.Errorf("%s distro guidance is missing %q", loc, text)
			}
		}
	}
}

func TestMatchLANG(t *testing.T) {
	cases := map[string]Locale{
		"zh_CN.UTF-8":  ZH,
		"zh-Hans":      ZH,
		"de_DE":        DE,
		"fr_FR":        FR,
		"en_US":        EN,
		"es_ES.UTF-8":  ES,
		"es_MX":        ES,
		"es-419":       ES,
		"pt_BR.UTF-8":  PT,
		"pt_PT":        PT,
		"ja_JP":        JA,
		"ja_JP.UTF-8":  JA,
		"ko_KR.UTF-8":  KO,
		"ru_RU":        RU,
		"C":            EN,
		"POSIX":        EN,
		"not a locale": EN,
		"":             EN,
	}
	for in, want := range cases {
		if got := Match(in); got != want {
			t.Errorf("Match(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every locale must keep the printf arguments of the English source.
func TestCatalogFormatArgsMatchEnglish(t *testing.T) {
	for _, key := range Keys(EN) {
		want := strings.Count(messages[EN][key], "%s")
		for _, loc := range order {
			got := strings.Count(messages[loc][key], "%s")
			if got != want || strings.Count(messages[loc][key], "%") != want {
				t.Errorf("locale %s key %q has %d %%s args, want %d", loc, key, got, want)
			}
		}
	}
}

// The F9 cycle must visit every supported locale once before wrapping.
func TestNextVisitsEveryLocale(t *testing.T) {
	seen := []Locale{}
	for loc := Next(EN); loc != EN; loc = Next(loc) {
		seen = append(seen, loc)
		if len(seen) > len(order) {
			t.Fatal("F9 cycle does not return to English")
		}
	}
	want := append([]Locale{}, order[1:]...)
	if fmt.Sprint(seen) != fmt.Sprint(want) {
		t.Errorf("F9 cycle = %v, want %v", seen, want)
	}
}

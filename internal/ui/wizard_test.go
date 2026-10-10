package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/Nomadcxx/sysc/internal/conflict"
	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/niri"
)

func TestWizardOrder(t *testing.T) {
	w := NewWizard(i18n.EN, []string{"org.sysc.weather"})
	if w.Page != PageTheme {
		t.Fatalf("start page = %v", w.Page)
	}
	w = w.Next()
	if w.Page != PageWallpaper {
		t.Fatalf("after theme = %v", w.Page)
	}
	w = w.Next()
	if w.Page != PagePlugins {
		t.Fatalf("after wallpaper = %v", w.Page)
	}
	w = w.Next()
	if w.Page != PageWeather {
		t.Fatalf("after plugins = %v", w.Page)
	}
	w.Location = "Berlin"
	w.Latitude = 52.52
	w.Longitude = 13.405
	w = w.Next()
	if w.Page != PageConfirm {
		t.Fatalf("after weather = %v", w.Page)
	}
	if w.Next().Page != PageConfirm {
		t.Fatal("confirm advanced past the end")
	}
}

func TestWeatherBlocksConfirm(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w.Page = PageWeather
	if w.Next().Page != PageWeather {
		t.Fatal("empty location advanced past weather")
	}
	w.Location = "Berlin"
	if w.Next().Page != PageWeather {
		t.Fatal("location without coordinates advanced past weather")
	}
	w.Latitude = 52.52
	w.Longitude = 13.405
	if w.Next().Page != PageConfirm {
		t.Fatal("complete weather did not advance")
	}
}

func TestWizardSkipsEmptyPluginPageAndCountsVisiblePages(t *testing.T) {
	for _, findings := range [][]conflict.Finding{nil, {{Name: "mako"}}} {
		w := NewWizard(i18n.EN, nil)
		w.Findings = findings
		w.Location, w.Latitude = "Berlin", 52.52
		count := 4 + len(findings)
		pages := []Page{PageTheme, PageWallpaper, PageWeather}
		if len(findings) != 0 {
			pages = append(pages, PageConflicts)
		}
		pages = append(pages, PageConfirm)
		for i, page := range pages {
			if w.Page != page || !strings.HasPrefix(w.Title(), fmt.Sprintf("%02d / %02d", i+1, count)) {
				t.Fatalf("page %d: got %v %q", i+1, w.Page, w.Title())
			}
			w = w.Next()
		}
		if !strings.Contains(w.Body(), i18n.T(i18n.EN, "plugin.catalog")) {
			t.Fatalf("confirmation omitted the shell catalog guidance: %s", w.Body())
		}
		for i := len(pages) - 1; i > 0; i-- {
			w = w.Back()
			if w.Page != pages[i-1] {
				t.Fatalf("back landed on %v, want %v", w.Page, pages[i-1])
			}
		}
	}
}

func TestConfirmWarnsPlainNiriSession(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w.Page = PageConfirm
	w.Preset = "standard"
	w.Mode = "dark"
	w.Location = "Berlin"
	body := w.Body()
	for _, label := range []string{"confirm.preset", "confirm.mode", "confirm.wallpaper", "confirm.plugins", "confirm.location"} {
		if !strings.Contains(body, i18n.T(i18n.EN, label)) {
			t.Fatalf("confirm body missing %s: %q", label, body)
		}
	}
	if !strings.Contains(body, "~/.local/bin") {
		t.Fatalf("confirm body missing the footprint: %q", body)
	}
	if strings.Contains(body, "niri-session") {
		t.Fatal("confirm warned when the session can start user units")
	}
	w.PlainNiri = true
	body = w.Body()
	if !strings.Contains(body, "niri-session") {
		t.Fatalf("confirm body = %q, want a niri-session warning", body)
	}
	if !strings.Contains(body, "spawn-at-startup") {
		t.Fatalf("confirm body = %q, want the spawn line called out", body)
	}
}

func TestPluginsNoneState(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w.Page = PagePlugins
	if !strings.Contains(w.Body(), i18n.T(i18n.EN, "plugin.none")) {
		t.Fatalf("plugins body = %q, want the none label", w.Body())
	}
}

func TestF9CyclesLocale(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w = w.CycleLocale()
	if w.Locale != i18n.ZH {
		t.Fatalf("locale = %v", w.Locale)
	}
	if i18n.T(w.Locale, "theme.title") == "" {
		t.Fatal("copy did not resolve after the locale switch")
	}
	if w.Back().Page != PageTheme {
		t.Fatal("back moved past the first page")
	}
}

func TestCycleModeToggles(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	if w.Mode != "dark" {
		t.Fatalf("default mode = %q", w.Mode)
	}
	if w = w.CycleMode(); w.Mode != "light" {
		t.Fatalf("after one toggle = %q", w.Mode)
	}
	if w = w.CycleMode(); w.Mode != "dark" {
		t.Fatalf("after two toggles = %q", w.Mode)
	}
}

func TestWallpaperCopyMentionsDirectory(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w.Page = PageWallpaper
	body := w.Body()
	if !strings.Contains(body, w.WallpaperDir) {
		t.Fatalf("wallpaper body = %q, want the directory", body)
	}
	if strings.Contains(strings.ToLower(body), "engine") {
		t.Fatalf("wallpaper body still promises an engine: %q", body)
	}
}

func TestWizardOrderWithConflicts(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w.Findings = []conflict.Finding{{Name: "mako", Kind: conflict.KindNotifications}}
	w.Page = PageWeather
	w.Location = "Berlin"
	w.Latitude = 52.52
	w.Longitude = 13.405
	if w = w.Next(); w.Page != PageConflicts {
		t.Fatalf("after weather with findings = %v", w.Page)
	}
	if w = w.Next(); w.Page != PageConfirm {
		t.Fatalf("after conflicts = %v", w.Page)
	}
	if w.Back().Page != PageConflicts {
		t.Fatal("back from confirm skipped the conflicts page")
	}
}

func TestConflictsPageSkippedWhenEmpty(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w.Page = PageWeather
	w.Location = "Berlin"
	w.Latitude = 52.52
	w.Longitude = 13.405
	if w = w.Next(); w.Page != PageConfirm {
		t.Fatalf("empty conflicts page was not skipped: %v", w.Page)
	}
	if w.Back().Page != PageWeather {
		t.Fatalf("back landed on the empty conflicts page: %v", w.Back().Page)
	}
}

func TestConflictCycleAndBody(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w.Findings = []conflict.Finding{{
		Name:        "mako",
		Kind:        conflict.KindNotifications,
		PIDs:        []int{100},
		UnitEnabled: true,
		NiriLines:   []niri.Spawn{{Name: "mako"}},
	}}
	w.Choices = map[string]conflict.Choice{"mako": conflict.HandOver}
	w.Page = PageConflicts

	w = w.CycleChoice(true)
	if w.Choices["mako"] != conflict.KeepBoth {
		t.Fatalf("after one cycle = %v", w.Choices["mako"])
	}
	w = w.CycleChoice(true)
	if w.Choices["mako"] != conflict.SkipSYSC {
		t.Fatalf("after two cycles = %v", w.Choices["mako"])
	}
	w = w.CycleChoice(true)
	if w.Choices["mako"] != conflict.HandOver {
		t.Fatalf("after three cycles = %v", w.Choices["mako"])
	}
	w = w.CycleChoice(false)
	if w.Choices["mako"] != conflict.SkipSYSC {
		t.Fatalf("after backwards cycle = %v", w.Choices["mako"])
	}

	for _, want := range []string{"mako", i18n.T(i18n.EN, "conflicts.choice.skip"), i18n.T(i18n.EN, "conflicts.state.running")} {
		if !strings.Contains(w.Body(), want) {
			t.Fatalf("conflicts body missing %q: %q", want, w.Body())
		}
	}
	if w.MoveConflict(1).ConflictRow != 0 {
		t.Fatal("row moved past the end")
	}
	if w.MoveConflict(-1).ConflictRow != 0 {
		t.Fatal("row moved before the start")
	}
}

func TestConflictBodyFits80x24(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w.Findings = []conflict.Finding{
		{Name: "mako", Kind: conflict.KindNotifications, PIDs: []int{1}, UnitEnabled: true, NiriLines: []niri.Spawn{{Name: "mako"}}},
		{Name: "dunst", Kind: conflict.KindNotifications, PIDs: []int{2}, BusOwner: true},
		{Name: "waybar", Kind: conflict.KindTray, PIDs: []int{3}, UnitEnabled: true},
		{Name: "quickshell", Kind: conflict.KindShell, PIDs: []int{4}, Cmdline: "quickshell -c noctalia", NiriLines: []niri.Spawn{{Name: "quickshell"}}},
	}
	w.Choices = map[string]conflict.Choice{
		"mako":       conflict.HandOver,
		"dunst":      conflict.HandOver,
		"waybar":     conflict.KeepBoth,
		"quickshell": conflict.KeepBoth,
	}
	w.Page = PageConflicts
	width, height := 80, 24
	out := View(i18n.EN, w.Title(), w.Body(), w.Page, StepWizard, width, height, NewBeamsTextEffect(width, BannerHeight(), Banner()))
	lines := strings.Split(out, "\n")
	if len(lines) != height {
		t.Fatalf("rendered %d lines, want exactly %d", len(lines), height)
	}
	for i, l := range lines {
		if got := lipgloss.Width(l); got > width {
			t.Fatalf("line %d is %d cells wide: %q", i, got, stripANSI(l))
		}
	}
}

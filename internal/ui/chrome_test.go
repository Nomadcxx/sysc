package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"
)

func TestBannerContainsCowboyLine(t *testing.T) {
	if !strings.Contains(Banner(), "SEE YOU IN SPACE COWBOY") {
		t.Fatal("banner lost the cowboy line")
	}
}

func TestNavFollowsLocale(t *testing.T) {
	en := Nav(i18n.EN, PageTheme, StepWizard)
	if !strings.Contains(en, "Language") {
		t.Fatalf("en nav = %q", en)
	}
	zh := Nav(i18n.ZH, PageTheme, StepWizard)
	if strings.Contains(zh, "Quit") {
		t.Fatalf("zh nav still uses the English word Quit: %q", zh)
	}
	if !strings.Contains(zh, i18n.T(i18n.ZH, "nav.quit")) {
		t.Fatalf("zh nav missing the zh quit label: %q", zh)
	}
}

func TestNavWaitHasNoCancel(t *testing.T) {
	wait := Nav(i18n.EN, PageTheme, StepInstalling)
	if !strings.Contains(wait, i18n.T(i18n.EN, "nav.wait")) {
		t.Fatalf("installing nav = %q", wait)
	}
	if strings.Contains(wait, "Esc") || strings.Contains(wait, "Quit") {
		t.Fatalf("installing nav offers a cancel: %q", wait)
	}
}

func TestNavPageAccurate(t *testing.T) {
	theme := Nav(i18n.EN, PageTheme, StepWizard)
	if !strings.Contains(theme, "Preset") {
		t.Fatalf("theme nav = %q", theme)
	}
	if !strings.Contains(theme, i18n.T(i18n.EN, "nav.mode")) {
		t.Fatalf("theme nav missing the mode hint: %q", theme)
	}
	wall := Nav(i18n.EN, PageWallpaper, StepWizard)
	if strings.Contains(wall, "↑↓") {
		t.Fatalf("wallpaper nav advertises dead arrows: %q", wall)
	}
	weather := Nav(i18n.EN, PageWeather, StepWizard)
	if !strings.Contains(weather, "Ctrl+C") {
		t.Fatalf("weather nav must advertise the real quit key: %q", weather)
	}
	if strings.Contains(weather, "q Quit") {
		t.Fatalf("weather nav still advertises q Quit: %q", weather)
	}
	confirm := Nav(i18n.EN, PageConfirm, StepWizard)
	if !strings.Contains(confirm, "Install") {
		t.Fatalf("confirm nav = %q", confirm)
	}
	done := Nav(i18n.EN, PageTheme, StepDone)
	if !strings.Contains(done, i18n.T(i18n.EN, "nav.close")) {
		t.Fatalf("done nav = %q", done)
	}
}

func TestCanvasStaysBlackAndGuidanceHasOwnBackground(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previous)
	for _, size := range [][2]int{{80, 24}, {120, 44}, {60, 12}} {
		width, height := size[0], size[1]
		view := View(i18n.EN, "01/04 Theme", Control("Preset", "Standard", 60, true), PageTheme, StepWizard, width, height, nil)
		buf := cellbuf.NewBuffer(width, height)
		cellbuf.SetContent(buf, view)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				cell := buf.Cell(x, y)
				if cell == nil || cell.Style.Bg == nil {
					t.Fatalf("%dx%d: terminal background exposed at (%d,%d)", width, height, x, y)
				}
			}
		}
	}
	out := View(i18n.EN, "Theme", "body", PageTheme, StepWizard, 80, 24, nil)
	if !strings.Contains(out, "48;2;0;0;0") {
		t.Fatal("black canvas background was not set")
	}
	f := measure(i18n.EN, PageTheme, StepWizard, 80, 24, nil, "Guidance")
	if !strings.Contains(f.advice, "48;2;22;22;22") {
		t.Fatal("guidance has no charcoal background")
	}
	for _, area := range []string{f.header, f.actions, f.nav, Control("", "choice", 72, true)} {
		if strings.Contains(area, "48;2;22;22;22") {
			t.Fatal("guidance background leaked into the canvas or controls")
		}
	}
	backgrounds := regexp.MustCompile(`48;2;(\d+);(\d+);(\d+)`).FindAllStringSubmatch(out, -1)
	for _, rgb := range backgrounds {
		if !(rgb[1] == "0" && rgb[2] == "0" && rgb[3] == "0") &&
			!(rgb[1] == "22" && rgb[2] == "22" && rgb[3] == "22") {
			t.Fatalf("unexpected background: %v", rgb)
		}
	}
	beams := NewBeamsTextEffect(80, BannerHeight(), Banner())
	beams.Update()
	for _, rgb := range regexp.MustCompile(`48;2;(\d+);(\d+);(\d+)`).FindAllStringSubmatch(beams.Render(), -1) {
		if rgb[1] != "0" || rgb[2] != "0" || rgb[3] != "0" {
			t.Fatalf("non-black banner background: %v", rgb)
		}
	}
}

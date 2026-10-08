package ui

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/i18n"
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

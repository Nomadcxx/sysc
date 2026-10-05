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
	en := Nav(i18n.EN, StepWizard)
	if !strings.Contains(en, "Language") {
		t.Fatalf("en nav = %q", en)
	}
	zh := Nav(i18n.ZH, StepWizard)
	if strings.Contains(zh, "Quit") {
		t.Fatalf("zh nav still uses the English word Quit: %q", zh)
	}
	if !strings.Contains(zh, i18n.T(i18n.ZH, "nav.quit")) {
		t.Fatalf("zh nav missing the zh quit label: %q", zh)
	}
}

func TestNavWaitHasNoCancel(t *testing.T) {
	wait := Nav(i18n.EN, StepInstalling)
	if !strings.Contains(wait, i18n.T(i18n.EN, "nav.wait")) {
		t.Fatalf("installing nav = %q", wait)
	}
	if strings.Contains(wait, "Esc") || strings.Contains(wait, "Quit") {
		t.Fatalf("installing nav offers a cancel: %q", wait)
	}
}

package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/i18n"
)

// AUD-08: the pin only carries amd64 assets; a non-amd64 host must be
// refused by name instead of silently installing foreign-arch binaries.
func TestAuditRefusesUnsupportedHostArch(t *testing.T) {
	home := setupHome(t)
	_, err := Run(context.Background(), Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true, Arch: "arm64",
		Systemctl: noopSystemctl, Now: time.Unix(1000, 0),
	})
	if err == nil || !strings.Contains(err.Error(), "arm64") {
		t.Errorf("AUDIT: arm64 host not refused by name, got %v", err)
	}
}

// AUD-16: the primary --yes refusal is hardcoded English while every
// catalog carries refuse.weather.
func TestAuditWeatherRefusalUsesLocale(t *testing.T) {
	home := setupHome(t)
	zh := i18n.ZH
	_, err := Run(context.Background(), Options{
		Home: home, Pin: loadPin(t), Loc: &zh,
		Systemctl: noopSystemctl, Now: time.Unix(1000, 0),
	})
	if err == nil {
		t.Fatal("missing weather answers accepted")
	}
	if got, want := err.Error(), i18n.T(zh, "refuse.weather"); got != want {
		t.Errorf("refusal = %q, want catalog %q", got, want)
	}
}

func TestAuditLongLocationIsTruncatedToBytes(t *testing.T) {
	home := setupHome(t)
	a := weatherAnswers()
	a.Location = "城市城市城市城市城市城市城市城市城市城市城市城市城市城市城市" // 30 runes, 90 bytes
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: a, Yes: true,
		Download:  func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:      func(string, string, []string) error { return nil },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: noopSystemctl,
		Now:       time.Unix(1000, 0),
	}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "sysc-shell", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), a.Location) {
		t.Fatal("90-byte location was written verbatim; shell would refuse to start")
	}
	if !strings.Contains(string(data), "城市城市城市城市城市城市城市城市城市城市城市") {
		t.Fatalf("expected an 80-byte prefix of the label, got %s", data)
	}
}

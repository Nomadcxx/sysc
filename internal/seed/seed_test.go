package seed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSeedAddsWeatherToDefaultBar(t *testing.T) {
	data, err := ConfigJSON(Answers{
		Preset:       "standard",
		Mode:         "dark",
		WallpaperDir: "/tmp/walls",
		Plugins:      []string{"org.sysc.weather", "org.sysc.media"},
		Latitude:     52.52,
		Longitude:    13.405,
		Location:     "Berlin",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	weather := got["weather"].(map[string]any)
	if weather["latitude"] != 52.52 {
		t.Errorf("latitude = %v", weather["latitude"])
	}
	if weather["location"] != "Berlin" {
		t.Errorf("location = %v", weather["location"])
	}
	if _, ok := weather["city"]; ok {
		t.Error("city must not be set together with coordinates")
	}

	theme := got["theme"].(map[string]any)
	if theme["preset"] != "standard" {
		t.Errorf("preset = %v", theme["preset"])
	}
	gen := got["theme-gen"].(map[string]any)
	if gen["source"] != "wallpaper" || gen["mode"] != "dark" {
		t.Errorf("theme-gen = %v", gen)
	}
	wall := got["wallpaper"].(map[string]any)
	if wall["image_directory"] != "/tmp/walls" {
		t.Errorf("image_directory = %v", wall["image_directory"])
	}
	plugins := got["plugins"].(map[string]any)
	enabled := plugins["enabled"].([]any)
	if len(enabled) != 2 || enabled[0] != "org.sysc.weather" {
		t.Errorf("plugins.enabled = %v", enabled)
	}

	bar := got["bar"].(map[string]any)
	items := bar["items"].(map[string]any)
	right := items["right"].([]any)
	found := false
	for _, raw := range right {
		if item, ok := raw.(map[string]any); ok && item["id"] == "weather" {
			found = true
		}
	}
	if !found {
		t.Errorf("weather item missing from right section: %v", right)
	}
	if len(right) < 5 {
		t.Errorf("right section lost default items: %v", right)
	}
}

func TestSeedRejectsEmptyWeather(t *testing.T) {
	if _, err := ConfigJSON(Answers{Preset: "standard", Mode: "dark"}); err == nil {
		t.Fatal("expected error without coordinates")
	}
	if _, err := ConfigJSON(Answers{Preset: "standard", Mode: "dark", Latitude: 52.52, Longitude: 13.405}); err == nil {
		t.Fatal("expected error without a location label")
	}
}

func TestSeedDoesNotClobberExistingConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sysc-shell", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"theme":{"preset":"expressive"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Write(path, Answers{
		Preset: "standard", Mode: "dark", Latitude: 1, Longitude: 2, Location: "X",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"theme":{"preset":"expressive"}}` {
		t.Fatalf("existing config overwritten: %s", got)
	}
}

// Installing sysc-lock over an earlier setup must make it the session locker:
// a configuration without one gains the locker and the idle lock, and every
// other key survives.
func TestSeedActivatesLockerInExistingConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"theme":{"preset":"expressive"},"weather":{"latitude":-37.6}}`), 0o640); err != nil {
		t.Fatal(err)
	}
	err := Write(path, Answers{Locker: "sysc-lock", IdleLock: 5 * time.Minute,
		Preset: "standard", Latitude: 1, Longitude: 2, Location: "X"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["session"]["locker"] != "sysc-lock" || got["idle"]["lock"] != "5m0s" {
		t.Fatalf("locker not activated: %s", data)
	}
	if got["theme"]["preset"] != "expressive" || got["weather"]["latitude"] != -37.6 {
		t.Fatalf("existing keys changed: %s", data)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, want 0640", fi.Mode().Perm())
	}
}

// A locker the user already chose, and the idle choice that goes with it, are
// theirs: the installer never replaces them.
func TestSeedKeepsAnExistingLockerChoice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	for _, existing := range []string{
		`{"session":{"locker":"swaylock"}}`,
		`{"session":{"locker":"sysc-lock"},"idle":{"lock":"15m"}}`,
		`{"session":{"locker":"sysc-lock"}}`,
		`not json`,
	} {
		if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Write(path, Answers{Locker: "sysc-lock", IdleLock: 5 * time.Minute,
			Latitude: 1, Longitude: 2, Location: "X"}); err != nil {
			t.Fatalf("%s: %v", existing, err)
		}
		if got, _ := os.ReadFile(path); string(got) != existing {
			t.Fatalf("existing %s rewritten as %s", existing, got)
		}
	}
}

func TestSeedFreshConfigCarriesIdleLock(t *testing.T) {
	data, err := ConfigJSON(Answers{Locker: "sysc-lock", IdleLock: 5 * time.Minute,
		Latitude: 1, Longitude: 2, Location: "X"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["idle"]["lock"] != "5m0s" {
		t.Fatalf("idle = %v", got["idle"])
	}
	data, _ = ConfigJSON(Answers{Locker: "sysc-lock", Latitude: 1, Longitude: 2, Location: "X"})
	if strings.Contains(string(data), `"idle"`) {
		t.Fatalf("zero IdleLock wrote an idle section: %s", data)
	}
}

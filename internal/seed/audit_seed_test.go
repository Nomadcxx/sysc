package seed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The shell validates weather coordinates (lat ±90, lon ±180) and its loader
// uses DisallowUnknownFields + fails startup — an out-of-range seed is a
// dead bar, not a warning.
func TestAuditRejectsOutOfRangeCoordinates(t *testing.T) {
	a := Answers{Latitude: 95, Longitude: 200, Location: "Nowhere"}
	if _, err := ConfigJSON(a); err == nil {
		t.Errorf("AUDIT: ConfigJSON accepted lat=95 lon=200 (shell range is ±90/±180)")
	}
}

// config.go:458 maxWeatherLocationBytes = 80 BYTES. 30 CJK runes = 90 bytes.
func TestAuditRejectsOverlongLocation(t *testing.T) {
	loc := strings.Repeat("北", 30)
	a := Answers{Latitude: 52.52, Longitude: 13.405, Location: loc}
	data, err := ConfigJSON(a)
	if err == nil {
		t.Errorf("AUDIT: ConfigJSON accepted %d-byte location label (shell max is 80 bytes)", len(loc))
		_ = data
	}
}

func TestAuditNeverOverwritesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"mine":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Write(path, Answers{Latitude: 1, Longitude: 1, Location: "X"})
	if err != nil {
		t.Fatalf("Write over existing: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != `{"mine":true}` {
		t.Errorf("existing config was modified")
	}
}

// A directory at the config path makes Stat succeed → Write silently no-ops;
// the install reports success with no seeded config.
func TestAuditConfigPathDirectoryIsNotSilentlySkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	err := Write(path, Answers{Latitude: 1, Longitude: 1, Location: "X"})
	if err == nil {
		t.Errorf("AUDIT: Write to a directory path returned nil (silent no-op, run reports success)")
	}
}

func TestAuditSeedValidatesAgainstShellKeys(t *testing.T) {
	a := Answers{Preset: "standard", Mode: "dark", Latitude: 52.52, Longitude: 13.405,
		Location: "Berlin", WallpaperDir: "/w", Plugins: []string{"org.sysc.weather"}}
	data, err := ConfigJSON(a)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"theme", "theme-gen", "weather", "bar", "wallpaper", "plugins"} {
		if _, ok := doc[k]; !ok {
			t.Errorf("seed missing %q key", k)
		}
	}
	w := doc["weather"].(map[string]any)
	for _, k := range []string{"latitude", "longitude", "location"} {
		if _, ok := w[k]; !ok {
			t.Errorf("weather missing %q", k)
		}
	}
	right := doc["bar"].(map[string]any)["items"].(map[string]any)["right"].([]any)
	last := right[len(right)-1].(map[string]any)
	if last["id"] != "weather" {
		t.Errorf("weather item not appended to right section: %v", right)
	}
}

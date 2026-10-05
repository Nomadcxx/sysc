package niri

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIncludeIdempotent(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.kdl")
	sidecar := filepath.Join(dir, "sysc.kdl")
	write(t, config, "include \"sysc-shell.kdl\"\n\ninput { keyboard { xkb { layout \"us\"; } } }\n")
	opts := Options{ConfigPath: config, SidecarPath: sidecar, StateDir: filepath.Join(dir, "state"), Binds: DefaultBinds, Now: time.Unix(1000, 0)}

	for i := 0; i < 2; i++ {
		if _, err := Apply(opts); err != nil {
			t.Fatalf("apply %d: %v", i, err)
		}
	}
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), `include "sysc.kdl"`); got != 1 {
		t.Fatalf("sysc.kdl includes = %d; want 1\n%s", got, data)
	}
	if !strings.Contains(string(data), `include "sysc-shell.kdl"`) {
		t.Fatalf("sysc-shell.kdl include was touched:\n%s", data)
	}
}

func TestCommentShellSpawnOnly(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.kdl")
	write(t, config, strings.Join([]string{
		`spawn-at-startup "sysc-shell"`,
		`spawn-at-startup "cliphist"`,
		`spawn-at-startup "gnome-keyring-daemon" "--start"`,
		"",
	}, "\n"))
	opts := Options{ConfigPath: config, SidecarPath: filepath.Join(dir, "sysc.kdl"), StateDir: filepath.Join(dir, "state"), Now: time.Unix(1000, 0)}
	res, err := Apply(opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.SpawnCommented != 1 {
		t.Fatalf("SpawnCommented = %d; want 1", res.SpawnCommented)
	}
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `// spawn-at-startup "sysc-shell"`) {
		t.Fatalf("shell spawn not commented:\n%s", text)
	}
	if !strings.Contains(text, `spawn-at-startup "cliphist"`) {
		t.Fatalf("cliphist spawn changed:\n%s", text)
	}
	if !strings.Contains(text, `spawn-at-startup "gnome-keyring-daemon" "--start"`) {
		t.Fatalf("keyring spawn changed:\n%s", text)
	}
}

func TestKeepSpawnLeavesAutostart(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.kdl")
	write(t, config, "spawn-at-startup \"sysc-shell\"\ninput {}\n")
	opts := Options{
		ConfigPath:  config,
		SidecarPath: filepath.Join(dir, "sysc.kdl"),
		StateDir:    filepath.Join(dir, "state"),
		Binds:       DefaultBinds,
		Now:         time.Unix(1000, 0),
		KeepSpawn:   true,
	}
	res, err := Apply(opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.SpawnCommented != 0 {
		t.Fatalf("SpawnCommented = %d; want 0", res.SpawnCommented)
	}
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, `// spawn-at-startup "sysc-shell"`) {
		t.Fatalf("spawn commented despite KeepSpawn:\n%s", text)
	}
	if !strings.Contains(text, "spawn-at-startup \"sysc-shell\"\n") {
		t.Fatalf("spawn line changed:\n%s", text)
	}
	if !strings.Contains(text, `include "sysc.kdl"`) {
		t.Fatalf("include not added:\n%s", text)
	}
}

func TestSkipOccupiedBind(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.kdl")
	write(t, filepath.Join(dir, "extra.kdl"), "Mod+Space { hotkey-overlay; }\n")
	write(t, config, "include \"extra.kdl\"\nMod+Comma { consume-window-into-column; }\n")
	sidecar := filepath.Join(dir, "sysc.kdl")
	opts := Options{ConfigPath: config, SidecarPath: sidecar, StateDir: filepath.Join(dir, "state"), Binds: DefaultBinds, Now: time.Unix(1000, 0)}
	res, err := Apply(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BindsSkipped) != 2 {
		t.Fatalf("BindsSkipped = %v; want both keys", res.BindsSkipped)
	}
	data, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Mod+Comma") || strings.Contains(string(data), "Mod+Space") {
		t.Fatalf("sidecar bound an occupied key:\n%s", data)
	}
}

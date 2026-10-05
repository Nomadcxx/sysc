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

// Included files are full config fragments. A bare Mod+Key node is rejected
// ("unexpected node") and the include takes the user's whole config down with it.
func TestSidecarBindsInsideBindsBlock(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.kdl")
	sidecar := filepath.Join(dir, "sysc.kdl")
	write(t, config, "input {}\n")
	opts := Options{ConfigPath: config, SidecarPath: sidecar, StateDir: filepath.Join(dir, "state"), Binds: DefaultBinds, Now: time.Unix(1000, 0)}

	res, err := Apply(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BindsAdded) != len(DefaultBinds) || len(res.BindsSkipped) != 0 {
		t.Fatalf("BindsAdded=%v BindsSkipped=%v", res.BindsAdded, res.BindsSkipped)
	}
	data, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	assertBindsNested(t, got, []string{"Mod+Space", "Mod+Comma"})

	golden, err := os.ReadFile(filepath.Join("testdata", "sysc.kdl.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(golden) {
		t.Fatalf("sidecar mismatch:\n got:\n%s\nwant:\n%s", got, golden)
	}
}

// assertBindsNested fails when a bind is a top-level node instead of a child of binds { }.
func assertBindsNested(t *testing.T, text string, keys []string) {
	t.Helper()
	const open = "binds {"
	start := strings.Index(text, open)
	if start < 0 || strings.Count(text, open) != 1 {
		t.Fatalf("sidecar binds blocks = %d; want 1:\n%s", strings.Count(text, open), text)
	}
	rest := text[start+len(open):]
	closeRel := strings.Index(rest, "\n}")
	if closeRel < 0 {
		t.Fatalf("binds block is not closed:\n%s", text)
	}
	inner := rest[:closeRel]
	after := rest[closeRel+len("\n}"):]
	for _, key := range keys {
		line := key + " {"
		if !strings.Contains(inner, line) {
			t.Fatalf("%s is not inside the binds block:\n%s", key, text)
		}
		if strings.Contains(text[:start], line) || strings.Contains(after, line) {
			t.Fatalf("%s appears outside the binds block:\n%s", key, text)
		}
	}
	rejectTopLevel(t, text[:start], text)
	rejectTopLevel(t, after, text)
}

func rejectTopLevel(t *testing.T, section, full string) {
	t.Helper()
	for _, line := range strings.Split(section, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "//") {
			continue
		}
		t.Fatalf("unexpected top-level node %q:\n%s", trim, full)
	}
}

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

// A stow/chezmoi/yadm layout points ~/.config/niri/config.kdl at the dotfiles
// copy. Apply and Remove must edit that copy and leave the link in place.
func TestSymlinkedConfigApplyAndRemove(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dotfile := filepath.Join(home, "dotfiles", "niri", "config.kdl")
	write(t, dotfile, "spawn-at-startup \"sysc-shell\"\n")

	configDir := filepath.Join(home, ".config", "niri")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(configDir, "config.kdl")
	rel, err := filepath.Rel(configDir, dotfile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rel, config); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		ConfigPath:  config,
		SidecarPath: filepath.Join(configDir, "sysc.kdl"),
		StateDir:    filepath.Join(home, ".local", "state", "sysc"),
		Binds:       DefaultBinds,
		Now:         time.Unix(1000, 0),
	}
	if _, err := Apply(opts); err != nil {
		t.Fatal(err)
	}
	assertSymlink(t, config, rel)
	got := readFile(t, dotfile)
	if !strings.Contains(got, "include \"sysc.kdl\"") {
		t.Fatalf("dotfiles copy unchanged:\n%s", got)
	}
	if !strings.Contains(got, "// spawn-at-startup \"sysc-shell\"") {
		t.Fatalf("spawn not commented in dotfiles copy:\n%s", got)
	}
	if _, err := os.Lstat(config + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left beside symlink: %v", err)
	}
	if _, err := os.Lstat(dotfile + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left beside target: %v", err)
	}

	if err := Remove(opts); err != nil {
		t.Fatal(err)
	}
	assertSymlink(t, config, rel)
	got = readFile(t, dotfile)
	if strings.Contains(got, "include \"sysc.kdl\"") {
		t.Fatalf("uninstall left include in dotfiles copy:\n%s", got)
	}
	if strings.Contains(got, "// spawn-at-startup \"sysc-shell\"") || !strings.Contains(got, "spawn-at-startup \"sysc-shell\"") {
		t.Fatalf("uninstall did not restore spawn in dotfiles copy:\n%s", got)
	}
}

// Home-manager points config.kdl at a Nix store file the user cannot write.
// Install and uninstall must say so and leave the link alone.
func TestSymlinkedConfigReadOnlyTarget(t *testing.T) {
	t.Run("apply", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		config, target := readOnlyLinkedConfig(t, home, "spawn-at-startup \"sysc-shell\"\n")
		_, err := Apply(linkedOpts(home, config))
		if err == nil {
			t.Fatal("Apply succeeded on a read-only symlink target")
		}
		if !strings.Contains(err.Error(), "not writable") || !strings.Contains(err.Error(), config) {
			t.Fatalf("error = %v; want a not-writable message naming %s", err, config)
		}
		assertSymlink(t, config, target)
		if got := readFile(t, target); got != "spawn-at-startup \"sysc-shell\"\n" {
			t.Fatalf("read-only target changed:\n%s", got)
		}
	})

	t.Run("remove", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		original := "// spawn-at-startup \"sysc-shell\"\ninclude \"sysc.kdl\"\n"
		config, target := readOnlyLinkedConfig(t, home, original)
		err := Remove(linkedOpts(home, config))
		if err == nil {
			t.Fatal("Remove succeeded on a read-only symlink target")
		}
		if !strings.Contains(err.Error(), "not writable") || !strings.Contains(err.Error(), config) {
			t.Fatalf("error = %v; want a not-writable message naming %s", err, config)
		}
		assertSymlink(t, config, target)
		if got := readFile(t, target); got != original {
			t.Fatalf("read-only target changed:\n%s", got)
		}
	})
}

func linkedOpts(home, config string) Options {
	return Options{
		ConfigPath:  config,
		SidecarPath: filepath.Join(home, ".config", "niri", "sysc.kdl"),
		StateDir:    filepath.Join(home, ".local", "state", "sysc"),
		Binds:       DefaultBinds,
		Now:         time.Unix(1000, 0),
	}
}

func readOnlyLinkedConfig(t *testing.T, home, content string) (config, target string) {
	t.Helper()
	store := filepath.Join(home, "nix-store-"+strings.ReplaceAll(t.Name(), "/", "-"))
	target = filepath.Join(store, "config.kdl")
	write(t, target, content)
	if err := os.Chmod(store, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(store, 0o755) })

	configDir := filepath.Join(home, ".config", "niri")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config = filepath.Join(configDir, "config.kdl")
	if err := os.Symlink(target, config); err != nil {
		t.Fatal(err)
	}
	return config, target
}

func assertSymlink(t *testing.T, path, want string) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("symlink vanished: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is a regular file; want a symlink", path)
	}
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("symlink target = %q; want %q", got, want)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

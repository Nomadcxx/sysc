package niri

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func applyCase(t *testing.T, dir, configText string) (Result, string) {
	t.Helper()
	config := filepath.Join(dir, "config.kdl")
	write(t, config, configText)
	sidecar := filepath.Join(dir, "sysc.kdl")
	opts := Options{ConfigPath: config, SidecarPath: sidecar, StateDir: filepath.Join(dir, "state"), Binds: DefaultBinds, Now: time.Unix(1000, 0)}
	res, err := Apply(opts)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return res, sidecar
}

func TestOccupiedKeysFollowsAllIncludeForms(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	write(t, filepath.Join(home, "binds.kdl"), "Mod+Space { spawn \"fuzzel\"; }\n")

	cases := []struct{ name, inc string }{
		{"relative", `include "binds.kdl"`},
		{"absolute", `include "{DIR}/binds.kdl"`},
		{"optional", `include optional=true "binds.kdl"`},
		{"nested", `include "outer.kdl"`},
		{"home", `include "~/binds.kdl"`},
		{"nested-relative-to-included-dir", `include "sub/a.kdl"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, filepath.Join(dir, "binds.kdl"), "Mod+Space { spawn \"fuzzel\"; }\n")
			write(t, filepath.Join(dir, "outer.kdl"), "include \"binds.kdl\"\n")
			write(t, filepath.Join(dir, "sub", "a.kdl"), "include \"b.kdl\"\n")
			write(t, filepath.Join(dir, "sub", "b.kdl"), "Mod+Space { spawn \"fuzzel\"; }\n")

			inc := strings.ReplaceAll(tc.inc, "{DIR}", dir)
			res, _ := applyCase(t, dir, inc+"\ninput {}\n")

			if !slices.Contains(res.BindsSkipped, "Mod+Space") {
				t.Fatalf("Mod+Space not skipped; skipped=%v added=%v", res.BindsSkipped, res.BindsAdded)
			}
			if slices.Contains(res.BindsAdded, "Mod+Space") {
				t.Fatalf("Mod+Space added despite being bound; skipped=%v added=%v", res.BindsSkipped, res.BindsAdded)
			}
			if !slices.Contains(res.BindsAdded, "Mod+Comma") {
				t.Fatalf("Mod+Comma not added; skipped=%v added=%v", res.BindsSkipped, res.BindsAdded)
			}
		})
	}
}

func TestOccupiedKeysIncludeCycleTerminates(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.kdl"), "include \"b.kdl\"\n")
	write(t, filepath.Join(dir, "b.kdl"), "include \"a.kdl\"\nMod+Space { spawn \"fuzzel\"; }\n")

	res, _ := applyCase(t, dir, "include \"a.kdl\"\n")

	if !slices.Contains(res.BindsSkipped, "Mod+Space") {
		t.Fatalf("Mod+Space not skipped across cycle; skipped=%v added=%v", res.BindsSkipped, res.BindsAdded)
	}
}

func TestOccupiedKeysExcludesSidecarByAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	sidecar := filepath.Join(dir, "sysc.kdl")
	write(t, sidecar, "Mod+Space { spawn \"sysc-shell\"; }\n")

	res, _ := applyCase(t, dir, "include \""+sidecar+"\"\ninput {}\n")

	if slices.Contains(res.BindsSkipped, "Mod+Space") {
		t.Fatalf("own sidecar counted as occupied; skipped=%v added=%v", res.BindsSkipped, res.BindsAdded)
	}
	if !slices.Contains(res.BindsAdded, "Mod+Space") {
		t.Fatalf("Mod+Space not written; skipped=%v added=%v", res.BindsSkipped, res.BindsAdded)
	}
}

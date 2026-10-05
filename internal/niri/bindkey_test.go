package niri

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanonicalBind(t *testing.T) {
	same := [][2]string{
		{"Mod+Space", "mod+space"},
		{"Mod+Space", "MOD+SPACE"},
		{"Mod+Space", "Mod+space"},
		{"Mod+Comma", "mod+COMMA"},
		{"Super+A", "Win+a"},
		{"Super+A", "WIN+A"},
		{"Ctrl+A", "Control+a"},
		{"Ctrl+Shift+A", "shift+control+a"},
		{"Ctrl+A", "Control+Ctrl+a"},
		{"Super+Alt+L", "alt+win+l"},
		{"ISO_Level3_Shift+A", "Mod5+a"},
		{"ISO_Level3_Shift+A", "iso_level3_shift+A"},
		{"ISO_Level5_Shift+Comma", "mod3+comma"},
		{"Mod+Ctrl+Space", "ctrl+mod+SPACE"},
		{"Ctrl +A", "control+a"},
		{"Mod +Space", "mod+space"},
		{"XF86ScreenSaver", "xf86screensaver"},
		{"XF86ScreenSaver", "XF86SCREENSAVER"},
		{"XF86ScreenSaver", "XF86_ScreenSaver"},
		{"XF86AudioRaiseVolume", "xf86_audioraisevolume"},
		{"MouseLeft", "mouseleft"},
		{"WheelScrollDown", "wheelscrolldown"},
	}
	for _, pair := range same {
		a, aok := canonicalBind(pair[0])
		b, bok := canonicalBind(pair[1])
		if !aok || !bok || a != b {
			t.Errorf("canonicalBind(%q)=%q,%v and canonicalBind(%q)=%q,%v; want the same key", pair[0], a, aok, pair[1], b, bok)
		}
	}

	different := [][2]string{
		{"Mod+Space", "Super+Space"},
		{"Mod+Space", "Win+Space"},
		{"Mod+Space", "Mod+Shift+Space"},
		{"Mod+Space", "Mod+Spacebar"},
		{"Super+Space", "Mod4+Space"},
		{"Ctrl+A", "Control+B"},
		{"XF86ScreenSaver", "XF86Screensaver"},
		{"Mod+Comma", "Mod5+Comma"},
	}
	for _, pair := range different {
		a, aok := canonicalBind(pair[0])
		b, bok := canonicalBind(pair[1])
		if aok && bok && a == b {
			t.Errorf("canonicalBind(%q) and canonicalBind(%q) both %q; niri treats them as different keys", pair[0], pair[1], a)
		}
	}

	invalid := []string{"", "+", "Mod+", "Mod+ Space", "Mod4+Space", "Hyper+A", "Logo+A"}
	for _, s := range invalid {
		if _, ok := canonicalBind(s); ok {
			t.Errorf("canonicalBind(%q) parsed; niri rejects this spelling", s)
		}
	}
}

func TestApplySkipsNormalizedOccupiedBinds(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		key      string
		include  bool
		occupied bool
	}{
		{name: "lower mod space", line: `mod+space { spawn "fuzzel"; }`, key: "Mod+Space", occupied: true},
		{name: "upper mod space", line: `MOD+SPACE { spawn "fuzzel"; }`, key: "Mod+Space", occupied: true},
		{name: "mixed mod space", line: `Mod+space { spawn "fuzzel"; }`, key: "Mod+Space", occupied: true},
		{name: "mixed mod comma", line: `mod+Comma { spawn "x"; }`, key: "Mod+Comma", occupied: true},
		{name: "win alias", line: `Win+Space { spawn "x"; }`, key: "Super+Space", occupied: true},
		{name: "control alias", line: `Control+a { spawn "x"; }`, key: "Ctrl+A", occupied: true},
		{name: "modifier order", line: `Shift+Ctrl+a { spawn "x"; }`, key: "Ctrl+Shift+A", occupied: true},
		{name: "duplicate modifier", line: `Ctrl+Control+a { spawn "x"; }`, key: "Ctrl+A", occupied: true},
		{name: "mod5 alias", line: `Mod5+a { spawn "x"; }`, key: "ISO_Level3_Shift+A", occupied: true},
		{name: "mod3 alias", line: `mod3+comma { spawn "x"; }`, key: "ISO_Level5_Shift+Comma", occupied: true},
		{name: "quoted win", line: `"Win+Space" { spawn "x"; }`, key: "Super+Space", occupied: true},
		{name: "included lower", line: `mod+space { spawn "fuzzel"; }`, key: "Mod+Space", include: true, occupied: true},
		{name: "super is not mod", line: `Super+Space { spawn "x"; }`, key: "Mod+Space", occupied: false},
		{name: "mod4 is not super", line: `Mod4+Space { spawn "x"; }`, key: "Super+Space", occupied: false},
		{name: "extra modifier", line: `Mod+Shift+Space { spawn "x"; }`, key: "Mod+Space", occupied: false},
		{name: "longer keysym", line: `Mod+Spacebar { spawn "x"; }`, key: "Mod+Space", occupied: false},
		{name: "comment", line: `// mod+space { spawn "x"; }`, key: "Mod+Space", occupied: false},
		{name: "slashdash comment", line: `/- mod+space { spawn "x"; }`, key: "Mod+Space", occupied: false},
		{name: "screensaver exact distinct", line: `XF86Screensaver { spawn "x"; }`, key: "XF86ScreenSaver", occupied: false},
		{name: "screensaver case fold", line: `xf86screensaver { spawn "x"; }`, key: "XF86ScreenSaver", occupied: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			config := filepath.Join(dir, "config.kdl")
			body := "binds {\n    " + tt.line + "\n}\n"
			if tt.include {
				write(t, filepath.Join(dir, "extra.kdl"), body)
				body = "include \"extra.kdl\"\n"
			}
			write(t, config, body)
			opts := Options{
				ConfigPath:  config,
				SidecarPath: filepath.Join(dir, "sysc.kdl"),
				StateDir:    filepath.Join(dir, "state"),
				Binds:       []Bind{{Key: tt.key, Action: `spawn "sysc-shell"`}},
				Now:         time.Unix(1000, 0),
			}
			res, err := Apply(opts)
			if err != nil {
				t.Fatal(err)
			}
			if tt.occupied {
				if len(res.BindsSkipped) != 1 || res.BindsSkipped[0] != tt.key || len(res.BindsAdded) != 0 {
					t.Fatalf("BindsSkipped=%v BindsAdded=%v; want %s skipped", res.BindsSkipped, res.BindsAdded, tt.key)
				}
				return
			}
			if len(res.BindsAdded) != 1 || res.BindsAdded[0] != tt.key || len(res.BindsSkipped) != 0 {
				t.Fatalf("BindsSkipped=%v BindsAdded=%v; want %s added", res.BindsSkipped, res.BindsAdded, tt.key)
			}
		})
	}
}

func TestIssue9MixedCaseBindIsOccupied(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.kdl")
	write(t, config, "binds {\n    Mod+space { spawn \"fuzzel\"; }\n}\n")
	sidecar := filepath.Join(dir, "sysc.kdl")
	opts := Options{
		ConfigPath:  config,
		SidecarPath: sidecar,
		StateDir:    filepath.Join(dir, "state"),
		Binds:       DefaultBinds,
		Now:         time.Unix(1000, 0),
	}
	res, err := Apply(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BindsSkipped) != 1 || res.BindsSkipped[0] != "Mod+Space" {
		t.Fatalf("BindsSkipped=%v; want [Mod+Space]", res.BindsSkipped)
	}
	if len(res.BindsAdded) != 1 || res.BindsAdded[0] != "Mod+Comma" {
		t.Fatalf("BindsAdded=%v; want [Mod+Comma]", res.BindsAdded)
	}
	data, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Mod+Space") {
		t.Fatalf("sidecar stole the user's bind:\n%s", data)
	}
	cfg, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), `Mod+space { spawn "fuzzel"; }`) {
		t.Fatalf("user bind was rewritten:\n%s", cfg)
	}
}

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

func TestSpawnsAcrossIncludeTree(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config.kdl"), "include \"extra.kdl\"\nspawn-at-startup \"/usr/bin/mako\"\n")
	write(t, filepath.Join(dir, "extra.kdl"), "// spawn-at-startup \"dunst\"\nspawn-at-startup \"quickshell\" \"-c\" \"noctalia\"\n")

	opts := Options{ConfigPath: filepath.Join(dir, "config.kdl"), SidecarPath: filepath.Join(dir, "sysc.kdl")}
	got, err := Spawns(opts, []string{"mako", "dunst", "quickshell", "waybar"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("spawns = %+v; want mako and quickshell", got)
	}
	if got[0].Name != "mako" || got[0].Path != filepath.Join(dir, "config.kdl") || got[0].LineNo != 1 {
		t.Errorf("mako spawn = %+v", got[0])
	}
	if got[1].Name != "quickshell" || got[1].Path != filepath.Join(dir, "extra.kdl") || got[1].LineNo != 1 {
		t.Errorf("quickshell spawn = %+v", got[1])
	}
}

func TestCommentAndRestoreLineByteForByte(t *testing.T) {
	for _, tc := range []struct{ name, nl string }{
		{"lf", "\n"},
		{"crlf", "\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			config := filepath.Join(dir, "config.kdl")
			original := "input {}\r\nspawn-at-startup \"mako\"\r\noutput \"eDP-1\" {}\r\n"
			original = strings.ReplaceAll(original, "\r\n", tc.nl)
			write(t, config, original)

			hl, err := CommentLine(config, 1, `spawn-at-startup "mako"`)
			if err != nil {
				t.Fatal(err)
			}
			if hl.Commented != `// sysc-handover: spawn-at-startup "mako"` || hl.Original != `spawn-at-startup "mako"` {
				t.Fatalf("record = %+v", hl)
			}
			commentedText := readFile(t, config)
			if !strings.Contains(commentedText, hl.Commented) {
				t.Fatalf("commented line missing:\n%s", commentedText)
			}
			if bak := readFile(t, config+".sysc.bak"); bak != original {
				t.Fatalf("first backup changed: %q", bak)
			}

			if err := RestoreLine(hl); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, config); got != original {
				t.Fatalf("restore not byte-for-byte:\n got %q\nwant %q", got, original)
			}
		})
	}
}

func TestCommentLineRefusesChangedLine(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.kdl")
	write(t, config, "spawn-at-startup \"waybar\"\n")
	if _, err := CommentLine(config, 0, `spawn-at-startup "mako"`); err == nil {
		t.Fatal("CommentLine accepted a line that no longer matches")
	}
	if _, err := CommentLine(config, 5, `spawn-at-startup "waybar"`); err == nil {
		t.Fatal("CommentLine accepted an out-of-range line")
	}
}

func TestRestoreLineLeavesEditedLineAlone(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.kdl")
	write(t, config, "spawn-at-startup \"mako\"\n")
	hl, err := CommentLine(config, 0, `spawn-at-startup "mako"`)
	if err != nil {
		t.Fatal(err)
	}
	write(t, config, "// user replaced the line\n")
	if err := RestoreLine(hl); err == nil {
		t.Fatal("RestoreLine succeeded after the line changed")
	}
	if got := readFile(t, config); got != "// user replaced the line\n" {
		t.Fatalf("RestoreLine edited the file: %q", got)
	}
}

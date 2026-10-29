package preflight

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeLibrariesBeforeSwap(t *testing.T) {
	dir := t.TempDir()
	writeELF(t, filepath.Join(dir, "sysc-lock"), "libpam.so.0")
	env := runtimeEnv{libraryDirs: []string{dir}}
	if _, err := checkRuntime(dir, []string{"sysc-lock"}, env); err == nil || !strings.Contains(err.Error(), "libpam.so.0") {
		t.Fatalf("missing library: %v", err)
	}
	writeELF(t, filepath.Join(dir, "libpam.so.0"), "libaudit.so.1")
	if _, err := checkRuntime(dir, []string{"sysc-lock"}, env); err == nil || !strings.Contains(err.Error(), "libaudit.so.1") {
		t.Fatalf("missing transitive library: %v", err)
	}
	writeELF(t, filepath.Join(dir, "libaudit.so.1"))
	if _, err := checkRuntime(dir, []string{"sysc-lock"}, env); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeLibrariesInMultiarchDirectories(t *testing.T) {
	for _, base := range []string{"usr/lib", "lib"} {
		t.Run(base, func(t *testing.T) {
			root := t.TempDir()
			baseDir := filepath.Join(root, filepath.FromSlash(base))
			multiarch := filepath.Join(baseDir, "x86_64-linux-gnu")
			if err := os.MkdirAll(multiarch, 0755); err != nil {
				t.Fatal(err)
			}
			writeELF(t, filepath.Join(root, "sysc-lock"), "libpam.so.0")
			writeELF(t, filepath.Join(multiarch, "libpam.so.0"), "libaudit.so.1")
			writeELF(t, filepath.Join(multiarch, "libaudit.so.1"))
			if _, err := checkRuntime(root, []string{"sysc-lock"}, runtimeEnv{libraryDirs: []string{baseDir}}); err != nil {
				t.Fatalf("multiarch runtime check: %v", err)
			}
		})
	}
}

func TestRuntimeRejectsInvalidStagedBinary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sysc-lock"), []byte("not an executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := checkRuntime(dir, []string{"sysc-lock"}, runtimeEnv{}); err == nil {
		t.Fatal("invalid executable accepted")
	}
}

func TestRuntimeFontRequiredOnlyForShell(t *testing.T) {
	dir := t.TempDir()
	writeELF(t, filepath.Join(dir, "sysc-lock"))
	writeELF(t, filepath.Join(dir, "sysc-shell"))
	env := runtimeEnv{fontDirs: []string{dir}}
	if _, err := checkRuntime(dir, []string{"sysc-lock"}, env); err != nil {
		t.Fatal(err)
	}
	if _, err := checkRuntime(dir, []string{"sysc-shell"}, env); err == nil || !strings.Contains(err.Error(), "font") {
		t.Fatalf("missing font: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.ttf"), []byte("junk"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := checkRuntime(dir, []string{"sysc-shell"}, env); err == nil {
		t.Fatal("junk font accepted")
	}
	font := make([]byte, 28)
	copy(font, "OTTO\x00\x01")
	if err := os.WriteFile(filepath.Join(dir, "good.otf"), font, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := checkRuntime(dir, []string{"sysc-shell"}, env); err != nil {
		t.Fatal(err)
	}
	env.fontDirs = nil
	env.command = func(name string, args ...string) ([]byte, error) {
		if name != "fc-match" || !contains(args, "--format=%{file}") {
			t.Fatalf("unexpected font probe %s %v", name, args)
		}
		return []byte(filepath.Join(dir, "good.otf")), nil
	}
	if _, err := checkRuntime(dir, []string{"sysc-shell"}, env); err != nil {
		t.Fatalf("fontconfig match: %v", err)
	}
}

func TestRuntimeKittyWarningOnlyWhenWallsSelected(t *testing.T) {
	for _, tc := range []struct {
		name        string
		names       []string
		kitty       bool
		wantWarning bool
	}{
		{name: "missing for walls", names: []string{"sysc-walls-daemon"}, wantWarning: true},
		{name: "walls not selected", names: []string{"sysc-lock"}},
		{name: "kitty available", names: []string{"sysc-walls-daemon"}, kitty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			staging := t.TempDir()
			writeELF(t, filepath.Join(staging, "sysc-lock"))
			writeELF(t, filepath.Join(staging, "sysc-walls-daemon"))
			binDir := filepath.Join(staging, "bin")
			if err := os.Mkdir(binDir, 0700); err != nil {
				t.Fatal(err)
			}
			if tc.kitty {
				if err := os.WriteFile(filepath.Join(binDir, "kitty"), []byte("#!/bin/sh\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", binDir)

			warnings, err := checkRuntime(staging, tc.names, runtimeEnv{})
			if err != nil {
				t.Fatal(err)
			}
			gotWarning := strings.Contains(strings.Join(warnings, "\n"), "Kitty")
			if gotWarning != tc.wantWarning {
				t.Fatalf("Kitty warning = %v, want %v; warnings: %v", gotWarning, tc.wantWarning, warnings)
			}
		})
	}
}

func TestRuntimeKittyWarningPreservesClipboardWarning(t *testing.T) {
	staging := t.TempDir()
	writeELF(t, filepath.Join(staging, "sysc-walls-daemon"))
	writeELF(t, filepath.Join(staging, "sysc-clipboard"))
	binDir := filepath.Join(staging, "bin")
	if err := os.Mkdir(binDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	warnings, err := checkRuntime(staging, []string{"sysc-walls-daemon", "sysc-clipboard"}, runtimeEnv{})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "Kitty") || !strings.Contains(joined, "Clipboard history stays in memory") {
		t.Fatalf("runtime warnings lost: %v", warnings)
	}
}

func TestRuntimeClipboardSession(t *testing.T) {
	dir := t.TempDir()
	writeELF(t, filepath.Join(dir, "sysc-clipboard"))
	for _, tc := range []struct {
		name, owner, activatable, items, alias, locked, want string
		graphical                                            bool
		warning                                              bool
	}{
		{name: "tty", warning: true},
		{name: "missing", graphical: true, owner: "b false", want: "Secret Service"},
		{name: "activatable", graphical: true, owner: "b false", activatable: `as 1 "org.freedesktop.secrets"`, warning: true},
		{name: "locked", graphical: true, owner: "b true", items: "ao 0 ao 0", alias: `o "/org/freedesktop/secrets/collection/login"`, locked: "b true", want: "unlock"},
		{name: "missing-default", graphical: true, owner: "b true", items: "ao 0 ao 0", alias: `o "/"`, want: "default"},
		{name: "ready", graphical: true, owner: "b true", items: "ao 0 ao 0", alias: `o "/org/freedesktop/secrets/collection/login"`, locked: "b false"},
		{name: "existing-key", graphical: true, owner: "b true", items: `ao 1 "/org/freedesktop/secrets/collection/login/key" ao 0`},
		{name: "locked-key", graphical: true, owner: "b true", items: `ao 0 ao 1 "/org/freedesktop/secrets/collection/login/key"`, want: "unlock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := runtimeEnv{graphical: tc.graphical, command: func(name string, args ...string) ([]byte, error) {
				if name != "busctl" {
					return nil, errors.New("absent")
				}
				if !contains(args, "--auto-start=no") {
					t.Fatal("probe may activate a service")
				}
				var output string
				switch {
				case contains(args, "NameHasOwner"):
					output = tc.owner
				case contains(args, "ListActivatableNames"):
					output = tc.activatable
				case contains(args, "SearchItems"):
					output = tc.items
				case contains(args, "ReadAlias"):
					output = tc.alias
				case contains(args, "Locked"):
					output = tc.locked
				default:
					t.Fatalf("unexpected probe %v", args)
				}
				return []byte(output), nil
			}}
			warnings, err := checkRuntime(dir, []string{"sysc-clipboard"}, env)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && !strings.Contains(strings.Join(warnings, " "), tc.want) {
				t.Fatalf("wanted guidance %s: %v", tc.want, warnings)
			}
			if (len(warnings) > 0) != (tc.warning || tc.want != "") {
				t.Fatalf("warnings %v", warnings)
			}
		})
	}
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// A minimal ELF fixture keeps this check independent of host libraries or compilers.
func writeELF(t *testing.T, path string, libraries ...string) {
	t.Helper()
	strtab := []byte{0}
	var offsets []uint64
	for _, lib := range libraries {
		offsets = append(offsets, uint64(len(strtab)))
		strtab = append(strtab, append([]byte(lib), 0)...)
	}
	data := make([]byte, 256+len(strtab)+16*(len(libraries)+1))
	copy(data, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	put16 := func(off int, v uint16) { binary.LittleEndian.PutUint16(data[off:], v) }
	put32 := func(off int, v uint32) { binary.LittleEndian.PutUint32(data[off:], v) }
	put64 := func(off int, v uint64) { binary.LittleEndian.PutUint64(data[off:], v) }
	put16(16, 3)
	put16(18, 62)
	put32(20, 1)
	put64(40, 64)
	put16(52, 64)
	put16(58, 64)
	put16(60, 3)
	put32(128+4, 3)
	put64(128+24, 256)
	put64(128+32, uint64(len(strtab)))
	put32(192+4, 6)
	put64(192+24, uint64(256+len(strtab)))
	put64(192+32, uint64(16*(len(libraries)+1)))
	put32(192+40, 1)
	put64(192+56, 16)
	copy(data[256:], strtab)
	for i, off := range offsets {
		base := 256 + len(strtab) + 16*i
		put64(base, 1)
		put64(base+8, off)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

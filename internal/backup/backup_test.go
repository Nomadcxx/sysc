package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFirstBakNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.kdl")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	made, err := FirstBak(path)
	if err != nil || !made {
		t.Fatalf("first FirstBak = %v, %v; want true, nil", made, err)
	}
	if err := os.WriteFile(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	made, err = FirstBak(path)
	if err != nil || made {
		t.Fatalf("second FirstBak = %v, %v; want false, nil", made, err)
	}
	bak, err := os.ReadFile(path + ".sysc.bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(bak) != "original" {
		t.Fatalf("backup = %q; want original", bak)
	}

	state := filepath.Join(dir, "state")
	if _, err := StateCopy(state, "niri-config.kdl", path, time.Unix(1000, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := StateCopy(state, "niri-config.kdl", path, time.Unix(2000, 0)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("state copies = %d; want 2", len(entries))
	}
}

func TestStateCopyRotates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.kdl")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state")
	for i := 0; i < 7; i++ {
		if _, err := StateCopy(state, "niri-config.kdl", path, time.Unix(int64(1000+i), 0)); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxStateCopies {
		t.Fatalf("state copies = %d; want %d", len(entries), maxStateCopies)
	}
}

// Regression for #37: with a frozen clock every copy shares one stamp and the
// bare "<name>.<stamp>" is reused once rotated away; rotation must never
// delete the copy StateCopy just wrote.
func TestStateCopyKeepsNewestUnderCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.kdl")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state")
	var zero time.Time
	for i := 0; i < 8; i++ {
		dst, err := StateCopy(state, "niri-config.kdl", path, zero)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(dst); err != nil {
			t.Fatalf("copy %d deleted in its own run: %v", i, err)
		}
		entries, err := os.ReadDir(state)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) > maxStateCopies+1 {
			t.Fatalf("copy %d left %d files; want at most %d", i, len(entries), maxStateCopies+1)
		}
	}
}

func TestStateCopyRotatesOldest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.kdl")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state")
	base := time.Unix(1700000000, 0)
	newest := ""
	for i := 0; i < 7; i++ {
		dst, err := StateCopy(state, "niri-config.kdl", path, base.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		newest = dst
	}
	entries, err := os.ReadDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxStateCopies {
		t.Fatalf("state copies = %d; want %d", len(entries), maxStateCopies)
	}
	if _, err := os.Stat(newest); err != nil {
		t.Fatalf("newest copy missing after rotation: %v", err)
	}
	oldest := filepath.Join(state, "niri-config.kdl."+base.UTC().Format("20060102T150405Z"))
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Fatalf("oldest copy still present: %v", err)
	}
}

func TestFirstBakKeepsSourceMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.kdl")
	if err := os.WriteFile(path, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FirstBak(path); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path + ".sysc.bak")
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("backup mode = %04o, want 0600", got)
	}
}

func TestFileModeDefaultsForMissingFile(t *testing.T) {
	got, err := FileMode(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatal(err)
	}
	if got != 0o644 {
		t.Errorf("FileMode(missing) = %04o, want 0644", got)
	}
}

func TestFileModeReturnsUnexpectedStatErrors(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "file")
	if err := os.WriteFile(parent, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FileMode(filepath.Join(parent, "child")); err == nil {
		t.Fatal("FileMode on a non-directory parent returned no error")
	}
}

// #93: a private Niri config must not become world-readable through the
// timestamped snapshots, even when the state directory already exists with
// the permissive mode the v0.1.0 stamp writer used.
func TestStateCopyPreservesSourceModeAndTightensStateDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.kdl")
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state", "sysc", "backups")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	dst, err := StateCopy(state, "niri-config.kdl", path, time.Unix(1000, 0))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("snapshot mode = %04o, want 0600", got)
	}
	dirInfo, err := os.Stat(state)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("state dir mode = %04o, want 0700", got)
	}
}

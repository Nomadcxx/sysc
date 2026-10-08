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

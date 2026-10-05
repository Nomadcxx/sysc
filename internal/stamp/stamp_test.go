package stamp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStampAfterEnable(t *testing.T) {
	stateDir := t.TempDir()
	s := Stamp{
		Release:           "v0.1.0",
		Components:        map[string]string{"sysc-shell": "v0.1.0", "sysc-clipboard": "v0.1.1"},
		GSlapperInstalled: true,
		Started:           false,
	}

	if err := Write(stateDir, s, false); err != nil {
		t.Fatalf("write with units not enabled: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, FileName)); !os.IsNotExist(err) {
		t.Fatalf("stamp written before units were enabled: %v", err)
	}

	if err := Write(stateDir, s, true); err != nil {
		t.Fatalf("write after enable: %v", err)
	}
	got, err := Read(stateDir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Release != s.Release {
		t.Errorf("release = %q, want %q", got.Release, s.Release)
	}
	if got.Components["sysc-shell"] != "v0.1.0" || got.Components["sysc-clipboard"] != "v0.1.1" {
		t.Errorf("components = %v", got.Components)
	}
	if !got.GSlapperInstalled {
		t.Error("gslapper flag lost")
	}
	if got.Started {
		t.Error("started should be false for an SSH/TTY install")
	}
}

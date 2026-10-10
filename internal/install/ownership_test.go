package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

func ownershipOptions(t *testing.T) Options {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return Options{Home: home, Pin: pin.Pin{Components: []pin.Component{{ID: "sysc-shell", Binaries: []pin.Binary{{Name: "sysc-shell"}}}}}, Systemctl: noopSystemctl}
}

func putOwnedTestFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func TestOwnershipPreservesBackupModeAndModifiedBytes(t *testing.T) {
	o := ownershipOptions(t)
	path := filepath.Join(o.binDir(), "sysc-shell")
	putOwnedTestFile(t, path, []byte("foreign binary"), 0o750)
	var st stamp.Stamp
	if err := planFile(&st, path, []byte("SYSC binary"), "", o.systemctl()); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path + ".sysc.bak")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o750 {
		t.Errorf("backup mode = %o; want 750", fi.Mode().Perm())
	}
	if st.Files[0].RollbackSHA256 != fileHash([]byte("foreign binary")) {
		t.Fatalf("rollback ownership digest omitted: %+v", st.Files[0])
	}
	putOwnedTestFile(t, path, []byte("SYSC binary"), 0o755)
	putOwnedTestFile(t, st.Files[0].Backup, []byte("changed backup"), 0o750)
	if err := restoreFile(o, st.Files[0]); err == nil {
		t.Error("restored modified backup")
	}
	if err := planFile(&st, path, []byte("updated binary"), "", o.systemctl()); err == nil {
		t.Error("rerun accepted modified backup")
	}
}

func TestOwnershipPreservesModifiedEmptyFile(t *testing.T) {
	o := ownershipOptions(t)
	path := filepath.Join(o.binDir(), "sysc-shell")
	putOwnedTestFile(t, path, nil, 0o755)
	f := stamp.File{Path: path, SHA256: fileHash([]byte("installed binary"))}
	if err := restoreFile(o, f); err == nil {
		t.Fatal("removed user-truncated binary")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("empty file lost: %v", err)
	}
}

func TestOwnershipRejectsUnpinnedFileNames(t *testing.T) {
	o := ownershipOptions(t)
	for _, f := range []stamp.File{
		{Path: filepath.Join(o.binDir(), "unrelated"), SHA256: fileHash(nil)},
		{Path: filepath.Join(o.unitDir(), "unrelated.service"), Unit: "unrelated.service", SHA256: fileHash(nil)},
	} {
		if err := validateOwnedFile(o, f); err == nil {
			t.Errorf("accepted unpinned file %+v", f)
		}
	}
}

func TestOwnershipReloadsRestoredUnitBeforeStarting(t *testing.T) {
	o := ownershipOptions(t)
	var calls []string
	o.Systemctl = func(args ...string) error { calls = append(calls, strings.Join(args, " ")); return nil }
	path := filepath.Join(o.unitDir(), "sysc-shell.service")
	putOwnedTestFile(t, path, []byte("foreign unit"), 0o640)
	var st stamp.Stamp
	if err := planFile(&st, path, []byte("SYSC unit"), "sysc-shell.service", o.systemctl()); err != nil {
		t.Fatal(err)
	}
	putOwnedTestFile(t, path, []byte("SYSC unit"), 0o644)
	calls = nil
	if err := restoreFile(o, st.Files[0]); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(calls, "; ")
	if got != "stop sysc-shell.service; disable sysc-shell.service; daemon-reload; enable sysc-shell.service; start sysc-shell.service" {
		t.Fatalf("restoration order = %s", got)
	}
}

func TestOwnershipRetryAfterBackupCleanup(t *testing.T) {
	o := ownershipOptions(t)
	path := filepath.Join(o.binDir(), "sysc-shell")
	foreign := []byte("foreign binary")
	putOwnedTestFile(t, path, foreign, 0o755)
	var st stamp.Stamp
	if err := planFile(&st, path, []byte("SYSC binary"), "", o.systemctl()); err != nil {
		t.Fatal(err)
	}
	putOwnedTestFile(t, path, []byte("SYSC binary"), 0o755)
	if err := restoreFile(o, st.Files[0]); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(st.Files[0].Backup); err != nil {
		t.Fatal(err)
	}
	if err := restoreFile(o, st.Files[0]); err != nil {
		t.Fatalf("restoration retry failed: %v", err)
	}
}

func TestCheckOwnershipWithPacman(t *testing.T) {
	for _, code := range []string{"0", "1", "2"} {
		t.Run("exit_"+code, func(t *testing.T) {
			dir := t.TempDir()
			putOwnedTestFile(t, filepath.Join(dir, "pacman"), []byte("#!/bin/sh\nexit "+code+"\n"), 0o755)
			t.Setenv("PATH", dir)
			err := CheckOwnership(t.TempDir(), []pin.Component{{ID: "sysc-shell"}})
			if (err == nil) != (code == "1") {
				t.Fatalf("pacman exit %s: %v", code, err)
			}
		})
	}
}

func TestOwnershipUninstallsInterruptedUpgrade(t *testing.T) {
	o := ownershipOptions(t)
	path := filepath.Join(o.binDir(), "sysc-shell")
	var st stamp.Stamp
	if err := planFile(&st, path, []byte("first SYSC binary"), "", o.systemctl()); err != nil {
		t.Fatal(err)
	}
	putOwnedTestFile(t, path, []byte("first SYSC binary"), 0o755)
	if err := planFile(&st, path, []byte("upgraded SYSC binary"), "", o.systemctl()); err != nil {
		t.Fatal(err)
	}
	// Planning was stamped, but the process stopped before swapping binaries.
	if err := restoreFile(o, st.Files[0]); err != nil {
		t.Fatalf("interrupted upgrade cannot uninstall: %v", err)
	}
}

func TestOwnershipMissingUnitRetryLeavesFallbackAlone(t *testing.T) {
	o := ownershipOptions(t)
	o.Systemctl = func(args ...string) error {
		t.Fatalf("missing user unit changed fallback service: %v", args)
		return nil
	}
	f := stamp.File{Path: filepath.Join(o.unitDir(), "sysc-shell.service"), Unit: "sysc-shell.service", SHA256: fileHash([]byte("removed unit"))}
	if err := restoreFile(o, f); err != nil {
		t.Fatal(err)
	}
}

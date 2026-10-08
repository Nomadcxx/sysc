package dbusact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/stamp"
)

func TestContentClaimsNotificationsName(t *testing.T) {
	c := string(Content())
	for _, want := range []string{"Name=org.freedesktop.Notifications", "SystemdService=sysc-notify.service", "Exec=/bin/false"} {
		if !strings.Contains(c, want) {
			t.Fatalf("content missing %q:\n%s", want, c)
		}
	}
}

func TestWriteThenDelete(t *testing.T) {
	home := t.TempDir()
	a, err := Write(home)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(home, "dbus-1", "services", "org.freedesktop.Notifications.service")
	if a.Path != wantPath {
		t.Fatalf("path = %q, want %q", a.Path, wantPath)
	}
	if a.Backup != "" {
		t.Fatalf("unexpected backup %q", a.Backup)
	}
	data, err := os.ReadFile(a.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(Content()) {
		t.Fatalf("file content mismatch:\n%s", data)
	}
	if _, err := os.Stat(a.Path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp file left behind")
	}
	if err := Delete(a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.Path); !os.IsNotExist(err) {
		t.Fatal("file not removed")
	}
}

func TestWriteBacksUpForeignFileAndDeleteRestores(t *testing.T) {
	home := t.TempDir()
	path := Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("[D-BUS Service]\nName=org.freedesktop.Notifications\nExec=dunst\n")
	if err := os.WriteFile(path, foreign, 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := Write(home)
	if err != nil {
		t.Fatal(err)
	}
	if a.Backup != path+".sysc.bak" {
		t.Fatalf("backup = %q", a.Backup)
	}
	backupBytes, err := os.ReadFile(a.Backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(backupBytes) != string(foreign) {
		t.Fatalf("backup not byte-for-byte:\n%s", backupBytes)
	}
	if err := Delete(a); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(foreign) {
		t.Fatalf("foreign file not restored:\n%s", restored)
	}
	if _, err := os.Stat(a.Backup); !os.IsNotExist(err) {
		t.Fatal("backup file left behind")
	}
}

func TestDeleteMissingFileIsOK(t *testing.T) {
	home := t.TempDir()
	if err := Delete(stamp.Activation{Path: Path(home)}); err != nil {
		t.Fatal(err)
	}
}

func TestPathUsesDataHome(t *testing.T) {
	if got := Path("/data"); got != "/data/dbus-1/services/org.freedesktop.Notifications.service" {
		t.Fatalf("Path = %q", got)
	}
}

// A re-run must not back up our own file: Delete would then "restore" it and
// leave behind a file naming a service the uninstall just removed.
func TestWriteTwiceLeavesNoBackupOfOwnContent(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir); err != nil {
		t.Fatal(err)
	}
	a, err := Write(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a.Backup != "" {
		t.Fatalf("re-run stamped a backup of our own file: %+v", a)
	}
	if bak := Path(dir) + ".sysc.bak"; func() bool { _, err := os.Stat(bak); return err == nil }() {
		t.Fatalf("stray backup: %s", bak)
	}
	if err := Delete(a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(Path(dir)); !os.IsNotExist(err) {
		t.Fatalf("activation file survived uninstall after a second install")
	}
}

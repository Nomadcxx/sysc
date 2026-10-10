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

func TestRerunPreservesForeignBackup(t *testing.T) {
	home := t.TempDir()
	path := Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("foreign activation\n")
	if err := os.WriteFile(path, foreign, 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := Write(home)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Write(home)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("rerun forgot recovery record: first=%+v second=%+v", first, second)
	}
	if err := Delete(second); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(foreign) {
		t.Fatalf("restored = %q, error = %v", got, err)
	}
}

func TestDeletePreservesChangedActivation(t *testing.T) {
	for _, displaced := range []bool{false, true} {
		t.Run(map[bool]string{false: "without_backup", true: "with_backup"}[displaced], func(t *testing.T) {
			home := t.TempDir()
			path := Path(home)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if displaced {
				if err := os.WriteFile(path, []byte("first foreign"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			a, err := Write(home)
			if err != nil {
				t.Fatal(err)
			}
			changed := []byte("user changed activation")
			if err := os.WriteFile(path, changed, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := Delete(a); err == nil {
				t.Fatal("expected ownership error")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(changed) {
				t.Fatalf("changed activation overwritten: %q, error %v", got, err)
			}
			if displaced {
				if _, err := os.Stat(a.Backup); err != nil {
					t.Fatalf("backup lost: %v", err)
				}
			}
		})
	}
}

func TestWriteRetainsExistingFirstBackup(t *testing.T) {
	home := t.TempDir()
	path := Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("foreign activation")
	for _, name := range []string{path, path + ".sysc.bak"} {
		if err := os.WriteFile(name, foreign, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a, err := Write(home)
	if err != nil {
		t.Fatal(err)
	}
	if a.Backup != path+".sysc.bak" {
		t.Fatalf("lost existing first backup: %+v", a)
	}
}

func TestDeleteAlreadyRestoredActivation(t *testing.T) {
	home := t.TempDir()
	path := Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("foreign activation")
	for _, name := range []string{path, path + ".sysc.bak"} {
		if err := os.WriteFile(name, foreign, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Delete(stamp.Activation{Path: path, Backup: path + ".sysc.bak"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(foreign) {
		t.Fatalf("restored activation changed: %q, error %v", got, err)
	}
}

func TestPlanLeavesOriginalForInterruptedInstall(t *testing.T) {
	home := t.TempDir()
	path := Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("foreign activation")
	if err := os.WriteFile(path, foreign, 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := Plan(home)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(foreign) {
		t.Fatalf("planning replaced original: %q, error %v", got, err)
	}
	if a.Backup != path+".sysc.bak" {
		t.Fatalf("planning omitted recovery path: %+v", a)
	}
	if len(a.BackupSHA256) != 64 {
		t.Fatalf("planning omitted backup checksum: %+v", a)
	}
	state := t.TempDir()
	if err := stamp.Write(state, stamp.Stamp{Activation: &a}, true); err != nil {
		t.Fatal(err)
	}
	recovered, err := stamp.Read(state)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Activation == nil || *recovered.Activation != a {
		t.Fatalf("recovery record lost after restart: %+v", recovered.Activation)
	}
	if err := Delete(*recovered.Activation); err != nil {
		t.Fatalf("interrupted install recovery: %v", err)
	}
	got, err = os.ReadFile(path)
	if err != nil || string(got) != string(foreign) {
		t.Fatalf("recovery replaced original: %q, error %v", got, err)
	}
}

func TestWritePreservesForeignChangesWithExistingBackup(t *testing.T) {
	home := t.TempDir()
	path := Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("first foreign"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(home); err != nil {
		t.Fatal(err)
	}
	changed := []byte("user changed activation")
	if err := os.WriteFile(path, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(home); err == nil {
		t.Fatal("expected ownership error")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(changed) {
		t.Fatalf("rerun overwrote user change: %q, error %v", got, err)
	}
}

func TestActivationSymlinksArePreserved(t *testing.T) {
	for _, backup := range []bool{false, true} {
		t.Run(map[bool]string{false: "activation", true: "backup"}[backup], func(t *testing.T) {
			home := t.TempDir()
			a, err := Write(home)
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(home, "user-file")
			if err := os.WriteFile(target, Content(), 0o644); err != nil {
				t.Fatal(err)
			}
			link := a.Path
			if backup {
				a.Backup = a.Path + ".sysc.bak"
				link = a.Backup
			} else if err := os.Remove(a.Path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			if _, err := Write(home); err == nil {
				t.Fatal("write accepted symlink")
			}
			if err := Delete(a); err == nil {
				t.Fatal("delete accepted symlink")
			}
			if _, err := os.Readlink(link); err != nil {
				t.Fatalf("symlink removed: %v", err)
			}
		})
	}
}

func TestDeleteRejectsBackupOfOwnedContent(t *testing.T) {
	home := t.TempDir()
	a, err := Write(home)
	if err != nil {
		t.Fatal(err)
	}
	a.Backup = a.Path + ".sysc.bak"
	if err := os.WriteFile(a.Backup, Content(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Delete(a); err == nil {
		t.Fatal("restored backup of own activation")
	}
	if _, err := os.Stat(a.Backup); err != nil {
		t.Fatalf("invalid backup removed: %v", err)
	}
}

func TestDeleteRetryAfterBackupRemoved(t *testing.T) {
	home := t.TempDir()
	path := Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("foreign activation")
	if err := os.WriteFile(path, foreign, 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := Write(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(a); err != nil {
		t.Fatal(err)
	}
	// The restored file survived, but recording completion failed. The same
	// stamp must safely finish the next uninstall without its removed backup.
	if err := Delete(a); err != nil {
		t.Fatalf("retry failed after restoration: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(foreign) {
		t.Fatalf("retry changed restored activation: %q, error %v", got, err)
	}
	changed := []byte("user changed restored activation")
	if err := os.WriteFile(path, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Delete(a); err == nil {
		t.Fatal("retry accepted changed restored activation")
	}
	got, err = os.ReadFile(path)
	if err != nil || string(got) != string(changed) {
		t.Fatalf("retry lost user changes: %q, error %v", got, err)
	}
}

func TestDeletePreservesModifiedBackup(t *testing.T) {
	home := t.TempDir()
	path := Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("foreign activation"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := Write(home)
	if err != nil {
		t.Fatal(err)
	}
	changed := []byte("user changed backup")
	if err := os.WriteFile(a.Backup, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Delete(a); err == nil {
		t.Fatal("restored modified backup")
	}
	got, err := os.ReadFile(a.Backup)
	if err != nil || string(got) != string(changed) {
		t.Fatalf("changed backup lost: %q, error %v", got, err)
	}
	got, err = os.ReadFile(path)
	if err != nil || string(got) != string(Content()) {
		t.Fatalf("activation changed on failure: %q, error %v", got, err)
	}
}

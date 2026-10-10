// Package dbusact writes the user-level D-Bus activation file that makes
// sysc-notify the notifications daemon. The user data directory wins over
// /usr/share, so a packaged mako or dunst cannot answer the name first.
package dbusact

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Nomadcxx/sysc/internal/backup"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

// ServiceName is the bus name the activation file claims.
const ServiceName = "org.freedesktop.Notifications"

// Content is the activation file. Exec=/bin/false makes a request that races
// the unit fail fast instead of starting a second daemon; systemd starts
// SystemdService, and Type=dbus holds the name for it.
func Content() []byte {
	return []byte("[D-BUS Service]\n" +
		"Name=" + ServiceName + "\n" +
		"Exec=/bin/false\n" +
		"SystemdService=sysc-notify.service\n")
}

// Path is the activation file inside a user data home.
func Path(dataHome string) string {
	return filepath.Join(dataHome, "dbus-1", "services", ServiceName+".service")
}

// Plan saves the first foreign activation file without replacing it. Persist
// the returned record before Install so an interruption retains recovery data.
func Plan(dataHome string) (stamp.Activation, error) {
	path := Path(dataHome)
	a := stamp.Activation{Path: path}
	data, err := readRegular(path)
	if err != nil && !os.IsNotExist(err) {
		return a, err
	}
	saved, bakErr := readRegular(path + ".sysc.bak")
	if bakErr == nil {
		if bytes.Equal(saved, Content()) {
			return a, fmt.Errorf("activation backup contains SYSC's own file: %s", path+".sysc.bak")
		}
		a.Backup = path + ".sysc.bak"
		a.BackupSHA256 = digest(saved)
	} else if !os.IsNotExist(bakErr) {
		return a, bakErr
	}
	if os.IsNotExist(err) || bytes.Equal(data, Content()) {
		return a, nil
	}
	if a.Backup != "" {
		if !bytes.Equal(data, saved) {
			return a, fmt.Errorf("activation changed since its first backup: %s", path)
		}
		return a, nil
	}
	if _, err := backup.FirstBak(path); err != nil {
		return a, err
	}
	a.Backup = path + ".sysc.bak"
	a.BackupSHA256 = digest(data)
	return a, nil
}

func digest(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// Write installs the activation file, preserving the first foreign backup.
// The replacement is atomic: a crash leaves the previous file intact.
func Write(dataHome string) (stamp.Activation, error) {
	a, err := Plan(dataHome)
	if err != nil {
		return a, err
	}
	return a, writeAtomic(a.Path, Content())
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("activation file is not a regular file: %s", path)
	}
	return os.ReadFile(path)
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sysc-activation-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Delete removes the file Write placed, restoring a displaced foreign file
// byte-for-byte when one was backed up.
func Delete(a stamp.Activation) error {
	current, err := readRegular(a.Path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	missing := os.IsNotExist(err)
	if a.Backup != "" {
		if a.Backup != a.Path+".sysc.bak" {
			return fmt.Errorf("unexpected activation backup path: %s", a.Backup)
		}
		data, err := readRegular(a.Backup)
		if err != nil {
			if os.IsNotExist(err) && !missing && a.BackupSHA256 != "" && digest(current) == a.BackupSHA256 {
				return nil // Restoration completed before the stamp could be updated.
			}
			return err
		}
		if a.BackupSHA256 != "" && digest(data) != a.BackupSHA256 {
			return fmt.Errorf("activation backup changed after installation; preserving %s and %s; recover the original backup before retrying", a.Path, a.Backup)
		}
		if bytes.Equal(data, Content()) {
			return fmt.Errorf("activation backup contains SYSC's own file: %s", a.Backup)
		}
		if !missing && !bytes.Equal(current, Content()) && !bytes.Equal(current, data) {
			return fmt.Errorf("activation changed after installation; preserving %s and %s", a.Path, a.Backup)
		}
		if !bytes.Equal(current, data) || missing {
			if err := writeAtomic(a.Path, data); err != nil {
				return err
			}
		}
		if err := os.Remove(a.Backup); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if !missing && !bytes.Equal(current, Content()) {
		return fmt.Errorf("activation changed after installation; preserving %s", a.Path)
	}
	if err := os.Remove(a.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Install writes the activation file and asks the session bus to reload it.
func Install(dataHome string) (stamp.Activation, error) {
	a, err := Write(dataHome)
	if err != nil {
		return a, err
	}
	Reload()
	return a, nil
}

// Remove deletes the activation file and asks the session bus to reload.
func Remove(a stamp.Activation) error {
	if err := Delete(a); err != nil {
		return err
	}
	Reload()
	return nil
}

// Reload asks the session bus to reread activation files. It is best-effort
// and returns nothing: an SSH or TTY install has no session bus, and the file
// still lands for the next login.
func Reload() {
	_ = exec.Command("busctl", "--user", "call",
		"org.freedesktop.DBus", "/org/freedesktop/DBus",
		"org.freedesktop.DBus", "ReloadConfig").Run()
}

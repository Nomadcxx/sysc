// Package dbusact writes the user-level D-Bus activation file that makes
// sysc-notify the notifications daemon. The user data directory wins over
// /usr/share, so a packaged mako or dunst cannot answer the name first.
package dbusact

import (
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

// Write installs the activation file, backing up a foreign file first, and
// returns the stamp record. The write is atomic: a crash leaves the old file.
func Write(dataHome string) (stamp.Activation, error) {
	path := Path(dataHome)
	a := stamp.Activation{Path: path}
	if data, err := os.ReadFile(path); err == nil {
		if string(data) == string(Content()) {
			// Already ours (a re-run). Backing this up would make Delete
			// "restore" our own file and leave it behind after uninstall.
			return a, nil
		}
		created, err := backup.FirstBak(path)
		if err != nil {
			return a, err
		}
		if created {
			a.Backup = path + ".sysc.bak"
		}
	} else if !os.IsNotExist(err) {
		return a, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return a, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, Content(), 0o644); err != nil {
		os.Remove(tmp)
		return a, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return a, err
	}
	return a, nil
}

// Delete removes the file Write placed, restoring a displaced foreign file
// byte-for-byte when one was backed up.
func Delete(a stamp.Activation) error {
	if a.Backup != "" {
		data, err := os.ReadFile(a.Backup)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(a.Path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(a.Path, data, 0o644); err != nil {
			return err
		}
		if err := os.Remove(a.Backup); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
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

// Package units installs and orders the SYSC systemd user units.
package units

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Nomadcxx/sysc/internal/backup"
)

//go:embed templates/*.service
var templates embed.FS

// Unit is one embedded user unit.
type Unit struct {
	Name     string
	Template string
}

// All lists the units the installer can write.
var All = []Unit{
	{Name: "sysc-shell.service", Template: "templates/sysc-shell.service"},
	{Name: "sysc-clipboard.service", Template: "templates/sysc-clipboard.service"},
	{Name: "sysc-walls.service", Template: "templates/sysc-walls.service"},
	{Name: "sysc-notify.service", Template: "templates/sysc-notify.service"},
	{Name: "sysc-tray.service", Template: "templates/sysc-tray.service"},
}

// StopOrder stops the shell first: it owns the wallpaper children, and the
// companions last so notifications and tray icons stay up while the rest goes.
var StopOrder = []string{
	"sysc-shell.service",
	"sysc-walls.service",
	"sysc-clipboard.service",
	"sysc-tray.service",
	"sysc-notify.service",
}

// StartOrder starts dependencies before the shell; the notification daemon
// comes first so it owns the bus name before any client needs it.
var StartOrder = []string{
	"sysc-notify.service",
	"sysc-tray.service",
	"sysc-clipboard.service",
	"sysc-walls.service",
	"sysc-shell.service",
}

// Content returns the embedded unit file.
func Content(u Unit) ([]byte, error) {
	return templates.ReadFile(u.Template)
}

// Write installs the unit into dir, backing up a foreign unit first.
func Write(dir string, u Unit) error {
	data, err := Content(u)
	if err != nil {
		return err
	}
	dst := filepath.Join(dir, u.Name)
	if old, err := os.ReadFile(dst); err == nil && !bytes.Equal(old, data) {
		if _, err := backup.FirstBak(dst); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// StopAll stops every unit in StopOrder. run is e.g. systemctl --user.
func StopAll(run func(args ...string) error) error {
	for _, name := range StopOrder {
		if err := run("stop", name); err != nil {
			return err
		}
	}
	return nil
}

// StartAll starts every unit in StartOrder.
func StartAll(run func(args ...string) error) error {
	for _, name := range StartOrder {
		if err := run("start", name); err != nil {
			return err
		}
	}
	return nil
}

// GraphicalSessionActive reports whether the user session target is up.
func GraphicalSessionActive(run func(args ...string) error) bool {
	return run("is-active", "--quiet", "graphical-session.target") == nil
}

// ActiveUnits reports which of StopOrder's units are currently active.
func ActiveUnits(run func(args ...string) error) []string {
	var active []string
	for _, name := range StopOrder {
		if run("is-active", "--quiet", name) == nil {
			active = append(active, name)
		}
	}
	return active
}

// StartUnits starts the given units in StartOrder, attempting every unit
// and joining the errors so one failure cannot leave the rest stopped.
func StartUnits(run func(args ...string) error, names []string) error {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var errs []error
	for _, name := range StartOrder {
		if !want[name] {
			continue
		}
		if err := run("start", name); err != nil {
			errs = append(errs, fmt.Errorf("start %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

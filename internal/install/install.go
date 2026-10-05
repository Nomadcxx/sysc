// Package install runs the SYSC suite install and uninstall from answers.
package install

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/niri"
	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/seed"
	"github.com/Nomadcxx/sysc/internal/stamp"
	"github.com/Nomadcxx/sysc/internal/units"
)

// Status is the outcome of one task-list row.
type Status string

const (
	Done    Status = "done"
	Skipped Status = "skipped"
	Failed  Status = "failed"
)

// Task is one row of the install task list.
type Task struct {
	Name   string
	Status Status
	Reason string
}

// Options carries the answers and the injectable system seams.
type Options struct {
	Home           string
	Pin            pin.Pin
	Answers        seed.Answers
	Yes            bool
	Purge          bool
	RemoveGSlapper bool

	Client     *http.Client
	Download   func(ctx context.Context, staging string, assets []fetch.Asset) error
	Swap       func(binDir, staging string, names []string) error
	LookPath   func(string) (string, error)
	InstallPkg func(pkg string) error
	RemovePkg  func(pkg string) error
	Systemctl  func(args ...string) error
	Now        time.Time

	// Arch overrides the host architecture check (empty = runtime.GOARCH).
	Arch string
	// Loc enables localized refusal messages when set.
	Loc *i18n.Locale
}

// Result is the task list and the stamp written.
type Result struct {
	Tasks []Task
	Stamp stamp.Stamp
}

// xdgConfig mirrors os.UserConfigDir; the sysc-shell binary reads its config
// from there, so the installer must write it to the same place (AUD-05).
func (o Options) xdgConfig() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(o.Home, ".config")
}

func (o Options) xdgState() string {
	if v := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(o.Home, ".local", "state")
}

// binDir stays on ~/.local/bin: it is on PATH by convention, not via XDG.
func (o Options) binDir() string  { return filepath.Join(o.Home, ".local", "bin") }
func (o Options) unitDir() string { return filepath.Join(o.xdgConfig(), "systemd", "user") }
func (o Options) configPath() string {
	return filepath.Join(o.xdgConfig(), "sysc-shell", "config.json")
}
func (o Options) stateDir() string { return filepath.Join(o.xdgState(), "sysc") }
func (o Options) niriDir() string  { return filepath.Join(o.xdgConfig(), "niri") }

func (o Options) systemctl() func(args ...string) error {
	if o.Systemctl != nil {
		return o.Systemctl
	}
	return func(args ...string) error {
		return exec.Command("systemctl", append([]string{"--user"}, args...)...).Run()
	}
}

func unitFor(id string) (units.Unit, bool) {
	for _, u := range units.All {
		if strings.TrimSuffix(u.Name, ".service") == id {
			return u, true
		}
	}
	return units.Unit{}, false
}

// Run installs the pin: fetch and swap binaries, gSlapper, units, seed, niri,
// start, stamp.
func Run(ctx context.Context, opts Options) (Result, error) {
	var res Result
	if opts.Answers.Location == "" || (opts.Answers.Latitude == 0 && opts.Answers.Longitude == 0) {
		if opts.Loc != nil {
			return res, errors.New(i18n.T(*opts.Loc, "refuse.weather"))
		}
		return res, fmt.Errorf("weather coordinates are required")
	}
	// ponytail: sysc-shell caps weather labels at 80 bytes; truncate to the
	// longest valid UTF-8 prefix instead of failing the install on a long name.
	if len(opts.Answers.Location) > 80 {
		loc := opts.Answers.Location[:80]
		for !utf8.ValidString(loc) {
			loc = loc[:len(loc)-1]
		}
		opts.Answers.Location = loc
	}
	// The pin only carries amd64 assets; installing them on another host
	// architecture would swap silently broken binaries (AUD-08).
	arch := opts.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}
	if arch != "amd64" {
		return res, fmt.Errorf("unsupported host architecture: %s (installer only has amd64 assets)", arch)
	}
	download := opts.Download
	if download == nil {
		download = func(ctx context.Context, staging string, assets []fetch.Asset) error {
			return fetch.DownloadAll(ctx, opts.Client, staging, assets)
		}
	}
	swap := opts.Swap
	if swap == nil {
		swap = fetch.SwapAll
	}
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	systemctl := opts.systemctl()

	staging := filepath.Join(opts.stateDir(), "staging")
	os.RemoveAll(staging)

	// Fail fast before touching the system if an enabled component has no
	// matching unit (AUD-14).
	enabled := []pin.Component{}
	for _, c := range opts.Pin.Components {
		if c.Disabled {
			continue
		}
		if _, ok := unitFor(c.ID); !ok {
			return res, fmt.Errorf("%s: no matching SYSC unit", c.ID)
		}
		enabled = append(enabled, c)
	}

	// Download and verify everything first, then swap once: a late download
	// failure must never leave earlier components already swapped (AUD-10).
	// Rows keep component order; enabled rows flip to done only after swap.
	rows := make([]Task, 0, len(opts.Pin.Components))
	var allNames []string
	for _, c := range opts.Pin.Components {
		if c.Disabled {
			rows = append(rows, Task{Name: c.ID, Status: Skipped, Reason: c.Reason})
			continue
		}
		rows = append(rows, Task{Name: c.ID})
		var assets []fetch.Asset
		for _, b := range c.Binaries {
			a, ok := b.Assets["amd64"]
			if !ok {
				return res, fmt.Errorf("%s: no amd64 asset", c.ID)
			}
			assets = append(assets, fetch.Asset{Name: b.Name, URL: a.URL, SHA256: a.SHA256})
			allNames = append(allNames, b.Name)
		}
		if err := download(ctx, staging, assets); err != nil {
			return res, fmt.Errorf("%s: %w", c.ID, err)
		}
	}
	if len(allNames) > 0 {
		// Stop user units before swapping so an upgrade actually loads the
		// new binaries instead of keeping running ones (AUD-07). First
		// installs have nothing running; errors are not fatal.
		_ = units.StopAll(systemctl)
		if err := swap(opts.binDir(), staging, allNames); err != nil {
			return res, err
		}
		for i := range rows {
			if rows[i].Status == "" {
				rows[i].Status = Done
			}
		}
	}
	res.Tasks = append(res.Tasks, rows...)

	gslapperInstalled := false
	if _, err := lookPath("gslapper"); err == nil {
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped, Reason: "already on PATH"})
	} else if opts.InstallPkg == nil {
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped,
			Reason: fmt.Sprintf("no AUR helper found; install gSlapper with yay or paru (package %q)", opts.Pin.GSlapper.Package)})
	} else if err := opts.InstallPkg(opts.Pin.GSlapper.Package); err != nil {
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped, Reason: err.Error()})
	} else {
		gslapperInstalled = true
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Done})
	}

	for _, c := range enabled {
		u, _ := unitFor(c.ID)
		if err := units.Write(opts.unitDir(), u); err != nil {
			return res, err
		}
		if err := systemctl("enable", u.Name); err != nil {
			return res, fmt.Errorf("enable %s: %w", u.Name, err)
		}
	}

	if err := seed.Write(opts.configPath(), opts.Answers); err != nil {
		return res, err
	}

	if _, err := niri.Apply(niri.Options{
		ConfigPath:  filepath.Join(opts.niriDir(), "config.kdl"),
		SidecarPath: filepath.Join(opts.niriDir(), "sysc.kdl"),
		StateDir:    filepath.Join(opts.stateDir(), "backups"),
		Binds:       niri.DefaultBinds,
		Now:         opts.Now,
	}); err != nil {
		return res, err
	}

	st := stamp.Stamp{
		Release:           opts.Pin.Release,
		Components:        map[string]string{},
		GSlapperInstalled: gslapperInstalled,
	}
	for _, c := range enabled {
		st.Components[c.ID] = c.Tag
	}

	started := false
	if units.GraphicalSessionActive(systemctl) {
		if err := units.StartAll(systemctl); err != nil {
			// Persist the install state even when starting fails, so
			// Uninstall can find and roll back this installation (AUD-03).
			st.Started = false
			if werr := stamp.Write(opts.stateDir(), st, true); werr == nil {
				res.Stamp = st
			}
			return res, err
		}
		started = true
	}
	st.Started = started
	if err := stamp.Write(opts.stateDir(), st, true); err != nil {
		return res, err
	}
	res.Stamp = st
	return res, nil
}

// Uninstall removes what the stamp says SYSC installed, then keeps or purges
// the XDG config.
func Uninstall(opts Options) (Result, error) {
	var res Result
	st, err := stamp.Read(opts.stateDir())
	if err != nil {
		return res, fmt.Errorf("no SYSC installation found: %w", err)
	}
	systemctl := opts.systemctl()
	units.StopAll(systemctl)

	for _, c := range opts.Pin.Components {
		if _, ok := st.Components[c.ID]; !ok {
			continue
		}
		for _, b := range c.Binaries {
			os.Remove(filepath.Join(opts.binDir(), b.Name))
		}
		if u, ok := unitFor(c.ID); ok {
			// Disable first: removing the unit file alone leaves dangling
			// wants symlinks under graphical-session.target.wants (AUD-09).
			_ = systemctl("disable", u.Name)
			os.Remove(filepath.Join(opts.unitDir(), u.Name))
		}
		res.Tasks = append(res.Tasks, Task{Name: c.ID, Status: Done})
	}
	_ = systemctl("daemon-reload")

	if st.GSlapperInstalled && opts.RemoveGSlapper && opts.RemovePkg != nil {
		if err := opts.RemovePkg(opts.Pin.GSlapper.Package); err != nil {
			res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Failed, Reason: err.Error()})
		} else {
			res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Done})
		}
	} else {
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped, Reason: "not installed by SYSC"})
	}

	if err := niri.Remove(niri.Options{
		ConfigPath:  filepath.Join(opts.niriDir(), "config.kdl"),
		SidecarPath: filepath.Join(opts.niriDir(), "sysc.kdl"),
	}); err != nil {
		return res, err
	}

	if opts.Purge {
		os.RemoveAll(filepath.Dir(opts.configPath()))
	}
	os.Remove(filepath.Join(opts.stateDir(), stamp.FileName))
	return res, nil
}

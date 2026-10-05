// Package install runs the SYSC suite install and uninstall from answers.
package install

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nomadcxx/sysc/internal/fetch"
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
}

// Result is the task list and the stamp written.
type Result struct {
	Tasks []Task
	Stamp stamp.Stamp
}

func (o Options) binDir() string  { return filepath.Join(o.Home, ".local", "bin") }
func (o Options) unitDir() string { return filepath.Join(o.Home, ".config", "systemd", "user") }
func (o Options) configPath() string {
	return filepath.Join(o.Home, ".config", "sysc-shell", "config.json")
}
func (o Options) stateDir() string { return filepath.Join(o.Home, ".local", "state", "sysc") }
func (o Options) niriDir() string  { return filepath.Join(o.Home, ".config", "niri") }

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
		return res, fmt.Errorf("weather coordinates are required")
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

	for _, c := range opts.Pin.Components {
		if c.Disabled {
			res.Tasks = append(res.Tasks, Task{Name: c.ID, Status: Skipped, Reason: c.Reason})
			continue
		}
		var assets []fetch.Asset
		var names []string
		for _, b := range c.Binaries {
			a, ok := b.Assets["amd64"]
			if !ok {
				return res, fmt.Errorf("%s: no amd64 asset", c.ID)
			}
			assets = append(assets, fetch.Asset{Name: b.Name, URL: a.URL, SHA256: a.SHA256})
			names = append(names, b.Name)
		}
		if err := download(ctx, staging, assets); err != nil {
			return res, fmt.Errorf("%s: %w", c.ID, err)
		}
		if err := swap(opts.binDir(), staging, names); err != nil {
			return res, fmt.Errorf("%s: %w", c.ID, err)
		}
		res.Tasks = append(res.Tasks, Task{Name: c.ID, Status: Done})
	}

	gslapperInstalled := false
	if _, err := lookPath("gslapper"); err == nil {
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped, Reason: "already on PATH"})
	} else if opts.InstallPkg == nil {
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped, Reason: "no AUR helper found"})
	} else if err := opts.InstallPkg(opts.Pin.GSlapper.Package); err != nil {
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped, Reason: err.Error()})
	} else {
		gslapperInstalled = true
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Done})
	}

	for _, c := range opts.Pin.Components {
		if c.Disabled {
			continue
		}
		u, ok := unitFor(c.ID)
		if !ok {
			continue
		}
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

	started := false
	if units.GraphicalSessionActive(systemctl) {
		if err := units.StartAll(systemctl); err != nil {
			return res, err
		}
		started = true
	}

	st := stamp.Stamp{
		Release:           opts.Pin.Release,
		Components:        map[string]string{},
		GSlapperInstalled: gslapperInstalled,
		Started:           started,
	}
	for _, c := range opts.Pin.Components {
		if c.Disabled {
			continue
		}
		st.Components[c.ID] = c.Tag
	}
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
			os.Remove(filepath.Join(opts.unitDir(), u.Name))
		}
		res.Tasks = append(res.Tasks, Task{Name: c.ID, Status: Done})
	}

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
		os.RemoveAll(filepath.Join(opts.Home, ".config", "sysc-shell"))
	}
	os.Remove(filepath.Join(opts.stateDir(), stamp.FileName))
	return res, nil
}

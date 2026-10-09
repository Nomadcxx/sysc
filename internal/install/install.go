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
	"time"
	"unicode/utf8"

	"github.com/Nomadcxx/sysc/internal/conflict"
	"github.com/Nomadcxx/sysc/internal/dbusact"
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

	// Progress receives a detached snapshot of the task list every time it
	// changes: pending/skipped rows after download, done after swap, the
	// gSlapper row, and the final list. Nil disables progress reporting.
	Progress func(tasks []Task)

	// Arch overrides the host architecture check (empty = runtime.GOARCH).
	Arch string
	// Loc enables localized refusal messages when set.
	Loc *i18n.Locale
	// Findings and Conflicts are the detected competing providers and the
	// user's choice for each; empty means nothing to hand over.
	Findings  []conflict.Finding
	Conflicts map[string]conflict.Choice
	// ConflictEnv overrides conflict.Apply's system seams (tests).
	ConflictEnv conflict.Env
	// InNiriSession is true when WAYLAND_DISPLAY and NIRI_SOCKET are both
	// set. Combined with an inactive graphical-session.target, the installer
	// must not comment out the user's sysc-shell autostart. SSH and other
	// no-Wayland runs leave this false and still enable units without starting.
	InNiriSession bool
}

// Result is the task list and the stamp written.
type Result struct {
	Tasks []Task
	Stamp stamp.Stamp
	// SessionWarning is set when a live niri has no graphical-session.target.
	// Empty for SSH/TTY enable-only installs and for a real niri-session.
	SessionWarning string
	// Warnings are non-fatal conflict notices (kept providers, failed
	// handovers, activation file problems) for the final summary.
	Warnings []string
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

// xdgData is where the D-Bus activation file lives. The D-Bus spec gives this
// directory priority over /usr/share, so a packaged mako cannot win the name.
func (o Options) xdgData() string {
	if v := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(o.Home, ".local", "share")
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

func unitFor(c pin.Component) (units.Unit, bool) {
	name := c.Unit
	if name == "" {
		name = c.ID + ".service"
	}
	for _, u := range units.All {
		if u.Name == name {
			return u, true
		}
	}
	return units.Unit{}, false
}

func componentEnabled(enabled []pin.Component, id string) bool {
	for _, c := range enabled {
		if !c.Disabled && c.ID == id {
			return true
		}
	}
	return false
}

// mergeHandover replaces the record kept for a component so re-applying a
// handover does not stack, without forgetting what only the earlier record
// knew: a commented spawn line is no longer detected, so dropping it would
// leave the marker in the config forever, and a provider whose unit is now
// off was still enabled when we first took it over.
func mergeHandover(hs []stamp.Handover, h stamp.Handover) []stamp.Handover {
	var out []stamp.Handover
	for _, prev := range hs {
		if prev.Name != h.Name {
			out = append(out, prev)
			continue
		}
		if h.Unit == "" && prev.Unit != "" {
			h.Unit, h.UnitWasEnabled = prev.Unit, prev.UnitWasEnabled
		}
		for _, old := range prev.Lines {
			found := false
			for _, line := range h.Lines {
				if line.Commented == old.Commented {
					found = true
					break
				}
			}
			if !found {
				h.Lines = append(h.Lines, old)
			}
		}
		continue
	}
	return append(out, h)
}

// Run installs the pin: fetch and swap binaries, gSlapper, units, seed, niri,
// start, stamp.
func Run(ctx context.Context, opts Options) (res Result, err error) {
	if componentEnabled(opts.Pin.Components, "sysc-lock") {
		opts.Answers.Locker = "sysc-lock"
	}
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
	// Coordinate range (and the shell's other seed rules) must fail before any
	// download, stop, or swap. seed.Write only reports them when no config
	// file exists yet, which is after the binaries are already in place.
	if _, err := seed.ConfigJSON(opts.Answers); err != nil {
		return res, err
	}
	// A zero Now (production callers leave it unset) would stamp every state
	// copy 00010101T000000Z and make rotation delete the newest copy (#37).
	if opts.Now.IsZero() {
		opts.Now = time.Now()
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
	// niri.Apply and seed.Write are otherwise the first time these paths are
	// checked, which is after binaries are swapped and units are enabled.
	if err := requireNiriConfig(filepath.Join(opts.niriDir(), "config.kdl")); err != nil {
		return res, err
	}
	if err := requireSeedTarget(opts.configPath()); err != nil {
		return res, err
	}
	download := opts.Download
	if download == nil {
		client := opts.Client
		if client == nil {
			client = fetch.NewClient()
		}
		download = func(ctx context.Context, staging string, assets []fetch.Asset) error {
			return fetch.DownloadAll(ctx, client, staging, assets)
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
	// Resolve gslapper once: the stamp must remember whether SYSC owns the
	// installed package across re-runs (#36).
	_, gslapperErr := lookPath("gslapper")
	gslapperOnPath := gslapperErr == nil
	prev, prevErr := stamp.Read(opts.stateDir())

	staging := filepath.Join(opts.stateDir(), "staging")

	// Fail fast before touching the system if an enabled component has no
	// matching unit (AUD-14) or its unit template cannot be read.
	enabled := []pin.Component{}
	unitNames := []string{}
	for _, c := range opts.Pin.Components {
		if c.Disabled {
			continue
		}
		u, ok := unitFor(c)
		if !ok {
			return res, fmt.Errorf("%s: no matching SYSC unit", c.ID)
		}
		if _, err := units.Content(u); err != nil {
			return res, err
		}
		enabled = append(enabled, c)
		unitNames = append(unitNames, u.Name)
	}
	// A SkipSYSC conflict choice leaves the competing SYSC component neither
	// enabled nor started; its binary still installs like any other.
	loc := i18n.EN
	if opts.Loc != nil {
		loc = *opts.Loc
	}
	skipComponent := map[string]bool{}
	for _, f := range opts.Findings {
		if opts.Conflicts[f.Name] == conflict.SkipSYSC {
			skipComponent[f.Kind.Component()] = true
		}
	}
	os.RemoveAll(staging)

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
	emit := func(tasks []Task) {
		if opts.Progress != nil {
			opts.Progress(append([]Task(nil), tasks...))
		}
	}
	emit(rows)
	st := stamp.Stamp{
		Release:           opts.Pin.Release,
		Components:        map[string]string{},
		GSlapperInstalled: prevErr == nil && prev.GSlapperInstalled && gslapperOnPath,
	}
	if prevErr == nil {
		// Re-runs keep what earlier runs handed over or wrote, so uninstall
		// can still reverse it.
		st.HandedOver = prev.HandedOver
		st.Activation = prev.Activation
	}
	for _, c := range enabled {
		st.Components[c.ID] = c.Tag
	}
	// persist records the install as soon as it can be undone, then again as
	// later steps finish (gSlapper, units, niri, start).
	persist := func() error {
		if err := stamp.Write(opts.stateDir(), st, true); err != nil {
			return err
		}
		res.Stamp = st
		return nil
	}

	// If anything below fails after the units were stopped, start the ones
	// that were running again so a failed upgrade does not leave the user
	// without a bar until the next login (#39).
	restarted := false
	var wasActive []string
	defer func() {
		if err == nil || restarted || len(wasActive) == 0 {
			return
		}
		if rerr := units.StartUnits(systemctl, wasActive); rerr != nil {
			err = fmt.Errorf("%w (restarting previously running units: %v)", err, rerr)
		}
	}()

	// The stamp has to exist from the first change uninstall is responsible
	// for. That is the swap when there are binaries (a failed swap is rolled
	// back inside SwapAll and does not get here). With no binaries, seed and
	// niri are the first writes, so record those before they run.
	if len(allNames) > 0 {
		// Stop user units before swapping so an upgrade actually loads the
		// new binaries instead of keeping running ones (AUD-07). First
		// installs have nothing running; errors are not fatal.
		wasActive = units.ActiveUnits(systemctl, unitNames...)
		_ = units.StopUnits(systemctl, unitNames)
		if err := swap(opts.binDir(), staging, allNames); err != nil {
			return res, err
		}
		for i := range rows {
			if rows[i].Status == "" {
				rows[i].Status = Done
			}
		}
		// Swap is the first change uninstall must be able to see. Write the
		// stamp before gSlapper, unit enable, seed, or niri — any of those
		// can fail or the process can be interrupted, and a missing stamp
		// makes uninstall report that nothing is installed.
		if err := persist(); err != nil {
			if rerr := fetch.RestoreBackups(opts.binDir(), allNames); rerr != nil {
				return res, fmt.Errorf("recording install: %w (rollback: %v)", err, rerr)
			}
			return res, err
		}
	} else if err := persist(); err != nil {
		return res, err
	}
	res.Tasks = append(res.Tasks, rows...)
	emit(res.Tasks)

	if gslapperOnPath {
		reason := "already on PATH"
		if st.GSlapperInstalled {
			reason = "already installed by SYSC"
		}
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped, Reason: reason})
	} else if opts.InstallPkg == nil {
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped,
			Reason: fmt.Sprintf("no package installer configured for gSlapper (package %q)", opts.Pin.GSlapper.Package)})
	} else if err := opts.InstallPkg(opts.Pin.GSlapper.Package); err != nil {
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped, Reason: err.Error()})
	} else {
		st.GSlapperInstalled = true
		if err := persist(); err != nil {
			return res, err
		}
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Done})
	}
	emit(res.Tasks)

	startNames := []string{}
	for _, c := range enabled {
		if skipComponent[c.ID] {
			continue
		}
		u, _ := unitFor(c)
		if err := units.Write(opts.unitDir(), u); err != nil {
			return res, err
		}
		if err := systemctl("enable", u.Name); err != nil {
			return res, fmt.Errorf("enable %s: %w", u.Name, err)
		}
		startNames = append(startNames, u.Name)
	}
	if len(enabled) > 0 {
		if err := persist(); err != nil {
			return res, err
		}
	}

	if err := seed.Write(opts.configPath(), opts.Answers); err != nil {
		return res, err
	}

	// A plain `niri` (not niri-session) never activates graphical-session.target.
	// The user units Requisite= that target, so commenting out spawn-at-startup
	// here would leave the next login with no bar. SSH has no live session and
	// still takes the enable-only path below.
	sessionUp := units.GraphicalSessionActive(systemctl)
	keepSpawn := opts.InNiriSession && !sessionUp
	if _, err := niri.Apply(niri.Options{
		ConfigPath:  filepath.Join(opts.niriDir(), "config.kdl"),
		SidecarPath: filepath.Join(opts.niriDir(), "sysc.kdl"),
		StateDir:    filepath.Join(opts.stateDir(), "backups"),
		Binds:       niri.DefaultBinds,
		Now:         opts.Now,
		KeepSpawn:   keepSpawn,
	}); err != nil {
		return res, err
	}
	if keepSpawn {
		loc := i18n.EN
		if opts.Loc != nil {
			loc = *opts.Loc
		}
		res.SessionWarning = i18n.T(loc, "warn.niri_session")
	}

	if err := persist(); err != nil {
		return res, err
	}

	for _, f := range opts.Findings {
		row := Task{Name: "conflict:" + f.Name}
		switch opts.Conflicts[f.Name] {
		case conflict.HandOver:
			_, err := conflict.Apply(opts.ConflictEnv, f, conflict.HandOver, func(h stamp.Handover) error {
				st.HandedOver = mergeHandover(st.HandedOver, h)
				return persist()
			})
			if err != nil {
				row.Status, row.Reason = Failed, err.Error()
				res.Warnings = append(res.Warnings, fmt.Sprintf(i18n.T(loc, "warn.conflict_failed"), f.Name))
			} else {
				row.Status = Done
			}
		case conflict.SkipSYSC:
			row.Status, row.Reason = Skipped, "SYSC component not enabled"
		default:
			row.Status, row.Reason = Skipped, "kept both"
			if opts.Yes {
				res.Warnings = append(res.Warnings, fmt.Sprintf(i18n.T(loc, "warn.conflict_kept"), f.Name))
			}
		}
		res.Tasks = append(res.Tasks, row)
	}
	emit(res.Tasks)

	if !skipComponent[conflict.KindNotifications.Component()] &&
		componentEnabled(enabled, conflict.KindNotifications.Component()) {
		act, aerr := dbusact.Install(opts.xdgData())
		if aerr != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf(i18n.T(loc, "conflicts.dbus_failed"), dbusact.Path(opts.xdgData())))
		} else {
			st.Activation = &act
			if err := persist(); err != nil {
				return res, err
			}
		}
	}

	started := false
	restarted = true
	if sessionUp {
		if err := units.StartUnits(systemctl, startNames); err != nil {
			// The stamp from the swap already makes this recoverable (AUD-03).
			// Refresh started=false; a failed rewrite leaves the earlier stamp.
			st.Started = false
			_ = persist()
			return res, err
		}
		started = true
	}
	st.Started = started
	if err := persist(); err != nil {
		return res, err
	}
	emit(res.Tasks)
	return res, nil
}

// requireNiriConfig rejects a missing or non-regular niri config before the
// install edits anything else. The error matches niri.Apply.
func requireNiriConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("niri config not found at %s", path)
		}
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("niri config not found at %s", path)
	}
	return nil
}

// requireSeedTarget rejects a directory sitting on the shell config path.
// seed.Write would fail the same way, but only after the swap.
func requireSeedTarget(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.IsDir() {
		return fmt.Errorf("seed %s: path exists and is a directory", path)
	}
	return nil
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
	var ownedUnits []string
	for _, c := range opts.Pin.Components {
		if _, owned := st.Components[c.ID]; owned {
			if u, ok := unitFor(c); ok {
				ownedUnits = append(ownedUnits, u.Name)
			}
		}
	}
	_ = units.StopUnits(systemctl, ownedUnits)

	for i := len(st.HandedOver) - 1; i >= 0; i-- {
		h := st.HandedOver[i]
		row := Task{Name: "conflict:" + h.Name}
		var errs []error
		if h.Unit != "" && h.UnitWasEnabled {
			// enable --now: the handover stopped a running provider, so
			// restoring it means running again, not only enabled.
			if err := systemctl("enable", "--now", h.Unit); err != nil {
				errs = append(errs, err)
			}
		}
		for _, hl := range h.Lines {
			if err := niri.RestoreLine(hl); err != nil {
				errs = append(errs, err)
			}
		}
		if err := errors.Join(errs...); err != nil {
			row.Status, row.Reason = Failed, err.Error()
		} else {
			row.Status = Done
		}
		res.Tasks = append(res.Tasks, row)
	}
	if st.Activation != nil {
		row := Task{Name: "dbus-activation"}
		if err := dbusact.Remove(*st.Activation); err != nil {
			row.Status, row.Reason = Failed, err.Error()
		} else {
			row.Status = Done
		}
		res.Tasks = append(res.Tasks, row)
	}

	for _, c := range opts.Pin.Components {
		if _, ok := st.Components[c.ID]; !ok {
			continue
		}
		for _, b := range c.Binaries {
			os.Remove(filepath.Join(opts.binDir(), b.Name))
			// Swap keeps <name>.bak next to the binary it replaced.
			os.Remove(filepath.Join(opts.binDir(), b.Name+".bak"))
		}
		if u, ok := unitFor(c); ok {
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

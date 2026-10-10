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

	Client         *http.Client
	Download       func(ctx context.Context, staging string, assets []fetch.Asset) error
	Swap           func(binDir, staging string, names []string) error
	LookPath       func(string) (string, error)
	InstallPkg     func(pkg string) error
	RemovePkg      func(pkg string) error
	Systemctl      func(args ...string) error
	Now            time.Time
	CheckRuntime   func(staging string, names []string) ([]string, error)
	CheckOwnership func(home string, components []pin.Component) error

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
	if c.BinaryOnly {
		return units.Unit{}, false
	}
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

// idleLock matches the sysc-shell Settings default for When idle.
const idleLock = 5 * time.Minute

// Run installs the pin: fetch and swap binaries, gSlapper, units, seed, niri,
// start, stamp.
func Run(ctx context.Context, opts Options) (res Result, err error) {
	if componentEnabled(opts.Pin.Components, "sysc-lock") {
		opts.Answers.Locker = "sysc-lock"
		// The shell's When idle setting is lock or screensaver, not both: an
		// installed sysc-walls screensaver keeps idle, and lock stays manual
		// and before sleep.
		if !componentEnabled(opts.Pin.Components, "sysc-walls") {
			opts.Answers.IdleLock = idleLock
		}
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
	lock, err := lockState(opts.stateDir())
	if err != nil {
		return res, err
	}
	defer lock.Close()
	if opts.CheckOwnership != nil {
		if err := opts.CheckOwnership(opts.Home, opts.Pin.Components); err != nil {
			return res, err
		}
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
	if prevErr != nil && !os.IsNotExist(prevErr) {
		return res, fmt.Errorf("read existing installation: %w", prevErr)
	}
	if prevErr == nil && prev.Files == nil {
		prev.Files, err = legacyFiles(opts, prev, true)
		if err != nil {
			return res, err
		}
	}

	staging := filepath.Join(opts.stateDir(), "staging")

	// Fail fast before touching the system if an enabled component has no
	// matching unit (AUD-14) or its unit template cannot be read.
	enabled := []pin.Component{}
	unitNames := []string{}
	for _, c := range opts.Pin.Components {
		if c.Disabled {
			continue
		}
		enabled = append(enabled, c)
		if c.BinaryOnly {
			continue
		}
		u, ok := unitFor(c)
		if !ok {
			return res, fmt.Errorf("%s: no matching SYSC unit", c.ID)
		}
		if _, err := units.Content(u); err != nil {
			return res, err
		}
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
	if err := os.RemoveAll(staging); err != nil {
		return res, err
	}
	opts.Answers.Tray = componentEnabled(enabled, "sysc-tray") && !skipComponent["sysc-tray"]

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
	if opts.CheckRuntime != nil {
		warnings, err := opts.CheckRuntime(staging, allNames)
		if err != nil {
			return res, err
		}
		res.Warnings = append(res.Warnings, warnings...)
	}
	st := stamp.Stamp{
		Release:           opts.Pin.Release,
		Components:        map[string]string{},
		Files:             []stamp.File{},
		GSlapperInstalled: prevErr == nil && prev.GSlapperInstalled && gslapperOnPath,
	}
	if prevErr == nil {
		// Re-runs keep what earlier runs handed over or wrote, so uninstall
		// can still reverse it.
		st.HandedOver = prev.HandedOver
		st.Activation = prev.Activation
		st.Files = append(st.Files, prev.Files...)
		for id, tag := range prev.Components {
			st.Components[id] = tag
		}
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

	// Record the intended bytes before replacing files. PreviousSHA256 allows
	// uninstall to recover an interruption before the replacement completed.
	unitNames = nil
	for _, c := range enabled {
		if skipComponent[c.ID] || c.BinaryOnly {
			continue
		}
		u, _ := unitFor(c)
		data, err := units.Content(u)
		if err != nil {
			return res, err
		}
		if err := planFile(&st, filepath.Join(opts.unitDir(), u.Name), data, u.Name, systemctl); err != nil {
			return res, err
		}
		unitNames = append(unitNames, u.Name)
	}
	for _, name := range allNames {
		data, err := regularData(filepath.Join(staging, name))
		if os.IsNotExist(err) && opts.Swap != nil {
			continue
		} // Injected swap owns its test files.
		if err != nil {
			return res, err
		}
		path := filepath.Join(opts.binDir(), name)
		if _, err := regularData(path); err == nil {
			bak, bakErr := regularData(path + ".bak")
			if bakErr == nil {
				trusted := false
				for _, f := range prev.Files {
					trusted = trusted || (f.Path == path && f.RollbackSHA256 == fileHash(bak))
				}
				if !trusted {
					return res, fmt.Errorf("preserving existing rollback backup %s.bak; move it aside before rerunning", path)
				}
				if err := os.Remove(path + ".bak"); err != nil {
					return res, err
				}
			} else if !os.IsNotExist(bakErr) {
				return res, bakErr
			}
		}
		if err := planFile(&st, path, data, "", systemctl); err != nil {
			return res, err
		}
	}
	if len(allNames) > 0 {
		wasActive = units.ActiveUnits(systemctl, unitNames...)
		if err := units.StopUnits(systemctl, wasActive); err != nil {
			return res, err
		}
		if err := persist(); err != nil {
			return res, err
		}
		if err := swap(opts.binDir(), staging, allNames); err != nil {
			// SwapAll rolls back on failure; restore the previous ownership record.
			var recordErr error
			if prevErr == nil {
				recordErr = stamp.Write(opts.stateDir(), prev, true)
			} else {
				recordErr = os.Remove(filepath.Join(opts.stateDir(), stamp.FileName))
			}
			return res, errors.Join(err, recordErr)
		}
		for i := range st.Files {
			for _, name := range allNames {
				if st.Files[i].Path == filepath.Join(opts.binDir(), name) {
					st.Files[i].PreviousSHA256 = ""
				}
			}
		}
		for i := range rows {
			if rows[i].Status == "" {
				rows[i].Status = Done
			}
		}
	}
	if err := persist(); err != nil {
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
		if skipComponent[c.ID] || c.BinaryOnly {
			continue
		}
		u, _ := unitFor(c)
		if err := units.Write(opts.unitDir(), u); err != nil {
			return res, err
		}
		for i := range st.Files {
			if st.Files[i].Unit == u.Name {
				st.Files[i].PreviousSHA256 = ""
			}
		}
		if err := persist(); err != nil {
			return res, err
		}
		if err := systemctl("daemon-reload"); err != nil {
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
		act, aerr := dbusact.Plan(opts.xdgData())
		if aerr == nil && st.Activation != nil && st.Activation.BackupSHA256 != "" &&
			(st.Activation.Backup != act.Backup || st.Activation.BackupSHA256 != act.BackupSHA256) {
			aerr = errors.New("notification activation backup changed; preserving the original recovery record")
		}
		if aerr == nil {
			st.Activation = &act
			if err := persist(); err != nil {
				return res, err
			}
			_, aerr = dbusact.Install(opts.xdgData())
		}
		if aerr != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf(i18n.T(loc, "conflicts.dbus_failed"), dbusact.Path(opts.xdgData()))+": "+aerr.Error())
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
	lock, err := lockState(opts.stateDir())
	if err != nil {
		return res, err
	}
	defer lock.Close()
	st, err := stamp.Read(opts.stateDir())
	if err != nil {
		return res, fmt.Errorf("no SYSC installation found: %w", err)
	}
	if st.Files == nil {
		st.Files, err = legacyFiles(opts, st, false)
		if err != nil {
			return res, err
		}
	}
	persist := func() error {
		res.Stamp = st
		return stamp.Write(opts.stateDir(), st, true)
	}
	if err := persist(); err != nil {
		return res, err
	}
	var failures []error
	record := func(name string, err error) {
		row := Task{Name: name, Status: Done}
		if err != nil {
			row.Status, row.Reason = Failed, err.Error()
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
		}
		res.Tasks = append(res.Tasks, row)
	}
	// Remove units before binaries. Each completed file is persisted separately
	// so a later failure leaves only pending recovery records for a retry.
	for _, isUnit := range []bool{true, false} {
		for i := 0; i < len(st.Files); {
			f := st.Files[i]
			if (f.Unit != "") != isUnit {
				i++
				continue
			}
			err := restoreFile(opts, f)
			if err == nil {
				backups := map[string]string{}
				if f.Backup != "" {
					backups[f.Backup] = f.BackupSHA256
				}
				if f.Unit == "" && f.RollbackSHA256 != "" {
					backups[f.Path+".bak"] = f.RollbackSHA256
				}
				for path, hash := range backups {
					data, removeErr := regularData(path)
					if os.IsNotExist(removeErr) {
						continue
					}
					if removeErr == nil && fileHash(data) != hash {
						removeErr = fmt.Errorf("preserving modified backup %s; recover it before retrying", path)
					}
					if removeErr == nil {
						removeErr = os.Remove(path)
					}
					err = errors.Join(err, removeErr)
				}
			}
			if err != nil {
				record(filepath.Base(f.Path), err)
				i++
				continue
			}
			st.Files = append(st.Files[:i], st.Files[i+1:]...)
			if err := persist(); err != nil {
				return res, errors.Join(errors.Join(failures...), err)
			}
			record(filepath.Base(f.Path), nil)
		}
	}
	record("daemon-reload", opts.systemctl()("daemon-reload"))

	for i := len(st.HandedOver) - 1; i >= 0; i-- {
		h := st.HandedOver[i]
		var errs []error
		if h.Unit != "" && h.UnitWasEnabled {
			if err := opts.systemctl()("enable", "--now", h.Unit); err != nil {
				errs = append(errs, err)
			}
		}
		for _, line := range h.Lines {
			if err := niri.RestoreLine(line); err != nil {
				errs = append(errs, err)
			}
		}
		err := errors.Join(errs...)
		if err == nil {
			st.HandedOver = append(st.HandedOver[:i], st.HandedOver[i+1:]...)
			if err := persist(); err != nil {
				return res, errors.Join(errors.Join(failures...), err)
			}
		}
		record("conflict:"+h.Name, err)
	}
	if st.Activation != nil {
		err := error(nil)
		if st.Activation.Path != dbusact.Path(opts.xdgData()) {
			err = errors.New("unexpected notification activation path")
		} else {
			err = dbusact.Remove(*st.Activation)
		}
		if err == nil {
			st.Activation = nil
			if err := persist(); err != nil {
				return res, errors.Join(errors.Join(failures...), err)
			}
		}
		record("dbus-activation", err)
	}
	if st.GSlapperInstalled && opts.RemoveGSlapper {
		var err error
		if opts.RemovePkg == nil {
			err = errors.New("no package remover configured")
		} else {
			err = opts.RemovePkg(opts.Pin.GSlapper.Package)
		}
		if err == nil {
			st.GSlapperInstalled = false
			if err := persist(); err != nil {
				return res, errors.Join(errors.Join(failures...), err)
			}
		}
		record("gslapper", err)
	} else {
		res.Tasks = append(res.Tasks, Task{Name: "gslapper", Status: Skipped, Reason: "kept"})
	}
	record("niri", niri.Remove(niri.Options{
		ConfigPath:  filepath.Join(opts.niriDir(), "config.kdl"),
		SidecarPath: filepath.Join(opts.niriDir(), "sysc.kdl"),
	}))
	// Keep configuration and the retry record while any restoration is pending.
	if len(failures) > 0 {
		return res, errors.Join(failures...)
	}
	if opts.Purge {
		if err := os.RemoveAll(filepath.Dir(opts.configPath())); err != nil {
			return res, err
		}
	}
	if err := os.Remove(filepath.Join(opts.stateDir(), stamp.FileName)); err != nil {
		return res, err
	}
	return res, nil
}

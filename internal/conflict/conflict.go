// Package conflict detects desktop components that already own the jobs
// sysc-notify and sysc-tray do: the notification name on the session bus, the
// tray watcher, or a bar/shell that spawns its own copies. Detection is
// read-only; only Apply, driven by an explicit user choice, changes anything,
// and every action it takes is recorded for uninstall.
package conflict

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Nomadcxx/sysc/internal/niri"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

// Kind is the class of component a finding belongs to.
type Kind string

const (
	KindNotifications Kind = "notifications"
	KindTray          Kind = "tray"
	KindShell         Kind = "shell"
)

// Choice is what the user decided for one finding.
type Choice string

const (
	HandOver Choice = "handover"
	KeepBoth Choice = "keep"
	SkipSYSC Choice = "skip"
)

// Finding is one existing component that may conflict with a companion.
type Finding struct {
	Name        string
	Kind        Kind
	PIDs        []int
	Cmdline     string
	Unit        string
	UnitEnabled bool
	BusOwner    bool
	NiriLines   []niri.Spawn
}

// Env carries every side effect and path Detect and Apply need, so tests can
// run against fake /proc, fake systemctl/busctl, and fake signals.
type Env struct {
	ProcRoot   string
	Systemctl  func(args ...string) (string, error)
	Busctl     func(args ...string) (string, error)
	NiriConfig string
	Signal     func(pid int, sig syscall.Signal) error
	Now        func() time.Time
	Sleep      func(time.Duration)
}

var known = map[string]Kind{
	"mako":           KindNotifications,
	"dunst":          KindNotifications,
	"swaync":         KindNotifications,
	"fnott":          KindNotifications,
	"waybar":         KindTray,
	"noctalia":       KindShell,
	"noctalia-shell": KindShell,
	"qs":             KindShell,
	"quickshell":     KindShell,
	"dms":            KindShell,
}

var unitFor = map[string]string{
	"mako":   "mako.service",
	"dunst":  "dunst.service",
	"swaync": "swaync.service",
	"fnott":  "fnott.service",
	"waybar": "waybar.service",
}

// Names lists every program Detect looks for, for the niri include scan.
func Names() []string {
	names := make([]string, 0, len(known))
	for n := range known {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Component is the SYSC component id that competes with this kind. A
// SkipSYSC choice for a finding skips this component.
func (k Kind) Component() string {
	switch k {
	case KindNotifications:
		return "sysc-notify"
	case KindTray:
		return "sysc-tray"
	default:
		return "sysc-shell"
	}
}

func kindRank(k Kind) int {
	switch k {
	case KindNotifications:
		return 0
	case KindTray:
		return 1
	default:
		return 2
	}
}

type procInfo struct {
	PID     int
	Comm    string
	Cmdline string
	Args    []string
}

func (e Env) procRoot() string {
	if e.ProcRoot != "" {
		return e.ProcRoot
	}
	return "/proc"
}

func (e Env) systemctl() func(...string) (string, error) {
	if e.Systemctl != nil {
		return e.Systemctl
	}
	return func(args ...string) (string, error) {
		out, err := exec.Command("systemctl", append([]string{"--user"}, args...)...).Output()
		return string(out), err
	}
}

func (e Env) busctl() func(...string) (string, error) {
	if e.Busctl != nil {
		return e.Busctl
	}
	return func(args ...string) (string, error) {
		out, err := exec.Command("busctl", append([]string{"--user"}, args...)...).Output()
		return string(out), err
	}
}

func (e Env) signal() func(int, syscall.Signal) error {
	if e.Signal != nil {
		return e.Signal
	}
	return syscall.Kill
}

func (e Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e Env) sleep(d time.Duration) {
	if e.Sleep != nil {
		e.Sleep(d)
		return
	}
	time.Sleep(d)
}

// scanProc reads /proc/<pid>/comm and cmdline and returns one entry per
// numeric pid directory that can be read.
func scanProc(root string) []procInfo {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []procInfo
	for _, ent := range entries {
		pid, err := strconv.Atoi(ent.Name())
		if err != nil {
			continue
		}
		comm, err := os.ReadFile(filepath.Join(root, ent.Name(), "comm"))
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, ent.Name(), "cmdline"))
		if err != nil {
			continue
		}
		args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		var cmdline string
		if len(args) > 0 {
			cmdline = strings.Join(args, " ")
		}
		out = append(out, procInfo{PID: pid, Comm: strings.TrimSpace(string(comm)), Cmdline: cmdline, Args: args})
	}
	return out
}

func baseName(s string) string {
	return filepath.Base(strings.TrimSpace(s))
}

var mainPIDRe = regexp.MustCompile(`Main PID:\s*(\d+)\s*\(([^)]+)\)`)

type busStatus struct {
	PID  int
	Comm string
	Unit string
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// parseBusStatus tolerates both the transaction-status key=value form
// (PID=, Comm=, Unit=) and the human status form (● unit - desc, Main PID:).
func parseBusStatus(out string) busStatus {
	var b busStatus
	if m := mainPIDRe.FindStringSubmatch(out); m != nil {
		b.PID = atoi(m[1])
		b.Comm = strings.TrimSpace(m[2])
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "Unit="); ok && b.Unit == "" {
			b.Unit = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "Comm="); ok && b.Comm == "" {
			b.Comm = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "PID="); ok && b.PID == 0 {
			b.PID = atoi(v)
		}
	}
	if b.Unit == "" {
		first := strings.TrimSpace(strings.SplitN(out, "\n", 2)[0])
		first = strings.TrimSpace(strings.TrimPrefix(first, "●"))
		if i := strings.Index(first, " - "); i > 0 {
			first = first[:i]
		}
		if i := strings.IndexAny(first, " \t"); i > 0 {
			first = first[:i]
		}
		if strings.Contains(first, ".") {
			b.Unit = first
		}
	}
	return b
}

// Detect reports components that would fight the companions. It never
// mutates state and never fails because a probe is unavailable: a missing
// session bus is a normal SSH install.
func Detect(env Env) ([]Finding, error) {
	byName := map[string]*Finding{}
	get := func(name string, kind Kind) *Finding {
		f, ok := byName[name]
		if !ok {
			f = &Finding{Name: name, Kind: kind}
			byName[name] = f
		}
		return f
	}

	for _, p := range scanProc(env.procRoot()) {
		name := p.Comm
		if _, ok := known[name]; !ok {
			name = ""
		}
		if name == "" && len(p.Args) > 0 {
			if c := baseName(p.Args[0]); known[c] == KindShell || c == "quickshell" || c == "qs" {
				name = c
			}
		}
		if name == "" {
			continue
		}
		f := get(name, known[name])
		f.PIDs = append(f.PIDs, p.PID)
		if f.Kind == KindShell && p.Cmdline != "" {
			f.Cmdline = truncateCmdline(p.Cmdline)
		}
	}

	bus := func(name string, kind Kind) {
		out, err := env.busctl()("status", name)
		if err != nil {
			return
		}
		st := parseBusStatus(out)
		fname := st.Comm
		if _, ok := known[fname]; !ok {
			if st.Unit != "" {
				fname = strings.TrimSuffix(st.Unit, ".service")
			}
		}
		if fname == "" {
			return
		}
		fkind, ok := known[fname]
		if !ok {
			fkind = kind
		}
		f := get(fname, fkind)
		f.BusOwner = true
		if st.PID > 0 && !containsInt(f.PIDs, st.PID) {
			f.PIDs = append(f.PIDs, st.PID)
		}
		if st.Unit != "" {
			f.Unit = st.Unit
			if out2, err := env.systemctl()("is-enabled", st.Unit); err == nil && strings.HasPrefix(strings.TrimSpace(out2), "enabled") {
				f.UnitEnabled = true
			}
		}
	}
	bus("org.freedesktop.Notifications", KindNotifications)
	bus("org.kde.StatusNotifierWatcher", KindTray)

	for name, unit := range unitFor {
		out, err := env.systemctl()("is-enabled", unit)
		if err != nil || !strings.HasPrefix(strings.TrimSpace(out), "enabled") {
			continue
		}
		f := get(name, known[name])
		f.Unit = unit
		f.UnitEnabled = true
	}

	if env.NiriConfig != "" {
		opts := niri.Options{
			ConfigPath:  env.NiriConfig,
			SidecarPath: filepath.Join(filepath.Dir(env.NiriConfig), "sysc.kdl"),
			Binds:       niri.DefaultBinds,
		}
		if spawns, err := niri.Spawns(opts, Names()); err == nil {
			for _, sp := range spawns {
				f := get(sp.Name, known[sp.Name])
				f.NiriLines = append(f.NiriLines, sp)
			}
		}
	}

	findings := make([]Finding, 0, len(byName))
	for _, f := range byName {
		findings = append(findings, *f)
	}
	sort.Slice(findings, func(i, j int) bool {
		if kindRank(findings[i].Kind) != kindRank(findings[j].Kind) {
			return kindRank(findings[i].Kind) < kindRank(findings[j].Kind)
		}
		return findings[i].Name < findings[j].Name
	})
	return findings, nil
}

func truncateCmdline(s string) string {
	const max = 80
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// Defaults is the wizard's starting point: notification daemons hand over,
// bars and other shells keep running (they do more than notifications).
func Defaults(findings []Finding, keepAll, handAll bool) map[string]Choice {
	out := make(map[string]Choice, len(findings))
	for _, f := range findings {
		switch {
		case keepAll:
			out[f.Name] = KeepBoth
		case handAll:
			out[f.Name] = HandOver
		case f.Kind == KindNotifications:
			out[f.Name] = HandOver
		default:
			out[f.Name] = KeepBoth
		}
	}
	return out
}

const stopWait = 3 * time.Second

// Apply carries out one Hand over choice: stop the process (gracefully for a
// unit, SIGTERM otherwise), disable its unit if it was enabled, and comment
// each niri spawn line with the handover marker. Everything it will touch is
// recorded through record before the first change, so a crash mid-handover
// still reverses.
func Apply(env Env, f Finding, c Choice, record func(stamp.Handover) error) (stamp.Handover, error) {
	h := stamp.Handover{Name: f.Name, Unit: f.Unit, UnitWasEnabled: f.UnitEnabled, Marker: niri.HandoverMarker}
	if c != HandOver {
		return h, nil
	}
	for _, sp := range f.NiriLines {
		h.Lines = append(h.Lines, niri.PlannedLine(sp))
	}
	if record != nil {
		if err := record(h); err != nil {
			return h, err
		}
	}
	var errs []error

	if f.Unit != "" {
		if _, err := env.systemctl()("stop", f.Unit); err != nil {
			errs = append(errs, fmt.Errorf("stop %s: %w", f.Unit, err))
		}
	}
	alive := waitGone(env, f.PIDs, stopWait)
	if len(alive) > 0 {
		for _, pid := range alive {
			_ = env.signal()(pid, syscall.SIGTERM)
		}
		alive = waitGone(env, alive, stopWait)
	}
	if len(alive) > 0 {
		errs = append(errs, fmt.Errorf("still running (PID %s)", intList(alive)))
	}
	if f.UnitEnabled && f.Unit != "" {
		if _, err := env.systemctl()("disable", f.Unit); err != nil {
			errs = append(errs, fmt.Errorf("disable %s: %w", f.Unit, err))
		}
	}
	for i, sp := range f.NiriLines {
		hl, err := niri.CommentLine(sp.Path, sp.LineNo, sp.Line)
		if err != nil {
			errs = append(errs, fmt.Errorf("comment %s: %w", sp.Path, err))
			continue
		}
		h.Lines[i] = hl
	}
	return h, errors.Join(errs...)
}

// waitGone returns the pids still alive after deadline. A pid that is
// already gone does not appear; zombies and permission errors count as gone
// so a process we cannot signal does not block an install forever.
func waitGone(env Env, pids []int, timeout time.Duration) []int {
	if len(pids) == 0 {
		return nil
	}
	deadline := env.now().Add(timeout)
	for {
		var alive []int
		for _, pid := range pids {
			if err := env.signal()(pid, 0); err == nil {
				alive = append(alive, pid)
			}
		}
		if len(alive) == 0 || !env.now().Before(deadline) {
			return alive
		}
		env.sleep(100 * time.Millisecond)
	}
}

func intList(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ", ")
}

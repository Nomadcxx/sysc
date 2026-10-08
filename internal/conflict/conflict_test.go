package conflict

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/niri"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeProc(t *testing.T, procs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for pid, spec := range procs {
		parts := strings.SplitN(spec, "|", 2)
		dir := filepath.Join(root, pid)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(parts[0]+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var cmdline string
		if len(parts) > 1 {
			cmdline = strings.ReplaceAll(parts[1], " ", "\x00") + "\x00"
		}
		if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDetectMergesProcBusUnitAndNiri(t *testing.T) {
	root := fakeProc(t, map[string]string{
		"100": "mako|mako",
		"200": "waybar|waybar",
		"300": "quickshell|quickshell -c noctalia",
		"400": "bash|bash",
	})
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.kdl")
	writeFile(t, cfg, "spawn-at-startup \"dunst\"\ninclude \"extra.kdl\"\n")
	writeFile(t, filepath.Join(dir, "extra.kdl"), "spawn-at-startup \"waybar\"\n")

	bus := func(args ...string) (string, error) {
		switch args[1] {
		case "org.freedesktop.Notifications":
			return "● org.freedesktop.Notifications.service - mako\n     Active: active (running)\n   Main PID: 100 (mako)\n", nil
		case "org.kde.StatusNotifierWatcher":
			return "● org.kde.StatusNotifierWatcher.service - waybar\n   Main PID: 200 (waybar)\n", nil
		}
		return "", os.ErrNotExist
	}
	sys := func(args ...string) (string, error) {
		switch args[0] {
		case "is-enabled":
			switch args[1] {
			case "mako.service", "waybar.service":
				return "enabled\n", nil
			}
			return "", os.ErrNotExist
		}
		return "", nil
	}
	findings, err := Detect(Env{ProcRoot: root, Busctl: bus, Systemctl: sys, NiriConfig: cfg})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(findings))
	for i, f := range findings {
		got[i] = f.Name
	}
	want := []string{"dunst", "mako", "waybar", "quickshell"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", got, want)
	}
	byName := map[string]Finding{}
	for _, f := range findings {
		byName[f.Name] = f
	}
	if f := byName["mako"]; !f.BusOwner || f.Kind != KindNotifications || len(f.PIDs) != 1 || f.PIDs[0] != 100 || !f.UnitEnabled || f.Unit != "mako.service" {
		t.Fatalf("mako finding = %+v", f)
	}
	if f := byName["waybar"]; !f.BusOwner || f.Kind != KindTray || len(f.NiriLines) != 1 || f.NiriLines[0].Path != filepath.Join(dir, "extra.kdl") {
		t.Fatalf("waybar finding = %+v", f)
	}
	if f := byName["dunst"]; len(f.NiriLines) != 1 || f.NiriLines[0].Path != cfg {
		t.Fatalf("dunst finding = %+v", f)
	}
	if f := byName["quickshell"]; f.Kind != KindShell || !strings.Contains(f.Cmdline, "noctalia") {
		t.Fatalf("quickshell finding = %+v", f)
	}
}

func TestDetectWithoutSessionBusOrConfig(t *testing.T) {
	findings, err := Detect(Env{
		ProcRoot:  t.TempDir(),
		Busctl:    func(...string) (string, error) { return "", os.ErrNotExist },
		Systemctl: func(...string) (string, error) { return "", os.ErrNotExist },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDefaults(t *testing.T) {
	findings := []Finding{
		{Name: "mako", Kind: KindNotifications},
		{Name: "waybar", Kind: KindTray},
		{Name: "quickshell", Kind: KindShell},
	}
	d := Defaults(findings, false, false)
	if d["mako"] != HandOver || d["waybar"] != KeepBoth || d["quickshell"] != KeepBoth {
		t.Fatalf("defaults = %v", d)
	}
	k := Defaults(findings, true, false)
	if k["mako"] != KeepBoth || k["waybar"] != KeepBoth || k["quickshell"] != KeepBoth {
		t.Fatalf("keepAll = %v", k)
	}
	h := Defaults(findings, false, true)
	if h["mako"] != HandOver || h["waybar"] != HandOver || h["quickshell"] != HandOver {
		t.Fatalf("handAll = %v", h)
	}
}

func testClockEnv() Env {
	clock := time.Unix(0, 0)
	env := Env{
		Now:   func() time.Time { return clock },
		Sleep: func(d time.Duration) { clock = clock.Add(d) },
	}
	return env
}

func TestApplyRecordsBeforeAnyChangeThenHandsOver(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.kdl")
	writeFile(t, cfg, "input {}\nspawn-at-startup \"mako\"\noutput \"eDP-1\" {}\n")
	env := testClockEnv()
	killed := false
	var order []string
	env.Signal = func(pid int, sig syscall.Signal) error {
		if sig == 0 {
			if killed {
				return syscall.ESRCH
			}
			return nil
		}
		if sig == syscall.SIGTERM {
			order = append(order, "sigterm")
			killed = true
		}
		return nil
	}
	env.Systemctl = func(args ...string) (string, error) {
		order = append(order, "systemctl "+strings.Join(args, " "))
		return "", nil
	}
	f := Finding{
		Name: "mako", Kind: KindNotifications, PIDs: []int{100},
		Unit: "mako.service", UnitEnabled: true,
		NiriLines: []niri.Spawn{{Name: "mako", Path: cfg, LineNo: 1, Line: `spawn-at-startup "mako"`}},
	}
	h, err := Apply(env, f, HandOver, func(rec stamp.Handover) error {
		order = append(order, "record")
		if rec.Name != "mako" || rec.Unit != "mako.service" || !rec.UnitWasEnabled || len(rec.Lines) != 1 {
			t.Fatalf("recorded %+v", rec)
		}
		if rec.Lines[0].Commented != `// sysc-handover: spawn-at-startup "mako"` {
			t.Fatalf("planned line %+v", rec.Lines[0])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(order) == 0 || order[0] != "record" {
		t.Fatalf("record must come first: %v", order)
	}
	if len(order) < 3 || order[1] != "systemctl stop mako.service" || order[2] != "sigterm" {
		t.Fatalf("order = %v", order)
	}
	if order[len(order)-1] != "systemctl disable mako.service" {
		t.Fatalf("disable not last: %v", order)
	}
	if len(h.Lines) != 1 || h.Lines[0].Original != `spawn-at-startup "mako"` {
		t.Fatalf("handover = %+v", h)
	}
	data, _ := os.ReadFile(cfg)
	if !strings.Contains(string(data), `// sysc-handover: spawn-at-startup "mako"`) {
		t.Fatalf("config not commented:\n%s", data)
	}
}

func TestApplyStillRunningReportsButContinues(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.kdl")
	writeFile(t, cfg, "spawn-at-startup \"mako\"\n")
	env := testClockEnv()
	env.Signal = func(pid int, sig syscall.Signal) error { return nil } // never dies
	var calls []string
	env.Systemctl = func(args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", nil
	}
	f := Finding{Name: "mako", Kind: KindNotifications, PIDs: []int{100}, NiriLines: []niri.Spawn{{Name: "mako", Path: cfg, LineNo: 0, Line: `spawn-at-startup "mako"`}}}
	recorded := false
	_, err := Apply(env, f, HandOver, func(stamp.Handover) error { recorded = true; return nil })
	if err == nil || !strings.Contains(err.Error(), "still running (PID 100)") {
		t.Fatalf("err = %v", err)
	}
	if !recorded {
		t.Fatal("record not called")
	}
	if len(calls) != 0 {
		t.Fatalf("no unit so no systemctl calls, got %v", calls)
	}
	data, _ := os.ReadFile(cfg)
	if !strings.Contains(string(data), `// sysc-handover: spawn-at-startup "mako"`) {
		t.Fatal("config not commented despite live process")
	}
}

func TestApplyNonHandoverIsNoop(t *testing.T) {
	called := false
	h, err := Apply(Env{}, Finding{Name: "waybar", Kind: KindTray}, KeepBoth, func(stamp.Handover) error {
		called = true
		return nil
	})
	if err != nil || called || h.Name != "waybar" {
		t.Fatalf("h=%+v err=%v called=%v", h, err, called)
	}
}

func TestParseBusStatusKeyValue(t *testing.T) {
	b := parseBusStatus("PID=42\nComm=mako\nUnit=mako.service\n")
	if b.PID != 42 || b.Comm != "mako" || b.Unit != "mako.service" {
		t.Fatalf("parsed %+v", b)
	}
}

func TestApplyChangedLineFails(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.kdl")
	writeFile(t, cfg, "input {}\nspawn-at-startup \"dunst\"\n")
	env := testClockEnv()
	env.Signal = func(pid int, sig syscall.Signal) error { return syscall.ESRCH }
	f := Finding{Name: "mako", Kind: KindNotifications, NiriLines: []niri.Spawn{{Name: "mako", Path: cfg, LineNo: 1, Line: `spawn-at-startup "mako"`}}}
	_, err := Apply(env, f, HandOver, nil)
	if err == nil || !strings.Contains(err.Error(), "changed since detection") {
		t.Fatalf("err = %v", err)
	}
}

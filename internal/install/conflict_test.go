package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/conflict"
	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/niri"
	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

func companionsPin(t *testing.T) pin.Pin {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "pin", "testdata", "companions-enabled.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := pin.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func conflictOpts(t *testing.T, home string, p pin.Pin) Options {
	t.Helper()
	return Options{
		Home:      home,
		Pin:       p,
		Answers:   weatherAnswers(),
		Yes:       true,
		Download:  func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:      func(string, string, []string) error { return nil },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: noopSystemctl,
		Now:       time.Unix(1000, 0),
	}
}

func taskNamed(tasks []Task, name string) Task {
	for _, task := range tasks {
		if task.Name == name {
			return task
		}
	}
	return Task{}
}

// The stamp must record a handover before the provider is touched, so an
// interrupted install can still be reversed.
func TestRunRecordsHandoverBeforeStop(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	home := setupHome(t)
	sawStamp := false
	env := conflict.Env{
		Systemctl: func(args ...string) (string, error) {
			if len(args) == 2 && args[0] == "stop" {
				s := stampAt(t, home)
				if len(s.HandedOver) == 1 && s.HandedOver[0].Name == "mako" {
					sawStamp = true
				}
			}
			return "", nil
		},
	}
	opts := conflictOpts(t, home, loadPin(t))
	opts.ConflictEnv = env
	opts.Findings = []conflict.Finding{{
		Name: "mako", Kind: conflict.KindNotifications,
		Unit: "mako.service", UnitEnabled: true,
	}}
	opts.Conflicts = map[string]conflict.Choice{"mako": conflict.HandOver}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !sawStamp {
		t.Fatal("provider stopped before the handover reached the stamp")
	}
	s := stampAt(t, home)
	if len(s.HandedOver) != 1 || s.HandedOver[0].Unit != "mako.service" || !s.HandedOver[0].UnitWasEnabled {
		t.Fatalf("handover not recorded: %+v", s.HandedOver)
	}
	if row := taskNamed(res.Tasks, "conflict:mako"); row.Status != Done {
		t.Fatalf("conflict row = %+v", row)
	}
}

func TestRunWritesActivationForEnabledNotify(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	home := setupHome(t)
	_, err := Run(context.Background(), conflictOpts(t, home, companionsPin(t)))
	if err != nil {
		t.Fatal(err)
	}
	s := stampAt(t, home)
	if s.Activation == nil {
		t.Fatalf("no activation recorded: %+v", s)
	}
	path := filepath.Join(home, ".local", "share", "dbus-1", "services", "org.freedesktop.Notifications.service")
	if s.Activation.Path != path {
		t.Fatalf("activation path = %q", s.Activation.Path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "SystemdService=sysc-notify.service") {
		t.Fatalf("activation file = %q", data)
	}
}

// Uninstall restores exactly the stamped handovers; a user-commented line
// without the marker stays as the user wrote it.
func TestUninstallRestoresOnlyStampedHandovers(t *testing.T) {
	home := setupHome(t)
	cfg := filepath.Join(home, ".config", "niri", "config.kdl")
	content := "input {}\n// spawn-at-startup \"dunst\"\n// sysc-handover: spawn-at-startup \"mako\"\n"
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	actPath := filepath.Join(home, ".local", "share", "dbus-1", "services", "org.freedesktop.Notifications.service")
	if err := os.MkdirAll(filepath.Dir(actPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(actPath, []byte("ours"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := stamp.Stamp{
		Release:    "v0.1.0",
		Components: map[string]string{},
		HandedOver: []stamp.Handover{{
			Name: "mako", Unit: "mako.service", UnitWasEnabled: true,
			Lines: []stamp.HandoverLine{{
				Path:      cfg,
				Commented: "// sysc-handover: spawn-at-startup \"mako\"",
				Original:  "spawn-at-startup \"mako\"",
			}},
		}},
		Activation: &stamp.Activation{Path: actPath},
	}
	if err := stamp.Write(filepath.Join(home, ".local", "state", "sysc"), s, true); err != nil {
		t.Fatal(err)
	}

	res, err := Uninstall(Options{Home: home, Pin: loadPin(t), Systemctl: noopSystemctl})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "spawn-at-startup \"mako\"") || strings.Contains(got, "sysc-handover") {
		t.Fatalf("handover line not restored: %q", got)
	}
	if !strings.Contains(got, "// spawn-at-startup \"dunst\"") {
		t.Fatalf("user-commented line touched: %q", got)
	}
	if _, err := os.Stat(actPath); !os.IsNotExist(err) {
		t.Fatalf("activation file survived uninstall: %v", err)
	}
	if row := taskNamed(res.Tasks, "conflict:mako"); row.Status != Done {
		t.Fatalf("conflict row = %+v", row)
	}
	if row := taskNamed(res.Tasks, "dbus-activation"); row.Status != Done {
		t.Fatalf("activation row = %+v", row)
	}
}

// A second install must not forget the line the first handover commented: the
// commented line is no longer detected, so replacing the record would leave
// the marker in the config forever.
func TestReRunKeepsStampedHandoverLine(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	home := setupHome(t)
	cfg := filepath.Join(home, ".config", "niri", "config.kdl")
	line := `spawn-at-startup "mako"`
	if err := os.WriteFile(cfg, []byte("input {}\n"+line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := conflictOpts(t, home, loadPin(t))
	opts.ConflictEnv = conflict.Env{Systemctl: func(...string) (string, error) { return "", nil }}
	opts.Conflicts = map[string]conflict.Choice{"mako": conflict.HandOver}
	opts.Findings = []conflict.Finding{{
		Name: "mako", Kind: conflict.KindNotifications,
		NiriLines: []niri.Spawn{{Name: "mako", Path: cfg, LineNo: 1, Line: line}},
	}}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if s := stampAt(t, home); len(s.HandedOver) != 1 || len(s.HandedOver[0].Lines) != 1 {
		t.Fatalf("first handover = %+v", s.HandedOver)
	}

	// The provider came back by another route, so the new record knows only
	// the unit and no spawn line.
	opts.Findings = []conflict.Finding{{
		Name: "mako", Kind: conflict.KindNotifications, Unit: "mako.service", UnitEnabled: true,
	}}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	s := stampAt(t, home)
	if len(s.HandedOver) != 1 || len(s.HandedOver[0].Lines) != 1 || s.HandedOver[0].Unit != "mako.service" {
		t.Fatalf("re-run lost the commented line: %+v", s.HandedOver)
	}
	if _, err := Uninstall(Options{Home: home, Pin: loadPin(t), Systemctl: noopSystemctl}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sysc-handover") || !strings.Contains(string(data), line) {
		t.Fatalf("spawn line not restored by uninstall: %q", data)
	}
}

// Restoring a provider the handover stopped means running again, not only
// enabled again.
func TestUninstallRestartsReEnabledUnit(t *testing.T) {
	home := setupHome(t)
	s := stamp.Stamp{
		Release:    "v0.1.0",
		Components: map[string]string{},
		HandedOver: []stamp.Handover{{Name: "mako", Unit: "mako.service", UnitWasEnabled: true}},
	}
	if err := stamp.Write(filepath.Join(home, ".local", "state", "sysc"), s, true); err != nil {
		t.Fatal(err)
	}
	var calls []string
	if _, err := Uninstall(Options{
		Home: home, Pin: loadPin(t),
		Systemctl: func(args ...string) error {
			calls = append(calls, strings.Join(args, " "))
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		if call == "enable --now mako.service" {
			return
		}
	}
	t.Fatalf("provider re-enabled without starting it: %v", calls)
}

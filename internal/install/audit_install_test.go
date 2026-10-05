package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

type recorder struct {
	mu      sync.Mutex
	Systemd [][]string
	Swaps   []string
}

func (r *recorder) systemctl(args ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Systemd = append(r.Systemd, append([]string(nil), args...))
	return nil
}

func (r *recorder) first(op string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, call := range r.Systemd {
		if len(call) > 0 && call[0] == op {
			return i
		}
	}
	return -1
}

func (r *recorder) has(op string) bool { return r.first(op) >= 0 }

// Design: on (re)install, stop running units (shell->walls->clipboard) BEFORE
// swapping binaries, so an upgrade does not swap the file under a running
// process with no restart. Run never calls stop.
func TestAuditRunStopsUnitsBeforeSwap(t *testing.T) {
	home := setupHome(t)
	var swapped []string
	rec := &recorder{}
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download:  func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:      func(string, string, []string) error { swapped = append(swapped, "swap"); return nil },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: rec.systemctl, Now: time.Unix(1000, 0),
	}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if len(swapped) == 0 {
		t.Fatal("no swaps happened")
	}
	if !rec.has("stop") {
		t.Errorf("AUDIT: Run swapped %d component binaries without stopping any unit first", len(swapped))
	}
}

// Design: download and verify ALL binaries before renaming ANY. The spine
// downloads then swaps per component, so a late network failure leaves the
// early binaries installed with no stamp.
func TestAuditSecondComponentFailureLeavesFirstSwapped(t *testing.T) {
	home := setupHome(t)
	calls := 0
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download: func(_ context.Context, staging string, assets []fetch.Asset) error {
			calls++
			if calls == 3 { // third enabled component fails (network dies mid-run)
				return fmt.Errorf("connection reset")
			}
			for _, a := range assets {
				if err := os.MkdirAll(staging, 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(staging, a.Name), []byte("fake-elf"), 0o755); err != nil {
					return err
				}
			}
			return nil
		},
		// Swap NOT overridden: exercise the real fetch.SwapAll.
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: noopSystemctl,
		Now:       time.Unix(1000, 0),
	}
	if _, err := Run(context.Background(), opts); err == nil {
		t.Fatal("Run succeeded despite download failure")
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "sysc-shell")); err == nil {
		t.Errorf("AUDIT: sysc-shell swapped into binDir while sysc-walls download failed; no stamp exists to clean it")
	}
	if _, err := stamp.Read(filepath.Join(home, ".local", "state", "sysc")); err == nil {
		t.Errorf("stamp written despite failure (unexpected: changes invariant analysis)")
	}
}

// Enable can fail after the swap. The stamp is written as soon as that swap
// succeeds, so Uninstall can remove the partial install instead of refusing
// with "no SYSC installation found".
func TestAuditEnableFailureLeavesNoStamp(t *testing.T) {
	home := setupHome(t)
	enables := 0
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download: func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:     func(string, string, []string) error { return nil },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: func(args ...string) error {
			if args[0] == "enable" {
				enables++
				if enables == 2 {
					return fmt.Errorf("systemctl enable failed")
				}
			}
			return nil
		},
		Now: time.Unix(1000, 0),
	}
	if _, err := Run(context.Background(), opts); err == nil {
		t.Fatal("Run succeeded despite enable failure")
	}
	if _, err := stamp.Read(filepath.Join(home, ".local", "state", "sysc")); err != nil {
		t.Fatalf("stamp missing after enable failure: %v", err)
	}
	if _, err := Uninstall(Options{Home: home, Pin: loadPin(t), Systemctl: noopSystemctl}); err != nil {
		t.Fatalf("Uninstall could not see the partial install: %v", err)
	}
}

// If start fails after enable, the stamp from the swap is already on disk,
// so Uninstall can roll the install back instead of reporting that nothing
// is installed.
func TestAuditStartFailureStateIsRecoverable(t *testing.T) {
	home := setupHome(t)
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download: func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:     func(string, string, []string) error { return nil },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: func(args ...string) error {
			if args[0] == "start" {
				return fmt.Errorf("start %s failed", args[len(args)-1])
			}
			return nil
		},
		Now: time.Unix(1000, 0),
	}
	if _, err := Run(context.Background(), opts); err == nil {
		t.Fatal("Run succeeded despite start failure")
	}
	_, err := Uninstall(Options{Home: home, Pin: loadPin(t), Systemctl: noopSystemctl})
	if err != nil {
		t.Errorf("AUDIT: after start failure the machine is left enabled+installed but Uninstall refuses: %v", err)
	}
}

func TestAuditSSHSessionEnablesOnly(t *testing.T) {
	home := setupHome(t)
	rec := &recorder{}
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download: func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:     func(string, string, []string) error { return nil },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: func(args ...string) error {
			rec.mu.Lock()
			rec.Systemd = append(rec.Systemd, append([]string(nil), args...))
			rec.mu.Unlock()
			if len(args) >= 2 && args[0] == "is-active" {
				return fmt.Errorf("inactive")
			}
			return nil
		},
		Now: time.Unix(1000, 0),
	}
	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stamp.Started {
		t.Errorf("stamp claims started in an inactive session")
	}
	if rec.has("start") {
		t.Errorf("units started despite inactive graphical session")
	}
	if !rec.has("enable") {
		t.Errorf("units not enabled")
	}
}

func TestAuditComponentWithoutUnitIsSilent(t *testing.T) {
	p := loadPin(t)
	p.Components = append(p.Components, pin.Component{
		ID: "sysc-extra", Tag: "v9.9.9",
		Binaries: []pin.Binary{{Name: "sysc-extra", Assets: map[string]pin.Asset{
			"amd64": {URL: "https://example.invalid/x", SHA256: strings.Repeat("4", 64)},
		}}},
	})
	home := setupHome(t)
	downloads := 0
	opts := Options{
		Home: home, Pin: p, Answers: weatherAnswers(), Yes: true,
		Download:  func(context.Context, string, []fetch.Asset) error { downloads++; return nil },
		Swap:      func(string, string, []string) error { return nil },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: noopSystemctl, Now: time.Unix(1000, 0),
	}
	_, err := Run(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "sysc-extra") {
		t.Fatalf("Run must fail fast naming sysc-extra, got %v", err)
	}
	if downloads != 0 {
		t.Errorf("no downloads expected before unit validation failed, got %d", downloads)
	}
	if exists(filepath.Join(home, ".local/state/sysc", stamp.FileName)) {
		t.Errorf("no stamp expected after fail-fast")
	}
}

func TestAuditUninstallDisablesUnits(t *testing.T) {
	home := uninstallHome(t)
	rec := &recorder{}
	if _, err := Uninstall(Options{Home: home, Pin: loadPin(t), Systemctl: rec.systemctl}); err != nil {
		t.Fatal(err)
	}
	if rec.has("disable") {
		return
	}
	t.Errorf("AUDIT: uninstall removed unit files but never ran systemctl disable/daemon-reload; " +
		"graphical-session.target.wants symlinks dangle")
}

// Design: the seeded config targets $XDG_CONFIG_HOME/sysc-shell/config.json —
// which is where the shell itself looks (os.UserConfigDir). The installer
// hardcodes $HOME/.config instead.
func TestAuditSeedHonorsXDGConfigHome(t *testing.T) {
	home := setupHome(t)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if err := os.MkdirAll(filepath.Join(xdg, "niri"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "niri", "config.kdl"), []byte("input {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download:  func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:      func(string, string, []string) error { return nil },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: noopSystemctl, Now: time.Unix(1000, 0),
	}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(xdg, "sysc-shell", "config.json")); err != nil {
		t.Errorf("AUDIT: no config at $XDG_CONFIG_HOME/sysc-shell/config.json (shell reads it there); instead written under Home/.config")
	}
}

// Design/claim 8: a skipped component must name the distro package.
func TestAuditGslapperSkipReasonNamesPackage(t *testing.T) {
	home := setupHome(t)
	res, err := Run(context.Background(), Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download:  func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:      func(string, string, []string) error { return nil },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: noopSystemctl, Now: time.Unix(1000, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range res.Tasks {
		if task.Name == "gslapper" {
			if !strings.Contains(strings.ToLower(task.Reason), "gslapper") {
				t.Errorf("AUDIT: gslapper skip reason %q does not name the package to install manually", task.Reason)
			}
			return
		}
	}
	t.Fatal("no gslapper task")
}

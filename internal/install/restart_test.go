package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/fetch"
)

var restartOrderStarts = []string{
	"start sysc-clipboard.service",
	"start sysc-walls.service",
	"start sysc-shell.service",
}

func trailingStarts(t *testing.T, rec *recorder, want []string) {
	t.Helper()
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.Systemd) < len(want) {
		t.Fatalf("recorded %d calls; want at least %d: %v", len(rec.Systemd), len(want), rec.Systemd)
	}
	tail := rec.Systemd[len(rec.Systemd)-len(want):]
	for i, call := range tail {
		if strings.Join(call, " ") != want[i] {
			t.Fatalf("call %d after failure = %q; want %q (all: %v)", i, strings.Join(call, " "), want[i], rec.Systemd)
		}
	}
}

// #39: a failed swap must restart the units that were running before Run
// stopped them, in StartOrder.
func TestFailedSwapRestartsPreviouslyActiveUnits(t *testing.T) {
	home := setupHome(t)
	rec := &recorder{}
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download:  func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:      func(string, string, []string) error { return fmt.Errorf("swap boom") },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: rec.systemctl, Now: time.Unix(1000, 0),
	}
	_, err := Run(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "swap boom") {
		t.Fatalf("Run error = %v; want swap failure", err)
	}
	trailingStarts(t, rec, restartOrderStarts)
}

// A failure later in the run (here: read-only niri config) must restart them too.
func TestFailedNiriApplyRestartsUnits(t *testing.T) {
	home := setupHome(t)
	niriDir := filepath.Join(home, ".config", "niri")
	if err := os.Chmod(niriDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(niriDir, 0o755) // let t.TempDir clean up

	rec := &recorder{}
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download:  func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:      func(string, string, []string) error { return nil },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: rec.systemctl, Now: time.Unix(1000, 0),
	}
	_, err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("Run succeeded despite read-only niri config")
	}
	trailingStarts(t, rec, restartOrderStarts)
}

// A first install has nothing running to restore.
func TestFirstInstallFailureStartsNothing(t *testing.T) {
	home := setupHome(t)
	rec := &recorder{}
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download: func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:     func(string, string, []string) error { return fmt.Errorf("swap boom") },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: func(args ...string) error {
			rec.mu.Lock()
			rec.Systemd = append(rec.Systemd, append([]string(nil), args...))
			rec.mu.Unlock()
			if len(args) > 0 && args[0] == "is-active" {
				return fmt.Errorf("inactive")
			}
			return nil
		},
		Now: time.Unix(1000, 0),
	}
	if _, err := Run(context.Background(), opts); err == nil {
		t.Fatal("Run succeeded despite swap failure")
	}
	if rec.has("start") {
		t.Fatalf("fresh install failure started units: %v", rec.Systemd)
	}
}

// StartAll already attempted every unit; the deferred restart must not run
// a second time on top of it.
func TestStartAllFailureNotRetried(t *testing.T) {
	home := setupHome(t)
	rec := &recorder{}
	starts := 0
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
				if args[len(args)-1] == "graphical-session.target" {
					return nil
				}
				return fmt.Errorf("inactive")
			}
			if len(args) > 0 && args[0] == "start" {
				starts++
				return fmt.Errorf("start refused")
			}
			return nil
		},
		Now: time.Unix(1000, 0),
	}
	if _, err := Run(context.Background(), opts); err == nil {
		t.Fatal("Run succeeded despite start failure")
	}
	if starts != 1 {
		t.Fatalf("start attempted %d times; want exactly 1 (no retry after StartAll)", starts)
	}
}

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
	"github.com/Nomadcxx/sysc/internal/stamp"
)

func rerunOpts(t *testing.T, home string, lookPath func(string) (string, error), installPkg func(string) error) Options {
	t.Helper()
	return Options{
		Home:       home,
		Pin:        loadPin(t),
		Answers:    weatherAnswers(),
		Yes:        true,
		Download:   func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:       func(string, string, []string) error { return nil },
		LookPath:   lookPath,
		InstallPkg: installPkg,
		Systemctl:  noopSystemctl,
		Now:        time.Unix(1000, 0),
	}
}

func stampAt(t *testing.T, home string) stamp.Stamp {
	t.Helper()
	s, err := stamp.Read(filepath.Join(home, ".local", "state", "sysc"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func gslapperTask(t *testing.T, tasks []Task) Task {
	t.Helper()
	for _, task := range tasks {
		if task.Name == "gslapper" {
			return task
		}
	}
	t.Fatal("no gslapper task")
	return Task{}
}

// #36: a re-run finds gslapper on PATH (installed by the first run) and must
// not drop the ownership flag, or uninstall leaves the AUR package behind.
func TestRerunKeepsGSlapperOwnership(t *testing.T) {
	home := setupHome(t)
	notFound := func(string) (string, error) { return "", os.ErrNotExist }
	if _, err := Run(context.Background(), rerunOpts(t, home, notFound, func(string) error { return nil })); err != nil {
		t.Fatal(err)
	}
	if !stampAt(t, home).GSlapperInstalled {
		t.Fatal("first install did not record gslapper ownership")
	}

	onPath := func(string) (string, error) { return "/usr/bin/gslapper", nil }
	res, err := Run(context.Background(), rerunOpts(t, home, onPath, func(string) error {
		return fmt.Errorf("must not reinstall")
	}))
	if err != nil {
		t.Fatal(err)
	}
	if task := gslapperTask(t, res.Tasks); task.Status != Skipped || task.Reason != "already installed by SYSC" {
		t.Fatalf("gslapper task = %+v", task)
	}
	if !stampAt(t, home).GSlapperInstalled {
		t.Fatal("re-run dropped gslapper ownership (#36)")
	}

	removed := 0
	if _, err := Uninstall(Options{Home: home, Pin: loadPin(t), RemoveGSlapper: true,
		Systemctl: noopSystemctl, RemovePkg: func(string) error { removed++; return nil }}); err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("uninstall removed gslapper %d times; want 1", removed)
	}
}

// A gslapper the user already had must never be claimed, across re-runs too.
func TestRerunDoesNotClaimUserGSlapper(t *testing.T) {
	home := setupHome(t)
	onPath := func(string) (string, error) { return "/usr/bin/gslapper", nil }
	var res Result
	var err error
	for i := 0; i < 2; i++ {
		res, err = Run(context.Background(), rerunOpts(t, home, onPath, nil))
		if err != nil {
			t.Fatal(err)
		}
	}
	if stampAt(t, home).GSlapperInstalled {
		t.Fatal("SYSC claimed a gslapper the user installed")
	}
	if task := gslapperTask(t, res.Tasks); task.Reason != "already on PATH" {
		t.Fatalf("gslapper task = %+v", task)
	}
	removed := 0
	if _, err := Uninstall(Options{Home: home, Pin: loadPin(t), RemoveGSlapper: true,
		Systemctl: noopSystemctl, RemovePkg: func(string) error { removed++; return nil }}); err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("uninstall removed the user's gslapper %d times", removed)
	}
}

// Once gslapper leaves PATH without SYSC replacing it, ownership is stale.
func TestRerunAfterManualRemoval(t *testing.T) {
	home := setupHome(t)
	notFound := func(string) (string, error) { return "", os.ErrNotExist }
	if _, err := Run(context.Background(), rerunOpts(t, home, notFound, func(string) error { return nil })); err != nil {
		t.Fatal(err)
	}
	if !stampAt(t, home).GSlapperInstalled {
		t.Fatal("first install did not record gslapper ownership")
	}
	res, err := Run(context.Background(), rerunOpts(t, home, notFound, nil))
	if err != nil {
		t.Fatal(err)
	}
	if stampAt(t, home).GSlapperInstalled {
		t.Fatal("gate kept gslapper ownership after it left PATH")
	}
	if task := gslapperTask(t, res.Tasks); task.Status != Skipped {
		t.Fatalf("gslapper task = %+v", task)
	}
}

// #37 end-to-end: an unset Now must become wall clock, so state copies get
// real timestamps instead of 00010101T000000Z.
func TestRunDefaultsNowToWallClock(t *testing.T) {
	home := setupHome(t)
	opts := rerunOpts(t, home, func(string) (string, error) { return "", os.ErrNotExist }, nil)
	opts.Now = time.Time{}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(home, ".local", "state", "sysc", "backups"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	found := false
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "niri-config.kdl.") {
			continue
		}
		raw := strings.TrimPrefix(e.Name(), "niri-config.kdl.")
		ts, err := time.Parse("20060102T150405Z", raw)
		if err != nil {
			t.Fatalf("backup name %q is not a timestamp: %v", e.Name(), err)
		}
		if ts.Before(now.Add(-time.Hour)) || ts.After(now.Add(time.Hour)) {
			t.Fatalf("backup stamp %s is not wall clock (now %s); zero Now leaked (#37)", ts, now)
		}
		found = true
	}
	if !found {
		t.Fatal("no niri backup written")
	}
}

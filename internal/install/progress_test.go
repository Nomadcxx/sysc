package install

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/fetch"
)

func TestProgressStartsBeforeDownloadsAndRetainsFailure(t *testing.T) {
	o := safetyOptions(t)
	var latest []Task
	o.Progress = func(tasks []Task) { latest = tasks }
	download := o.Download
	count := 0
	o.Download = func(ctx context.Context, dir string, assets []fetch.Asset) error {
		count++
		if len(latest) != len(o.Pin.Components) {
			t.Fatalf("download began without visible component list: %+v", latest)
		}
		if latest[count-1].Status != Status("downloading") {
			t.Fatalf("current component not shown downloading: %+v", latest)
		}
		if count == 2 {
			if latest[0].Status != Status("verified") {
				t.Fatalf("completed download has no verification tick: %+v", latest[0])
			}
			return errors.New("download interrupted")
		}
		return download(ctx, dir, assets)
	}
	res, err := Run(context.Background(), o)
	if err == nil || count != 2 {
		t.Fatalf("want second download failure, count=%d err=%v", count, err)
	}
	if len(res.Tasks) != len(o.Pin.Components) || res.Tasks[1].Status != Failed || res.Tasks[1].Reason != "sysc-clipboard: download interrupted" {
		t.Fatalf("failure screen lost component progress: %+v", res.Tasks)
	}
}

func TestProgressReportsServiceAndDesktopSetup(t *testing.T) {
	o := safetyOptions(t)
	var latest []Task
	o.Progress = func(tasks []Task) { latest = tasks }
	o.Systemctl = func(args ...string) error {
		if args[0] == "enable" {
			for _, row := range latest {
				if row.Name == args[1] && row.Status == Status("running") {
					return nil
				}
			}
			t.Fatalf("service setup hidden from progress: %v, %+v", args, latest)
		}
		return noopSystemctl(args...)
	}
	res, err := Run(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range res.Tasks {
		if row.Name == "desktop-config" && row.Status == Done {
			found = true
		}
	}
	if !found {
		t.Fatalf("desktop configuration has no completed row: %+v", res.Tasks)
	}
}

func TestRunReportsDetachedProgress(t *testing.T) {
	home := setupHome(t)
	var snaps [][]Task
	opts := Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download:  func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:      func(string, string, []string) error { return nil },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: noopSystemctl, Now: time.Unix(1000, 0),
		Progress: func(tasks []Task) { snaps = append(snaps, tasks) },
	}
	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) < 3 {
		t.Fatalf("snapshots = %d, want at least 3", len(snaps))
	}
	pending := false
	for _, task := range snaps[0] {
		if task.Status == "" {
			pending = true
		}
	}
	if !pending {
		t.Fatalf("first snapshot = %+v, want at least one pending row", snaps[0])
	}
	last := snaps[len(snaps)-1]
	if len(last) != len(res.Tasks) {
		t.Fatalf("last snapshot rows = %d, result rows = %d", len(last), len(res.Tasks))
	}
	for _, task := range last {
		if task.Status != Done && task.Status != Skipped {
			t.Fatalf("final task %q status = %q, want done/skipped", task.Name, task.Status)
		}
	}
	// Snapshots must be detached from live rows: mutating an early one must
	// not leak into any later snapshot.
	snaps[0][0].Status = Failed
	for _, task := range last {
		if task.Status == Failed {
			t.Fatalf("snapshot aliases live task memory: %+v", task)
		}
	}
}

func TestRunNilProgressIsClean(t *testing.T) {
	home := setupHome(t)
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
}

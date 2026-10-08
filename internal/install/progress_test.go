package install

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/fetch"
)

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

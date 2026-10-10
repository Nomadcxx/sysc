package install

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

func safetyOptions(t *testing.T) Options {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	return Options{Home: setupHome(t), Pin: loadPin(t), Answers: weatherAnswers(),
		Download: versionedDownload("candidate"), Systemctl: noopSystemctl,
		LookPath: func(string) (string, error) { return "/unused/gslapper", nil },
	}
}

func TestUninstallRetainsStampAfterRemovalFailure(t *testing.T) {
	o := safetyOptions(t)
	if _, err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(o.binDir(), "sysc-shell")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(path, "keep"), "user data")
	res, err := Uninstall(o)
	if err == nil {
		t.Fatal("uninstall accepted failed binary removal")
	}
	failed := false
	for _, row := range res.Tasks {
		failed = failed || row.Status == Failed
	}
	if !failed {
		t.Fatal("uninstall did not report failed task")
	}
	if _, err := stamp.Read(o.stateDir()); err != nil {
		t.Fatal("retry stamp lost:", err)
	}
}

func TestUninstallRestoresFirstForeignFilesAfterRerun(t *testing.T) {
	o := safetyOptions(t)
	binary := filepath.Join(o.binDir(), "sysc-shell")
	unit := filepath.Join(o.unitDir(), "sysc-shell.service")
	writeFile(t, binary, "original binary")
	writeFile(t, unit, "original unit")
	for i := 0; i < 2; i++ {
		if _, err := Run(context.Background(), o); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Uninstall(o); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{binary: "original binary", unit: "original unit"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s: got %q %v, want %q", path, got, err, want)
		}
	}
}

func TestUninstallPreservesModifiedOwnedFile(t *testing.T) {
	o := safetyOptions(t)
	if _, err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(o.unitDir(), "sysc-shell.service")
	writeFile(t, path, "user customization")
	if _, err := Uninstall(o); err == nil {
		t.Fatal("uninstall removed modified unit")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "user customization" {
		t.Fatalf("user unit lost: %q %v", data, err)
	}
	if _, err := stamp.Read(o.stateDir()); err != nil {
		t.Fatal("retry stamp lost", err)
	}
}

func TestUninstallCanRetryDisableFailure(t *testing.T) {
	o := safetyOptions(t)
	if _, err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	o.Systemctl = func(args ...string) error {
		if args[0] == "disable" {
			return errors.New("permission denied")
		}
		return nil
	}
	if _, err := Uninstall(o); err == nil {
		t.Fatal("ignored disable failure")
	}
	if _, err := stamp.Read(o.stateDir()); err != nil {
		t.Fatal("retry stamp lost", err)
	}
	o.Systemctl = noopSystemctl
	if _, err := Uninstall(o); err != nil {
		t.Fatal("retry failed", err)
	}
	if _, err := stamp.Read(o.stateDir()); !os.IsNotExist(err) {
		t.Fatal("successful uninstall kept stamp", err)
	}
}

func TestConcurrentInstallRefusesSecondRun(t *testing.T) {
	o := safetyOptions(t)
	staged := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	download := o.Download
	o.Download = func(ctx context.Context, dir string, assets []fetch.Asset) error {
		if err := download(ctx, dir, assets); err != nil {
			return err
		}
		if len(assets) > 0 && assets[0].Name == "sysc-shell" {
			close(staged)
			<-release
		}
		return nil
	}
	go func() { _, err := Run(context.Background(), o); finished <- err }()
	<-staged
	second := o
	second.Download = download
	_, err := Run(context.Background(), second)
	close(release)
	firstErr := <-finished
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	if err == nil || !strings.Contains(err.Error(), "in progress") {
		t.Fatalf("second run error=%v, want lock refusal", err)
	}
}

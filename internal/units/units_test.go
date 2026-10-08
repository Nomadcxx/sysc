package units

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStopOrderShellFirst(t *testing.T) {
	var calls []string
	run := func(args ...string) error {
		calls = append(calls, strings.Join(args, " "))
		return nil
	}
	if err := StopAll(run); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"stop sysc-shell.service",
		"stop sysc-walls.service",
		"stop sysc-clipboard.service",
		"stop sysc-tray.service",
		"stop sysc-notify.service",
	}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("stop order = %v; want %v", calls, want)
	}

	calls = nil
	if err := StartAll(run); err != nil {
		t.Fatal(err)
	}
	want = []string{
		"start sysc-notify.service",
		"start sysc-tray.service",
		"start sysc-clipboard.service",
		"start sysc-walls.service",
		"start sysc-shell.service",
	}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("start order = %v; want %v", calls, want)
	}
}

func TestWriteBacksUpForeignUnit(t *testing.T) {
	dir := t.TempDir()
	u := All[0]
	dst := filepath.Join(dir, u.Name)
	if err := os.WriteFile(dst, []byte("foreign"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, u); err != nil {
		t.Fatal(err)
	}
	bak, err := os.ReadFile(dst + ".sysc.bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(bak) != "foreign" {
		t.Fatalf("backup = %q; want foreign", bak)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "WantedBy=graphical-session.target") {
		t.Fatalf("unit missing session target:\n%s", data)
	}
}

func TestActiveUnitsAndStartUnits(t *testing.T) {
	var calls []string
	run := func(args ...string) error {
		calls = append(calls, strings.Join(args, " "))
		return nil
	}
	active := ActiveUnits(run)
	wantActive := []string{
		"sysc-shell.service",
		"sysc-walls.service",
		"sysc-clipboard.service",
		"sysc-tray.service",
		"sysc-notify.service",
	}
	if strings.Join(active, ",") != strings.Join(wantActive, ",") {
		t.Fatalf("ActiveUnits = %v; want %v", active, wantActive)
	}
	for _, call := range calls {
		if !strings.HasPrefix(call, "is-active") {
			t.Fatalf("ActiveUnits ran %q; want only is-active", call)
		}
	}

	calls = nil
	failing := "sysc-walls.service"
	runErr := func(args ...string) error {
		calls = append(calls, strings.Join(args, " "))
		if len(args) == 2 && args[0] == "start" && args[1] == failing {
			return fmt.Errorf("boom")
		}
		return nil
	}
	err := StartUnits(runErr, []string{"sysc-shell.service", "sysc-walls.service", "sysc-clipboard.service"})
	if err == nil || !strings.Contains(err.Error(), failing) {
		t.Fatalf("StartUnits error = %v; want a %s failure", err, failing)
	}
	want := []string{"start sysc-clipboard.service", "start sysc-walls.service", "start sysc-shell.service"}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("StartUnits order = %v; want %v (every unit attempted)", calls, want)
	}
}

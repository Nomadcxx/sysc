package units

import (
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
	want := []string{"stop sysc-shell.service", "stop sysc-walls.service", "stop sysc-clipboard.service"}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("stop order = %v; want %v", calls, want)
	}

	calls = nil
	if err := StartAll(run); err != nil {
		t.Fatal(err)
	}
	want = []string{"start sysc-clipboard.service", "start sysc-walls.service", "start sysc-shell.service"}
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

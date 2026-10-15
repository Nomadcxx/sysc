package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/install"
	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

const seedConfig = `{"weather":{"latitude":51.5,"longitude":-0.12,"location":"London"}}`

// updateEnv points the stamp, the shell config and the staging tree at temp
// directories, so a test never reads or writes the owner's real install.
func updateEnv(t *testing.T) (stateDir, home string) {
	t.Helper()
	root := t.TempDir()
	stateDir = filepath.Join(root, "state")
	home = filepath.Join(root, "home")
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(filepath.Join(home, ".config", "sysc-shell"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "sysc-shell", "config.json"), []byte(seedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return stateDir, home
}

func writeStamp(t *testing.T, stateDir string, s stamp.Stamp) {
	t.Helper()
	if err := stamp.Write(filepath.Join(stateDir, "sysc"), s, true); err != nil {
		t.Fatal(err)
	}
}

func TestParseUpdateRefusesUnknownArguments(t *testing.T) {
	var errOut bytes.Buffer
	if _, err := parseUpdate([]string{"extra"}, &errOut); err == nil {
		t.Fatal("parseUpdate accepted a positional argument")
	}
	o, err := parseUpdate([]string{"--apply", "--yes"}, &errOut)
	if err != nil {
		t.Fatalf("parseUpdate(--apply --yes) = %v", err)
	}
	if !o.Yes || !o.Apply {
		t.Fatalf("parseUpdate(--apply --yes) = %+v, want Yes and Apply", o)
	}
	if plain, err := parseUpdate(nil, &errOut); err != nil || plain.Apply {
		t.Fatalf("parseUpdate() = %+v, %v, want a verb that fetches rather than applies", plain, err)
	}
}

func TestUpdateRefusesWithoutAStamp(t *testing.T) {
	updateEnv(t)
	var out, errOut bytes.Buffer
	called := false
	got := runUpdate([]string{"--apply", "--yes"}, strings.NewReader(""), &out, &errOut, os.Getenv("HOME"),
		func(context.Context, install.Options) (install.Result, error) {
			called = true
			return install.Result{}, nil
		})
	if got != 1 {
		t.Fatalf("runUpdate without a stamp = %d, want 1", got)
	}
	if called {
		t.Fatal("runUpdate installed without a stamp")
	}
	if !strings.Contains(errOut.String(), "installed.json") {
		t.Fatalf("refusal %q does not name the stamp path", errOut.String())
	}
}

func TestUpdateRefusesWhenTheSuiteIsAlreadyAtThePin(t *testing.T) {
	stateDir, home := updateEnv(t)
	p, err := pin.Load()
	if err != nil {
		t.Fatal(err)
	}
	components := map[string]string{}
	for _, c := range p.Components {
		if !c.Disabled {
			components[c.ID] = c.Tag
		}
	}
	writeStamp(t, stateDir, stamp.Stamp{Release: p.Release, Components: components})
	var out, errOut bytes.Buffer
	called := false
	got := runUpdate([]string{"--apply", "--yes"}, strings.NewReader(""), &out, &errOut, home,
		func(context.Context, install.Options) (install.Result, error) {
			called = true
			return install.Result{}, nil
		})
	if got != 0 {
		t.Fatalf("runUpdate at the current pin = %d, want 0 (stderr: %s)", got, errOut.String())
	}
	if called {
		t.Fatal("runUpdate reinstalled a suite already at the pin")
	}
	if want := "Already at " + p.Release + "."; out.String() != want+"\n" {
		t.Fatalf("runUpdate printed %q, want %q", out.String(), want+"\n")
	}
}

func TestUpdatePrintsTheDeltaAndRunsTheSamePipeline(t *testing.T) {
	stateDir, home := updateEnv(t)
	p, err := pin.Load()
	if err != nil {
		t.Fatal(err)
	}
	shellTag := ""
	for _, c := range p.Components {
		if c.ID == "sysc-shell" {
			shellTag = c.Tag
		}
	}
	writeStamp(t, stateDir, stamp.Stamp{Release: "v0.0.1", Components: map[string]string{"sysc-shell": "v0.0.1"}})
	var out, errOut bytes.Buffer
	var got install.Options
	got2 := runUpdate([]string{"--apply", "--yes"}, strings.NewReader(""), &out, &errOut, home,
		func(_ context.Context, opts install.Options) (install.Result, error) {
			got = opts
			return install.Result{Tasks: []install.Task{{Name: "sysc-shell", Status: install.Done}}}, nil
		})
	if got2 != 0 {
		t.Fatalf("runUpdate = %d, want 0 (stderr: %s)", got2, errOut.String())
	}
	want := "SYSC update v0.0.1 -> " + p.Release + "\n"
	if !strings.HasPrefix(out.String(), want) {
		t.Fatalf("runUpdate printed %q, want it to start with %q", out.String(), want)
	}
	if !strings.Contains(out.String(), "~ sysc-shell  v0.0.1 -> "+shellTag+"\n") {
		t.Fatalf("runUpdate did not print the moved component:\n%s", out.String())
	}
	if got.Pin.Release != p.Release {
		t.Fatalf("the pipeline got pin %q, want the fetched pin %q", got.Pin.Release, p.Release)
	}
	if got.Answers.Latitude != 51.5 || got.Answers.Location != "London" {
		t.Fatalf("the pipeline got answers %+v, want the ones recovered from the config", got.Answers)
	}
}

func TestUpdateAsksBeforeRunningThePipeline(t *testing.T) {
	stateDir, home := updateEnv(t)
	writeStamp(t, stateDir, stamp.Stamp{Release: "v0.0.1", Components: map[string]string{"sysc-shell": "v0.0.1"}})
	var out, errOut bytes.Buffer
	called := false
	got := runUpdate([]string{"--apply"}, strings.NewReader("n\n"), &out, &errOut, home,
		func(context.Context, install.Options) (install.Result, error) {
			called = true
			return install.Result{}, nil
		})
	if got != 1 {
		t.Fatalf("runUpdate refused answer = %d, want 1", got)
	}
	if called {
		t.Fatal("runUpdate installed after a refused answer")
	}
	if !strings.Contains(out.String(), "Proceed? [y/N]") || !strings.Contains(out.String(), "Aborted.") {
		t.Fatalf("runUpdate did not ask before installing:\n%s", out.String())
	}
}

func TestUpdateReportsAFailedTask(t *testing.T) {
	stateDir, home := updateEnv(t)
	writeStamp(t, stateDir, stamp.Stamp{Release: "v0.0.1", Components: map[string]string{"sysc-shell": "v0.0.1"}})
	var out, errOut bytes.Buffer
	got := runUpdate([]string{"--apply", "--yes"}, strings.NewReader(""), &out, &errOut, home,
		func(context.Context, install.Options) (install.Result, error) {
			return install.Result{Tasks: []install.Task{{Name: "sysc-shell", Status: install.Failed, Reason: "no asset"}}}, nil
		})
	if got != 1 {
		t.Fatalf("runUpdate with a failed task = %d, want 1", got)
	}
	if !strings.Contains(out.String(), "sysc-shell: failed (no asset)") {
		t.Fatalf("runUpdate did not print the task row:\n%s", out.String())
	}
}

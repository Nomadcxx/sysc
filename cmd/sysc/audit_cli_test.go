package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/install"
)

func TestAuditParseUninstallFlags(t *testing.T) {
	var out bytes.Buffer
	o, err := parseUninstall([]string{"--purge", "--yes"}, &out)
	if err != nil {
		t.Fatalf("parseUninstall: %v", err)
	}
	if !o.Purge || !o.Yes {
		t.Fatalf("got %+v, want Purge+Yes", o)
	}
}

func TestAuditUsageIsVisible(t *testing.T) {
	var out bytes.Buffer
	_, err := parseFlags([]string{"--help"}, &out)
	if err == nil {
		t.Fatal("expected flag.ErrHelp")
	}
	if !strings.Contains(out.String(), "Usage") {
		t.Fatalf("--help printed nothing: %q", out.String())
	}
	out.Reset()
	if _, err := parseUninstall([]string{"--help"}, &out); err == nil {
		t.Fatal("expected flag.ErrHelp for uninstall")
	} else if !strings.Contains(out.String(), "uninstall") {
		t.Fatalf("uninstall --help invisible: %q", out.String())
	}
}

func TestAuditUninstallCommandNoInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	var out bytes.Buffer
	code := runUninstall([]string{"--yes"}, strings.NewReader(""), &out, home)
	if code == 0 {
		t.Fatalf("uninstall succeeded against empty HOME (would touch real state): %s", out.String())
	}
	if !strings.Contains(out.String(), "no SYSC installation found") {
		t.Fatalf("out = %q, want 'no SYSC installation found'", out.String())
	}
}

func TestAnyFailed(t *testing.T) {
	if anyFailed([]install.Task{{Status: install.Done}, {Status: install.Skipped}}) {
		t.Fatal("done and skipped tasks must not count as failures")
	}
	if !anyFailed([]install.Task{{Status: install.Done}, {Status: install.Failed}}) {
		t.Fatal("a failed task must be reported")
	}
}

func TestAuditUninstallPromptAborts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	var out bytes.Buffer
	code := runUninstall(nil, strings.NewReader("n\n"), &out, home)
	if code != 1 {
		t.Fatalf("code = %d, want 1: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "This removes the SYSC suite binaries and user units.") {
		t.Fatalf("out = %q, want removal summary", out.String())
	}
	if !strings.Contains(out.String(), "Proceed? [y/N]") {
		t.Fatalf("out = %q, want prompt", out.String())
	}
	if !strings.Contains(out.String(), "Aborted.") {
		t.Fatalf("out = %q, want Aborted.", out.String())
	}
}

package main

import (
	"bytes"
	"strings"
	"testing"
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
	code := runUninstall([]string{"--yes"}, &out, home)
	if code == 0 {
		t.Fatalf("uninstall succeeded against empty HOME (would touch real state): %s", out.String())
	}
	if !strings.Contains(out.String(), "no SYSC installation found") {
		t.Fatalf("out = %q, want 'no SYSC installation found'", out.String())
	}
}

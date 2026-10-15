package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/stamp"
)

func TestRunVersionWithStamp(t *testing.T) {
	dir := t.TempDir()
	if err := stamp.Write(dir, stamp.Stamp{
		Release: "v0.1.1",
		Components: map[string]string{
			"sysc-tray":   "v0.1.1",
			"sysc-shell":  "v0.1.0",
			"sysc-walls":  "v1.0.2",
			"sysc-notify": "v0.1.0",
		},
	}, true); err != nil {
		t.Fatalf("write stamp: %v", err)
	}
	var out, errOut bytes.Buffer
	if code := runVersion(dir, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, errOut.String())
	}
	want := "sysc release: v0.1.1\n" +
		"sysc-notify: v0.1.0\n" +
		"sysc-shell: v0.1.0\n" +
		"sysc-tray: v0.1.1\n" +
		"sysc-walls: v1.0.2\n"
	if got := out.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunVersionWithoutStamp(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	if code := runVersion(dir, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	want := "no sysc install stamp at " + filepath.Join(dir, stamp.FileName) + "\n"
	if got := out.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
}

func TestRunVersionWithCorruptStamp(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, stamp.FileName), []byte("{"), 0o600); err != nil {
		t.Fatalf("write corrupt stamp: %v", err)
	}
	var out, errOut bytes.Buffer
	if code := runVersion(dir, &out, &errOut); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	if errOut.Len() == 0 {
		t.Fatal("stderr is empty, want the parse error verbatim")
	}
}

func TestPrintShellVersionPrintsTheShellLine(t *testing.T) {
	binDir := t.TempDir()
	script := "#!/bin/sh\necho 'sysc-shell 0.1.2 (1a2b3c4)'\n"
	if err := os.WriteFile(filepath.Join(binDir, "sysc-shell"), []byte(script), 0o700); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	var out bytes.Buffer
	printShellVersion(&out, binDir)
	if got, want := out.String(), "sysc-shell 0.1.2 (1a2b3c4)\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPrintShellVersionWarnsWhenTheShellIsMissing(t *testing.T) {
	var out bytes.Buffer
	printShellVersion(&out, t.TempDir())
	got := out.String()
	if want := "warning: sysc-shell version unavailable\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if strings.Contains(got, "no such file") {
		t.Fatalf("stdout leaks the exec error: %q", got)
	}
}

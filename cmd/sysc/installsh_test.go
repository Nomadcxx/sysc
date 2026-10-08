package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallShFetchesAndRuns exercises the release fetcher end to end with a
// stubbed curl: the staging dir must be unpredictable, mode 700, cleaned up,
// and the installer's exit status must pass through unchanged.
func TestInstallShFetchesAndRuns(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("sh not available: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(wd, "..", "..", "install.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatal(err)
	}

	mark := t.TempDir()
	installer := installerStub(mark)
	// The stub's quoted heredoc appends one newline after the script body.
	sum := sha256.Sum256([]byte(installer + "\n"))
	stubDir := filepath.Join(mark, "bin")
	if err := os.MkdirAll(stubDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeStub(t, filepath.Join(stubDir, "curl"), curlStub(hex.EncodeToString(sum[:]), installer))

	tmp := filepath.Join(mark, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(tmp, "sysc-install.keep")
	if err := os.MkdirAll(keep, 0o777); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		exit string
	}{
		{name: "success", exit: "0"},
		{name: "failure status passes through", exit: "3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(sh, script, "--city", "Testville")
			cmd.Env = append(envWithPath(stubDir, script),
				"TMPDIR="+tmp, "STUB_EXIT="+tc.exit)
			out, err := cmd.CombinedOutput()
			exitCode := 0
			if err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("run install.sh: %v\n%s", err, out)
				}
				exitCode = exitErr.ExitCode()
			}
			want := 0
			if tc.exit != "0" {
				want = 3
			}
			if exitCode != want {
				t.Fatalf("exit = %d, want %d\n%s", exitCode, want, out)
			}

			args := readMark(t, mark, "args")
			if !strings.Contains(args, "--yes\n--city\nTestville\n") {
				t.Fatalf("installer args:\n%s\nwant --yes before user args (no tty here)", args)
			}
			mode := strings.TrimSpace(readMark(t, mark, "mode"))
			if mode != "700" {
				t.Fatalf("staging dir mode = %q, want 700", mode)
			}

			entries, err := os.ReadDir(tmp)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "sysc-install.") && e.Name() != "sysc-install.keep" {
					t.Fatalf("staging dir %s survived the run", e.Name())
				}
			}
			if _, err := os.Stat(keep); err != nil {
				t.Fatalf("pre-existing unrelated dir was touched: %v", err)
			}
		})
	}
}

func installerStub(mark string) string {
	return strings.NewReplacer(
		"__MARK__", filepath.Join(mark, "args"),
		"__MODE__", filepath.Join(mark, "mode"),
	).Replace(`#!/bin/sh
printf '%s\n' "$@" > "__MARK__"
stat -c %a "$(dirname "$0")" > "__MODE__"
exit "${STUB_EXIT:-3}"
`)
}

func curlStub(hash, installer string) string {
	return strings.NewReplacer(
		"__HASH__", hash,
		"__INSTALLER__", installer,
	).Replace(`#!/bin/sh
out=
url=
while [ $# -gt 0 ]; do
  case "$1" in
    -fsSL) shift; url=$1 ;;
    -o) shift; out=$1 ;;
  esac
  shift
done
case "$url" in
  */SHA256SUMS)
    printf '%s  sysc-linux-amd64\n' "__HASH__" > "$out"
    ;;
  */sysc-linux-amd64)
    cat > "$out" << 'END_INSTALLER'
__INSTALLER__
END_INSTALLER
    ;;
  *)
    echo "unexpected url: $url" >&2
    exit 22
    ;;
esac
`)
}

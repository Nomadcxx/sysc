package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDotInstallPipedKeepsScriptStdin guards the curl | sh entry point.
// Bash reads a piped script from stdin as it runs, so the shell must not
// steal that fd for /dev/tty. With a pty the built installer is interactive
// and its stdin is the terminal; with no terminal it falls back to --yes.
func TestDotInstallPipedKeepsScriptStdin(t *testing.T) {
	for _, name := range []string{"bash", "dash", "script", "setsid"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("%s is required: %v", name, err)
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	install := filepath.Join(wd, "..", "..", ".install")
	if _, err := os.Stat(install); err != nil {
		t.Fatal(err)
	}

	shells := []struct {
		name  string
		inner string
	}{
		{name: "bash --posix", inner: "bash --posix"},
		{name: "dash", inner: "dash"},
	}
	for _, sh := range shells {
		t.Run(sh.name, func(t *testing.T) {
			t.Run("tty", func(t *testing.T) {
				runDotInstall(t, install, sh.inner, true)
			})
			t.Run("no-tty", func(t *testing.T) {
				runDotInstall(t, install, sh.inner, false)
			})
		})
	}
}

func runDotInstall(t *testing.T, install, inner string, tty bool) {
	t.Helper()
	mark := t.TempDir()
	binDir := filepath.Join(mark, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeStub(t, filepath.Join(binDir, "git"), gitStub(mark))
	writeStub(t, filepath.Join(binDir, "go"), goStub(mark))

	pwn := filepath.Join(mark, "pwned")
	pipeline := fmt.Sprintf(`cat "$DOT_INSTALL" | %s -s -- --city Testville`, inner)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if tty {
		// script allocates a pty. A command on that pty must not be
		// executed by the shell that is still reading .install.
		cmd = exec.CommandContext(ctx, "script", "-qe", "-c", pipeline, "/dev/null")
		cmd.Stdin = strings.NewReader("touch " + pwn + "\n")
	} else {
		// setsid drops the controlling terminal, so /dev/tty cannot be opened.
		outer := "dash"
		args := []string{"-c", pipeline}
		if strings.HasPrefix(inner, "bash") {
			outer = "bash"
			args = []string{"--posix", "-c", pipeline}
		}
		cmd = exec.CommandContext(ctx, "setsid", append([]string{outer}, args...)...)
		cmd.Stdin = bytes.NewReader(nil)
	}
	cmd.Env = envWithPath(binDir, install)
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run %s tty=%v: %v\n%s", inner, tty, err, out)
		}
		exitCode = exitErr.ExitCode()
	}
	// The stub installer exits 7. Reaching that status means the script
	// ran the binary and then cleaned up, instead of dying on exec </dev/tty
	// or treating the pty as the rest of the script.
	if exitCode != 7 {
		t.Fatalf("exit = %d, want 7 (installer status)\n%s", exitCode, out)
	}

	argsText := readMark(t, mark, "args")
	stdinKind := strings.TrimSpace(readMark(t, mark, "stdin"))
	if tty {
		if argsText != "--city\nTestville\n" {
			t.Fatalf("installer args:\n%s\nwant --city and Testville, without --yes", argsText)
		}
		if stdinKind != "tty" {
			t.Fatalf("installer stdin = %q, want tty\n%s", stdinKind, out)
		}
		if _, err := os.Stat(pwn); err == nil {
			t.Fatalf("pty input was executed as a shell command\n%s", out)
		}
	} else {
		if argsText != "--yes\n--city\nTestville\n" {
			t.Fatalf("installer args:\n%s\nwant --yes before the user args", argsText)
		}
		if stdinKind != "notty" {
			t.Fatalf("installer stdin = %q, want notty\n%s", stdinKind, out)
		}
		if strings.Contains(string(out), "/dev/tty") {
			t.Fatalf("missing tty was reported instead of falling back to --yes\n%s", out)
		}
	}

	dest := strings.TrimSpace(readMark(t, mark, "clone-dest"))
	if dest == "" {
		t.Fatal("git stub was not invoked")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("clone dir %s still exists after install", dest)
	}
	if _, err := os.Stat(filepath.Dir(dest)); !os.IsNotExist(err) {
		t.Fatalf("temp dir %s still exists after install", filepath.Dir(dest))
	}
}

func envWithPath(binDir, install string) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "PATH=") {
			continue
		}
		env = append(env, e)
	}
	return append(env, "PATH="+binDir+":"+os.Getenv("PATH"), "DOT_INSTALL="+install)
}

func writeStub(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readMark(t *testing.T, mark, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(mark, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func gitStub(mark string) string {
	return fmt.Sprintf(`#!/bin/sh
if [ "$1" != "clone" ]; then
  echo "unexpected git invocation: $*" >&2
  exit 1
fi
dest=
for dest in "$@"; do
  :
done
mkdir -p "$dest"
printf '%%s\n' "$dest" > %q
`, filepath.Join(mark, "clone-dest"))
}

func goStub(mark string) string {
	return fmt.Sprintf(`#!/bin/sh
out=
prev=
for a in "$@"; do
  if [ "$prev" = "-o" ]; then
    out=$a
  fi
  prev=$a
done
if [ -z "$out" ]; then
  echo "go stub: missing -o" >&2
  exit 1
fi
cat > "$out" << 'END'
#!/bin/sh
printf '%%s\n' "$@" > %q
if [ -t 0 ]; then echo tty; else echo notty; fi > %q
exit 7
END
chmod +x "$out"
`, filepath.Join(mark, "args"), filepath.Join(mark, "stdin"))
}

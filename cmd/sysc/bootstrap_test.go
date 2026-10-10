package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapsPreserveUninstallSubcommand(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".install", "install.sh"} {
		t.Run(name, func(t *testing.T) {
			mark := t.TempDir()
			bin := filepath.Join(mark, "bin")
			if err := os.MkdirAll(bin, 0755); err != nil {
				t.Fatal(err)
			}
			writeStub(t, filepath.Join(bin, "git"), gitStub(mark))
			writeStub(t, filepath.Join(bin, "go"), goStub(mark))
			installer := installerStub(mark)
			hash := sha256.Sum256([]byte(installer + "\n"))
			writeStub(t, filepath.Join(bin, "curl"), curlStub(hex.EncodeToString(hash[:]), installer))
			script := filepath.Join(wd, "../..", name)
			cmd := exec.Command("setsid", "sh", script, "uninstall", "--purge")
			cmd.Env = envWithPath(bin, script)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("expected stub exit: %v %s", err, output)
			}
			args := readMark(t, mark, "args")
			if args != "uninstall\n--yes\n--purge\n" {
				t.Fatalf("uninstall must remain the first argument: args=%q", args)
			}
		})
	}
}

func TestCLIRejectsUnexpectedArguments(t *testing.T) {
	for _, parse := range []struct {
		name string
		fn   func([]string, io.Writer) (options, error)
	}{
		{"install", parseFlags},
		{"uninstall", parseUninstall},
	} {
		for _, args := range [][]string{{"--yes", "uninstall"}, {"typo", "--yes"}, {"--", "unexpected"}} {
			t.Run(parse.name+"/"+args[len(args)-1], func(t *testing.T) {
				if _, err := parse.fn(args, io.Discard); err == nil {
					t.Fatalf("accepted unexpected positional arguments: %q", args)
				}
			})
		}
	}
}

func TestUninstallReportsUnexpectedArguments(t *testing.T) {
	var out bytes.Buffer
	if code := runUninstall([]string{"typo"}, strings.NewReader(""), &out, t.TempDir()); code != 2 {
		t.Fatalf("uninstall exit = %d, want usage error 2", code)
	}
	if !strings.Contains(out.String(), "unexpected arguments: typo") {
		t.Fatalf("uninstall hid its usage error: %q", out.String())
	}
}

func TestDotInstallCleansUpBuildFailures(t *testing.T) {
	for _, fail := range []string{"git", "go"} {
		t.Run(fail, func(t *testing.T) {
			mark := t.TempDir()
			bin := filepath.Join(mark, "bin")
			if err := os.MkdirAll(bin, 0755); err != nil {
				t.Fatal(err)
			}
			writeStub(t, filepath.Join(bin, "git"), gitStub(mark))
			writeStub(t, filepath.Join(bin, "go"), goStub(mark))
			if fail == "git" {
				writeStub(t, filepath.Join(bin, "git"), gitStub(mark)+"exit 9\n")
			} else {
				writeStub(t, filepath.Join(bin, "go"), "#!/bin/sh\nexit 9\n")
			}
			cmd := exec.Command("setsid", "sh", "../../.install")
			cmd.Env = envWithPath(bin, "../../.install")
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 9 {
				t.Fatalf("bootstrap exit: %v\n%s", err, out)
			}
			dest := strings.TrimSpace(readMark(t, mark, "clone-dest"))
			if _, err := os.Stat(filepath.Dir(dest)); !os.IsNotExist(err) {
				t.Fatalf("temp directory remains after %s failure: %s", fail, filepath.Dir(dest))
			}
		})
	}
}

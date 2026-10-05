package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/install"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

func TestLiveNiriSession(t *testing.T) {
	if !liveNiriSession("wayland-1", "/run/user/1000/niri.sock") {
		t.Fatal("wayland display and niri socket is a live niri session")
	}
	if liveNiriSession("", "") || liveNiriSession("", "/run/user/1000/niri.sock") {
		t.Fatal("missing WAYLAND_DISPLAY is the SSH/TTY enable-only path")
	}
	if liveNiriSession("wayland-1", "") {
		t.Fatal("wayland without NIRI_SOCKET is not a niri session")
	}
}

func TestPrintTasksSaysUnitsEnabledButNotStarted(t *testing.T) {
	var buf bytes.Buffer
	printTasks(&buf, install.Result{
		Tasks:          []install.Task{{Name: "sysc-shell", Status: install.Done}},
		Stamp:          stamp.Stamp{Release: "v0.1.0", Started: false},
		SessionWarning: "start Niri with niri-session",
	})
	out := buf.String()
	if !strings.Contains(out, "sysc-shell: done") {
		t.Fatalf("task row missing:\n%s", out)
	}
	if !strings.Contains(out, "enabled but not started") {
		t.Fatalf("output hid that units were not started:\n%s", out)
	}
	if !strings.Contains(out, "niri-session") {
		t.Fatalf("output hid the niri-session warning:\n%s", out)
	}

	buf.Reset()
	printTasks(&buf, install.Result{
		Tasks: []install.Task{{Name: "sysc-shell", Status: install.Done}},
		Stamp: stamp.Stamp{Release: "v0.1.0", Started: true},
	})
	if strings.Contains(buf.String(), "enabled but not started") {
		t.Fatalf("started install reported units were not started:\n%s", buf.String())
	}

	buf.Reset()
	printTasks(&buf, install.Result{
		Tasks: []install.Task{{Name: "sysc-shell", Status: install.Done}},
	})
	if strings.Contains(buf.String(), "enabled but not started") {
		t.Fatalf("uninstall-style result reported units were not started:\n%s", buf.String())
	}
}

func TestParseFlags(t *testing.T) {
	o, err := parseFlags([]string{"--yes", "--city", "Berlin", "--lang", "de"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Yes || o.City != "Berlin" || o.Lang != "de" {
		t.Fatalf("options = %+v", o)
	}
	o, err = parseFlags([]string{"--lat", "52.52", "--lon", "13.405"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if o.Lat != 52.52 || o.Lon != 13.405 {
		t.Fatalf("options = %+v", o)
	}
	if _, err := parseFlags([]string{"--nope"}, io.Discard); err == nil {
		t.Fatal("unknown flag accepted")
	}
}

func TestInstallShPicksAmd64(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "curl.log")
	stub := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stub("uname", "#!/bin/sh\necho x86_64\n")
	stub("curl", "#!/bin/sh\necho \"$@\" >> "+log+"\nfor last; do :; done\ncase \"$last\" in\n  *SHA256SUMS) printf '0000000000000000000000000000000000000000000000000000000000000000  sysc-linux-amd64\\n' > \"$last\" ;;\n  *) printf '#!/bin/sh\\nexit 0\\n' > \"$last\"; chmod +x \"$last\" ;;\nesac\n")
	stub("sha256sum", "#!/bin/sh\nexit 0\n")

	cmd := exec.Command("sh", "../../install.sh")
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sysc-linux-amd64") {
		t.Fatalf("curl did not fetch the amd64 asset:\n%s", data)
	}
}

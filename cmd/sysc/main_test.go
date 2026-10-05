package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFlags(t *testing.T) {
	o, err := parseFlags([]string{"--yes", "--city", "Berlin", "--lang", "de"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.Yes || o.City != "Berlin" || o.Lang != "de" {
		t.Fatalf("options = %+v", o)
	}
	o, err = parseFlags([]string{"--lat", "52.52", "--lon", "13.405"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Lat != 52.52 || o.Lon != 13.405 {
		t.Fatalf("options = %+v", o)
	}
	if _, err := parseFlags([]string{"--nope"}); err == nil {
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

package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

func runtimeInstallOptions(t *testing.T) Options {
	t.Helper()
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME"} {
		t.Setenv(key, "")
	}
	return Options{
		Home: setupHome(t), Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download:  func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:      func(string, string, []string) error { return nil },
		LookPath:  func(string) (string, error) { return "/usr/bin/gslapper", nil },
		Systemctl: noopSystemctl, Now: time.Unix(1000, 0),
	}
}

func TestRuntimeCheckRunsAfterVerifiedDownloadsBeforeChanges(t *testing.T) {
	for _, checksumOK := range []bool{true, false} {
		name := "runtime refusal"
		if !checksumOK {
			name = "checksum refusal"
		}
		t.Run(name, func(t *testing.T) {
			opts := runtimeInstallOptions(t)
			const payload = "verified release executable"
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(payload)) }))
			defer server.Close()
			sum := sha256.Sum256([]byte(payload))
			digest := hex.EncodeToString(sum[:])
			if !checksumOK {
				digest = strings.Repeat("0", 64)
			}
			for i := range opts.Pin.Components {
				for j := range opts.Pin.Components[i].Binaries {
					opts.Pin.Components[i].Binaries[j].Assets["amd64"] = pin.Asset{URL: server.URL + "/release", SHA256: digest}
				}
			}
			opts.Download = nil
			opts.Client = server.Client()
			original := map[string]string{
				filepath.Join(opts.binDir(), "sysc-shell"):          "old executable",
				opts.configPath():                                   `{"session":{"locker":"sysc-lock"},"tray":{"enabled":false}}`,
				filepath.Join(opts.unitDir(), "sysc-shell.service"): "old unit",
				filepath.Join(opts.niriDir(), "config.kdl"):         "input {}\n",
			}
			for path, data := range original {
				writeFile(t, path, data)
			}
			checked := false
			refusal := errors.New("missing runtime prerequisite")
			opts.CheckRuntime = func(staging string, names []string) ([]string, error) {
				checked = true
				if len(names) == 0 {
					t.Fatal("runtime check received no downloads")
				}
				for _, name := range names {
					data, err := os.ReadFile(filepath.Join(staging, name))
					if err != nil || string(data) != payload {
						t.Fatalf("runtime check before verified staging: %s %q %v", name, data, err)
					}
				}
				return nil, refusal
			}
			opts.Systemctl = func(args ...string) error {
				t.Fatalf("systemctl called before prerequisite refusal: %v", args)
				return nil
			}
			opts.Swap = func(string, string, []string) error { t.Fatal("swap called before prerequisite refusal"); return nil }
			_, err := Run(context.Background(), opts)
			if err == nil {
				t.Fatal("install accepted failed preflight")
			}
			if checksumOK && !errors.Is(err, refusal) {
				t.Fatalf("runtime refusal lost: %v", err)
			}
			if checked != checksumOK {
				t.Fatalf("runtime check called %v with valid checksum %v", checked, checksumOK)
			}
			for path, want := range original {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatalf("existing %s changed: %q %v", path, got, err)
				}
			}
			if exists(filepath.Join(opts.stateDir(), stamp.FileName)) {
				t.Fatal("failed preflight wrote installed stamp")
			}
			if exists(filepath.Join(opts.binDir(), "sysc-clipboard")) {
				t.Fatal("failed preflight installed another binary")
			}
			if exists(filepath.Join(opts.niriDir(), "sysc.kdl")) {
				t.Fatal("failed preflight changed niri integration")
			}
		})
	}
}

func TestRuntimeWarningsReachResult(t *testing.T) {
	opts := runtimeInstallOptions(t)
	const warning = "Unlock the clipboard keyring at login."
	checked := false
	opts.CheckRuntime = func(string, []string) ([]string, error) { checked = true; return []string{warning}, nil }
	opts.Swap = func(string, string, []string) error {
		if !checked {
			t.Fatal("swap before runtime check")
		}
		return nil
	}
	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Warnings, warning) {
		t.Fatalf("runtime warning lost: %v", res.Warnings)
	}
}

func TestEmbeddedTerminalIsInstalledWithoutUserUnit(t *testing.T) {
	opts := runtimeInstallOptions(t)
	p, err := pin.Load()
	if err != nil {
		t.Fatal(err)
	}
	opts.Pin = p
	fetched, swapped := false, false
	opts.Download = func(_ context.Context, _ string, assets []fetch.Asset) error {
		for _, asset := range assets {
			if asset.Name == "sysc-terminal" {
				fetched = true
			}
		}
		return nil
	}
	opts.Swap = func(_ string, _ string, names []string) error {
		swapped = slices.Contains(names, "sysc-terminal")
		return nil
	}
	opts.Systemctl = func(args ...string) error {
		for _, arg := range args {
			if strings.Contains(arg, "sysc-terminal") {
				t.Fatalf("terminal received a user unit: %v", args)
			}
		}
		return nil
	}
	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !fetched || !swapped {
		t.Fatalf("terminal fetched %v swapped %v", fetched, swapped)
	}
	if res.Stamp.Components["sysc-terminal"] != "v0.1.0" {
		t.Fatalf("terminal not stamped: %v", res.Stamp.Components)
	}
	stored, err := stamp.Read(opts.stateDir())
	if err != nil || stored.Components["sysc-terminal"] != "v0.1.0" {
		t.Fatalf("terminal stamp not persisted: %+v %v", stored, err)
	}
	if exists(filepath.Join(opts.unitDir(), "sysc-terminal.service")) {
		t.Fatal("terminal service written")
	}
	for _, task := range res.Tasks {
		if task.Name == "sysc-terminal" {
			if task.Status != Done {
				t.Fatalf("terminal task: %+v", task)
			}
			return
		}
	}
	t.Fatal("terminal missing from result tasks")
}

func TestEmbeddedTraySeedsNewConfigAndKeepsExistingChoice(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "new config"
		if existing {
			name = "existing config"
		}
		t.Run(name, func(t *testing.T) {
			opts := runtimeInstallOptions(t)
			p, err := pin.Load()
			if err != nil {
				t.Fatal(err)
			}
			opts.Pin = p
			const original = `{"session":{"locker":"sysc-lock"},"tray":{"enabled":false},"user-choice":true}`
			if existing {
				writeFile(t, opts.configPath(), original)
			}
			if _, err := Run(context.Background(), opts); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(opts.configPath())
			if err != nil {
				t.Fatal(err)
			}
			if existing {
				if string(data) != original {
					t.Fatalf("existing config changed: %s", data)
				}
				return
			}
			var cfg struct {
				Tray struct {
					Enabled bool `json:"enabled"`
				} `json:"tray"`
			}
			if err := json.Unmarshal(data, &cfg); err != nil {
				t.Fatal(err)
			}
			if !cfg.Tray.Enabled {
				t.Fatalf("new config did not enable installed tray: %s", data)
			}
		})
	}
}

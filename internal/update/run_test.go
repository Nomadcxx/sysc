package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/stamp"
)

var errChecksum = errors.New("checksum mismatch for sysc-linux-amd64: got a want b")

const goodSum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// fetchEnv builds a stamped install whose Download seam only ever writes bytes,
// so the parent half is exercised without a release server.
func fetchEnv(t *testing.T) (stateDir string, seen *[]string) {
	t.Helper()
	stateDir = filepath.Join(t.TempDir(), "sysc")
	if err := stamp.Write(stateDir, stamp.Stamp{Release: "v0.1.3"}, true); err != nil {
		t.Fatal(err)
	}
	seen = &[]string{}
	return stateDir, seen
}

func writeDownload(t *testing.T, sums string, installer string) func(context.Context, string, string, string, string) error {
	t.Helper()
	return func(_ context.Context, dir, url, name, sha string) error {
		body := sums
		if name != "SHA256SUMS" {
			if sha == "" {
				t.Errorf("%s was fetched without a digest", name)
			}
			body = installer
		}
		return os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755)
	}
}

func TestFetchRefusesWithoutAStamp(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "sysc")
	var out, errOut strings.Builder
	downloaded := false
	got := Fetch(context.Background(), Options{
		StateDir: stateDir, Arch: "amd64", Out: &out, ErrOut: &errOut,
		RefuseNoStamp: "No SYSC install stamp at %s.",
		Download: func(context.Context, string, string, string, string) error {
			downloaded = true
			return nil
		},
		Run: func(context.Context, string, ...string) int { return 0 },
	})
	if got != 1 {
		t.Fatalf("Fetch without a stamp = %d, want 1", got)
	}
	if downloaded {
		t.Fatal("Fetch downloaded without a stamp")
	}
	want := "No SYSC install stamp at " + filepath.Join(stateDir, stamp.FileName) + ".\n"
	if errOut.String() != want {
		t.Fatalf("Fetch printed %q, want %q", errOut.String(), want)
	}
}

func TestFetchRefusesANonAmd64Host(t *testing.T) {
	stateDir, _ := fetchEnv(t)
	var out, errOut strings.Builder
	downloaded := false
	got := Fetch(context.Background(), Options{
		StateDir: stateDir, Arch: "arm64", Out: &out, ErrOut: &errOut,
		RefuseDistro: "Arch-family distributions only.",
		Download: func(context.Context, string, string, string, string) error {
			downloaded = true
			return nil
		},
	})
	if got != 1 {
		t.Fatalf("Fetch on arm64 = %d, want 1", got)
	}
	if downloaded {
		t.Fatal("Fetch downloaded on a host the pin cannot install")
	}
	if errOut.String() != "Arch-family distributions only.\n" {
		t.Fatalf("Fetch printed %q, want the named refusal", errOut.String())
	}
}

func TestFetchRefusesAFailedInstallerDownload(t *testing.T) {
	stateDir, _ := fetchEnv(t)
	var out, errOut strings.Builder
	started := false
	stagingDir := ""
	got := Fetch(context.Background(), Options{
		StateDir: stateDir, Arch: "amd64", Out: &out, ErrOut: &errOut,
		RefuseNoStamp: "No SYSC install stamp at %s.",
		Download: func(_ context.Context, dir, _, name, _ string) error {
			stagingDir = filepath.Dir(dir)
			if name != "SHA256SUMS" {
				return errChecksum
			}
			return os.WriteFile(filepath.Join(dir, name), []byte(goodSum+"  sysc-linux-amd64\n"), 0o755)
		},
		Run: func(context.Context, string, ...string) int { started = true; return 0 },
	})
	if got != 1 {
		t.Fatalf("Fetch with a failed download = %d, want 1", got)
	}
	if started {
		t.Fatal("Fetch ran an installer it could not verify")
	}
	if !strings.Contains(errOut.String(), "checksum mismatch") {
		t.Fatalf("Fetch printed %q, want the fetch client mismatch", errOut.String())
	}
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("Fetch left %d entries in %s", len(entries), stagingDir)
	}
}

func TestFetchHandsTheFetchedInstallerTheWork(t *testing.T) {
	stateDir, _ := fetchEnv(t)
	var out, errOut strings.Builder
	var gotPath string
	var gotArgs []string
	downloaded := map[string]string{}
	got := Fetch(context.Background(), Options{
		StateDir: stateDir, Arch: "amd64", Yes: true, Out: &out, ErrOut: &errOut,
		Base:          "https://example.invalid/dl",
		RefuseNoStamp: "No SYSC install stamp at %s.",
		Download: func(_ context.Context, dir, url, name, sha string) error {
			downloaded[name] = url
			return os.WriteFile(filepath.Join(dir, name), []byte(goodSum+"  sysc-linux-amd64\n"), 0o755)
		},
		Run: func(_ context.Context, path string, args ...string) int {
			gotPath, gotArgs = path, args
			info, err := os.Stat(path)
			if err != nil {
				t.Errorf("the fetched installer is missing: %v", err)
			} else if info.Mode().Perm() != 0o700 {
				t.Errorf("the fetched installer is mode %o, want 700", info.Mode().Perm())
			}
			return 3
		},
	})
	if got != 3 {
		t.Fatalf("Fetch = %d, want the child exit code", got)
	}
	if gotArgs == nil || strings.Join(gotArgs, " ") != "update --apply --yes" {
		t.Fatalf("Fetch ran %v, want [update --apply --yes]", gotArgs)
	}
	if filepath.Base(gotPath) != "sysc-linux-amd64" || !strings.HasPrefix(filepath.Base(filepath.Dir(gotPath)), "update-") {
		t.Fatalf("Fetch ran %q, want the fetched asset inside a fresh update- dir", gotPath)
	}
	if downloaded["sysc-linux-amd64"] != "https://example.invalid/dl/sysc-linux-amd64" {
		t.Fatalf("Fetch fetched %q", downloaded["sysc-linux-amd64"])
	}
	if _, err := os.Stat(gotPath); err == nil {
		t.Fatal("Fetch left the fetched installer behind")
	}
}

func TestFetchRunsWithoutYes(t *testing.T) {
	stateDir, _ := fetchEnv(t)
	var out, errOut strings.Builder
	var gotArgs []string
	Fetch(context.Background(), Options{
		StateDir: stateDir, Arch: "amd64", Out: &out, ErrOut: &errOut,
		RefuseNoStamp: "No SYSC install stamp at %s.",
		Download:      writeDownload(t, goodSum+"  sysc-linux-amd64\n", "installer"),
		Run: func(_ context.Context, _ string, args ...string) int {
			gotArgs = args
			return 0
		},
	})
	if strings.Join(gotArgs, " ") != "update --apply" {
		t.Fatalf("Fetch ran %v, want [update --apply]", gotArgs)
	}
}

func TestFetchChecksTheStampBeforeTheArch(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "sysc") // never installed
	var out, errOut strings.Builder
	got := Fetch(context.Background(), Options{
		StateDir:      stateDir,
		Arch:          "arm64",
		Out:           &out,
		ErrOut:        &errOut,
		RefuseDistro:  "Arch-family distributions only.",
		RefuseNoStamp: "No SYSC install stamp at %s.",
	})
	if got != 1 {
		t.Fatalf("Fetch without a stamp = %d, want 1", got)
	}
	want := "No SYSC install stamp at " + filepath.Join(stateDir, stamp.FileName) + ".\n"
	if errOut.String() != want {
		t.Fatalf("Fetch printed %q, want %q", errOut.String(), want)
	}
}

func TestFetchSweepsAStaleStagingDir(t *testing.T) {
	stateDir, _ := fetchEnv(t)
	stale := filepath.Join(stateDir, "staging", "update-stale")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	got := Fetch(context.Background(), Options{
		StateDir: stateDir, Arch: "amd64", Out: &out, ErrOut: &errOut,
		RefuseNoStamp: "No SYSC install stamp at %s.",
		Download:      writeDownload(t, goodSum+"  sysc-linux-amd64\n", "installer"),
		Run:           func(context.Context, string, ...string) int { return 0 },
	})
	if got != 0 {
		t.Fatalf("Fetch = %d, want 0 (stderr: %s)", got, errOut.String())
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("a stale staging dir survived the next run")
	}
}

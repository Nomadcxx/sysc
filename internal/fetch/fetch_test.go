package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestChecksumMismatchDoesNotWrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	staging := t.TempDir()
	wrong := sha256.Sum256([]byte("world"))
	err := DownloadAll(context.Background(), srv.Client(), staging, []Asset{{
		Name:   "sysc-shell",
		URL:    srv.URL,
		SHA256: hex.EncodeToString(wrong[:]),
	}})
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	entries, err := os.ReadDir(staging)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("staging not empty after mismatch: %v", entries)
	}
}

func TestSwapRestoresBakOnFailure(t *testing.T) {
	binDir := t.TempDir()
	staging := t.TempDir()
	for _, name := range []string{"one", "two"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("old-"+name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(staging, name), []byte("new-"+name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	orig := rename
	rename = func(oldpath, newpath string) error {
		if filepath.Base(newpath) == "two" {
			return os.ErrPermission
		}
		return orig(oldpath, newpath)
	}
	defer func() { rename = orig }()

	err := SwapAll(binDir, staging, []string{"one", "two"})
	if err == nil {
		t.Fatal("expected swap failure")
	}

	got, err := os.ReadFile(filepath.Join(binDir, "one"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-one" {
		t.Fatalf("first binary not restored: %q", got)
	}
	got, err = os.ReadFile(filepath.Join(binDir, "two"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-two" {
		t.Fatalf("second binary changed: %q", got)
	}
	entries, err := os.ReadDir(binDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".new" {
			t.Fatalf("leftover staging file %s", e.Name())
		}
	}
}

// A leftover name.bak from an earlier install is not this swap's backup.
// When the destination did not exist, a later failure must remove the new
// binary instead of copying that stale backup back into place.
func TestStaleBakNotRestoredOnFreshSwapFailure(t *testing.T) {
	binDir := t.TempDir()
	staging := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "one.bak"), []byte("stale-one"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "one"), []byte("new-one"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := SwapAll(binDir, staging, []string{"one", "two"})
	if err == nil {
		t.Fatal("expected swap failure")
	}
	if data, err := os.ReadFile(filepath.Join(binDir, "one")); err == nil {
		t.Fatalf("fresh swap left binary %q; want it removed", data)
	}
}

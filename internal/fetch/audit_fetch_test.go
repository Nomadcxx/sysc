package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func auditBody() []byte { return []byte("ELFfakebinary") }

func auditSHA(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func auditServer(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 && status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Fix contract: traversal names must be refused loudly or contained.
func TestAuditAssetNameTraversesOutOfStaging(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	body := auditBody()
	srv := auditServer(t, http.StatusOK, body)

	err := DownloadAll(context.Background(), srv.Client(), staging, []Asset{
		{Name: "../evil", URL: srv.URL, SHA256: auditSHA(body)},
	})
	if err == nil {
		t.Log("DownloadAll accepted the name; checking containment")
	} else {
		t.Logf("refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); err == nil {
		t.Errorf("AUDIT: staged download escaped staging dir: wrote %s", filepath.Join(dir, "evil"))
	}
}

func TestAuditSwapAllNameTraversesOutOfBinDir(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(binDir, "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "evil"), auditBody(), 0o755); err != nil {
		t.Fatal(err)
	}
	err := SwapAll(binDir, staging, []string{"../evil"})
	if err == nil {
		t.Log("SwapAll accepted the name; checking containment")
	} else {
		t.Logf("refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); err == nil {
		t.Errorf("AUDIT: swap wrote outside binDir: %s", filepath.Join(dir, "evil"))
	}
}

// Design: download ALL + verify BEFORE any rename. Within one SwapAll list a
// mid-list failure on a fresh install rolls back to nothing and leaves the
// earlier binary swapped with no .bak — an orphan with no stamp to clean it.
func TestAuditRollbackOrphanOnFreshInstall(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	staging := filepath.Join(dir, "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "one"), []byte("bin-one"), 0o755); err != nil {
		t.Fatal(err)
	}
	// "two" deliberately absent from staging -> swapOne fails at open.
	err := SwapAll(binDir, staging, []string{"one", "two"})
	if err == nil {
		t.Fatal("SwapAll succeeded despite missing staged file")
	}
	if _, statErr := os.Stat(filepath.Join(binDir, "one")); statErr == nil {
		if _, bakErr := os.Stat(filepath.Join(binDir, "one.bak")); bakErr != nil {
			t.Errorf("AUDIT: orphaned %s in binDir with no .bak and a returned error (no stamp would exist either)",
				filepath.Join(binDir, "one"))
		}
	}
	entries, _ := os.ReadDir(binDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".new") {
			t.Errorf("AUDIT: leftover temp in binDir: %s", e.Name())
		}
	}
}

func TestAuditChecksumCaseInsensitiveAccepted(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	body := auditBody()
	srv := auditServer(t, http.StatusOK, body)
	upper := strings.ToUpper(auditSHA(body))
	err := DownloadAll(context.Background(), srv.Client(), staging, []Asset{
		{Name: "prog", URL: srv.URL, SHA256: upper},
	})
	if err != nil {
		t.Fatalf("DownloadAll with uppercase sha: %v", err)
	}
	if _, err := os.Stat(filepath.Join(staging, "prog")); err != nil {
		t.Fatalf("expected staged file: %v", err)
	}
	t.Logf("AUDIT-INFO: EqualFold accepted uppercase pin sha; not fail-closed on case variant")
}

func TestAuditShortAndNonHexSHAFailClosed(t *testing.T) {
	dir := t.TempDir()
	body := auditBody()
	srv := auditServer(t, http.StatusOK, body)
	for i, want := range []string{"abc123", "00", strings.Repeat("z", 64)} {
		staging := filepath.Join(dir, "st-0"+string(rune('0'+i)))
		err := DownloadAll(context.Background(), srv.Client(), staging, []Asset{
			{Name: "prog", URL: srv.URL, SHA256: want},
		})
		if err == nil {
			t.Errorf("pin sha %q accepted", want)
			continue
		}
		entries, _ := os.ReadDir(staging)
		for _, e := range entries {
			t.Errorf("mismatch left file behind: %s", e.Name())
		}
	}
}

func TestAudit404LeavesNoStagingFiles(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	srv := auditServer(t, http.StatusNotFound, nil)
	err := DownloadAll(context.Background(), srv.Client(), staging, []Asset{
		{Name: "prog", URL: srv.URL, SHA256: auditSHA(auditBody())},
	})
	if err == nil {
		t.Fatal("404 accepted")
	}
	entries, _ := os.ReadDir(staging)
	if len(entries) != 0 {
		t.Errorf("404 left %d files in staging", len(entries))
	}
}

// No size cap anywhere: a hostile/misconfigured CDN can fill the disk.
func TestAuditNoDownloadSizeCap(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	body := make([]byte, 4<<20) // 4 MiB
	for i := range body {
		body[i] = byte(i)
	}
	srv := auditServer(t, http.StatusOK, body)
	err := DownloadAll(context.Background(), srv.Client(), staging, []Asset{
		{Name: "huge", URL: srv.URL, SHA256: auditSHA(body)},
	})
	if err != nil {
		t.Fatalf("4MiB download failed: %v", err)
	}
	st, err := os.Stat(filepath.Join(staging, "huge"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("AUDIT-INFO: unbounded download accepted, %d bytes staged (no cap)", st.Size())
}

// Fix contract: a symlinked destination is never silently replaced; the swap
// refuses and the symlink stays intact.
func TestAuditSwapAllReplacesSymlinkedBinary(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	staging := filepath.Join(dir, "staging")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "prog"), []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(dir, "real-prog")
	if err := os.WriteFile(real, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(binDir, "prog")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := SwapAll(binDir, staging, []string{"prog"}); err == nil {
		t.Fatal("SwapAll replaced a symlinked binary without refusing")
	} else {
		t.Logf("refused: %v", err)
	}
	if _, err := os.ReadDir(binDir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(binDir)
	for _, e := range entries {
		if e.Name() != "prog" && !strings.HasPrefix(e.Name(), ".") {
			t.Errorf("leftover during refused swap: %s", e.Name())
		}
	}
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("symlink vanished: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("AUDIT: user symlink %s silently replaced by regular file", link)
	}
	data, err := os.ReadFile(real)
	if err != nil || string(data) != "old" {
		t.Errorf("symlink target modified: %q %v", data, err)
	}
}

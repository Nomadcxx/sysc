package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAuditGapTruncatedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "200")
		_, _ = w.Write([]byte("only-forty-bytes-only-forty-bytes-only-40!!"))
	}))
	defer srv.Close()
	body := []byte("only-forty-bytes-only-forty-bytes-only-40!!")
	sum := sha256.Sum256(body)
	staging := t.TempDir()
	err := DownloadAll(context.Background(), srv.Client(), staging, []Asset{{Name: "one", URL: srv.URL + "/one", SHA256: hex.EncodeToString(sum[:])}})
	if err == nil {
		t.Errorf("AUDIT: truncated body (declared 200, sent 40) accepted: err=nil")
	}
	entries, _ := os.ReadDir(staging)
	for _, e := range entries {
		t.Logf("leftover in staging: %s", e.Name())
	}
}

func TestAuditGapAbsolutePathNameContained(t *testing.T) {
	body := []byte("evil-body")
	sum := sha256.Sum256(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	staging := t.TempDir()
	assets := []Asset{{Name: "/tmp/evil", URL: srv.URL + "/x", SHA256: hex.EncodeToString(sum[:])}}
	err := DownloadAll(context.Background(), srv.Client(), staging, assets)
	if err == nil {
		t.Fatal("absolute asset name must be refused")
	}
	if _, statErr := os.Stat("/tmp/evil"); statErr == nil {
		t.Errorf("absolute asset name wrote to real /tmp/evil")
	}
	entries, _ := os.ReadDir(staging)
	for _, e := range entries {
		t.Errorf("leftover in staging: %s", e.Name())
	}
}

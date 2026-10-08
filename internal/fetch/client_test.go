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
	"time"
)

func TestDownloadAllNilClient(t *testing.T) {
	body := []byte("binary-bytes")
	sum := sha256.Sum256(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	staging := t.TempDir()
	err := DownloadAll(context.Background(), nil, staging, []Asset{{
		Name:   "sysc-shell",
		URL:    srv.URL,
		SHA256: hex.EncodeToString(sum[:]),
	}})
	if err != nil {
		t.Fatalf("DownloadAll with nil client: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(staging, "sysc-shell"))
	if err != nil {
		t.Fatalf("staged file: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("staged bytes = %q, want %q", got, body)
	}
}

func TestNewClientCeilings(t *testing.T) {
	c := NewClient()
	if c.Timeout != 10*time.Minute {
		t.Fatalf("timeout = %v, want 10m", c.Timeout)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type %T, want *http.Transport", c.Transport)
	}
	if tr.ResponseHeaderTimeout != 30*time.Second {
		t.Fatalf("response header timeout = %v, want 30s", tr.ResponseHeaderTimeout)
	}
	if tr.TLSHandshakeTimeout != 15*time.Second {
		t.Fatalf("TLS handshake timeout = %v, want 15s", tr.TLSHandshakeTimeout)
	}
}

func TestDownloadAllRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := DownloadAll(ctx, nil, t.TempDir(), []Asset{{Name: "sysc-shell", URL: srv.URL}})
	if err == nil {
		t.Fatal("expected context error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("took %v, context not honoured", elapsed)
	}
}

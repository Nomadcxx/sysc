package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/pin"
)

func TestRunWithoutInjectedClientDoesNotPanic(t *testing.T) {
	body := []byte("shell-bytes")
	sum := sha256.Sum256(body)
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p := pin.Pin{
		Release: "vtest",
		Components: []pin.Component{{
			ID:  "sysc-shell",
			Tag: "vtest",
			Binaries: []pin.Binary{{
				Name: "sysc-shell",
				Assets: map[string]pin.Asset{
					"amd64": {URL: srv.URL, SHA256: hex.EncodeToString(sum[:])},
				},
			}},
		}},
	}
	opts := Options{
		Home:      setupHome(t),
		Pin:       p,
		Answers:   weatherAnswers(),
		Yes:       true,
		Swap:      func(string, string, []string) error { return nil },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: noopSystemctl,
		Now:       time.Unix(1000, 0),
	}
	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run without injected client: %v", err)
	}
	if !hit.Load() {
		t.Fatal("test server never saw a request")
	}
	if len(res.Tasks) == 0 {
		t.Fatal("no tasks reported")
	}
}

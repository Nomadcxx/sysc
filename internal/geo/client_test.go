package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGuessNilClientUsesDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"latitude":1.5,"longitude":2.5,"city":"Stubville"}`))
	}))
	defer srv.Close()

	p, err := Guess(context.Background(), nil, srv.URL)
	if err != nil {
		t.Fatalf("Guess with nil client: %v", err)
	}
	if p.City != "Stubville" || p.Latitude != 1.5 || p.Longitude != 2.5 {
		t.Fatalf("place = %+v", p)
	}
}

func TestSearchNilClientUsesDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"latitude":3.5,"longitude":4.5,"name":"Stubville"}]}`))
	}))
	defer srv.Close()

	p, err := Search(context.Background(), nil, srv.URL, "Stubville")
	if err != nil {
		t.Fatalf("Search with nil client: %v", err)
	}
	if p.City != "Stubville" || p.Latitude != 3.5 || p.Longitude != 4.5 {
		t.Fatalf("place = %+v", p)
	}
}

func TestGuessHangingServerHonoursContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := Guess(ctx, nil, srv.URL); err == nil {
		t.Fatal("expected context error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("took %v, context not honoured", elapsed)
	}
}

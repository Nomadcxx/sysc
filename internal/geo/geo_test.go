package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGuessResolvesStubEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"latitude":52.52,"longitude":13.405,"city":"Berlin"}`))
	}))
	defer srv.Close()
	p, err := Guess(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if p.Latitude != 52.52 || p.Longitude != 13.405 || p.City != "Berlin" {
		t.Fatalf("place = %+v", p)
	}
}

func TestGuessRejectsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"city":"Nowhere"}`))
	}))
	defer srv.Close()
	if _, err := Guess(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("Guess accepted a response without coordinates")
	}
}

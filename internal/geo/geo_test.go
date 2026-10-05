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

func TestSearchResolvesStubEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") != "Berlin" {
			t.Errorf("name query = %q", r.URL.Query().Get("name"))
		}
		w.Write([]byte(`{"results":[{"latitude":52.52,"longitude":13.405,"name":"Berlin"}]}`))
	}))
	defer srv.Close()
	p, err := Search(context.Background(), srv.Client(), srv.URL, "Berlin")
	if err != nil {
		t.Fatal(err)
	}
	if p.Latitude != 52.52 || p.Longitude != 13.405 || p.City != "Berlin" {
		t.Fatalf("place = %+v", p)
	}
}

func TestSearchRejectsNoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	if _, err := Search(context.Background(), srv.Client(), srv.URL, "Nowhere"); err == nil {
		t.Fatal("Search accepted an empty result list")
	}
}

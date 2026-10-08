package ui

import "testing"

func TestBannerHeight(t *testing.T) {
	if got := BannerHeight(); got != len(asciiHeaderLines)+2 {
		t.Fatalf("BannerHeight() = %d, want %d", got, len(asciiHeaderLines)+2)
	}
	if got := BannerHeight(); got != 11 {
		t.Fatalf("BannerHeight() = %d, want 11 for the current banner", got)
	}
}

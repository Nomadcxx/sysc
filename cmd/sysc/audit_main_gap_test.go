package main

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestAuditGapLatWithoutLon(t *testing.T) {
	o, err := parseFlags([]string{"--yes", "--lat", "95"}, io.Discard)
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	_, err = answersFor(context.Background(), o, nil)
	if err == nil || !strings.Contains(err.Error(), "--lon") {
		t.Errorf("AUDIT: --lat without --lon: err=%v (must name --lon, no network)", err)
	} else {
		t.Logf("rejected by name: %v", err)
	}
	o2, err := parseFlags([]string{"--yes", "--lon", "9"}, io.Discard)
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	_, err = answersFor(context.Background(), o2, nil)
	if err == nil || !strings.Contains(err.Error(), "--lat") {
		t.Errorf("AUDIT: --lon without --lat: err=%v (must name --lat)", err)
	}
}

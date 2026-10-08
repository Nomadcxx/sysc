package main

import (
	"io"
	"testing"
)

func TestConflictFlags(t *testing.T) {
	if _, err := parseFlags([]string{"--keep-conflicts", "--handover=all"}, io.Discard); err == nil {
		t.Fatal("--keep-conflicts with --handover=all must be rejected")
	}
	if _, err := parseFlags([]string{"--handover=keep"}, io.Discard); err == nil {
		t.Fatal("--handover=keep must be rejected")
	}
	o, err := parseFlags([]string{"--keep-conflicts"}, io.Discard)
	if err != nil {
		t.Fatalf("--keep-conflicts: %v", err)
	}
	if !o.KeepConflicts {
		t.Fatal("--keep-conflicts not recorded")
	}
	o, err = parseFlags([]string{"--handover=all"}, io.Discard)
	if err != nil {
		t.Fatalf("--handover=all: %v", err)
	}
	if o.Handover != "all" {
		t.Fatalf("handover = %q, want all", o.Handover)
	}
}

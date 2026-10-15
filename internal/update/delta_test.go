package update

import (
	"reflect"
	"testing"

	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

// pinFromTestdata reads the repository's own pin so the expected rows follow
// the real component set instead of a second hand-written list drifting from it.
func pinFromTestdata(t *testing.T) pin.Pin {
	t.Helper()
	p, err := pin.Load()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestDeltaCoversEveryEnabledComponentOnce checks the real pin: every enabled
// component the stamp does not already carry appears as an added row, and a
// disabled one never does. A hand-built pin cannot catch a component that was
// added to pin.json and forgotten here.
func TestDeltaCoversEveryEnabledComponentOnce(t *testing.T) {
	p := pinFromTestdata(t)
	prev := stamp.Stamp{Release: "v0.0.1", Components: map[string]string{}}

	seen := map[string]int{}
	for _, r := range Delta(prev, p) {
		seen[r.ID]++
	}
	for _, c := range p.Components {
		want := 1
		if c.Disabled {
			want = 0
		}
		if seen[c.ID] != want {
			t.Fatalf("component %q produced %d rows, want %d (all rows: %v)", c.ID, seen[c.ID], want, seen)
		}
		if got := seen[c.ID]; got > 0 && r0(c, p) == "" {
			t.Fatalf("component %q has no tag", c.ID)
		}
	}
	if len(seen) > 0 {
		t.Logf("rows for every real component: %v", seen)
	}
}

func r0(c pin.Component, _ pin.Pin) string { return c.Tag }

func TestDeltaNamesEveryKindOfRow(t *testing.T) {
	p := pin.Pin{Release: "v0.2.0", Components: []pin.Component{
		{ID: "kept", Tag: "v1.0.0"},
		{ID: "moved", Tag: "v2.0.0"},
		{ID: "skipped", Tag: "v3.0.0", Disabled: true, Reason: "not shipped yet"},
		{ID: "legacy", Tag: "v0.5.0"},
	}}
	prev := stamp.Stamp{Release: "v0.1.0", Components: map[string]string{
		"kept":     "v1.0.0",
		"moved":    "v1.0.0",
		"gone":     "v0.9.0",
		"unmarked": "",
		"legacy":   "",
	}}

	var got []string
	for _, r := range Delta(prev, p) {
		got = append(got, r.String())
	}
	want := []string{
		// Sorted by id: gone, legacy, moved, skipped (disabled), unmarked.
		"- gone  v0.9.0",
		"? legacy  unknown -> v0.5.0",
		"~ moved  v1.0.0 -> v2.0.0",
		"- unmarked  ",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
}

func TestDeltaIsEmptyWhenNothingMoves(t *testing.T) {
	p := pin.Pin{Release: "v0.2.0", Components: []pin.Component{{ID: "kept", Tag: "v1.0.0"}}}
	prev := stamp.Stamp{Release: "v0.1.0", Components: map[string]string{"kept": "v1.0.0"}}
	if rows := Delta(prev, p); len(rows) != 0 {
		t.Fatalf("rows = %v, want none", rows)
	}
}

func TestChecksumFor(t *testing.T) {
	sums := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  sysc-linux-amd64\nbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  sysc-linux-arm64\n")
	got, err := ChecksumFor(sums, "sysc-linux-amd64")
	if err != nil || got != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := ChecksumFor(sums, "sysc-linux-x86"); err == nil {
		t.Fatal("missing asset must refuse")
	}
}

func TestChecksumForRefusesMalformedLines(t *testing.T) {
	cases := map[string]string{
		"short digest":    "abc  sysc-linux-amd64\n",
		"not hex":         "zzz  sysc-linux-amd64\n",
		"extra column":    "aaa bbb sysc-linux-amd64\n",
		"crlf":            "aaa  sysc-linux-amd64\r\n",
		"empty asset set": "\n",
	}
	for name, sums := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ChecksumFor([]byte(sums), "sysc-linux-amd64"); err == nil {
				t.Fatalf("ChecksumFor(%q) accepted a malformed sums file", sums)
			}
		})
	}
}

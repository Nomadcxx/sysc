// Package update implements the `sysc update` verb: the per-component delta
// between the installed stamp and a target pin, and the checksum lookup that
// stands between a downloaded installer and being run.
package update

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

// Row is one component's movement between two pins. Kind is "~" changed, "+"
// added, "-" removed and "?" an installed component whose version the stamp did
// not record, which is what a stamp written by an older installer looks like.
type Row struct {
	Kind string
	ID   string
	Old  string
	New  string
}

func (r Row) String() string {
	switch r.Kind {
	case "+":
		return fmt.Sprintf("+ %s  %s", r.ID, r.New)
	case "-":
		return fmt.Sprintf("- %s  %s", r.ID, r.Old)
	case "?":
		return fmt.Sprintf("? %s  unknown -> %s", r.ID, r.New)
	default:
		return fmt.Sprintf("~ %s  %s -> %s", r.ID, r.Old, r.New)
	}
}

// Delta reports what a move from prev to next would change, sorted by
// component id. A component the pin has disabled is not part of the suite, so
// it is left out entirely rather than reported as a removal the installer
// cannot act on.
func Delta(prev stamp.Stamp, next pin.Pin) []Row {
	inPin := map[string]pin.Component{}
	for _, c := range next.Components {
		if c.Disabled {
			continue
		}
		inPin[c.ID] = c
	}
	ids := map[string]bool{}
	for id := range inPin {
		ids[id] = true
	}
	for id := range prev.Components {
		ids[id] = true
	}
	names := make([]string, 0, len(ids))
	for id := range ids {
		names = append(names, id)
	}
	sort.Strings(names)

	var rows []Row
	for _, id := range names {
		c, pinned := inPin[id]
		old, installed := prev.Components[id]
		switch {
		case pinned && !installed:
			rows = append(rows, Row{"+", id, "", c.Tag})
		case !pinned && installed:
			rows = append(rows, Row{"-", id, old, ""})
		case pinned && installed && old != c.Tag:
			if old == "" {
				rows = append(rows, Row{"?", id, "", c.Tag})
			} else {
				rows = append(rows, Row{"~", id, old, c.Tag})
			}
		}
	}
	return rows
}

// ChecksumFor returns the sha256 digest SHA256SUMS records for one asset. A
// sums file is downloaded from the same place as the asset it describes, so it
// is checked hard rather than leniently: a short digest, a non-hex digest, a
// CRLF ending, a line with extra columns and a missing asset are all refused.
func ChecksumFor(sums []byte, asset string) (string, error) {
	for _, line := range strings.Split(string(sums), "\n") {
		if strings.ContainsRune(line, '\r') {
			return "", fmt.Errorf("SHA256SUMS: a line carries a carriage return")
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			if strings.TrimSpace(line) == "" {
				continue
			}
			return "", fmt.Errorf("SHA256SUMS: %q is not a digest and a name", line)
		}
		if fields[1] != asset {
			continue
		}
		if !isSHA256(fields[0]) {
			return "", fmt.Errorf("SHA256SUMS: %q is not a sha256 digest", fields[0])
		}
		return fields[0], nil
	}
	return "", fmt.Errorf("SHA256SUMS has no entry for %s", asset)
}

func isSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}

// Package backup keeps the first copy of every user file SYSC mutates.
package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxStateCopies = 5

// FirstBak copies path to path+".sysc.bak" unless that backup already exists.
// The first backup is sacred: later calls never overwrite it.
func FirstBak(path string) (bool, error) {
	bak := path + ".sysc.bak"
	if _, err := os.Stat(bak); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(bak, data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// StateCopy writes a timestamped copy of src under stateDir and rotates old
// timestamped copies down to maxStateCopies. It never touches *.sysc.bak.
func StateCopy(stateDir, name, src string, now time.Time) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", err
	}
	stamp := now.UTC().Format("20060102T150405Z")
	dst := filepath.Join(stateDir, name+"."+stamp)
	for i := 1; ; i++ {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			break
		}
		dst = filepath.Join(stateDir, fmt.Sprintf("%s.%s.%d", name, stamp, i))
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", err
	}
	rotate(stateDir, name, dst)
	return dst, nil
}

func rotate(stateDir, name, keep string) {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return
	}
	prefix := name + "."
	var copies []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) || strings.HasSuffix(e.Name(), ".sysc.bak") {
			continue
		}
		copies = append(copies, e.Name())
	}
	// Plain lexical order puts the bare "<name>.<stamp>" copy before its
	// suffixed siblings, but the bare name is reused once rotated away, so
	// the newest copy could both sort first and be deleted in its own run
	// (#37). Order by the (stamp, suffix) each name actually encodes.
	sort.Slice(copies, func(i, j int) bool {
		si, ni := parseCopy(copies[i], prefix)
		sj, nj := parseCopy(copies[j], prefix)
		if si != sj {
			return si < sj
		}
		return ni < nj
	})
	keepName := filepath.Base(keep)
	for len(copies) > maxStateCopies {
		victim := copies[0]
		copies = copies[1:]
		if victim == keepName {
			continue
		}
		os.Remove(filepath.Join(stateDir, victim))
	}
}

// parseCopy splits a "<name>.<stamp>[.<n>]" copy name into its stamp and
// numeric suffix (0 when the name has no suffix).
func parseCopy(copyName, prefix string) (string, int) {
	rest := strings.TrimPrefix(copyName, prefix)
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) == 1 {
		return parts[0], 0
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return parts[0], 0
	}
	return parts[0], n
}

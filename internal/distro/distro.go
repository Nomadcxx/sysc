// Package distro gates the installer to the Arch family.
package distro

import (
	"fmt"
	"strings"
)

var archFamily = map[string]bool{
	"arch":        true,
	"cachyos":     true,
	"manjaro":     true,
	"endeavouros": true,
}

// ParseOSRelease extracts ID and ID_LIKE from /etc/os-release content.
func ParseOSRelease(data []byte) (id string, idLike []string) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"'`)
		switch key {
		case "ID":
			id = strings.ToLower(value)
		case "ID_LIKE":
			for _, v := range strings.Fields(value) {
				idLike = append(idLike, strings.ToLower(v))
			}
		}
	}
	return id, idLike
}

// Family returns nil when id, or one of idLike, is in the Arch family, and a
// named refusal otherwise.
func Family(id string, idLike []string) error {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return fmt.Errorf("unknown distribution: /etc/os-release has no ID")
	}
	if archFamily[id] {
		return nil
	}
	for _, like := range idLike {
		if archFamily[strings.ToLower(like)] {
			return nil
		}
	}
	return fmt.Errorf("unsupported distribution %q: SYSC v1 supports the Arch family only", id)
}

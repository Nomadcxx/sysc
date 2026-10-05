package niri

import (
	"fmt"
	"strings"
)

// modBits is the modifier set niri stores on a hotkey. Mod is the compositor
// modifier (niri's Modifiers::COMPOSITOR), not Super. Numeric values are a
// local encoding; occupancy only cares that equal sets compare equal.
type modBits uint8

const (
	modCtrl modBits = 1 << iota
	modShift
	modAlt
	modSuper
	modLevel3
	modLevel5
	modCompositor
)

// canonicalBind returns niri's identity for a hotkey string.
//
// niri-config parses binds in FromStr for Key (niri-config/src/binds.rs):
// the last '+'-separated component is the keysym or pointer trigger, and
// every preceding component is a modifier. Modifiers are trimmed and matched
// with ASCII case-folding. Aliases niri accepts are Ctrl/Control, Super/Win,
// ISO_Level3_Shift/Mod5, and ISO_Level5_Shift/Mod3. Mod4 is not a modifier.
// Duplicate modifiers and modifier order do not change the set.
//
// The keysym goes through xkb_keysym_from_name with KEYSYM_CASE_INSENSITIVE,
// which prefers the lower-case name. XF86Screensaver and XF86ScreenSaver are
// the exception niri special-cases: only the exact spelling XF86Screensaver
// keeps that keysym; every other case variant resolves to XF86ScreenSaver.
// xkb also accepts an XF86_ underscore prefix as an alias of XF86.
func canonicalBind(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	parts := strings.Split(s, "+")
	key, ok := canonicalKeysym(parts[len(parts)-1])
	if !ok {
		return "", false
	}
	var mods modBits
	for _, part := range parts[:len(parts)-1] {
		bit, ok := modifierBit(strings.TrimSpace(part))
		if !ok {
			return "", false
		}
		mods |= bit
	}
	return fmt.Sprintf("%d+%s", mods, key), true
}

func modifierBit(part string) (modBits, bool) {
	switch strings.ToLower(part) {
	case "mod":
		return modCompositor, true
	case "ctrl", "control":
		return modCtrl, true
	case "shift":
		return modShift, true
	case "alt":
		return modAlt, true
	case "super", "win":
		return modSuper, true
	case "iso_level3_shift", "mod5":
		return modLevel3, true
	case "iso_level5_shift", "mod3":
		return modLevel5, true
	default:
		return 0, false
	}
}

func canonicalKeysym(key string) (string, bool) {
	if key == "" || strings.ContainsAny(key, " \t\r") {
		return "", false
	}
	if class, ok := screensaverClass(key); ok {
		return class, true
	}
	// xkb_keysym_from_name strips one underscore after an XF86_ prefix and retries.
	if len(key) > 5 && strings.EqualFold(key[:5], "xf86_") {
		return canonicalKeysym(key[:4] + key[5:])
	}
	return strings.ToLower(key), true
}

// screensaverClass distinguishes the two keysyms that case-fold onto each
// other. niri asks xkb case-insensitively first (that prefers XF86Screensaver),
// then retries a case-sensitive lookup on the original spelling and falls
// back to XF86ScreenSaver when the exact name misses.
func screensaverClass(key string) (string, bool) {
	probe := key
	if len(key) > 5 && strings.EqualFold(key[:5], "xf86_") {
		probe = key[:4] + key[5:]
	}
	if !strings.EqualFold(probe, "XF86ScreenSaver") {
		return "", false
	}
	if key == "XF86Screensaver" {
		return "XF86Screensaver", true
	}
	return "XF86ScreenSaver", true
}

// leadingNodeName is the KDL node name at the start of a line. A // comment
// and a commented-out /- node are not live binds. Quoted names are decoded
// the way KDL presents them to niri.
func leadingNodeName(line string) string {
	line = strings.TrimRight(line, "\r")
	line = strings.TrimLeft(line, " \t")
	if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/-") {
		return ""
	}
	if line[0] == '"' {
		rest := line[1:]
		end := strings.IndexByte(rest, '"')
		if end < 0 {
			return ""
		}
		return rest[:end]
	}
	for i, c := range line {
		if c == ' ' || c == '\t' || c == '{' {
			return line[:i]
		}
	}
	return line
}

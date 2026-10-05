package ui

import "strings"

// asciiHeaderLines is the SYSC block lettering from the greet installer.
var asciiHeaderLines = []string{
	"  ██████████████░████░       ████░   ██████████████░  ██████████████░",
	"████████████████░████░       ████░ ████████████████░████████████████░",
	"████░            ██████░   ██████░ ████░            ████░            ",
	"██████████████░    ████████████░   ██████████████░  ████░            ",
	"  ██████████████░    ████████░       ██████████████░████░            ",
	"            ████░      ████░                   ████░████░            ",
	"████████████████░      ████░       ████████████████░████████████████░",
	"██████████████░        ████░       ██████████████░    ██████████████░",
	"           //////////////SEE YOU IN SPACE COWBOY//////////",
}

// Banner returns the eight-line SYSC header plus the cowboy underline.
func Banner() string {
	return strings.Join(asciiHeaderLines, "\n")
}

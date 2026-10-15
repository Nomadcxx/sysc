package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Nomadcxx/sysc/internal/stamp"
)

// shellVersionTimeout bounds the post-install version probe so a wedged binary
// cannot hold the run open.
const shellVersionTimeout = 2 * time.Second

// runVersion reports the installed suite from the stamp and nothing else. It
// reads no binary, so it answers before any unit is touched.
func runVersion(stateDir string, args []string, out, errOut io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintf(errOut, "unexpected arguments: %s\n", strings.Join(args, " "))
		return 2
	}
	st, err := stamp.Read(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(out, "no sysc install stamp at %s\n", filepath.Join(stateDir, stamp.FileName))
			return 0
		}
		fmt.Fprintln(errOut, err)
		return 1
	}
	fmt.Fprintf(out, "sysc release: %s\n", st.Release)
	for _, id := range slices.Sorted(maps.Keys(st.Components)) {
		fmt.Fprintf(out, "%s: %s\n", id, st.Components[id])
	}
	return 0
}

// printShellVersion asks the freshly installed shell what it is. The installer
// has no version of its own: the stamp names the suite, the binary answers for
// itself. A failure is a warning, never a failed install.
func printShellVersion(out io.Writer, binDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), shellVersionTimeout)
	defer cancel()
	raw, err := exec.CommandContext(ctx, filepath.Join(binDir, "sysc-shell"), "--version").Output()
	if err != nil {
		fmt.Fprintln(out, "warning: sysc-shell version unavailable")
		return
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(raw)), "\n")
	if strings.TrimSpace(line) == "" {
		fmt.Fprintln(out, "warning: sysc-shell version unavailable")
		return
	}
	fmt.Fprintln(out, line)
}

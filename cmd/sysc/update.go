package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/install"
	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/preflight"
	"github.com/Nomadcxx/sysc/internal/seed"
	"github.com/Nomadcxx/sysc/internal/stamp"
	"github.com/Nomadcxx/sysc/internal/update"
)

// installRunner is the seam the update path calls the existing pipeline
// through, so a test can prove what an update does without installing anything.
type installRunner func(ctx context.Context, opts install.Options) (install.Result, error)

// readOSRelease is the /etc/os-release reader. The tests replace it because
// the apply gate must behave the same on the Arch host CI runs and elsewhere.
var readOSRelease = func() []byte {
	data, _ := os.ReadFile("/etc/os-release")
	return data
}

func parseUpdate(args []string, out io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("sysc update", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.BoolVar(&o.Yes, "yes", false, "no prompts")
	fs.BoolVar(&o.Apply, "apply", false, "apply the update this installer carries (set by the installer that fetched it)")
	fs.StringVar(&o.Lang, "lang", "", "installer language (en, zh-Hans, de, fr, es, pt, ja, ko, ru)")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	return o, nil
}

// runUpdate is the whole verb. Without --apply it fetches the release installer
// that carries the current pin and hands it the work; with --apply it is that
// fetched installer, and it is the only party that knows the new pin.
func runUpdate(args []string, in io.Reader, out, errOut io.Writer, home string, run installRunner) int {
	o, err := parseUpdate(args, errOut)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(errOut, err)
		return 2
	}
	loc := i18n.Match(o.Lang)
	osr := readOSRelease()
	if !o.Apply {
		return update.Fetch(context.Background(), update.Options{
			StateDir:      filepath.Join(stateHome(), "sysc"),
			Arch:          runtime.GOARCH,
			OSRelease:     osr,
			Yes:           o.Yes,
			Out:           out,
			ErrOut:        errOut,
			RefuseDistro:  i18n.T(loc, "refuse.distro"),
			RefuseNoStamp: i18n.T(loc, "refuse.stamp"),
		})
	}
	return applyUpdate(o, in, out, errOut, home, osr, run)
}

// applyUpdate prints the delta against this installer's own pin, asks, and
// hands the work to the same install.Run an install uses. It never swaps a
// binary itself.
func applyUpdate(o options, in io.Reader, out, errOut io.Writer, home string, osRelease []byte, run installRunner) int {
	loc := i18n.Match(o.Lang)
	if os.Geteuid() == 0 {
		fmt.Fprintln(errOut, "Run the installer as your desktop user, without sudo.")
		return 1
	}
	prev, err := stamp.Read(filepath.Join(stateHome(), "sysc"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(errOut, i18n.T(loc, "refuse.stamp")+"\n", filepath.Join(stateHome(), "sysc", stamp.FileName))
			return 1
		}
		fmt.Fprintln(errOut, err)
		return 1
	}
	// The parent half performs the same check before fetching. This half
	// cannot trust that: the fetched installer is a standalone binary, and a
	// stamped install must not update on a host a fresh install refuses.
	if preflight.CheckDistro(osRelease) != nil {
		fmt.Fprintln(errOut, i18n.T(loc, "refuse.distro"))
		return 1
	}
	p, err := pin.Load()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	rows := update.Delta(prev, p)
	if len(rows) == 0 {
		fmt.Fprintf(out, i18n.T(loc, "update.current")+"\n", p.Release)
		return 0
	}
	fmt.Fprintf(out, "SYSC update %s -> %s\n", prev.Release, p.Release)
	for _, r := range rows {
		fmt.Fprintln(out, r.String())
	}
	if !o.Yes {
		fmt.Fprint(out, "Proceed? [y/N] ")
		if !confirm(in) {
			fmt.Fprintln(out, "Aborted.")
			return 1
		}
	}
	answers, err := seed.AnswersFromConfig(filepath.Join(configHome(home), "sysc-shell", "config.json"))
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	opts := installOptions(home, p, answers, true, loc, osRelease, runPackage)
	opts.InNiriSession = liveNiriSession(os.Getenv("WAYLAND_DISPLAY"), os.Getenv("NIRI_SOCKET"))
	// Updates don't rerun interactive conflict detection or provider handover.
	res, err := run(context.Background(), opts)
	printTasks(out, res)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if anyFailed(res.Tasks) {
		fmt.Fprintln(errOut, "update finished with failed tasks")
		return 1
	}
	return 0
}

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
	"strings"

	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/install"
	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/seed"
	"github.com/Nomadcxx/sysc/internal/stamp"
	"github.com/Nomadcxx/sysc/internal/update"
)

// installRunner is the seam the update path calls the existing pipeline
// through, so a test can prove what an update does without installing anything.
type installRunner func(ctx context.Context, opts install.Options) (install.Result, error)

func parseUpdate(args []string, out io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("sysc update", flag.ContinueOnError)
	fs.SetOutput(out)
	o.Apply = true
	fs.BoolVar(&o.Yes, "yes", false, "no prompts")
	fs.BoolVar(&o.Apply, "apply", true, "apply the update this installer carries (set by the fetching installer)")
	fs.StringVar(&o.Lang, "lang", "", "installer language (en, zh-Hans, de, fr, es, pt, ja, ko, ru)")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	return o, nil
}

// runUpdate is the child half: the freshly fetched installer is the only party
// that knows the new pin, so it prints the delta, asks, and hands the work to
// the same install.Run an install uses. It never swaps a binary itself.
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
	if os.Geteuid() == 0 {
		fmt.Fprintln(errOut, "Run the installer as your desktop user, without sudo.")
		return 1
	}
	prev, err := stamp.Read(filepath.Join(stateHome(), "sysc"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(errOut, "%s %s\n", i18n.T(loc, "refuse.stamp"), filepath.Join(stateHome(), "sysc", stamp.FileName))
			return 1
		}
		fmt.Fprintln(errOut, err)
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
	osr, _ := os.ReadFile("/etc/os-release")
	res, err := run(context.Background(), installOptions(home, p, answers, true, loc, osr, runPackage))
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

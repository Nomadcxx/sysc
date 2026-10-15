package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

// DefaultBase is where a release installer and its checksum list are fetched
// from. The pin is embedded in a binary, so the only way to learn a newer one
// is to fetch the release binary that carries it.
const DefaultBase = "https://github.com/Nomadcxx/sysc/releases/latest/download"

// staleStagingAge bounds which update-* dirs a new run sweeps: anything older
// is a crash leftover, anything younger may belong to a live run.
const staleStagingAge = time.Hour

// Options configures the fetching half of `sysc update`. The refusal strings
// come from the caller because the installer owns its translated text.
type Options struct {
	// StateDir is the directory holding installed.json.
	StateDir string
	// Arch is the machine name, naming the release asset to fetch.
	Arch string
	// Base is the release asset base URL; DefaultBase when empty.
	Base string
	// Yes hands --yes to the fetched installer.
	Yes bool

	Out, ErrOut io.Writer

	// RefuseDistro and RefuseNoStamp are the named refusals this verb prints.
	// RefuseNoStamp is a format string taking the stamp path, because the
	// caller owns the translated text and only this half knows the path.
	RefuseDistro  string
	RefuseNoStamp string

	// Download and Run are the seams tests replace. Download fetches url into
	// dir as name, verifying sha when it is not empty.
	Download func(ctx context.Context, dir, url, name, sha string) error
	Run      func(ctx context.Context, path string, args ...string) int
}

// Fetch is the parent half of the verb: prove an install exists, fetch the
// current release installer together with its checksum list, and hand the work
// to it. It never swaps a binary, never writes the stamp, and never learns the
// new pin, which is what keeps the embedded-pin trap structurally impossible.
func Fetch(ctx context.Context, o Options) int {
	base := o.Base
	if base == "" {
		base = DefaultBase
	}
	download := o.Download
	if download == nil {
		download = defaultDownload
	}
	run := o.Run
	if run == nil {
		run = runInstaller
	}
	// The stamp comes first: "nothing is installed" answers both roles, and a
	// host the pin cannot install should hear it before the distro refusal.
	stampPath := filepath.Join(o.StateDir, stamp.FileName)
	if _, err := stamp.Read(o.StateDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(o.ErrOut, o.RefuseNoStamp+"\n", stampPath)
			return 1
		}
		fmt.Fprintln(o.ErrOut, err)
		return 1
	}
	if o.Arch != "amd64" {
		fmt.Fprintln(o.ErrOut, o.RefuseDistro)
		return 1
	}
	asset := "sysc-linux-" + o.Arch
	staging := filepath.Join(o.StateDir, "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		fmt.Fprintln(o.ErrOut, err)
		return 1
	}
	// A run that died before its defer leaves its update-* dir behind; sweep
	// ones old enough to be dead so staging does not grow run over run. The
	// bound keeps a concurrently downloading parent's dir out of reach.
	if entries, err := os.ReadDir(staging); err == nil {
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), "update-") {
				continue
			}
			if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > staleStagingAge {
				_ = os.RemoveAll(filepath.Join(staging, e.Name()))
			}
		}
	}
	dir, err := os.MkdirTemp(staging, "update-")
	if err != nil {
		fmt.Fprintln(o.ErrOut, err)
		return 1
	}
	defer os.RemoveAll(dir)

	if err := download(ctx, dir, base+"/SHA256SUMS", "SHA256SUMS", ""); err != nil {
		fmt.Fprintln(o.ErrOut, err)
		return 1
	}
	sums, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		fmt.Fprintln(o.ErrOut, err)
		return 1
	}
	sha, err := ChecksumFor(sums, asset)
	if err != nil {
		fmt.Fprintln(o.ErrOut, err)
		return 1
	}
	if err := download(ctx, dir, base+"/"+asset, asset, sha); err != nil {
		fmt.Fprintln(o.ErrOut, err)
		return 1
	}
	path := filepath.Join(dir, asset)
	if err := os.Chmod(path, 0o700); err != nil {
		fmt.Fprintln(o.ErrOut, err)
		return 1
	}
	args := []string{"update", "--apply"}
	if o.Yes {
		args = append(args, "--yes")
	}
	return run(ctx, path, args...)
}

func defaultDownload(ctx context.Context, dir, url, name, sha string) error {
	client := fetch.NewClient()
	if sha == "" {
		return fetch.FetchOne(ctx, client, dir, url, name)
	}
	return fetch.DownloadAll(ctx, client, dir, []fetch.Asset{{Name: name, URL: url, SHA256: sha}})
}

// runInstaller hands the fetched binary the apply flag and the caller's
// terminal, so the child owns every line the user sees and its exit code is the
// only status this half reports.
func runInstaller(ctx context.Context, path string, args ...string) int {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		return 1
	}
	return 0
}

// Package fetch downloads pinned release assets, verifies their checksums,
// and swaps them into place with a rollback copy of the previous binary.
package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Asset is one pinned download: the install name it stages under, where it
// comes from, and the SHA256 it must have.
type Asset struct {
	Name   string
	URL    string
	SHA256 string
}

// rename is a seam so tests can inject a failure mid-swap.
var rename = os.Rename

// safeName refuses asset names that would escape staging or binDir. Names
// come from the embedded pin, but this is the write-boundary check.
// ponytail: basename-only; a component needing subdir layout would extend
// this, not bypass it.
func safeName(name string) error {
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) {
		return fmt.Errorf("invalid asset name %q: must be a plain file name", name)
	}
	return nil
}

// DownloadAll fetches every asset into staging, verifying each SHA256. A
// failed download or checksum leaves no file behind for that asset.
func DownloadAll(ctx context.Context, client *http.Client, staging string, assets []Asset) error {
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	for _, a := range assets {
		if err := safeName(a.Name); err != nil {
			return fmt.Errorf("download %s: %w", a.Name, err)
		}
		if err := download(ctx, client, staging, a); err != nil {
			return err
		}
	}
	return nil
}

func download(ctx context.Context, client *http.Client, staging string, a Asset) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", a.Name, resp.Status)
	}

	tmp, err := os.CreateTemp(staging, ".download-*")
	if err != nil {
		return err
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(tmp, h), resp.Body)
	closeErr := tmp.Close()
	if copyErr != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("download %s: %w", a.Name, copyErr)
	}
	if closeErr != nil {
		os.Remove(tmp.Name())
		return closeErr
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, a.SHA256) {
		os.Remove(tmp.Name())
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", a.Name, got, a.SHA256)
	}
	return rename(tmp.Name(), filepath.Join(staging, a.Name))
}

// RestoreBackups returns each named binary to the state it had before this
// run: a destination with a .bak is restored from it, one without a .bak did
// not exist before the run and is removed. Used when a step after a
// successful swap fails and the whole swap must be undone.
func RestoreBackups(binDir string, names []string) error {
	var errs []string
	for _, name := range names {
		if err := safeName(name); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		dst := filepath.Join(binDir, name)
		bak := dst + ".bak"
		if _, err := os.Stat(bak); err == nil {
			if err := os.Rename(bak, dst); err != nil {
				errs = append(errs, err.Error())
			}
			continue
		}
		if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("restoring %d binaries: %s", len(errs), strings.Join(errs, "; "))
	}
	return nil
}

// SwapAll moves each staged binary into binDir. The current destination is
// kept as name.bak and the staged file is renamed over it, so a running
// process keeps its old inode. On any failure, destinations already swapped
// are restored only from backups this run created.
func SwapAll(binDir, staging string, names []string) error {
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	var swapped []string
	backedUp := map[string]bool{}
	for _, name := range names {
		if err := safeName(name); err != nil {
			Rollback(binDir, swapped, backedUp)
			return err
		}
		madeBak, err := swapOne(binDir, staging, name)
		if err != nil {
			Rollback(binDir, swapped, backedUp)
			return err
		}
		if madeBak {
			backedUp[name] = true
		}
		swapped = append(swapped, name)
	}
	return nil
}

// swapOne reports whether it wrote name.bak for a binary that was already
// installed. A preexisting name.bak is not a backup of this run.
func swapOne(binDir, staging, name string) (bool, error) {
	src := filepath.Join(staging, name)
	dst := filepath.Join(binDir, name)
	newPath := dst + ".new"
	if fi, err := os.Lstat(dst); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("refusing to replace symlinked binary %s", dst)
	}
	if err := copyFile(src, newPath); err != nil {
		return false, err
	}
	madeBak := false
	if _, err := os.Stat(dst); err == nil {
		if err := copyFile(dst, dst+".bak"); err != nil {
			os.Remove(newPath)
			return false, err
		}
		madeBak = true
	}
	if err := rename(newPath, dst); err != nil {
		os.Remove(newPath)
		return false, err
	}
	return madeBak, nil
}

// Rollback undoes names already swapped by this run. backedUp is the set of
// names whose .bak this run wrote; those are copied back and the .bak is
// left in place. Every other name is removed. A leftover name.bak from an
// earlier install is not restored, because that would revive a binary this
// run did not replace.
func Rollback(binDir string, names []string, backedUp map[string]bool) error {
	var firstErr error
	for _, name := range names {
		dst := filepath.Join(binDir, name)
		if !backedUp[name] {
			if err := os.Remove(dst); err != nil && !os.IsNotExist(err) && firstErr == nil {
				firstErr = err
			}
			continue
		}
		bak := filepath.Join(binDir, name+".bak")
		if err := copyFile(bak, dst); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

package install

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/stamp"
	"github.com/Nomadcxx/sysc/internal/units"
)

// lockState uses a kernel lock: a crash releases it, and the inode stays put
// so another process cannot acquire a different lock by recreating the file.
func lockState(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "install.lock")
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.New("another SYSC install or uninstall is in progress")
		}
		return nil, err
	}
	return f, nil
}

// CheckOwnership refuses a new package takeover, but allows recorded user installs to upgrade.
// The CLI wires this read-only guard before install mutations.
func CheckOwnership(home string, components []pin.Component) error {
	if _, err := exec.LookPath("pacman"); err != nil {
		return err
	}
	for _, c := range components {
		if c.Disabled {
			continue
		}
		out, err := exec.Command("pacman", "-Qq", "--", c.ID).CombinedOutput()
		if err == nil {
			o := Options{Home: home}
			st, readErr := stamp.Read(o.stateDir())
			if readErr == nil && st.Components[c.ID] != "" && len(c.Binaries) > 0 {
				local := true
				for _, b := range c.Binaries {
					if _, err := regularData(filepath.Join(o.binDir(), b.Name)); err != nil {
						local = false
					}
				}
				if local {
					continue
				} // Run validates or backs up these recorded user files.
			}
			return fmt.Errorf("%s is managed by pacman; update it with your AUR helper. To change install methods, follow https://nomadcxx.github.io/sysc/docs/start/install/", c.ID)
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return fmt.Errorf("check package ownership: %s: %w", strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func fileHash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func regularData(path string) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing non-regular owned file %s", path)
	}
	return os.ReadFile(path)
}

// planFile keeps the original foreign backup across upgrades. A prior owned
// file must still match the stamp before the installer replaces it.
func planFile(st *stamp.Stamp, path string, data []byte, unit string, systemctl func(...string) error) error {
	for i := range st.Files {
		f := &st.Files[i]
		if f.Path != path {
			continue
		}
		if f.Backup != "" {
			saved, err := regularData(f.Backup)
			if err != nil {
				return err
			}
			if f.BackupSHA256 != "" && fileHash(saved) != f.BackupSHA256 {
				return fmt.Errorf("preserving modified backup %s; recover the original backup before retrying", f.Backup)
			}
			f.BackupSHA256 = fileHash(saved)
		}
		old, err := regularData(path)
		if err == nil && fileHash(old) != f.SHA256 && fileHash(old) != f.PreviousSHA256 {
			return fmt.Errorf("%s changed since SYSC installed it; preserve your changes before rerunning", path)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		f.PreviousSHA256 = ""
		if err == nil {
			f.PreviousSHA256 = fileHash(old)
			if unit == "" {
				f.RollbackSHA256 = fileHash(old)
			}
		}
		f.SHA256 = fileHash(data)
		return nil
	}
	f := stamp.File{Path: path, SHA256: fileHash(data), Unit: unit}
	if original, err := regularData(path); err == nil {
		fi, err := os.Stat(path)
		if err != nil {
			return err
		}
		f.Backup = path + ".sysc.bak"
		// Exclusive creation preserves the first backup and its permissions,
		// including while its bytes are being written.
		bak, err := os.OpenFile(f.Backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fi.Mode().Perm())
		if err == nil {
			if err := bak.Chmod(fi.Mode().Perm()); err != nil {
				bak.Close()
				os.Remove(f.Backup)
				return err
			}
			_, writeErr := bak.Write(original)
			if err := errors.Join(writeErr, bak.Close()); err != nil {
				os.Remove(f.Backup)
				return err
			}
		} else if !os.IsExist(err) {
			return err
		}
		saved, err := regularData(f.Backup)
		if err != nil {
			return err
		}
		if fileHash(original) != fileHash(saved) {
			return fmt.Errorf("existing backup differs from %s; preserve both files before rerunning", path)
		}
		f.PreviousSHA256 = fileHash(original)
		if unit == "" {
			f.RollbackSHA256 = fileHash(original)
		}
		f.BackupSHA256 = fileHash(saved)
		if unit != "" {
			f.WasEnabled = systemctl("is-enabled", "--quiet", unit) == nil
			f.WasActive = systemctl("is-active", "--quiet", unit) == nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	st.Files = append(st.Files, f)
	return nil
}

// legacyFiles upgrades the old component-only record without claiming files
// for skipped components or disabling system-wide services.
func legacyFiles(o Options, st stamp.Stamp, upgrading bool) ([]stamp.File, error) {
	files := []stamp.File{}
	for _, c := range o.Pin.Components {
		if _, owned := st.Components[c.ID]; !owned {
			continue
		}
		for _, b := range c.Binaries {
			path := filepath.Join(o.binDir(), b.Name)
			data, err := regularData(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if upgrading {
				// Old stamps have no file hashes. Preserve current bytes as the
				// upgrade baseline rather than claiming an unverifiable binary.
				inventory := stamp.Stamp{Files: files}
				if err := planFile(&inventory, path, data, "", o.systemctl()); err != nil {
					return nil, err
				}
				files = inventory.Files
				old, err := regularData(path + ".bak")
				if err == nil {
					saved := path + ".bak.legacy"
					if err := os.Link(path+".bak", saved); err != nil {
						if !os.IsExist(err) {
							return nil, err
						}
						prior, err := regularData(saved)
						if err != nil {
							return nil, err
						}
						if fileHash(prior) != fileHash(old) {
							return nil, fmt.Errorf("preserving conflicting legacy backups %s and %s.bak", saved, path)
						}
					}
					if err := os.Remove(path + ".bak"); err != nil {
						return nil, err
					}
				} else if !os.IsNotExist(err) {
					return nil, err
				}
				continue
			}
			a, ok := b.Assets["amd64"]
			if !ok || c.Tag != st.Components[c.ID] || !strings.EqualFold(fileHash(data), a.SHA256) {
				return nil, fmt.Errorf("preserving unverified legacy binary %s; use the original installer pin or move your changes aside before retrying", path)
			}
			files = append(files, stamp.File{Path: path, SHA256: fileHash(data)})
		}
		if u, ok := unitFor(c); ok {
			path := filepath.Join(o.unitDir(), u.Name)
			data, err := regularData(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			expected, err := units.Content(u)
			if err != nil {
				return nil, err
			}
			if upgrading && string(data) != string(expected) {
				inventory := stamp.Stamp{Files: files}
				if err := planFile(&inventory, path, data, u.Name, o.systemctl()); err != nil {
					return nil, err
				}
				files = inventory.Files
				continue
			}
			if string(data) != string(expected) {
				return nil, fmt.Errorf("legacy SYSC unit %s has user changes; preserve it before uninstalling", path)
			}
			f := stamp.File{Path: path, SHA256: fileHash(data), Unit: u.Name}
			if _, err := regularData(path + ".sysc.bak"); err == nil {
				f.Backup = path + ".sysc.bak"
			} else if !os.IsNotExist(err) {
				return nil, err
			}
			files = append(files, f)
		}
	}
	for i := range files {
		f := &files[i]
		saved, err := regularData(f.Path + ".sysc.bak")
		if err == nil {
			f.Backup = f.Path + ".sysc.bak"
			f.BackupSHA256 = fileHash(saved)
			if upgrading && f.Unit != "" {
				f.WasEnabled = o.systemctl()("is-enabled", "--quiet", f.Unit) == nil
				f.WasActive = o.systemctl()("is-active", "--quiet", f.Unit) == nil
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return files, nil
}

func validateOwnedFile(o Options, f stamp.File) error {
	validHash := func(hash string) bool {
		data, err := hex.DecodeString(hash)
		return err == nil && len(data) == sha256.Size
	}
	if !validHash(f.SHA256) || (f.PreviousSHA256 != "" && !validHash(f.PreviousSHA256)) || (f.RollbackSHA256 != "" && (!validHash(f.RollbackSHA256) || f.Unit != "")) || (f.BackupSHA256 != "" && !validHash(f.BackupSHA256)) || (f.Backup != "" && f.Backup != f.Path+".sysc.bak") || (f.Backup == "" && f.BackupSHA256 != "") {
		return fmt.Errorf("invalid owned file record %s", f.Path)
	}
	for _, c := range o.Pin.Components {
		if f.Unit != "" {
			if u, ok := unitFor(c); ok && !c.BinaryOnly && u.Name == f.Unit && f.Path == filepath.Join(o.unitDir(), u.Name) {
				return nil
			}
		} else {
			for _, b := range c.Binaries {
				if f.Path == filepath.Join(o.binDir(), b.Name) && b.Name == filepath.Base(b.Name) && b.Name != "." && b.Name != ".." {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("owned file is not in the current suite pin: %s", f.Path)
}

func restoreFile(o Options, f stamp.File) error {
	if err := validateOwnedFile(o, f); err != nil {
		return err
	}
	data, err := regularData(f.Path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	missing := os.IsNotExist(err)
	var saved []byte
	if f.Backup != "" {
		saved, err = regularData(f.Backup)
		if err != nil {
			if os.IsNotExist(err) && !missing && f.BackupSHA256 != "" && fileHash(data) == f.BackupSHA256 {
				return nil
			}
			return err
		}
		if f.BackupSHA256 != "" && fileHash(saved) != f.BackupSHA256 {
			return fmt.Errorf("preserving modified backup %s; recover the original backup before retrying", f.Backup)
		}
	}
	if !missing && fileHash(data) != f.SHA256 && fileHash(data) != f.PreviousSHA256 && (f.Backup == "" || string(data) != string(saved)) {
		return fmt.Errorf("preserving modified file %s; restore the SYSC version or move your changes aside before retrying", f.Path)
	}
	if missing && f.Backup == "" {
		return nil
	}
	if f.Unit != "" {
		if err := o.systemctl()("stop", f.Unit); err != nil {
			return err
		}
		if err := o.systemctl()("disable", f.Unit); err != nil {
			return err
		}
	}
	if f.Backup != "" {
		fi, err := os.Stat(f.Backup)
		if err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(f.Path), ".restore-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		if _, err := tmp.Write(saved); err != nil {
			tmp.Close()
			return err
		}
		if err := tmp.Chmod(fi.Mode().Perm()); err != nil {
			tmp.Close()
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		if err := os.Rename(tmp.Name(), f.Path); err != nil {
			return err
		}
		if f.Unit != "" {
			if err := o.systemctl()("daemon-reload"); err != nil {
				return err
			}
		}
		if f.Unit != "" && f.WasEnabled {
			if err := o.systemctl()("enable", f.Unit); err != nil {
				return err
			}
		}
		if f.Unit != "" && f.WasActive {
			if err := o.systemctl()("start", f.Unit); err != nil {
				return err
			}
		}
	} else if err := os.Remove(f.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

package preflight

import (
	"context"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

type runtimeEnv struct {
	libraryDirs, fontDirs []string
	graphical             bool
	command               func(string, ...string) ([]byte, error)
}

// CheckRuntime checks verified downloads before the installer replaces binaries.
// Warnings cover prerequisites that can only be checked at the next login.
func CheckRuntime(staging string, names []string) ([]string, error) {
	home, _ := os.UserHomeDir()
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	return checkRuntime(staging, names, runtimeEnv{
		libraryDirs: []string{"/usr/lib", "/usr/lib64", "/lib", "/lib64"},
		fontDirs:    []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(dataHome, "fonts"), filepath.Join(home, ".fonts")},
		graphical:   os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") != "",
		command: func(name string, args ...string) ([]byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return exec.CommandContext(ctx, name, args...).Output()
		},
	})
}

func checkRuntime(staging string, names []string, env runtimeEnv) ([]string, error) {
	for _, name := range names {
		if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
			return nil, fmt.Errorf("invalid staged binary name %q", name)
		}
		if err := checkELF(filepath.Join(staging, name), env.libraryDirs, map[string]bool{}); err != nil {
			return nil, fmt.Errorf("%s runtime: %w", name, err)
		}
	}
	if slices.Contains(names, "sysc-shell") && !hasFont(env) {
		return nil, errors.New("sysc-shell needs a usable font; install noto-fonts with pacman, then retry")
	}
	if slices.Contains(names, "sysc-clipboard") {
		if !env.graphical {
			return []string{"Clipboard history stays in memory unless Secret Service is unlocked at login or --key-file is configured in the clipboard unit. Keyring readiness will be checked in your desktop session."}, nil
		}
		warnings, err := checkSecretService(env)
		if err != nil {
			return []string{"Clipboard can run with history in memory: " + err.Error() + ". For persistent history, unlock a Secret Service keyring or configure --key-file in the clipboard unit."}, nil
		}
		return warnings, nil
	}
	return nil, nil
}

func checkELF(path string, dirs []string, seen map[string]bool) error {
	if seen[path] {
		return nil
	}
	seen[path] = true
	f, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("read ELF executable %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	for _, prog := range f.Progs {
		if prog.Type == elf.PT_INTERP {
			data, err := io.ReadAll(io.LimitReader(prog.Open(), 4096))
			if err != nil {
				return err
			}
			interpreter := strings.TrimRight(string(data), "\x00")
			if !filepath.IsAbs(interpreter) {
				return fmt.Errorf("invalid ELF interpreter %q", interpreter)
			}
			if _, err := os.Stat(interpreter); err != nil {
				return fmt.Errorf("missing ELF interpreter %s; install glibc with pacman", interpreter)
			}
		}
	}
	libraries, err := f.ImportedLibraries()
	if err != nil {
		return err
	}
	for _, lib := range libraries {
		if filepath.Base(lib) != lib {
			return fmt.Errorf("unsupported shared library path %q", lib)
		}
		found := ""
		for _, dir := range dirs {
			candidate := filepath.Join(dir, lib)
			if _, err := os.Stat(candidate); err == nil {
				found = candidate
				break
			}
		}
		if found == "" {
			return fmt.Errorf("missing shared library %s; install its Arch package with pacman, then retry", lib)
		}
		dependency, err := elf.Open(found)
		if err != nil {
			return fmt.Errorf("shared library %s: %w", lib, err)
		}
		compatible := dependency.Class == f.Class && dependency.Machine == f.Machine
		dependency.Close()
		if !compatible {
			return fmt.Errorf("shared library %s has the wrong architecture", lib)
		}
		if err := checkELF(found, dirs, seen); err != nil {
			return err
		}
	}
	return nil
}

func hasFont(env runtimeEnv) bool {
	if env.command != nil {
		if out, err := env.command("fc-match", "--format=%{file}", "sans"); err == nil {
			return validFont(strings.TrimSpace(string(out)))
		}
	}
	// ponytail: header/table bounds check only; fontconfig performs full matching
	// when installed. A future font parser would belong to the rendering library.
	for _, dir := range env.fontDirs {
		found := false
		_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && validFont(path) {
				found = true
				return fs.SkipAll
			}
			return nil
		})
		if found {
			return true
		}
	}
	return false
}

func validFont(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return false
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	switch string(header[:4]) {
	case "\x00\x01\x00\x00", "OTTO", "true":
		tables := int64(binary.BigEndian.Uint16(header[4:6]))
		return tables > 0 && info.Size() >= 12+16*tables
	case "ttcf":
		fonts := int64(binary.BigEndian.Uint32(header[8:12]))
		return fonts > 0 && info.Size() >= 12+4*fonts
	}
	return false
}

func checkSecretService(env runtimeEnv) ([]string, error) {
	const guidance = "Secret Service is unavailable; install and configure gnome-keyring or another provider and unlock its default collection"
	if env.command == nil {
		return nil, errors.New(guidance)
	}
	probe := func(args ...string) (string, error) {
		out, err := env.command("busctl", append([]string{"--user", "--auto-start=no", "--timeout=5s"}, args...)...)
		return strings.TrimSpace(string(out)), err
	}
	owner, err := probe("call", "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "NameHasOwner", "s", "org.freedesktop.secrets")
	if err != nil {
		return nil, fmt.Errorf("%s (session bus check: %w)", guidance, err)
	}
	if owner != "b true" {
		available, err := probe("call", "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "ListActivatableNames")
		if err == nil && slices.Contains(strings.Fields(available), `"org.freedesktop.secrets"`) {
			return []string{"Secret Service is installed and will start on demand. Clipboard history stays in memory unless its keyring is unlocked or --key-file is configured in the clipboard unit."}, nil
		}
		return nil, errors.New(guidance)
	}
	items, err := probe("call", "org.freedesktop.secrets", "/org/freedesktop/secrets", "org.freedesktop.Secret.Service", "SearchItems", "a{ss}", "2", "application", "sysc-clipboard", "purpose", "clipboard-history-v1")
	if err != nil {
		return nil, fmt.Errorf("%s (check clipboard key: %w)", guidance, err)
	}
	fields := strings.Fields(items)
	if len(fields) < 4 || fields[0] != "ao" {
		return nil, errors.New("Secret Service returned invalid clipboard key information")
	}
	unlocked, err := strconv.Atoi(fields[1])
	if err != nil || unlocked < 0 || unlocked > len(fields)-4 || fields[unlocked+2] != "ao" {
		return nil, errors.New("Secret Service returned invalid clipboard key information")
	}
	if unlocked > 0 {
		return nil, nil
	}
	locked, err := strconv.Atoi(fields[3])
	if err != nil || locked < 0 {
		return nil, errors.New("Secret Service returned invalid clipboard key information")
	}
	if locked > 0 {
		return nil, errors.New("the clipboard key is locked; unlock the keyring")
	}
	alias, err := probe("call", "org.freedesktop.secrets", "/org/freedesktop/secrets", "org.freedesktop.Secret.Service", "ReadAlias", "s", "default")
	if err != nil {
		return nil, fmt.Errorf("%s (read default collection: %w)", guidance, err)
	}
	parts := strings.Fields(alias)
	if len(parts) != 2 || parts[0] != "o" {
		return nil, errors.New("Secret Service returned an invalid default collection")
	}
	path, err := strconv.Unquote(parts[1])
	if err != nil || !strings.HasPrefix(path, "/") {
		return nil, errors.New("Secret Service returned an invalid default collection")
	}
	if path == "/" {
		return nil, errors.New("Secret Service has no default collection; create and unlock a default keyring")
	}
	state, err := probe("get-property", "org.freedesktop.secrets", path, "org.freedesktop.Secret.Collection", "Locked")
	if err != nil {
		return nil, fmt.Errorf("%s (check default collection: %w)", guidance, err)
	}
	if state != "b false" {
		return nil, errors.New("the default Secret Service collection is locked; unlock the default keyring")
	}
	return nil, nil
}

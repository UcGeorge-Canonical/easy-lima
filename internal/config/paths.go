package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Paths struct{ Config, State, Cache, Home string }

func UserPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	if runtime.GOOS == "darwin" {
		c := filepath.Join(home, "Library", "Application Support", "easy-lima")
		return Paths{Config: c, State: filepath.Join(c, "state"), Cache: filepath.Join(home, "Library", "Caches", "easy-lima"), Home: home}, nil
	}
	if runtime.GOOS != "linux" {
		return Paths{}, model.Fail("dependency.unsupported", "easy-lima requires macOS or Linux.")
	}
	resolve := func(key, fallback string) (string, error) {
		p := os.Getenv(key)
		if p == "" {
			p = filepath.Join(home, fallback)
		}
		if !filepath.IsAbs(p) {
			return "", model.Invalid(key, "must be absolute")
		}
		return filepath.Join(p, "easy-lima"), nil
	}
	c, err := resolve("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return Paths{}, err
	}
	s, err := resolve("XDG_STATE_HOME", ".local/state")
	if err != nil {
		return Paths{}, err
	}
	k, err := resolve("XDG_CACHE_HOME", ".cache")
	return Paths{Config: c, State: s, Cache: k, Home: home}, err
}
func ResolvePath(raw, base string) (string, error) {
	if strings.HasPrefix(raw, "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		raw = filepath.Join(h, raw[2:])
	}
	if raw == "" {
		return "", model.Invalid("path", "cannot be empty")
	}
	if !filepath.IsAbs(raw) {
		raw = filepath.Join(base, raw)
	}
	raw = filepath.Clean(raw)
	if !files.PathOK(raw) {
		return "", model.Invalid("path", "must be absolute and contain no control characters")
	}
	return raw, nil
}
func Executable(raw, base string) (string, error) {
	if raw == "" {
		raw = "limactl"
	}
	if strings.ContainsRune(raw, os.PathSeparator) {
		p, err := ResolvePath(raw, base)
		if err != nil {
			return "", err
		}
		raw = p
	}
	p, err := exec.LookPath(raw)
	if err != nil {
		return "", model.Fail("dependency.missing", "The requested executable is unavailable.")
	}
	return filepath.Abs(p)
}

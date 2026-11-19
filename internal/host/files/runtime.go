package files

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

// RuntimeDir keeps persistent metadata in the configured cache, while allowing
// Unix sockets on hosts whose home path cannot fit OpenSSH's temporary suffix.
// A hash of the cache path separates independent application installations.
func RuntimeDir(cache string) (string, error) {
	path := filepath.Join(cache, "run")
	if len(path) <= 60 {
		return path, nil
	}
	base := "/tmp"
	if runtime.GOOS == "darwin" {
		base = "/private/tmp"
	}
	path = filepath.Join(base, fmt.Sprintf("el-%d-%s", os.Getuid(), Hash([]byte(cache))[:12]))
	if info, err := os.Lstat(path); err == nil {
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !ok || owner.Uid != uint32(os.Getuid()) || !info.IsDir() || info.Mode().Perm() != 0700 {
			return "", model.Fail("ownership.conflict", "The short runtime directory has conflicting ownership or permissions.")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return path, nil
}

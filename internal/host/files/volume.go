package files

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Volume struct {
	Identity       string `json:"identity"`
	CapacityGroup  string `json:"capacity_group"`
	AvailableBytes int64  `json:"available_bytes"`
	MountPoint     string `json:"mount_point"`
	Filesystem     string `json:"filesystem"`
	QuotaKnown     bool   `json:"quota_known"`
}

func ExistingAncestor(path string) (string, error) {
	for {
		info, err := os.Stat(path)
		if err == nil {
			if !info.IsDir() {
				return "", model.Fail("storage.unavailable", "A storage path traverses a non-directory.")
			}
			return path, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		next := filepath.Dir(path)
		if next == path {
			return "", err
		}
		path = next
	}
}
func RootIdentity(path string) (string, error) {
	i, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	s, ok := i.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("file identity unavailable")
	}
	volume, err := nativeVolumeIdentity(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", volume, s.Ino), nil
}
func Allocated(path string) (int64, error) {
	var bytes int64
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		i, e := d.Info()
		if e != nil {
			return e
		}
		if s, ok := i.Sys().(*syscall.Stat_t); ok {
			bytes += s.Blocks * 512
		}
		return nil
	})
	return bytes, err
}

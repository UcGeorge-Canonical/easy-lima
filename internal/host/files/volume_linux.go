//go:build linux

package files

import (
	"fmt"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"golang.org/x/sys/unix"
	"path/filepath"
)

func InspectVolume(path string) (Volume, error) {
	p, err := ExistingAncestor(path)
	if err != nil {
		return Volume{}, err
	}
	var s unix.Statfs_t
	if err = unix.Statfs(p, &s); err != nil {
		return Volume{}, err
	}
	switch uint64(s.Type) {
	case 0x6969, 0x517B, 0xFF534D42, 0x65735546:
		return Volume{}, model.Fail("resource.unsupported", "Network or userspace filesystems are not qualified for VM or state storage.")
	}
	mount := p
	for parent := filepath.Dir(mount); parent != mount; parent = filepath.Dir(mount) {
		var above unix.Statfs_t
		if unix.Statfs(parent, &above) != nil || above.Fsid != s.Fsid || above.Type != s.Type {
			break
		}
		mount = parent
	}
	id := fmt.Sprintf("linux:%x:%v", s.Type, s.Fsid)
	return Volume{Identity: id, CapacityGroup: Hash([]byte(id)), AvailableBytes: int64(s.Bavail) * int64(s.Bsize), MountPoint: mount, Filesystem: fmt.Sprintf("%x", s.Type), QuotaKnown: false}, nil
}

//go:build darwin

package files

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"golang.org/x/sys/unix"
)

func cstr(b []byte) string { return strings.TrimRight(string(b), "\x00") }

// diskutil is queried only for filesystem metadata. APFS volumes that share a
// container share one capacity group, even when their device IDs differ.
func InspectVolume(path string) (Volume, error) {
	p, err := ExistingAncestor(path)
	if err != nil {
		return Volume{}, err
	}
	var s unix.Statfs_t
	if err = unix.Statfs(p, &s); err != nil {
		return Volume{}, err
	}
	kind := cstr(s.Fstypename[:])
	if kind != "apfs" && kind != "hfs" {
		return Volume{}, model.Fail("resource.unsupported", "This macOS filesystem is not qualified for VM or state storage.")
	}
	mount := cstr(s.Mntonname[:])
	v := Volume{Identity: fmt.Sprintf("darwin:%v", s.Fsid), AvailableBytes: int64(s.Bavail) * int64(s.Bsize), MountPoint: mount, Filesystem: kind}
	r, err := (process.OS{}).Run(context.Background(), process.Request{Executable: "/usr/sbin/diskutil", Args: []string{"info", "-plist", mount}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 128 << 10})
	if err != nil || r.ExitCode != 0 || r.Truncated {
		return Volume{}, model.Fail("storage.unavailable", "Could not establish the APFS capacity group.")
	}
	values := plistStrings(r.Stdout)
	if uuid := values["VolumeUUID"]; uuid != "" {
		v.Identity = "darwin:" + strings.ToLower(strings.ReplaceAll(uuid, "-", ""))
	}
	group := v.Identity
	if kind == "apfs" {
		if ref := values["APFSContainerReference"]; ref != "" {
			group = "apfs:" + ref
		} else {
			return Volume{}, model.Fail("storage.unavailable", "The APFS shared container could not be determined.")
		}
	}
	v.CapacityGroup = Hash([]byte(group))
	return v, nil
}
func plistStrings(b []byte) map[string]string {
	out := map[string]string{}
	d := xml.NewDecoder(strings.NewReader(string(b)))
	var key string
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		if e, ok := tok.(xml.StartElement); ok {
			switch e.Name.Local {
			case "key":
				_ = d.DecodeElement(&key, &e)
			case "string", "integer":
				var v string
				_ = d.DecodeElement(&v, &e)
				if key != "" {
					out[key] = v
					key = ""
				}
			}
		}
	}
	return out
}

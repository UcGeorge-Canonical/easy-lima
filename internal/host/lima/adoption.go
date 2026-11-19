package lima

import (
	"context"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"path/filepath"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

// AdoptNative keeps the installed disk/image and user settings. It accepts only
// the pinned prototype's already materialized image-base and networking shapes.
func AdoptNative(b []byte, source string, imageBase ...[]byte) ([]byte, config.Settings, error) {
	n, err := Parse(b)
	if err != nil {
		return nil, config.Settings{}, err
	}
	root := n.Content[0]
	s, _, err := NativeSettings(b)
	if err != nil {
		return nil, s, err
	}
	s.Environment.Presets = []string{"base"}
	s.Environment.Template = "adopted"
	if base := get(root, "base"); base != nil {
		var values []string
		if err = base.Decode(&values); err != nil {
			return nil, s, model.Fail("dependency.unsupported", "The prototype's base graph is not recognized.")
		}
		if len(values) != 1 || values[0] != "template:_images/ubuntu-24.04" {
			return nil, s, model.Fail("dependency.unsupported", "Only the pinned Ubuntu image base is supported for prototype adoption.")
		}
		if get(root, "images") == nil {
			if len(imageBase) != 1 {
				return nil, s, model.Fail("dependency.unsupported", "The exact installed legacy image base must be materialized for adoption.")
			}
			baseNode, e := Parse(imageBase[0])
			if e != nil {
				return nil, s, e
			}
			imageNode := get(baseNode.Content[0], "images")
			if imageNode == nil {
				return nil, s, model.Fail("config.invalid", "The materialized legacy image list is missing.")
			}
			var images []Image
			if e = imageNode.Decode(&images); e != nil {
				return nil, s, e
			}
			if e = set(root, "images", images); e != nil {
				return nil, s, e
			}
		}
		remove(root, "base")
	}
	mounts := get(root, "mounts")
	if mounts == nil {
		return nil, s, model.Fail("binding.conflict", "The legacy VM has no source mount.")
	}
	var native []struct {
		Location   string `yaml:"location"`
		MountPoint string `yaml:"mountPoint"`
		Writable   bool   `yaml:"writable"`
	}
	if err = mounts.Decode(&native); err != nil {
		return nil, s, err
	}
	workspace := false
	s.Mounts = []config.Mount{}
	for _, m := range native {
		if m.MountPoint == "/workspace" {
			if m.Location != source || !m.Writable {
				return nil, s, model.Fail("binding.conflict", "Legacy workspace mount differs from the recorded project.")
			}
			workspace = true
			continue
		}
		if !filepath.IsAbs(m.Location) || strings.Contains(m.Location, "~") {
			return nil, s, model.Fail("binding.conflict", "Legacy mounts must have explicit absolute sources.")
		}
		s.Mounts = append(s.Mounts, config.Mount{Source: m.Location, Target: m.MountPoint, Writable: m.Writable})
	}
	if !workspace {
		return nil, s, model.Fail("binding.conflict", "Legacy workspace mount is missing.")
	}
	ssh := get(root, "ssh")
	if ssh == nil {
		return nil, s, model.Fail("dependency.unsupported", "Legacy SSH settings are missing.")
	}
	if v := get(ssh, "forwardAgent"); v != nil && v.Value != "false" {
		return nil, s, model.Fail("dependency.unsupported", "Ambient agent forwarding must be disabled in the legacy VM before adoption.")
	}
	for _, k := range []string{"forwardAgent", "loadDotSSHPubKeys", "overVsock"} {
		if err = set(ssh, k, false); err != nil {
			return nil, s, err
		}
	}
	if err = set(root, "propagateProxyEnv", false); err != nil {
		return nil, s, err
	}
	result, err := encode(n)
	if err != nil {
		return nil, s, err
	}
	if err = CheckNative(result); err != nil {
		return nil, s, err
	}
	return result, s, nil
}

func (a Adapter) PrototypeImageBase(ctx context.Context, path string, expected []byte) ([]byte, error) {
	n, e := Parse(expected)
	if e != nil {
		return nil, e
	}
	base := get(n.Content[0], "base")
	if base == nil {
		return expected, nil
	}
	var refs []string
	if e = base.Decode(&refs); e != nil || len(refs) != 1 || refs[0] != "template:_images/ubuntu-24.04" {
		return nil, model.Fail("dependency.unsupported", "Unrecognized prototype base graph.")
	}
	r, e := a.runner().Run(ctx, process.Request{Executable: a.Executable, Args: []string{"template", "copy", "--embed-all", path, "-"}, Env: a.env(), Timeout: 15 * time.Second, Limit: 8 << 20})
	if e != nil {
		return nil, e
	}
	if r.ExitCode != 0 || r.Truncated {
		return nil, model.Fail("config.invalid", "The installed prototype image base could not be frozen.")
	}
	current, e := files.Read(path, 8<<20)
	if e != nil {
		return nil, e
	}
	if files.Hash(current) != files.Hash(expected) {
		return nil, model.Fail("config.drift", "Prototype inputs changed during preview.")
	}
	return r.Stdout, nil
}

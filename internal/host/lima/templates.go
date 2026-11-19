package lima

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

// Template freezes a bounded local base graph before asking the installed Lima
// CLI to apply its native merge semantics. No mutable or remote include reaches
// Lima; images remain references and are admitted separately by storage planning.
func Template(ctx context.Context, executable, source string) ([]byte, error) {
	temp, err := os.MkdirTemp("", "easy-lima-template-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)
	active := map[string]bool{}
	copies := map[string]string{}
	total := 0
	var freeze func(string, int) (string, error)
	freeze = func(p string, depth int) (string, error) {
		p, err = filepath.Abs(p)
		if err != nil {
			return "", err
		}
		p, err = filepath.EvalSymlinks(p)
		if err != nil {
			return "", err
		}
		if active[p] || depth > 16 {
			return "", model.Invalid("base", "local include cycle or depth exceeds 16")
		}
		if target, ok := copies[p]; ok {
			return target, nil
		}
		if len(copies) >= 64 {
			return "", model.Invalid("base", "more than 64 local inputs")
		}
		raw, e := files.Read(p, 8<<20)
		if e != nil {
			return "", e
		}
		total += len(raw)
		if total > 16<<20 {
			return "", model.Invalid("base", "local input graph exceeds 16 MiB")
		}
		n, e := Parse(raw)
		if e != nil {
			return "", e
		}
		root := n.Content[0]
		target := filepath.Join(temp, files.Hash([]byte(p))+".yaml")
		copies[p] = target
		active[p] = true
		if base := get(root, "base"); base != nil && base.Tag != "!!null" {
			var refs []string
			if e = base.Decode(&refs); e != nil {
				return "", model.Invalid("base", "expected a list of local YAML paths")
			}
			paths := []string{}
			for _, ref := range refs {
				u, e := url.Parse(ref)
				if e != nil || (u.Scheme != "" && u.Scheme != "file") || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
					return "", model.Invalid("base", "only local file includes are supported")
				}
				ref = u.Path
				if !filepath.IsAbs(ref) {
					ref = filepath.Join(filepath.Dir(p), ref)
				}
				child, e := freeze(ref, depth+1)
				if e != nil {
					return "", e
				}
				paths = append(paths, child)
			}
			if e = set(root, "base", paths); e != nil {
				return "", e
			}
		}
		if images := get(root, "images"); images != nil {
			for _, image := range images.Content {
				loc := get(image, "location")
				if loc == nil {
					continue
				}
				u, e := url.Parse(loc.Value)
				if e != nil {
					return "", e
				}
				if u.Scheme == "" && !filepath.IsAbs(loc.Value) {
					loc.Value = filepath.Join(filepath.Dir(p), loc.Value)
				}
			}
		}
		raw, e = encode(n)
		if e != nil {
			return "", e
		}
		if e = files.WriteNew(target, raw); e != nil {
			return "", e
		}
		active[p] = false
		return target, nil
	}
	root, err := freeze(source, 0)
	if err != nil {
		return nil, err
	}
	result, err := (process.OS{}).Run(ctx, process.Request{Executable: executable, Args: []string{"template", "copy", "--embed-all", root, "-"}, Env: process.Environment(map[string]string{"LIMA_HOME": filepath.Join(temp, "namespace")}), Timeout: 15 * time.Second, Limit: 8 << 20})
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 || result.Truncated {
		return nil, model.Fail("config.invalid", "Lima could not resolve the frozen local template graph.")
	}
	if strings.Contains(string(result.Stdout), temp) {
		return nil, model.Fail("config.invalid", "The resolved template retains a temporary include.")
	}
	return result.Stdout, nil
}

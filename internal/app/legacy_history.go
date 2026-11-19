package app

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/environment"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
)

// The pinned prototype hashes the script plus Python's JSON list encoding.
// This is evidence only: adoption never executes or retries that script.
func legacySetup(root, relative string, cfg legacyConfig) *environment.LegacySetup {
	out := &environment.LegacySetup{Status: "unknown"}
	script, err := files.Read(filepath.Join(root, ".local-launcher", "provision.sh"), 1<<20)
	if err != nil {
		return out
	}
	values := map[string]any{"lxd_channel": "5.21/stable", "juju_channel": "3.6/stable", "go_channel": "latest/stable", "rockcraft": false, "charmcraft": false}
	merge := func(overrides map[string]any) {
		for k, v := range overrides {
			if _, ok := values[k]; ok {
				values[k] = v
			}
		}
	}
	merge(cfg.Defaults)
	parents := []string{}
	for p := relative; ; p = filepath.Dir(p) {
		parents = append(parents, p)
		if p == "." {
			break
		}
	}
	for i := len(parents) - 1; i >= 0; i-- {
		merge(cfg.Projects[filepath.ToSlash(parents[i])])
	}
	encoded := []string{}
	for _, k := range []string{"lxd_channel", "juju_channel", "go_channel", "rockcraft", "charmcraft"} {
		value := values[k]
		if k == "rockcraft" || k == "charmcraft" {
			if _, ok := value.(bool); !ok {
				return out
			}
		} else {
			if _, ok := value.(string); !ok {
				return out
			}
		}
		b, e := json.Marshal(value)
		if e != nil {
			return out
		}
		encoded = append(encoded, string(b))
	}
	out.ExpectedDigest = files.Hash(append(script, []byte("["+strings.Join(encoded, ", ")+"]")...))
	return out
}

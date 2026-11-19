package project

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Entry struct {
	Path, Name, Canonical string
	Symlink, Cycle        bool
	Error                 string
}
type TreeOptions struct {
	Hidden, Dependencies bool
	Limit                int
}

var dependencies = map[string]bool{"node_modules": true, ".venv": true, "venv": true, "vendor": true, "__pycache__": true, ".git": true, ".cache": true}

// Children reads only the expanded directory. Filtering never scans descendants.
func Children(ctx context.Context, path string, ancestors []string, opts TreeOptions) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	canonical, _ := filepath.EvalSymlinks(path)
	out := []Entry{}
	if opts.Limit <= 0 {
		opts.Limit = 10000
	}
	for {
		rows, e := f.ReadDir(128)
		for _, d := range rows {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			name := d.Name()
			if !opts.Hidden && strings.HasPrefix(name, ".") {
				continue
			}
			if !opts.Dependencies && dependencies[name] {
				continue
			}
			p := filepath.Join(path, name)
			link := d.Type()&os.ModeSymlink != 0
			if !d.IsDir() && !link {
				continue
			}
			en := Entry{Path: p, Name: name, Symlink: link, Canonical: filepath.Join(canonical, name)}
			if link {
				target, x := filepath.EvalSymlinks(p)
				if x != nil {
					en.Error = "unavailable"
				} else {
					en.Canonical = target
					i, x := os.Stat(target)
					if x != nil || !i.IsDir() {
						continue
					}
					for _, a := range ancestors {
						if Same(a, target) {
							en.Cycle = true
							break
						}
					}
				}
			}
			out = append(out, en)
			if len(out) >= opts.Limit {
				sortEntries(out)
				return out, nil
			}
		}
		if e != nil {
			if !errors.Is(e, io.EOF) {
				return out, e
			}
			break
		}
	}
	sortEntries(out)
	return out, nil
}
func sortEntries(e []Entry) {
	sort.Slice(e, func(i, j int) bool { return strings.ToLower(e[i].Name) < strings.ToLower(e[j].Name) })
}

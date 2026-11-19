package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

// Resolve follows existing ancestors even for a missing leaf, so stop/inspect
// can address a recorded project after it has moved or been deleted.
func Resolve(path, cwd string, required bool) (string, error) {
	p, err := config.ResolvePath(path, cwd)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err == nil {
		i, e := os.Stat(resolved)
		if e != nil {
			return "", e
		}
		if !i.IsDir() {
			return "", model.Invalid("project", "must be a directory")
		}
		return resolved, nil
	}
	if required || !errors.Is(err, os.ErrNotExist) {
		return "", model.Fail("target.not_found", "The selected project directory is unavailable.")
	}
	leaf := []string{}
	cur := p
	for {
		resolved, err = filepath.EvalSymlinks(cur)
		if err == nil {
			for i := len(leaf) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, leaf[i])
			}
			return resolved, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p, nil
		}
		leaf = append(leaf, filepath.Base(cur))
		cur = parent
	}
}
func Same(a, b string) bool {
	if a == b {
		return true
	}
	ai, ae := os.Stat(a)
	bi, be := os.Stat(b)
	return ae == nil && be == nil && os.SameFile(ai, bi)
}
func Overlap(a, b string) bool {
	if config.Overlaps(a, b) {
		return true
	}
	for p := a; ; p = filepath.Dir(p) {
		if Same(p, b) {
			return true
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	for p := b; ; p = filepath.Dir(p) {
		if Same(p, a) {
			return true
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return false
}
func Slug(path string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(filepath.Base(path)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
		if b.Len() >= 24 {
			break
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		s = "project"
	}
	return s
}

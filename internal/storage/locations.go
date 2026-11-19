package storage

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
)

// Location is a read-only starting point for the interactive storage picker.
// Selecting one does not create or reserve a directory.
type Location struct {
	Label, Path, Filesystem string
	AvailableBytes          int64
}

func Locations(ctx context.Context, cwd, home string) []Location {
	paths := []string{home}
	parents := []string{"/Volumes"}
	if runtime.GOOS == "linux" {
		parents = []string{"/mnt", "/media", "/run/media"}
	}
	for _, parent := range parents {
		entries, _ := os.ReadDir(parent)
		for i, e := range entries {
			if i >= 128 || ctx.Err() != nil {
				break
			}
			p := filepath.Join(parent, e.Name())
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				paths = append(paths, p)
				// Linux desktop mounts are often one level below a username.
				if runtime.GOOS == "linux" && parent != "/mnt" {
					children, _ := os.ReadDir(p)
					for j, c := range children {
						if j < 128 && c.IsDir() {
							paths = append(paths, filepath.Join(p, c.Name()))
						}
					}
				}
			}
		}
	}
	seen := map[string]bool{}
	result := []Location{}
	for _, p := range paths {
		if ctx.Err() != nil {
			break
		}
		v, err := files.InspectVolume(p)
		if err != nil || seen[v.MountPoint] {
			continue
		}
		seen[v.MountPoint] = true
		label := filepath.Base(v.MountPoint)
		start := v.MountPoint
		if v.MountPoint == "/" || v.MountPoint == "/System/Volumes/Data" {
			label = "System drive · home folder"
			start = home
		}
		result = append(result, Location{label, start, v.Filesystem, v.AvailableBytes})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	for _, entry := range []struct{ label, path string }{{"Current project folder", cwd}, {"Home folder", home}} {
		if v, err := files.InspectVolume(entry.path); err == nil {
			result = append(result, Location{entry.label, entry.path, v.Filesystem, v.AvailableBytes})
		}
	}
	return result
}

//go:build linux || (darwin && !cgo)

package files

import "strings"

func nativeVolumeIdentity(path string) (string, error) {
	v, e := InspectVolume(path)
	return strings.ToLower(strings.ReplaceAll(v.Identity, "-", "")), e
}

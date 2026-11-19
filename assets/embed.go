// Package assets contains the immutable setup code and release input catalogue.
package assets

import "embed"

//go:embed catalog.json ubuntu-24.04.yaml presets/* guest/* github-known-hosts
var FS embed.FS

func Read(name string) ([]byte, error) { return FS.ReadFile(name) }

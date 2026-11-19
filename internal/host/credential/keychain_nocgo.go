//go:build darwin && !cgo

package credential

import (
	"context"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type unavailableStore struct{}

func Native() Store { return unavailableStore{} }
func (unavailableStore) Get(context.Context, string, bool) ([]byte, error) {
	return nil, model.Fail("dependency.unsupported", "Remembering requires a native macOS build with Keychain support.")
}
func (unavailableStore) Put(context.Context, string, []byte, bool) error {
	return model.Fail("dependency.unsupported", "Remembering requires a native macOS build with Keychain support.")
}
func (unavailableStore) Delete(context.Context, string, bool) error {
	return model.Fail("dependency.unsupported", "Remembering requires a native macOS build with Keychain support.")
}

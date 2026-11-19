//go:build darwin && cgo

package credential

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework Security -framework LocalAuthentication
#include <stdlib.h>
#include "keychain_darwin.h"
*/
import "C"
import (
	"context"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"strings"
	"unsafe"
)

type nativeStore struct{}

func Native() Store { return nativeStore{} }
func interactiveFlag(v bool) C.int {
	if v {
		return 1
	}
	return 0
}
func keychainError(status C.int) error {
	switch int(status) {
	case 0:
		return nil
	case -25300:
		return model.Fail("profile.unlock_required", "No remembered passphrase is available for this profile.")
	case -25308, -25293, -128:
		return model.Fail("credential.interaction_required", "Keychain requires user interaction or was cancelled.")
	default:
		return model.Fail("credential.denied", "Keychain did not permit the credential request.")
	}
}
func (nativeStore) Get(ctx context.Context, id string, interactive bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s := C.CString(id)
	defer C.free(unsafe.Pointer(s))
	var p unsafe.Pointer
	var n C.size_t
	status := C.el_get_secret(s, interactiveFlag(interactive), &p, &n)
	if err := keychainError(status); err != nil {
		return nil, err
	}
	defer C.el_free_secret(p, n)
	return C.GoBytes(p, C.int(n)), nil
}
func (nativeStore) Put(ctx context.Context, id string, b []byte, interactive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(b) == 0 {
		return model.Fail("secret.input_invalid", "Cannot remember an empty passphrase.")
	}
	s := C.CString(id)
	defer C.free(unsafe.Pointer(s))
	return keychainError(C.el_put_secret(s, unsafe.Pointer(&b[0]), C.size_t(len(b)), interactiveFlag(interactive)))
}
func (nativeStore) Delete(ctx context.Context, id string, interactive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s := C.CString(id)
	defer C.free(unsafe.Pointer(s))
	return keychainError(C.el_delete_secret(s, interactiveFlag(interactive)))
}

func (nativeStore) GetExternal(ctx context.Context, ref Reference, interactive bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ref.Kind != "keychain" || ref.Service == "" || ref.Account == "" || strings.ContainsRune(ref.Service+ref.Account, 0) {
		return nil, model.Fail("record.invalid", "Invalid external credential reference.")
	}
	service, account := C.CString(ref.Service), C.CString(ref.Account)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	var p unsafe.Pointer
	var n C.size_t
	status := C.el_get_external_secret(service, account, interactiveFlag(interactive), &p, &n)
	if err := keychainError(status); err != nil {
		return nil, err
	}
	defer C.el_free_secret(p, n)
	return C.GoBytes(p, C.int(n)), nil
}

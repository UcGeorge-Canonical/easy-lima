// Package files implements private, durable and conflict-aware local files.
package files

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

const SmallLimit = 64 << 10

func Hash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func Canonical(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var x any
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.UseNumber()
	if err = d.Decode(&x); err != nil {
		return nil, err
	}
	b, err = json.Marshal(x)
	return append(b, '\n'), err
}
func PathOK(path string) bool {
	return filepath.IsAbs(path) && len(path) <= 8192 && !strings.ContainsFunc(path, unicode.IsControl)
}
func PrivateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	i, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
		return model.Fail("settings.edited", "Expected a private directory; found another file type.")
	}
	if i.Mode().Perm()&0077 != 0 {
		return os.Chmod(path, 0700)
	}
	return nil
}
func Read(path string, limit int64) ([]byte, error) {
	i, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !i.Mode().IsRegular() {
		return nil, model.Fail("settings.edited", "Expected a regular file; symbolic links are not accepted here.")
	}
	if i.Size() > limit {
		return nil, model.Fail("record.invalid", "The file exceeds its size limit.")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if len(b) > int(limit) {
		return nil, model.Fail("record.invalid", "The file exceeds its size limit.")
	}
	return b, err
}
func Digest(path string) (string, error) {
	b, err := Read(path, 8<<20)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return Hash(b), nil
}
func SyncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// Replace requires the exact old digest (empty means absent). Callers hold an
// appropriate operation/publication guard across comparison and rename.
func Replace(path string, b []byte, old string, mode os.FileMode) error {
	d, err := Digest(path)
	if err != nil {
		return err
	}
	if d != old {
		return model.Fail("settings.edited", fmt.Sprintf("Managed file %q changed; preserve it and review the conflict.", path))
	}
	if d == Hash(b) {
		return nil
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".easy-lima-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	// Recheck immediately before publication, preserving intervening edits.
	if d, err = Digest(path); err != nil {
		return err
	}
	if d != old {
		return model.Fail("settings.edited", "The destination changed during publication.")
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	return SyncDir(filepath.Dir(path))
}
func WriteNew(path string, b []byte) error { return Replace(path, b, "", 0600) }
func RemoveExpected(path, digest string) error {
	got, err := Digest(path)
	if err != nil {
		return err
	}
	if got == "" {
		return nil
	}
	if got != digest {
		return model.Fail("settings.edited", "The file changed; refusing to remove it.")
	}
	if err = os.Remove(path); err != nil {
		return err
	}
	return SyncDir(filepath.Dir(path))
}

type Envelope struct {
	Payload json.RawMessage `json:"payload"`
	SHA256  string          `json:"sha256"`
}

func Wrap(v any) ([]byte, error) {
	b, err := Canonical(v)
	if err != nil {
		return nil, err
	}
	return Canonical(Envelope{Payload: json.RawMessage(b), SHA256: Hash(b)})
}
func Unwrap(b []byte, v any) error {
	var e Envelope
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return model.Fail("record.invalid", "Invalid document envelope.")
	}
	var raw any
	d = json.NewDecoder(strings.NewReader(string(e.Payload)))
	d.UseNumber()
	if err := d.Decode(&raw); err != nil {
		return err
	}
	p, err := Canonical(raw)
	if err != nil {
		return err
	}
	if Hash(p) != e.SHA256 {
		return model.Fail("record.invalid", "Document digest does not match its payload.")
	}
	d = json.NewDecoder(strings.NewReader(string(e.Payload)))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

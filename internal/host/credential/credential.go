// Package credential stores optional passphrases in the host's credential store.
// Secret bytes are never passed through process arguments or environment values.
package credential

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type Store interface {
	Get(context.Context, string, bool) ([]byte, error)
	Put(context.Context, string, []byte, bool) error
	Delete(context.Context, string, bool) error
}

func Clear(b []byte) { clear(b) }
func ReadFD(fd int) ([]byte, error) {
	if fd < 3 {
		return nil, model.Fail("secret.input_invalid", "Passphrase descriptor must be at least 3.")
	}
	dup, err := unix.Dup(fd)
	if err != nil {
		return nil, model.Fail("secret.input_invalid", "The inherited passphrase descriptor is not readable.")
	}
	unix.CloseOnExec(dup)
	f := os.NewFile(uintptr(dup), "passphrase")
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		Clear(b)
		return nil, model.Fail("secret.input_invalid", "The passphrase input is unreadable or exceeds 64 KiB.")
	}
	if bytes.HasSuffix(b, []byte("\r\n")) {
		b = b[:len(b)-2]
	} else if bytes.HasSuffix(b, []byte("\n")) {
		b = b[:len(b)-1]
	}
	if bytes.IndexByte(b, 0) >= 0 {
		Clear(b)
		return nil, model.Fail("secret.input_invalid", "The passphrase contains a NUL byte.")
	}
	return b, nil
}
func Prompt(label string) ([]byte, error) { return PromptContext(context.Background(), label) }
func PromptContext(ctx context.Context, label string) ([]byte, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, model.Fail("profile.unlock_required", "A passphrase is required; supply --passphrase-fd in noninteractive use.")
	}
	defer f.Close()
	if _, err = io.WriteString(f, label+": "); err != nil {
		return nil, err
	}
	previous, err := term.GetState(int(f.Fd()))
	if err != nil {
		return nil, err
	}
	defer term.Restore(int(f.Fd()), previous)
	type reply struct {
		value []byte
		err   error
	}
	ready := make(chan reply)
	go func() {
		b, e := term.ReadPassword(int(f.Fd()))
		select {
		case ready <- reply{b, e}:
		case <-ctx.Done():
			Clear(b)
		}
	}()
	var b []byte
	select {
	case <-ctx.Done():
		return nil, model.Fail("operation.cancelled", "Passphrase input cancelled.")
	case r := <-ready:
		b, err = r.value, r.err
	}
	_, _ = io.WriteString(f, "\n")
	if err != nil {
		return nil, model.Fail("secret.input_invalid", "Passphrase input was cancelled.")
	}
	return b, nil
}

// External references are observation-only: migration must never edit or delete
// a saved passphrase owned by another application.
type Reference struct {
	Kind    string `json:"kind"`
	Service string `json:"service"`
	Account string `json:"account"`
}
type ExternalReader interface {
	GetExternal(context.Context, Reference, bool) ([]byte, error)
}

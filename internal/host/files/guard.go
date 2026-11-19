package files

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"time"
)

type Guard struct{ f *os.File }

// Lock does not create a lock file for readers. It must be initialized by the
// owning explicit action before diagnostic access can take a shared lock.
func Lock(ctx context.Context, path string, exclusive, create bool) (*Guard, error) {
	flags := os.O_RDONLY
	if create {
		flags = os.O_RDWR | os.O_CREATE
	}
	f, err := os.OpenFile(path, flags|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	mode := unix.LOCK_SH
	if exclusive {
		mode = unix.LOCK_EX
	}
	for {
		err = unix.Flock(int(f.Fd()), mode|unix.LOCK_NB)
		if err == nil {
			return &Guard{f: f}, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
func (g *Guard) Close() error {
	if g == nil || g.f == nil {
		return nil
	}
	f := g.f
	g.f = nil
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return f.Close()
}

//go:build contracts

package agent

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRemovalWaitsForAsynchronousSocketShutdown(t *testing.T) {
	dir, err := os.MkdirTemp("", "ela-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "agent.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	closed := make(chan struct{})
	go func() { time.Sleep(100 * time.Millisecond); listener.Close(); close(closed) }()
	if err := waitSocketInactive(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
	default:
		t.Fatal("retired ownership while listener remained active")
	}
}

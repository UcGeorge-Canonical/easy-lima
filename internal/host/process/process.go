// Package process keeps subprocess output separate from operational evidence.
package process

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Request struct {
	Executable     string
	Args           []string
	Env            []string
	Dir            string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Capture        io.Writer
	Limit          int
	Timeout        time.Duration
}
type Result struct {
	Stdout, Stderr             []byte
	ExitCode                   int
	Started, Exited, Truncated bool
}
type Runner interface {
	Run(context.Context, Request) (Result, error)
}
type OS struct{}
type boundedBuffer struct {
	mu        sync.Mutex
	b         bytes.Buffer
	limit     int
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	left := b.limit - b.b.Len()
	if left > len(p) {
		left = len(p)
	}
	if left > 0 {
		b.b.Write(p[:left])
	}
	if n > left {
		b.truncated = true
	}
	return n, nil
}

type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}
func (OS) Run(ctx context.Context, r Request) (Result, error) {
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	if r.Limit <= 0 {
		r.Limit = 1 << 20
	}
	out, stderr := &boundedBuffer{limit: r.Limit}, &boundedBuffer{limit: r.Limit}
	cmd := exec.CommandContext(ctx, r.Executable, r.Args...)
	cmd.Dir = r.Dir
	cmd.Env = r.Env
	cmd.Stdin = r.Stdin
	// Cancel only the foreground control process; descendants may be persistent
	// VM agents or guest jobs whose lifetime is reconciled by the coordinator.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 3 * time.Second
	cmd.Stdout = out
	cmd.Stderr = stderr
	streams := &sync.Mutex{}
	if r.Stdout != nil {
		cmd.Stdout = io.MultiWriter(out, &lockedWriter{mu: streams, w: r.Stdout})
	}
	if r.Stderr != nil {
		cmd.Stderr = io.MultiWriter(stderr, &lockedWriter{mu: streams, w: r.Stderr})
	}
	if r.Capture != nil {
		w := &lockedWriter{mu: streams, w: r.Capture}
		cmd.Stdout = io.MultiWriter(cmd.Stdout, w)
		cmd.Stderr = io.MultiWriter(cmd.Stderr, w)
	}
	v := Result{ExitCode: -1}
	if err := cmd.Start(); err != nil {
		return v, model.Fail("dependency.missing", "The required program could not be started.")
	}
	v.Started = true
	err := cmd.Wait()
	v.Stdout = out.b.Bytes()
	v.Stderr = stderr.b.Bytes()
	v.Truncated = out.truncated || stderr.truncated
	if cmd.ProcessState != nil {
		v.Exited = cmd.ProcessState.Exited()
		v.ExitCode = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() != nil {
		return v, model.Fail("operation.unknown", "The foreground observation ended; an already dispatched effect may still be active.")
	}
	var ee *exec.ExitError
	if err != nil && !errors.As(err, &ee) {
		return v, model.Fail("operation.unknown", "The subprocess result is incomplete.")
	}
	return v, nil
}

// Environment deliberately excludes ambient SSH, Git and Lima override values.
// HOME remains the user's real home; large assets receive explicit paths.
func Environment(extra map[string]string) []string {
	allow := map[string]bool{"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "LANG": true, "LC_ALL": true, "TERM": true, "DISPLAY": true, "DBUS_SESSION_BUS_ADDRESS": true, "XDG_RUNTIME_DIR": true, "XDG_CONFIG_HOME": true, "XDG_STATE_HOME": true, "XDG_CACHE_HOME": true, "SYSTEMROOT": true}
	out := []string{}
	for _, v := range os.Environ() {
		k, _, _ := strings.Cut(v, "=")
		if allow[k] {
			if _, override := extra[k]; !override {
				out = append(out, v)
			}
		}
	}
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}
func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func ShellArgs(args []string) string {
	q := make([]string, len(args))
	for i, s := range args {
		q[i] = Quote(s)
	}
	return strings.Join(q, " ")
}

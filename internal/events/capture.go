// Package events keeps opt-in raw output separate from operational evidence.
package events

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

const captureLimit = 32 << 20

var sensitive = regexp.MustCompile(`(?i)(authorization\s*:|\b(password|passphrase|access[_-]?token|client[_-]?secret|private[_-]?key)\s*[=:]|\bgh[pousr]_[A-Za-z0-9]+|github_pat_[A-Za-z0-9_]+|\bAKIA[A-Z0-9]{16}\b|\bBearer\s+[A-Za-z0-9._-]+)`)

type Capture struct {
	mu            sync.Mutex
	file          *os.File
	line          []byte
	pem, dropping bool
	remaining     int
	err           error
}

func Open(dir, id string) (*Capture, error) {
	if !model.ValidID(id) {
		return nil, model.Fail("input.invalid", "Invalid capture operation ID.")
	}
	if err := files.PrivateDir(dir); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, id+".raw.log")
	if st, err := os.Lstat(p); err == nil && !st.Mode().IsRegular() {
		return nil, model.Fail("settings.edited", "Raw capture path is not a regular file.")
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	remaining := captureLimit - int(st.Size())
	if remaining < 0 {
		remaining = 0
	}
	return &Capture{file: f, remaining: remaining}, nil
}
func (c *Capture) emit(b []byte) {
	if c.remaining <= 0 || c.err != nil {
		return
	}
	if len(b) > c.remaining {
		b = b[:c.remaining]
	}
	n, e := c.file.Write(b)
	c.remaining -= n
	c.err = e
}
func (c *Capture) flush() {
	text := string(c.line)
	if strings.Contains(text, "-----BEGIN ") && strings.Contains(text, "PRIVATE KEY-----") {
		c.pem = true
	}
	if c.pem || c.dropping || sensitive.Match(c.line) {
		c.emit([]byte("[redacted]\n"))
	} else {
		c.emit(c.line)
	}
	if strings.Contains(text, "-----END ") && strings.Contains(text, "PRIVATE KEY-----") {
		c.pem = false
	}
	c.line = c.line[:0]
	c.dropping = false
}
func (c *Capture) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(b)
	for len(b) > 0 {
		at := bytes.IndexByte(b, '\n')
		end := len(b)
		if at >= 0 {
			end = at + 1
		}
		if len(c.line)+end > 64<<10 {
			c.dropping = true
		}
		if !c.dropping {
			c.line = append(c.line, b[:end]...)
		}
		b = b[end:]
		if at >= 0 {
			c.flush()
		}
	}
	// A full capture must still drain the producer and may not hold up a VM job.
	return n, nil
}
func (c *Capture) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.line) > 0 || c.dropping {
		c.flush()
	}
	if e := c.file.Sync(); c.err == nil {
		c.err = e
	}
	if e := c.file.Close(); c.err == nil {
		c.err = e
	}
	return c.err
}

var _ io.WriteCloser = (*Capture)(nil)

// Package connection provides pinned VM transport, distinct from Git identity.
package connection

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"golang.org/x/crypto/ssh"
)

type Endpoint struct {
	Address      string `json:"address"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	IdentityFile string `json:"identity_file"`
	VMID         string `json:"vm_id"`
}

func ReadEndpoint(root, name, id string) (Endpoint, error) {
	b, err := files.Read(filepath.Join(root, name, "ssh.config"), 64<<10)
	if err != nil {
		return Endpoint{}, model.Fail("check.incomplete", "The VM transport configuration is unavailable.")
	}
	e := Endpoint{VMID: id}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, " ")
		if !ok {
			key, val, ok = strings.Cut(line, "\t")
		}
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		switch strings.ToLower(key) {
		case "hostname":
			e.Address = val
		case "port":
			e.Port, _ = strconv.Atoi(val)
		case "user":
			e.User = val
		case "identityfile":
			if e.IdentityFile != "" && e.IdentityFile != val {
				return Endpoint{}, model.Fail("dependency.unsupported", "Multiple VM-login identities are unsupported.")
			}
			e.IdentityFile = val
		}
	}
	if e.Address != "127.0.0.1" && e.Address != "::1" {
		return e, model.Fail("ownership.conflict", "VM SSH must use a literal loopback endpoint.")
	}
	if e.Port < 1 || e.Port > 65535 || e.User == "" || strings.ContainsAny(e.User, " \t\n\r\x00") || !files.PathOK(e.IdentityFile) {
		return e, model.Fail("record.invalid", "VM SSH connection hints are incomplete.")
	}
	if !strings.HasPrefix(e.IdentityFile, filepath.Clean(root)+string(filepath.Separator)) {
		return e, model.Fail("ownership.conflict", "The VM-login key is outside the selected Lima namespace.")
	}
	return e, nil
}
func (e Endpoint) AddressPort() string { return net.JoinHostPort(e.Address, strconv.Itoa(e.Port)) }

type Client struct {
	SSH      *ssh.Client
	Endpoint Endpoint
	HostKey  ssh.PublicKey
}

func Dial(ctx context.Context, e Endpoint, pin ssh.PublicKey, enroll bool) (*Client, error) {
	if pin == nil && !enroll {
		return nil, model.Fail("trust.mismatch", "The VM has no enrolled host key. Use an explicit setup or repair action.")
	}
	b, err := files.Read(e.IdentityFile, 1<<20)
	if err != nil {
		return nil, model.Fail("dependency.missing", "The recorded Lima login key is unavailable.")
	}
	signer, err := ssh.ParsePrivateKey(b)
	clear(b)
	if err != nil {
		return nil, model.Fail("dependency.unsupported", "The Lima login key could not be parsed.")
	}
	var observed ssh.PublicKey
	cfg := &ssh.ClientConfig{User: e.User, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyAlgorithms: []string{ssh.KeyAlgoED25519, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}, HostKeyCallback: func(host string, remote net.Addr, key ssh.PublicKey) error {
		observed = key
		if pin != nil && !bytes.Equal(pin.Marshal(), key.Marshal()) {
			return model.Fail("trust.mismatch", "The VM host key changed; no profile credentials were forwarded.")
		}
		return nil
	}, Timeout: 10 * time.Second}
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", e.AddressPort())
	if err != nil {
		return nil, model.Fail("check.incomplete", "The VM SSH endpoint is not reachable.")
	}
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	c, chans, reqs, err := ssh.NewClientConn(conn, e.AddressPort(), cfg)
	if err != nil {
		conn.Close()
		var me *model.Error
		if errors.As(err, &me) {
			return nil, me
		}
		if observed != nil && pin != nil && !bytes.Equal(observed.Marshal(), pin.Marshal()) {
			return nil, model.Fail("trust.mismatch", "The VM host key differs from its enrolled key.")
		}
		return nil, model.Fail("check.incomplete", "The pinned VM SSH handshake could not be established.")
	}
	_ = conn.SetDeadline(time.Time{})
	return &Client{ssh.NewClient(c, chans, reqs), e, observed}, nil
}
func (c *Client) Close() error { return c.SSH.Close() }
func (c *Client) Run(ctx context.Context, args []string, in io.Reader, out, stderr io.Writer) (int, error) {
	if ctx.Err() != nil {
		return 255, model.Fail("check.incomplete", "The guest command was cancelled before dispatch.")
	}
	// Channel creation can block too (for example, while sshd waits for the
	// guest login service). Closing the transport interrupts both channel and
	// exec requests; closing a session alone does not bound a stuck channel open.
	type opened struct {
		session *ssh.Session
		err     error
	}
	opening := make(chan opened, 1)
	go func() {
		session, err := c.SSH.NewSession()
		opening <- opened{session, err}
	}()
	var session *ssh.Session
	select {
	case <-ctx.Done():
		c.Close()
		return 255, model.Fail("check.incomplete", "The VM SSH session did not become ready before cancellation or timeout. Inspect the guest SSH and login services, then retry the saved operation.")
	case result := <-opening:
		if result.err != nil {
			return 255, model.Fail("check.incomplete", "The VM SSH session could not be opened; no guest command was dispatched.")
		}
		session = result.session
	}
	defer session.Close()
	session.Stdin = in
	// SSH drains stdout and stderr concurrently. Callers may deliberately
	// use one writer; serialize writes and avoid a shared ReaderFrom fast path.
	mu := new(sync.Mutex)
	if out != nil {
		session.Stdout = streamWriter{mu, out}
	}
	if stderr != nil {
		session.Stderr = streamWriter{mu, stderr}
	}
	done := make(chan error, 1)
	go func() { done <- session.Run(process.ShellArgs(args)) }()
	select {
	case <-ctx.Done():
		c.Close()
		return 255, model.Fail("operation.unknown", "The guest command result is unknown after cancellation or timeout. Inspect the guest SSH and login services, then reconcile the saved operation before retrying.")
	case err := <-done:
		if err == nil {
			return 0, nil
		}
		var ee *ssh.ExitError
		if errors.As(err, &ee) {
			return ee.ExitStatus(), nil
		}
		return 255, model.Fail("operation.unknown", "The guest command did not report an exit status.")
	}
}

type limitBuffer struct {
	b         bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitBuffer) Write(p []byte) (int, error) {
	n := len(p)
	left := b.limit - b.b.Len()
	if left > len(p) {
		left = len(p)
	}
	if left > 0 {
		b.b.Write(p[:left])
	}
	if left < n {
		b.truncated = true
	}
	return n, nil
}
func (c *Client) Output(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	out := &limitBuffer{limit: 256 << 10}
	errout := &limitBuffer{limit: 16 << 10}
	exit, err := c.Run(ctx, args, nil, out, errout)
	if err != nil {
		return nil, err
	}
	if out.truncated {
		return nil, model.Fail("output.truncated", "Guest response exceeded its bound.")
	}
	if exit != 0 {
		return nil, model.Fail("operation.failed", fmt.Sprintf("Guest check returned status %d.", exit))
	}
	return out.b.Bytes(), nil
}
func (c *Client) ExecWorkspace(ctx context.Context, args []string, in io.Reader, out, stderr io.Writer) (int, error) {
	return c.Run(ctx, append([]string{"sh", "-c", `cd /workspace && exec "$@"`, "easy-lima-exec"}, args...), in, out, stderr)
}

type streamWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (w streamWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

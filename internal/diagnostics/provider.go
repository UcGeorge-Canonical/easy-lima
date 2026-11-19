// Package diagnostics contains probes which never repair, activate, or publish.
package diagnostics

import (
	"bytes"
	"context"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/assets"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"golang.org/x/crypto/ssh"
)

// GitHub authenticates only with the already observed, single profile key. The
// greeting is bounded and is never returned verbatim or written to a log.
func GitHub(ctx context.Context, c *agent.Client, expected *string) error {
	keys, err := c.Agent.Signers()
	if err != nil || len(keys) != 1 {
		return model.Fail("identity.key_mismatch", "Exactly one profile key is required for the provider check.")
	}
	pins, err := assets.Read("github-known-hosts")
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, line := range strings.Split(string(pins), "\n") {
		_, _, key, _, _, e := ssh.ParseKnownHosts([]byte(line))
		if e == nil {
			allowed[string(key.Marshal())] = true
		}
	}
	cfg := &ssh.ClientConfig{User: "git", Auth: []ssh.AuthMethod{ssh.PublicKeys(keys...)}, HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
		if !allowed[string(k.Marshal())] {
			return model.Fail("trust.mismatch", "GitHub's host key does not match the bundled publisher keys.")
		}
		return nil
	}, Timeout: 8 * time.Second}
	d := net.Dialer{Timeout: 8 * time.Second}
	raw, err := d.DialContext(ctx, "tcp", "github.com:22")
	if err != nil {
		return model.Fail("provider.unverified", "GitHub could not be reached.")
	}
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(10 * time.Second))
	conn, ch, req, err := ssh.NewClientConn(raw, "github.com:22", cfg)
	if err != nil {
		if model.AsError(err).Code == "trust.mismatch" {
			return err
		}
		return model.Fail("provider.unverified", "GitHub did not complete profile authentication.")
	}
	client := ssh.NewClient(conn, ch, req)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return model.Fail("provider.unverified", "GitHub did not provide its account greeting.")
	}
	defer session.Close()
	out := &greetingBuffer{}
	session.Stdout = out
	session.Stderr = out
	if err = session.Shell(); err != nil {
		return model.Fail("provider.unverified", "GitHub's account greeting was unavailable.")
	}
	// Git hosting services normally close with a nonzero exit status because
	// interactive shells are disabled. Authentication is established by the
	// pinned SSH connection and the bounded account greeting on either stream.
	_ = session.Wait()
	if out.truncated {
		return model.Fail("provider.unverified", "GitHub's account greeting could not be bounded or verified.")
	}
	return verifyGreeting(out.Bytes(), expected)
}

type greetingBuffer struct {
	sync.Mutex
	buffer    bytes.Buffer
	truncated bool
}

func (b *greetingBuffer) Bytes() []byte { return b.buffer.Bytes() }
func (b *greetingBuffer) Len() int      { return b.buffer.Len() }

func (b *greetingBuffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	n := len(p)
	remaining := 4096 - b.Len()
	if n > remaining {
		b.truncated = true
		p = p[:remaining]
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}

func verifyGreeting(b []byte, expected *string) error {
	account := regexp.MustCompile(`(?m)^Hi ([A-Za-z0-9-]{1,39})! You've successfully authenticated,`).FindSubmatch(b)
	if len(account) != 2 {
		return model.Fail("provider.unverified", "GitHub authentication succeeded without a recognized account greeting.")
	}
	if expected != nil && !strings.EqualFold(string(account[1]), *expected) {
		return model.Fail("provider.account_mismatch", "The key authenticates as a different GitHub account from the profile's expected account.")
	}
	return nil
}

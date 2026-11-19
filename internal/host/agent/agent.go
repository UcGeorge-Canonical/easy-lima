// Package agent manages independent OpenSSH agents without inheriting the
// caller's SSH_AUTH_SOCK. External agents are observed, never administered.
package agent

import (
	"context"
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

type Binding struct {
	SocketPath              string `json:"socket_path"`
	SupervisorKind          string `json:"supervisor_kind"`
	SupervisorRef           string `json:"supervisor_ref"`
	OwnershipEvidenceDigest string `json:"ownership_evidence_digest"`
}
type Client struct {
	Conn  net.Conn
	Agent sshagent.ExtendedAgent
}

func Connect(ctx context.Context, path string) (*Client, error) {
	i, err := os.Lstat(path)
	if err != nil || i.Mode()&os.ModeSocket == 0 {
		return nil, model.Fail("profile.unlock_required", "The profile's SSH agent socket is unavailable.")
	}
	if s, ok := i.Sys().(*syscall.Stat_t); !ok || int(s.Uid) != os.Getuid() {
		return nil, model.Fail("ownership.conflict", "The agent socket belongs to another user.")
	}
	d := net.Dialer{Timeout: 3 * time.Second}
	c, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, model.Fail("profile.unlock_required", "The profile's SSH agent is not reachable.")
	}
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	return &Client{c, sshagent.NewClient(c)}, nil
}
func (c *Client) Close() { c.Conn.Close() }

// Service managers can return before the agent closes its listener. Confirm
// inactivity before retiring its ownership record; an inaccessible socket is
// not evidence that the process has stopped.
func waitSocketInactive(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		c, err := (&net.Dialer{Timeout: 250 * time.Millisecond}).DialContext(ctx, "unix", path)
		if errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED) {
			return nil
		}
		if err != nil {
			return model.Fail("operation.unknown", "The agent socket could not be confirmed inactive.")
		}
		c.Close()
		select {
		case <-ctx.Done():
			return model.Fail("operation.unknown", "The agent socket remains active after stopping its service; resume removal after it exits.")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
func (c *Client) Verify(fingerprint string) error {
	keys, err := c.Agent.List()
	if err != nil {
		return model.Fail("profile.unlock_required", "The profile agent could not list its keys.")
	}
	if len(keys) == 0 {
		return model.Fail("profile.unlock_required", "The profile key is not loaded.")
	}
	if len(keys) != 1 {
		return model.Fail("identity.key_mismatch", "The profile agent exposes more than the selected key.")
	}
	pub, err := ssh.ParsePublicKey(keys[0].Blob)
	if err != nil || ssh.FingerprintSHA256(pub) != fingerprint {
		return model.Fail("identity.key_mismatch", "The loaded agent key differs from this profile.")
	}
	return nil
}
func (c *Client) SignProbe(fingerprint string) error {
	if err := c.Verify(fingerprint); err != nil {
		return err
	}
	signers, err := c.Agent.Signers()
	if err != nil || len(signers) != 1 {
		return model.Fail("identity.key_mismatch", "The selected signing key is unavailable.")
	}
	data := make([]byte, 64)
	if _, err = rand.Read(data); err != nil {
		return err
	}
	challenge := signingChallenge(data)
	var sig *ssh.Signature
	if signers[0].PublicKey().Type() == ssh.KeyAlgoRSA {
		sig, err = c.Agent.SignWithFlags(signers[0].PublicKey(), challenge, sshagent.SignatureFlagRsaSha512)
	} else {
		sig, err = signers[0].Sign(rand.Reader, challenge)
	}
	if err != nil {
		return model.Fail("check.failed", "The profile agent refused the signing challenge.")
	}
	if err = signers[0].PublicKey().Verify(challenge, sig); err != nil {
		return model.Fail("check.failed", "The signing challenge did not verify.")
	}
	return nil
}
func Socket(cache, id string) (string, error) {
	dir, err := files.RuntimeDir(cache)
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, id+".sock")
	if len(p) >= 100 {
		return "", model.Invalid("agent socket", "the configured user cache path is too long for a Unix socket")
	}
	return p, nil
}
func Plan(ctx context.Context, id string, b Binding) (Binding, error) {
	planned, _, err := plan(ctx, id, b)
	return planned, err
}
func VerifyOwnership(ctx context.Context, id string, b Binding) error {
	return verifySupervisor(ctx, id, b)
}
func Ensure(ctx context.Context, id string, b Binding) (Binding, error) {
	if b.SupervisorKind == "external" {
		return b, model.Fail("profile.external_managed", "An external agent cannot be started or reconfigured by easy-lima.")
	}
	if !model.ValidID(id) || !files.PathOK(b.SocketPath) {
		return b, model.Fail("record.invalid", "Invalid profile agent binding.")
	}
	if err := files.PrivateDir(filepath.Dir(b.SocketPath)); err != nil {
		return b, err
	}
	g, err := files.Lock(ctx, filepath.Join(filepath.Dir(b.SocketPath), id+".lock"), true, true)
	if err != nil {
		return b, err
	}
	defer g.Close()
	if c, err := Connect(ctx, b.SocketPath); err == nil {
		c.Close()
		return b, verifySupervisor(ctx, id, b)
	}
	if info, e := os.Lstat(b.SocketPath); e == nil {
		// Stale socket removal requires both supervisor and listener absence.
		if info.Mode()&os.ModeSocket == 0 {
			return b, model.Fail("ownership.conflict", "The agent socket path is occupied by another object.")
		}
		if e = supervisorInactive(ctx, id, b); e != nil {
			return b, e
		}
		conn, e := net.DialTimeout("unix", b.SocketPath, time.Second)
		if e == nil {
			conn.Close()
			return b, model.Fail("ownership.conflict", "The old agent socket is active.")
		}
		if !errors.Is(e, syscall.ECONNREFUSED) && !errors.Is(e, syscall.ENOENT) {
			return b, model.Fail("operation.unknown", "The old agent socket cannot be proven inactive.")
		}
		current, e := os.Lstat(b.SocketPath)
		if e == nil {
			if !os.SameFile(info, current) {
				return b, model.Fail("ownership.conflict", "The agent socket changed during recovery.")
			}
			if e = os.Remove(b.SocketPath); e != nil {
				return b, e
			}
		}
		if e != nil && !os.IsNotExist(e) {
			return b, e
		}
	} else if !os.IsNotExist(e) {
		return b, e
	}
	return start(ctx, id, b)
}
func Wait(ctx context.Context, path string) error {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		c, err := Connect(ctx, path)
		if err == nil {
			c.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return model.Fail("profile.unlock_required", fmt.Sprintf("The owned agent did not become ready at %q.", path))
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// signingChallenge uses the OpenSSH SSHSIG protocol and Git's namespace.
// https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig
func signingChallenge(message []byte) []byte {
	digest := sha512.Sum512(message)
	return append([]byte("SSHSIG"), ssh.Marshal(struct {
		Namespace, Reserved, Hash string
		Digest                    []byte
	}{"git", "", "sha512", digest[:]})...)
}

// UnloadOwned succeeds only after observed empty keys or proven inactive ownership.
func UnloadOwned(ctx context.Context, id string, b Binding, fingerprint string) error {
	if err := VerifyOwnership(ctx, id, b); err != nil {
		return err
	}
	c, err := Connect(ctx, b.SocketPath)
	if err != nil {
		if model.AsError(err).Code != "profile.unlock_required" {
			return err
		}
		if e := supervisorInactive(ctx, id, b); e != nil {
			return e
		}
		return waitSocketInactive(ctx, b.SocketPath)
	}
	defer c.Close()
	keys, err := c.Agent.List()
	if err != nil {
		return model.Fail("operation.unknown", "The owned agent could not report its keys.")
	}
	if len(keys) == 0 {
		return nil
	}
	if err = c.Verify(fingerprint); err != nil {
		return err
	}
	if err = c.Agent.RemoveAll(); err != nil {
		return model.Fail("operation.unknown", "The owned agent could not confirm key removal.")
	}
	keys, err = c.Agent.List()
	if err != nil || len(keys) != 0 {
		return model.Fail("operation.unknown", "The owned agent is not confirmed empty.")
	}
	return nil
}

//go:build linux && contracts

package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

func TestDetachedAgentsIsolateKeysAndRecoverDeadProcess(t *testing.T) {
	if _, err := exec.LookPath("ssh-agent"); err != nil {
		t.Skip("OpenSSH unavailable")
	}
	dir, err := os.MkdirTemp("/tmp", "el-agent-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	bindings := []Binding{}
	ids := []string{model.NewID(), model.NewID()}
	fingerprints := []string{}
	for _, id := range ids {
		b, err := Plan(ctx, id, Binding{SocketPath: filepath.Join(dir, id+".sock"), SupervisorKind: "detached"})
		if err != nil {
			t.Fatal(err)
		}
		b, err = Ensure(ctx, id, b)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if e := Remove(context.Background(), id, b); e != nil {
				t.Error(e)
			}
		})
		bindings = append(bindings, b)
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		public, _ := ssh.NewPublicKey(pub)
		fingerprints = append(fingerprints, ssh.FingerprintSHA256(public))
		c, err := Connect(ctx, b.SocketPath)
		if err != nil {
			t.Fatal(err)
		}
		err = c.Agent.Add(sshagent.AddedKey{PrivateKey: key})
		if err == nil {
			err = c.SignProbe(ssh.FingerprintSHA256(public))
		}
		c.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, b := range bindings {
		c, err := Connect(ctx, b.SocketPath)
		if err != nil {
			t.Fatal(err)
		}
		if err = c.Verify(fingerprints[1-i]); err == nil {
			t.Fatal("agent exposed another profile's key")
		}
		c.Close()
	}
	// Simulate a crash, retaining the owned socket and process-start witness.
	raw, err := os.ReadFile(bindings[0].SupervisorRef + ".process")
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	var start string
	if _, err = fmt.Sscanf(string(raw), "%d %s", &pid, &start); err != nil {
		t.Fatal(err)
	}
	if err = syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	for {
		if _, err = os.Stat(fmt.Sprintf("/proc/%d", pid)); os.IsNotExist(err) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if _, err = Ensure(ctx, ids[0], bindings[0]); err != nil {
		t.Fatal(err)
	}
	c, err := Connect(ctx, bindings[0].SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := c.Agent.List()
	c.Close()
	if err != nil || len(keys) != 0 {
		t.Fatalf("restarted agent silently substituted a key: %v", err)
	}
	c, err = Connect(ctx, bindings[1].SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err = c.SignProbe(fingerprints[1]); err != nil {
		t.Fatal(err)
	}
}

//go:build contracts

package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"golang.org/x/crypto/ssh"
)

func TestAgentReloadUnderRetainedVMClaim(t *testing.T) {
	if os.Getenv("EASY_LIMA_TEST_AGENTS") != "1" {
		t.Skip("opt in to a disposable owned profile service")
	}
	ctx := context.Background()
	root, err := os.MkdirTemp("", "el-agent-recovery-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	t.Setenv("HOME", root)
	paths := config.Paths{Config: filepath.Join(root, "config"), State: filepath.Join(root, "state"), Cache: filepath.Join(root, "cache"), Home: root}
	vmroot := filepath.Join(root, "vms")
	if err = os.Mkdir(vmroot, 0700); err != nil {
		t.Fatal(err)
	}
	volume, err := files.RootIdentity(vmroot)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Init(ctx, paths.State, vmroot, volume)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(private, "disposable contract fixture")
	if err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(root, "fixture-key")
	if err = os.WriteFile(key, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatal(err)
	}
	pub, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(key+".pub", ssh.MarshalAuthorizedKey(pub), 0600); err != nil {
		t.Fatal(err)
	}
	s := Service{Paths: paths}
	created := s.Execute(ctx, Request{Command: "profile.import", Label: "recovery-test", Key: key, GitName: "Recovery Test", GitEmail: "test@example.invalid", PassphraseFD: -1, Yes: true})
	if created.ExitCode != 0 {
		t.Fatalf("profile import: %+v", created.Errors)
	}
	db, err = store.Open(ctx, paths.State, true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p, err := db.Profile(ctx, "recovery-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := agent.Remove(ctx, p.Profile.ID, p.Agent); err != nil {
			t.Error(err)
		}
	}()
	op, err := db.Prepare(ctx, model.NewPlan("up", model.Global()), nil, []store.Claim{{Kind: "profile", Key: p.Profile.ID, Mode: "shared"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	c, err := agent.Connect(ctx, p.Agent.SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	err = c.Agent.RemoveAll()
	c.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ensureProfileForConnection(ctx, db, op, p, Request{PassphraseFD: -1}); err != nil {
		t.Fatal(err)
	}
	after, err := db.Operation(ctx, op.Operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Operation.Status == "completed" || after.Operation.Status == "failed_known" {
		t.Fatal("key activation terminated its enclosing operation")
	}
	c, err = agent.Connect(ctx, p.Agent.SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err = c.SignProbe(*p.Profile.Fingerprint); err != nil {
		t.Fatal(err)
	}
	if err = db.Finish(ctx, op, "completed"); err != nil {
		t.Fatal(err)
	}
}

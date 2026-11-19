package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

func TestMissingAgentKeyDoesNotCompleteEnclosingVMOperation(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	vmroot := filepath.Join(root, "vms")
	if err := os.Mkdir(vmroot, 0700); err != nil {
		t.Fatal(err)
	}
	volume, err := files.RootIdentity(vmroot)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Init(ctx, filepath.Join(root, "state"), vmroot, volume)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	op, err := db.Prepare(ctx, model.NewPlan("up", model.Global()), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := store.ProfileRecord{}
	p.Profile.ID = model.NewID()
	p.Profile.AgentManagement = "owned"
	p.Profile.Fingerprint = model.Ptr("missing-key-fingerprint")
	p.Key.Path = filepath.Join(root, "missing-key")
	service := Service{}
	if err = service.loadProfileKey(ctx, db, op, p, Request{PassphraseFD: -1}, false); err == nil {
		t.Fatal("missing key unexpectedly loaded")
	}
	after, err := db.Operation(ctx, op.Operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Operation.Status != op.Operation.Status {
		t.Fatal("agent failure terminated the VM operation")
	}
}

func TestExecWithoutGuestStatusReturnsTransportExit(t *testing.T) {
	s := Service{Paths: config.Paths{State: filepath.Join(t.TempDir(), "missing")}}
	r := s.Execute(context.Background(), Request{Command: "exec", GuestArgs: []string{"true"}, PassphraseFD: -1})
	if r.ExitCode != 255 || len(r.Errors) != 1 {
		t.Fatalf("lost exec preflight failure: %+v", r)
	}
}

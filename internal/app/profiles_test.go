package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

func TestDesiredProfileResumesAfterFilePublication(t *testing.T) {
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
	service := Service{Paths: config.Paths{Config: filepath.Join(root, "config"), State: db.Dir}}
	p := config.Profile{SchemaVersion: 1, ID: model.NewID(), Label: "work", Git: model.GitIdentity{Name: "Original Developer", Email: "work@example.invalid", SignCommits: true, SignTags: true}}
	p.Key.Path = "/test/key"
	p.Key.PublicPath = "/test/key.pub"
	op, err := db.Prepare(ctx, model.NewPlan("profile.import", model.Scope{Kind: "profile", ID: &p.ID}), map[string]string{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.SaveExternalProfile(ctx, op, p, "public", "fingerprint", agent.Binding{SupervisorKind: "external"}); err != nil {
		t.Fatal(err)
	}
	if err = db.Finish(ctx, op, "completed"); err != nil {
		t.Fatal(err)
	}
	raw, err := config.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(service.Paths.Config, "profiles", p.ID+".toml")
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err = files.WriteNew(target, raw); err != nil {
		t.Fatal(err)
	}
	desired := p
	desired.Git.Name = "Updated Developer"
	in := ProfileFileInput{ProfileID: p.ID, OldDigest: files.Hash(raw), Desired: desired}
	op, err = db.Prepare(ctx, model.NewPlan("profile.edit", model.Scope{Kind: "profile", ID: &p.ID}), in, []store.Claim{{Kind: "profile", Key: p.ID, Mode: "exclusive"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	db.Fault = func(boundary string) error {
		if boundary == "sql_committed" {
			return errors.New("power loss after file publication")
		}
		return nil
	}
	if err = service.writeDesiredProfile(ctx, db, op, in); err == nil {
		t.Fatal("fault did not fire")
	}
	db.Fault = nil
	if err = db.ReconcilePublication(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = service.resume(ctx, db, Request{Command: "operations.resume", ID: op.Operation.ID}); err != nil {
		t.Fatal(err)
	}
	f, err := config.LoadProfile(target)
	if err != nil || f.Value.Git.Name != desired.Git.Name {
		t.Fatalf("desired edit lost: %+v %v", f, err)
	}
	active, err := db.Profile(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active.Profile.Git.Name != p.Git.Name {
		t.Fatal("editing desired intent changed the applied identity")
	}
	if claims, e := db.PendingClaims(ctx, op.Operation.ID); e != nil || len(claims) != 0 {
		t.Fatalf("claims retained: %+v %v", claims, e)
	}
	// A later user edit must never be overwritten when resuming an old intent.
	f.Value.Git.Email = "user-edit@example.invalid"
	raw, _ = config.Encode(f.Value)
	if err = files.Replace(target, raw, f.Digest, 0600); err != nil {
		t.Fatal(err)
	}
	if err = service.writeDesiredProfile(ctx, db, op, in); err == nil || model.AsError(err).Code != "settings.edited" {
		t.Fatalf("user edit accepted: %v", err)
	}
}

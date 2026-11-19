package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func TestBackupRestorePreservesPendingClaims(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	op, err := s.Prepare(ctx, model.NewPlan("test", model.Global()), map[string]string{"frozen": "input"}, []Claim{{Kind: "vm", Key: model.NewID(), Mode: "exclusive"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup")
	manifest, err := s.Backup(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateRestore(ctx, s.Dir, path); err != nil {
		t.Fatal(err)
	}
	dir := s.Dir
	s.Close()
	// Recover a corrupted main file without allowing SQLite to mutate it first.
	if err = os.WriteFile(filepath.Join(dir, "state.sqlite3"), []byte("damaged database"), 0600); err != nil {
		t.Fatal(err)
	}
	reg, err := Restore(ctx, dir, path)
	if err != nil {
		t.Fatal(err)
	}
	if reg.ID != manifest.RegistryID || reg.Generation != manifest.RecoveryGeneration+1 {
		t.Fatalf("wrong restored lineage: %#v", reg)
	}
	restored, err := Open(ctx, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	claims, err := restored.PendingClaims(ctx, op.Operation.ID)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claims lost: %#v %v", claims, err)
	}
	if err = restored.VerifyProjection(ctx); err != nil {
		t.Fatal(err)
	}
	if err = restored.VerifyHead(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreAdmitsOnlyCommittedOwnerPublicationPrefix(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	n, err := s.Namespace(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{model.NewID(), model.NewID()}
	for _, id := range ids {
		v := NewVM{ID: id, NamespaceID: n.ID, InstanceName: "el-" + id, Alias: "easy-lima-" + id, SourcePath: filepath.Join(n.Root, "project-"+id)}
		if _, err = s.Prepare(ctx, model.NewPlan("up", model.VMScope(id, v.SourcePath)), nil, nil, &v); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Change(ctx, Change{Kind: "binding", Namespace: true, VMIDs: ids}, func(*sql.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	s.Fault = func(at string) error {
		if at == "local_published" {
			return errors.New("power loss")
		}
		return nil
	}
	err = s.Change(ctx, Change{Kind: "binding", Namespace: true, VMIDs: ids}, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, "UPDATE registrations SET owner_generation=owner_generation+1")
		return e
	})
	if err == nil {
		t.Fatal("fault not reached")
	}
	s.Fault = nil
	backup := filepath.Join(t.TempDir(), "backup")
	if _, err = s.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	r, _ := s.Registry(ctx)
	event, err := s.readEvent(ctx, *r.LastNamespaceEventID)
	if err != nil {
		t.Fatal(err)
	}
	// Both all-old and a partially published prefix must remain recoverable.
	if _, err = ValidateRestore(ctx, s.Dir, backup); err != nil {
		t.Fatal(err)
	}
	first := event.Payload.MarkerChanges[0]
	path := filepath.Join(n.Root, first.RelativePath)
	if err = files.Replace(path, []byte(first.NewPayload), first.OldDigest, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateRestore(ctx, s.Dir, backup); err != nil {
		t.Fatal(err)
	}
	if err = files.Replace(path, []byte("foreign owner"), first.NewDigest, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateRestore(ctx, s.Dir, backup); err == nil || model.AsError(err).Code != "ownership.conflict" {
		t.Fatalf("accepted foreign marker: %v", err)
	}
}

func TestRestoreRejectsOlderBackup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "backup")
	if _, err := s.Backup(ctx, path); err != nil {
		t.Fatal(err)
	}
	if err := s.Change(ctx, Change{Kind: "intent"}, func(tx *sql.Tx) error { _, e := tx.Exec("UPDATE schema_migrations SET backup_ref='newer'"); return e }); err != nil {
		t.Fatal(err)
	}
	before := fileState(t, s.Dir)
	if _, err := ValidateRestore(ctx, s.Dir, path); err == nil || model.AsError(err).Code != "state.backup_stale" {
		t.Fatalf("accepted stale state: %v", err)
	}
	after := fileState(t, s.Dir)
	for p, v := range before {
		if after[p] != v {
			t.Fatalf("preview changed %s", p)
		}
	}
}

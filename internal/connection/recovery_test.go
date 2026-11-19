package connection

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"golang.org/x/crypto/ssh"
)

func connectionFixture(t *testing.T) (*store.Store, store.OperationRecord, store.VMRecord, string) {
	t.Helper()
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
	t.Cleanup(func() { db.Close() })
	n, err := db.Namespace(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id := model.NewID()
	op, err := db.Prepare(ctx, model.NewPlan("up", model.VMScope(id, "/fixture")), nil, nil, &store.NewVM{ID: id, NamespaceID: n.ID, InstanceName: "fixture", Alias: "easy-lima-fixture", SourcePath: "/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := db.VM(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return db, op, v, root
}
func TestEnrollmentResumesPublicationWithoutReenrollment(t *testing.T) {
	db, op, v, _ := connectionFixture(t)
	ctx := context.Background()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Change(ctx, store.Change{Kind: "trust", OperationID: op.Operation.ID, Namespace: true, VMIDs: []string{v.VM.ID}}, func(tx *sql.Tx) error {
		if _, e := tx.Exec("INSERT INTO vm_trust VALUES(?,1,?,?,?,?,?,'local_first_use',?,?,?)", v.VM.ID, v.VM.NamespaceID, v.VM.InstanceName, key.Type(), PublicText(key), Fingerprint(key), op.Operation.ID, "fixture", model.Now()); e != nil {
			return e
		}
		_, e := tx.Exec("UPDATE registrations SET active_trust_generation=1 WHERE id=?", v.VM.ID)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	v, err = db.VM(ctx, v.VM.ID)
	if err != nil {
		t.Fatal(err)
	}
	// No server exists: the already committed pin must drive publication recovery.
	if _, err = Enroll(ctx, db, op, v, Endpoint{}); err != nil {
		t.Fatal(err)
	}
	if err = VerifyTrustExport(ctx, db, v.VM.ID); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(TrustPath(db.Dir, v.VM.ID)); err != nil {
		t.Fatal(err)
	}
	other, err := db.Prepare(ctx, model.NewPlan("start", model.Global()), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Enroll(ctx, db, other, v, Endpoint{}); err == nil {
		t.Fatal("ordinary unrelated operation silently repaired removed trust export")
	}
}
func TestRemovedSSHIncludeNeedsExplicitRepair(t *testing.T) {
	db, op, v, home := connectionFixture(t)
	ctx := context.Background()
	endpoint := Endpoint{Address: "127.0.0.1", Port: 2222, User: "fixture", IdentityFile: "/fixture/key", VMID: v.VM.ID}
	if err := PublishAlias(ctx, db, op.Operation.ID, home, "/usr/local/bin/easy-lima", v, endpoint); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".ssh", "config")
	unrelated := []byte("Host other\n  HostName example.invalid\n")
	if err := os.WriteFile(path, unrelated, 0600); err != nil {
		t.Fatal(err)
	}
	if err := PublishAlias(ctx, db, op.Operation.ID, home, "/usr/local/bin/easy-lima", v, endpoint); err == nil {
		t.Fatal("ordinary open recreated user-removed include")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(unrelated) {
		t.Fatal("user config changed on conflict")
	}
	repair, err := db.Prepare(ctx, model.NewPlan("repair", model.VMScope(v.VM.ID, "/fixture")), map[string]string{"kind": "alias"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = PublishAlias(ctx, db, repair.Operation.ID, home, "/usr/local/bin/easy-lima", v, endpoint); err != nil {
		t.Fatal("explicit alias repair failed", err)
	}
	if err = VerifyAlias(ctx, db, home, v); err != nil {
		t.Fatal(err)
	}
}

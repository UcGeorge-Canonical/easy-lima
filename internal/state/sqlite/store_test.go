package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	vmroot := filepath.Join(root, "vms")
	if err := os.Mkdir(vmroot, 0700); err != nil {
		t.Fatal(err)
	}
	identity, err := files.RootIdentity(vmroot)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Init(context.Background(), filepath.Join(root, "state"), vmroot, identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func fileState(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			out[p] = files.Hash(b) + info.ModTime().String()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestWALReadOnlyAndClose(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.Change(ctx, Change{Kind: "intent"}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE schema_migrations SET backup_ref='probe' WHERE version=1")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	dir := s.Dir
	s.Close()
	before := fileState(t, dir)
	if _, ok := before[filepath.Join(dir, "state.sqlite3-wal")]; !ok {
		t.Fatal("last connection close removed WAL")
	}
	r, err := Open(ctx, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	var ref string
	if err = r.DB.QueryRow("SELECT backup_ref FROM schema_migrations WHERE version=1").Scan(&ref); err != nil || ref != "probe" {
		t.Fatalf("snapshot missed committed WAL: %q %v", ref, err)
	}
	if _, err = r.DB.Exec("DELETE FROM schema_migrations"); err == nil {
		t.Fatal("read-only snapshot allowed writes")
	}
	if err = r.VerifyProjection(ctx); err != nil {
		t.Fatal(err)
	}
	r.Close()
	after := fileState(t, dir)
	if len(before) != len(after) {
		t.Fatalf("read changed file count: %d -> %d", len(before), len(after))
	}
	for p, v := range before {
		if after[p] != v {
			t.Fatalf("read changed %s", p)
		}
	}
}
func TestPublicationCrashBoundaries(t *testing.T) {
	for _, boundary := range []string{"intent_durable", "sql_committed", "local_published", "namespace_published"} {
		t.Run(boundary, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			s.Fault = func(at string) error {
				if at == boundary {
					return errors.New("injected crash")
				}
				return nil
			}
			err := s.Change(ctx, Change{Kind: "intent", Namespace: true}, func(tx *sql.Tx) error {
				_, e := tx.ExecContext(ctx, "UPDATE schema_migrations SET backup_ref='changed' WHERE version=1")
				return e
			})
			if err == nil {
				t.Fatal("fault did not fire")
			}
			s.Fault = nil
			if err = s.ReconcilePublication(ctx); err != nil {
				t.Fatal(err)
			}
			if err = s.VerifyHead(ctx); err != nil {
				t.Fatal(err)
			}
			if err = s.VerifyProjection(ctx); err != nil {
				t.Fatal(err)
			}
			var ref *string
			if err = s.DB.QueryRow("SELECT backup_ref FROM schema_migrations").Scan(&ref); err != nil {
				t.Fatal(err)
			}
			if boundary == "intent_durable" && ref != nil {
				t.Fatal("uncommitted intent changed authority")
			}
			if boundary != "intent_durable" && (ref == nil || *ref != "changed") {
				t.Fatal("lost committed effect")
			}
		})
	}
}
func TestForeignWitnessRefused(t *testing.T) {
	s := newTestStore(t)
	p := filepath.Join(s.Dir, "authority/head.json")
	d, _ := files.Digest(p)
	b, _ := files.Wrap(Head{SchemaVersion: 1, RegistryID: "foreign"})
	if err := files.Replace(p, b, d, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Change(context.Background(), Change{Kind: "intent"}, func(*sql.Tx) error { return nil }); err == nil {
		t.Fatal("foreign head accepted")
	}
}
func TestMissingReadDoesNotInitialize(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	if _, err := Open(context.Background(), dir, false); err == nil {
		t.Fatal("opened missing state")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("read initialized state")
	}
}

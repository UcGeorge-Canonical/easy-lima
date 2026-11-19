package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

//go:embed migrations/0001_initial.sql
var initialSchema string

type Store struct {
	DB       *sql.DB
	Dir      string
	ReadOnly bool
	guard    *files.Guard
	cleanup  func()
	Fault    func(string) error
	Progress func(string)
}
type Registry struct {
	ID                                string
	Schema                            int
	Generation                        int64
	Digest                            string
	LastEventID, LastNamespaceEventID *string
}
type Namespace struct {
	ID, Root, VolumeIdentity string
	Epoch                    int64
}
type Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func Open(ctx context.Context, dir string, write bool) (*Store, error) {
	g, err := files.Lock(ctx, filepath.Join(dir, "registry.lock"), false, false)
	if err != nil {
		return nil, model.Fail("state.unavailable", "The registry is unavailable. Run init for a new installation, or reconnect its metadata location.")
	}
	if _, err = os.Stat(filepath.Join(dir, "restore.intent.json")); err == nil {
		g.Close()
		return nil, model.Fail("state.unavailable", "An interrupted restore must be resumed before opening the registry.")
	}
	s := &Store{Dir: dir, ReadOnly: !write, guard: g}
	if write {
		s.DB, err = openDB(ctx, filepath.Join(dir, "state.sqlite3"), "rw")
	} else {
		s.DB, s.cleanup, err = snapshot(ctx, dir)
	}
	if err != nil {
		g.Close()
		return nil, model.Fail("state.unavailable", "The authoritative database could not be opened safely.")
	}
	r, err := s.Registry(ctx)
	if err != nil || r.Schema != 1 {
		s.Close()
		return nil, model.Fail("config.unsupported_schema", "The registry schema is unreadable or unsupported; no automatic migration was performed.")
	}
	return s, nil
}
func Init(ctx context.Context, dir, root, volume string) (*Store, error) {
	if err := files.PrivateDir(dir); err != nil {
		return nil, err
	}
	g, err := files.Lock(ctx, filepath.Join(dir, "registry.lock"), true, true)
	if err != nil {
		return nil, err
	}
	defer g.Close()
	path := filepath.Join(dir, "state.sqlite3")
	if _, err = os.Stat(path); err == nil {
		return nil, model.Fail("ownership.conflict", "A registry already exists; initialization never replaces it.")
	}
	for _, p := range []string{"authority/events", "recovery-inventory", "operations", "logs", "backups", "artifacts", "handoffs"} {
		if err = files.PrivateDir(filepath.Join(dir, p)); err != nil {
			return nil, err
		}
	}
	for _, p := range []string{"publication.lock", "namespace.lock"} {
		f, e := os.OpenFile(filepath.Join(dir, p), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil && !errors.Is(e, os.ErrExist) {
			return nil, e
		}
		if f != nil {
			f.Close()
		}
	}
	if err = files.PrivateDir(filepath.Join(root, "_easy-lima")); err != nil {
		return nil, err
	}
	if _, e := os.Stat(filepath.Join(root, "_easy-lima", "namespace.json")); e == nil {
		return nil, model.Fail("ownership.conflict", "The storage root is already bound to a registry.")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	d, err := openDB(ctx, path, "rw")
	if err != nil {
		return nil, err
	}
	s := &Store{DB: d, Dir: dir}
	if _, err = d.ExecContext(ctx, "PRAGMA journal_mode=WAL;"); err != nil {
		s.Close()
		return nil, err
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		s.Close()
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, initialSchema); err != nil {
		s.Close()
		return nil, err
	}
	id, ns := model.NewID(), model.NewID()
	now := model.Now()
	if _, err = tx.ExecContext(ctx, "INSERT INTO registry(singleton,registry_id,schema_version,recovery_digest,created_at) VALUES(1,?,1,?,?)", id, files.Hash(nil), now); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO storage_namespace(singleton,namespace_id,root,volume_identity) VALUES(1,?,?,?)", ns, root, volume); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations VALUES(1,?,?,NULL)", files.Hash([]byte(initialSchema)), now); err != nil {
		return nil, err
	}
	digest, err := projection(ctx, tx)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE registry SET recovery_digest=?", digest); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if err = s.Change(ctx, Change{Kind: "initialize", OperationID: model.NewID(), Namespace: true}, func(*sql.Tx) error { return nil }); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	if s.cleanup != nil {
		s.cleanup()
		s.cleanup = nil
	} else if s.DB != nil {
		s.DB.Close()
	}
	return s.guard.Close()
}
func (s *Store) Registry(ctx context.Context) (Registry, error) { return readRegistry(ctx, s.DB) }
func readRegistry(ctx context.Context, q Queryer) (Registry, error) {
	var r Registry
	err := q.QueryRowContext(ctx, "SELECT registry_id,schema_version,recovery_generation,recovery_digest,last_event_id,last_namespace_event_id FROM registry WHERE singleton=1").Scan(&r.ID, &r.Schema, &r.Generation, &r.Digest, &r.LastEventID, &r.LastNamespaceEventID)
	return r, err
}
func (s *Store) Namespace(ctx context.Context) (Namespace, error) { return readNamespace(ctx, s.DB) }
func readNamespace(ctx context.Context, q Queryer) (Namespace, error) {
	var n Namespace
	err := q.QueryRowContext(ctx, "SELECT namespace_id,root,volume_identity,epoch FROM storage_namespace WHERE singleton=1").Scan(&n.ID, &n.Root, &n.VolumeIdentity, &n.Epoch)
	return n, err
}
func (s *Store) fault(boundary string) error {
	if s.Fault != nil {
		return s.Fault(boundary)
	}
	return nil
}
func JSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
func Decode[T any](raw string) (T, error) {
	var v T
	err := json.Unmarshal([]byte(raw), &v)
	return v, err
}

// The digest excludes only acknowledgments/derived exports. Table order and
// primary-key order are fixed; timestamps in authority records remain covered.
var projectionTables = []string{"schema_migrations", "storage_namespace", "profiles", "profile_credential_revisions", "profile_revisions", "registrations", "environment_revisions", "operations", "operation_steps", "resource_claims", "binding_reservations", "owned_settings", "binding_changes", "profile_runtime", "vm_trust", "connection_bindings", "capacity_claims"}

func projection(ctx context.Context, q Queryer) (string, error) {
	all := map[string]any{}
	for _, table := range projectionTables {
		rows, err := q.QueryContext(ctx, "SELECT * FROM "+table+" ORDER BY 1,2")
		if err != nil {
			return "", err
		}
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			return "", err
		}
		data := []any{}
		for rows.Next() {
			values := make([]any, len(cols))
			dest := make([]any, len(cols))
			for i := range values {
				dest[i] = &values[i]
			}
			if err = rows.Scan(dest...); err != nil {
				rows.Close()
				return "", err
			}
			record := map[string]any{}
			for i, c := range cols {
				if c == "publication_pending" {
					continue
				}
				if b, ok := values[i].([]byte); ok {
					record[c] = string(b)
				} else {
					record[c] = values[i]
				}
			}
			data = append(data, record)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", err
		}
		all[table] = data
	}
	var id string
	var version int
	if err := q.QueryRowContext(ctx, "SELECT registry_id,schema_version FROM registry").Scan(&id, &version); err != nil {
		return "", err
	}
	all["registry_id"] = id
	all["schema_version"] = version
	b, err := files.Canonical(all)
	return files.Hash(b), err
}
func (s *Store) VerifyProjection(ctx context.Context) error {
	r, err := s.Registry(ctx)
	if err != nil {
		return err
	}
	d, err := projection(ctx, s.DB)
	if err != nil {
		return err
	}
	if d != r.Digest {
		return model.Fail("ownership.conflict", "The registry recovery projection differs from its recorded digest.")
	}
	return nil
}
func bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 5*time.Second)
}
func wrapDB(err error) error {
	if err == nil {
		return nil
	}
	var typed *model.Error
	if errors.As(err, &typed) {
		return err
	}
	return model.Fail("state.unavailable", fmt.Sprintf("The metadata transaction could not be completed (%T).", err))
}

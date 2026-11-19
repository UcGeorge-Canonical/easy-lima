package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	native "github.com/ncruces/go-sqlite3"
)

type BackupManifest struct {
	SchemaVersion      int               `json:"schema_version"`
	BackupID           string            `json:"backup_id"`
	RegistryID         string            `json:"registry_id"`
	DBSchemaVersion    int               `json:"db_schema_version"`
	RecoveryGeneration int64             `json:"recovery_generation"`
	RecoveryDigest     string            `json:"recovery_digest"`
	EventID            string            `json:"event_id"`
	EventHash          string            `json:"event_hash"`
	NamespaceID        string            `json:"namespace_id"`
	NamespaceEpoch     int64             `json:"namespace_epoch"`
	DBSHA256           string            `json:"db_sha256"`
	ArtifactHashes     map[string]string `json:"artifact_hashes"`
	CreatedAt          string            `json:"created_at"`
}

func databaseDigest(path string) (string, error) {
	b, e := files.Read(path, snapshotLimit)
	if e != nil {
		return "", e
	}
	return files.Hash(b), nil
}
func checkIntegrity(ctx context.Context, d *sql.DB) error {
	var result string
	if err := d.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return model.Fail("state.unavailable", "SQLite integrity verification failed.")
	}
	rows, err := d.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return model.Fail("state.unavailable", "SQLite foreign key verification failed.")
	}
	return rows.Err()
}
func standalone(ctx context.Context, path string) (*Store, error) {
	b, err := files.Read(path, snapshotLimit)
	if err != nil {
		return nil, err
	}
	b, err = mergeWAL(b, nil)
	if err != nil {
		return nil, err
	}
	d, cleanup, err := memorySnapshot(ctx, b)
	if err != nil {
		return nil, err
	}
	s := &Store{DB: d, ReadOnly: true, cleanup: cleanup}
	if err = checkIntegrity(ctx, d); err == nil {
		err = s.VerifyProjection(ctx)
	}
	if err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Backup(ctx context.Context, destination string) (BackupManifest, error) {
	var m BackupManifest
	if s.ReadOnly {
		return m, model.Fail("state.unavailable", "Backup requires an explicit maintenance connection.")
	}
	g, err := files.Lock(ctx, filepath.Join(s.Dir, "publication.lock"), true, false)
	if err != nil {
		return m, err
	}
	defer g.Close()
	if err = s.reconcileLocal(ctx); err != nil {
		return m, err
	}
	if err = s.VerifyProjection(ctx); err != nil {
		return m, err
	}
	if _, err = os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return m, model.Fail("settings.edited", "A backup destination must not already exist.")
	}
	if err = files.PrivateDir(filepath.Dir(destination)); err != nil {
		return m, err
	}
	if err = os.Mkdir(destination, 0700); err != nil {
		return m, err
	}
	r, err := s.Registry(ctx)
	if err != nil {
		return m, err
	}
	n, err := s.Namespace(ctx)
	if err != nil {
		return m, err
	}
	if r.LastEventID == nil {
		return m, model.Fail("state.unavailable", "A backup requires a complete local publication witness.")
	}
	event, err := s.readEvent(ctx, *r.LastEventID)
	if err != nil {
		return m, err
	}
	m = BackupManifest{SchemaVersion: 1, BackupID: model.NewID(), RegistryID: r.ID, DBSchemaVersion: r.Schema, RecoveryGeneration: r.Generation, RecoveryDigest: r.Digest, EventID: event.ID, EventHash: event.EventHash, NamespaceID: n.ID, NamespaceEpoch: n.Epoch, ArtifactHashes: map[string]string{}, CreatedAt: model.Now()}
	path := filepath.Join(destination, "state.sqlite3")
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	if err = raw(ctx, s.DB, func(c *native.Conn) error { return c.Backup("main", uri) }); err != nil {
		return m, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		return m, err
	}
	// The backup destination is a standalone database. Checkpointing this staging
	// copy is explicit maintenance and never checkpoints the live source.
	d, err := openDB(ctx, path, "rw")
	if err != nil {
		return m, err
	}
	err = raw(ctx, d, func(c *native.Conn) error { _, _, e := c.WALCheckpoint("main", native.CHECKPOINT_TRUNCATE); return e })
	d.Close()
	if err != nil {
		return m, err
	}
	verified, err := standalone(ctx, path)
	if err != nil {
		return m, err
	}
	vr, err := verified.Registry(ctx)
	verified.Close()
	if err != nil {
		return m, err
	}
	if vr.Digest != m.RecoveryDigest || vr.Generation != m.RecoveryGeneration {
		return m, model.Fail("state.unavailable", "Backup snapshot did not match its publication head.")
	}
	m.DBSHA256, err = databaseDigest(path)
	if err != nil {
		return m, err
	}
	var total int64
	for _, dir := range []string{"artifacts", "handoffs"} {
		err = filepath.WalkDir(filepath.Join(s.Dir, dir), func(path string, e fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if e.IsDir() {
				return nil
			}
			rel, er := filepath.Rel(s.Dir, path)
			if er != nil {
				return er
			}
			b, er := files.Read(path, 8<<20)
			if er != nil {
				return er
			}
			total += int64(len(b))
			if total > 256<<20 {
				return model.Fail("resource.unsupported", "Backup artifacts exceed the supported 256 MiB bound.")
			}
			target := filepath.Join(destination, rel)
			if er = files.PrivateDir(filepath.Dir(target)); er != nil {
				return er
			}
			if er = files.WriteNew(target, b); er != nil {
				return er
			}
			m.ArtifactHashes[rel] = files.Hash(b)
			return nil
		})
		if err != nil {
			return m, err
		}
	}
	b, err := files.Wrap(m)
	if err != nil {
		return m, err
	}
	if err = files.WriteNew(filepath.Join(destination, "manifest.json"), b); err != nil {
		return m, err
	}
	return m, files.SyncDir(destination)
}
func (s *Store) Checkpoint(ctx context.Context, mode string, timeout time.Duration) (int, int, error) {
	modes := map[string]native.CheckpointMode{"passive": native.CHECKPOINT_PASSIVE, "full": native.CHECKPOINT_FULL, "restart": native.CHECKPOINT_RESTART, "truncate": native.CHECKPOINT_TRUNCATE}
	m, ok := modes[mode]
	if !ok || timeout <= 0 || timeout > time.Minute {
		return 0, 0, model.Fail("input.invalid", "Checkpoint mode or timeout is invalid.")
	}
	if s.ReadOnly {
		return 0, 0, model.Fail("state.unavailable", "A read-only connection cannot checkpoint.")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	g, err := files.Lock(ctx, filepath.Join(s.Dir, "publication.lock"), true, false)
	if err != nil {
		return 0, 0, model.Fail("operation.busy", "The checkpoint could not acquire the publication guard.")
	}
	defer g.Close()
	var total, done int
	err = raw(ctx, s.DB, func(c *native.Conn) error {
		if e := c.BusyTimeout(timeout); e != nil {
			return e
		}
		defer c.BusyTimeout(2 * time.Second)
		var e error
		total, done, e = c.WALCheckpoint("main", m)
		return e
	})
	return total, done, err
}

func ReadBackup(ctx context.Context, path string) (BackupManifest, *Store, error) {
	var m BackupManifest
	b, err := files.Read(filepath.Join(path, "manifest.json"), 8<<20)
	if err != nil {
		return m, nil, err
	}
	if err = files.Unwrap(b, &m); err != nil {
		return m, nil, err
	}
	if m.SchemaVersion != 1 || m.DBSchemaVersion != 1 {
		return m, nil, model.Fail("config.unsupported_schema", "This backup schema is not supported.")
	}
	digest, err := databaseDigest(filepath.Join(path, "state.sqlite3"))
	if err != nil {
		return m, nil, err
	}
	if digest != m.DBSHA256 {
		return m, nil, model.Fail("record.invalid", "Backup database digest differs from its manifest.")
	}
	for rel, hash := range m.ArtifactHashes {
		if !safeArtifact(rel) {
			return m, nil, model.Fail("record.invalid", "The backup contains an invalid artifact path.")
		}
		d, e := files.Digest(filepath.Join(path, rel))
		if e != nil {
			return m, nil, e
		}
		if d != hash {
			return m, nil, model.Fail("record.invalid", "Backup artifact digest differs from its manifest.")
		}
	}
	d, err := standalone(ctx, filepath.Join(path, "state.sqlite3"))
	if err != nil {
		return m, nil, err
	}
	r, err := d.Registry(ctx)
	if err == nil && (r.ID != m.RegistryID || r.Digest != m.RecoveryDigest || r.Generation != m.RecoveryGeneration || r.LastEventID == nil || *r.LastEventID != m.EventID) {
		err = model.Fail("record.invalid", "The backup manifest does not describe its authoritative database.")
	}
	if err != nil {
		d.Close()
		return m, nil, err
	}
	return m, d, nil
}
func safeArtifact(rel string) bool {
	return !filepath.IsAbs(rel) && filepath.Clean(rel) == rel && !strings.HasPrefix(rel, "..") && (strings.HasPrefix(rel, "artifacts/") || strings.HasPrefix(rel, "handoffs/"))
}

// ValidateRestore uses independently retained local and namespace witnesses.
// A diagnostic inventory is deliberately absent from this decision.
func ValidateRestore(ctx context.Context, dir, backup string) (BackupManifest, error) {
	m, candidate, err := ReadBackup(ctx, backup)
	if err != nil {
		return m, err
	}
	defer candidate.Close()
	if _, err = os.Stat(filepath.Join(dir, "authority", "intent.json")); err == nil {
		return m, model.Fail("state.publication_pending", "Reconcile the committed local publication before considering a restore.")
	}
	b, err := files.Read(filepath.Join(dir, "authority", "head.json"), 65536)
	if err != nil {
		return m, err
	}
	var h Head
	if err = files.Unwrap(b, &h); err != nil {
		return m, err
	}
	if h.RegistryID != m.RegistryID || h.EventID != m.EventID || h.EventHash != m.EventHash || h.Sequence != m.RecoveryGeneration || h.RecoveryDigest != m.RecoveryDigest {
		return m, model.Fail("state.backup_stale", "The backup differs from the retained current ownership head. It cannot replace newer or unresolved effects.")
	}
	n, err := candidate.Namespace(ctx)
	if err != nil {
		return m, err
	}
	volume, err := files.RootIdentity(n.Root)
	if err != nil {
		return m, err
	}
	if volume != n.VolumeIdentity {
		return m, model.Fail("storage.unavailable", "Restore requires the original selected storage volume and directory.")
	}
	b, err = files.Read(filepath.Join(n.Root, "_easy-lima", "namespace.json"), 65536)
	if err != nil {
		return m, err
	}
	var marker NamespaceMarker
	if err = files.Unwrap(b, &marker); err != nil {
		return m, err
	}
	r, err := candidate.Registry(ctx)
	if err != nil {
		return m, err
	}
	if marker.RegistryID != r.ID || marker.NamespaceID != n.ID || marker.VolumeIdentity != n.VolumeIdentity {
		return m, model.Fail("ownership.conflict", "Restore namespace ownership differs from the backup.")
	}
	witness, err := candidate.readEvent(ctx, marker.LastEventID)
	if err != nil || witness.EventHash != marker.LastEventHash || witness.Payload.NamespaceEpoch != marker.Epoch {
		return m, model.Fail("ownership.conflict", "The namespace witness does not match its retained event.")
	}
	prefix := map[string]MarkerChange{}
	if r.LastNamespaceEventID != nil && marker.LastEventID != *r.LastNamespaceEventID {
		pending, e := candidate.readEvent(ctx, *r.LastNamespaceEventID)
		var status string
		if e == nil {
			e = candidate.DB.QueryRowContext(ctx, "SELECT publication_state FROM namespace_events WHERE id=?", pending.ID).Scan(&status)
		}
		if e != nil || status != "pending_namespace" || pending.PreviousNamespaceEventID == nil || *pending.PreviousNamespaceEventID != marker.LastEventID {
			return m, model.Fail("ownership.conflict", "The namespace witness is outside the retained publication prefix.")
		}
		// Namespace publication writes owners in order, then its head. Only the
		// exact old/new prefix of that committed event is recoverable.
		unpublished := false
		for _, change := range pending.Payload.MarkerChanges {
			if filepath.IsAbs(change.RelativePath) || strings.Contains(change.RelativePath, "..") || filepath.Base(change.RelativePath) != ".easy-lima-owner.json" {
				return m, model.Fail("record.invalid", "Invalid pending owner destination.")
			}
			got, e := files.Digest(filepath.Join(n.Root, change.RelativePath))
			if e != nil {
				return m, e
			}
			if got == change.NewDigest && !unpublished {
				prefix[change.RelativePath] = change
				continue
			}
			if got != change.OldDigest {
				return m, model.Fail("ownership.conflict", "Instance ownership is outside the retained publication prefix.")
			}
			unpublished = true
			prefix[change.RelativePath] = change
		}
	}
	rows, err := candidate.DB.QueryContext(ctx, "SELECT target,expected_digest FROM owned_settings WHERE field='owner'")
	if err != nil {
		return m, err
	}
	for rows.Next() {
		var rel, want string
		if err = rows.Scan(&rel, &want); err != nil {
			rows.Close()
			return m, err
		}
		if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
			rows.Close()
			return m, model.Fail("record.invalid", "Invalid ownership marker path.")
		}
		got, e := files.Digest(filepath.Join(n.Root, rel))
		if e != nil {
			rows.Close()
			return m, e
		}
		change, pending := prefix[rel]
		if got != want && !(pending && want == change.NewDigest && got == change.OldDigest) {
			rows.Close()
			return m, model.Fail("ownership.conflict", "An instance owner marker differs from the restore candidate.")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return m, err
	}
	if err = candidate.VerifyLegacyRetirement(ctx, ""); err != nil {
		return m, err
	}
	return m, nil
}

type RestoreFile struct {
	RelativePath string `json:"relative_path"`
	SHA256       string `json:"sha256"`
}
type RestoreIntent struct {
	SchemaVersion  int           `json:"schema_version"`
	RestoreID      string        `json:"restore_id"`
	RegistryID     string        `json:"registry_id"`
	ExpectedHead   Head          `json:"expected_head"`
	OldFiles       []RestoreFile `json:"old_file_set"`
	CandidateFiles []RestoreFile `json:"candidate_file_set"`
	BackupID       string        `json:"backup_id"`
	BackupPath     string        `json:"backup_path"`
	Phase          string        `json:"phase"`
	Finalization   *Event        `json:"finalization"`
}

func saveRestore(dir string, i RestoreIntent) error {
	p := filepath.Join(dir, "restore.intent.json")
	old, err := files.Digest(p)
	if err != nil {
		return err
	}
	b, err := files.Wrap(i)
	if err != nil {
		return err
	}
	return files.Replace(p, b, old, 0600)
}
func finalRestoreSQL(ctx context.Context, i RestoreIntent) func(*sql.Tx) error {
	return func(tx *sql.Tx) error {
		e := i.Finalization
		plan := model.NewPlan("state.restore", model.Global())
		plan.Add("metadata", "Restore the verified equal-generation authoritative snapshot and retain unresolved operations.")
		_, err := tx.ExecContext(ctx, "INSERT INTO operations(id,kind,scope_kind,token,status,inputs,plan,created_at,updated_at) VALUES(?,'state.restore','global',?,'completed',?,?,?,?)", e.OperationID, i.RestoreID, JSON(map[string]string{"backup_id": i.BackupID}), JSON(plan), e.CreatedAt, e.CreatedAt)
		return err
	}
}

// Restore holds the registry-open guard exclusively for the entire physical
// swap. Every file move is hash checked and resumable under the same fence.
func Restore(ctx context.Context, dir, backup string) (Registry, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	g, err := files.Lock(ctx, filepath.Join(dir, "registry.lock"), true, false)
	if err != nil {
		return Registry{}, model.Fail("operation.busy", "Close active easy-lima commands before restoring state.")
	}
	defer g.Close()
	var intent RestoreIntent
	fence := filepath.Join(dir, "restore.intent.json")
	b, err := files.Read(fence, 8<<20)
	if err == nil {
		if err = files.Unwrap(b, &intent); err != nil {
			return Registry{}, err
		}
		if intent.BackupPath != backup {
			return Registry{}, model.Fail("ownership.conflict", "An interrupted restore must resume with its original backup.")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Registry{}, err
	} else {
		m, e := ValidateRestore(ctx, dir, backup)
		if e != nil {
			return Registry{}, e
		}
		id := model.NewID()
		stage := filepath.Join(dir, "restore", id, "candidate")
		if e = files.PrivateDir(stage); e != nil {
			return Registry{}, e
		}
		candidateBytes, e := files.Read(filepath.Join(backup, "state.sqlite3"), snapshotLimit)
		if e != nil {
			return Registry{}, e
		}
		if e = files.WriteNew(filepath.Join(stage, "state.sqlite3"), candidateBytes); e != nil {
			return Registry{}, e
		}
		// Inspect the original file set through a detached, read-only snapshot.
		// Never open damaged state writable merely to decide whether to replace it.
		oldDB, cleanup, snapshotErr := snapshot(ctx, dir)
		if snapshotErr == nil {
			oldStore := &Store{DB: oldDB, Dir: dir, ReadOnly: true}
			check := oldStore.VerifyProjection(ctx)
			if check == nil {
				check = oldStore.VerifyHead(ctx)
			}
			oldDB.Close()
			cleanup()
			if check != nil {
				return Registry{}, check
			}
		}

		intent = RestoreIntent{SchemaVersion: 1, RestoreID: id, RegistryID: m.RegistryID, ExpectedHead: Head{1, m.RegistryID, m.RecoveryGeneration, m.EventID, m.EventHash, m.RecoveryDigest}, BackupID: m.BackupID, BackupPath: backup, Phase: "prepared", OldFiles: []RestoreFile{}, CandidateFiles: []RestoreFile{{"state.sqlite3", m.DBSHA256}}}
		for _, rel := range []string{"state.sqlite3", "state.sqlite3-wal", "state.sqlite3-shm", "state.sqlite3-journal"} {
			h, e := databaseDigest(filepath.Join(dir, rel))
			if errors.Is(e, os.ErrNotExist) {
				continue
			}
			if e != nil {
				return Registry{}, e
			}
			intent.OldFiles = append(intent.OldFiles, RestoreFile{rel, h})
		}
		if e = saveRestore(dir, intent); e != nil {
			return Registry{}, e
		}
	}
	if intent.SchemaVersion != 1 || !model.ValidID(intent.RestoreID) {
		return Registry{}, model.Fail("record.invalid", "Invalid restore fence.")
	}
	base := filepath.Join(dir, "restore", intent.RestoreID)
	oldDir := filepath.Join(base, "old")
	stage := filepath.Join(base, "candidate")
	if err = files.PrivateDir(oldDir); err != nil {
		return Registry{}, err
	}
	if intent.Phase == "prepared" {
		for _, f := range intent.OldFiles {
			if !physicalName(f.RelativePath) {
				return Registry{}, model.Fail("record.invalid", "Invalid restore file name.")
			}
			src, dst := filepath.Join(dir, f.RelativePath), filepath.Join(oldDir, f.RelativePath)
			if h, e := databaseDigest(dst); e == nil {
				if h != f.SHA256 {
					return Registry{}, model.Fail("ownership.conflict", "Quarantined state changed.")
				}
				continue
			}
			h, e := databaseDigest(src)
			if e != nil || h != f.SHA256 {
				return Registry{}, model.Fail("ownership.conflict", "The old database file set changed during restore.")
			}
			if e = os.Rename(src, dst); e != nil {
				return Registry{}, e
			}
		}
		if err = files.SyncDir(dir); err != nil {
			return Registry{}, err
		}
		if err = files.SyncDir(oldDir); err != nil {
			return Registry{}, err
		}
		intent.Phase = "old_quarantined"
		if err = saveRestore(dir, intent); err != nil {
			return Registry{}, err
		}
	}
	if intent.Phase == "old_quarantined" {
		for _, f := range intent.CandidateFiles {
			if !physicalName(f.RelativePath) {
				return Registry{}, model.Fail("record.invalid", "Invalid candidate name.")
			}
			dst := filepath.Join(dir, f.RelativePath)
			if h, e := databaseDigest(dst); e == nil {
				if h != f.SHA256 {
					return Registry{}, model.Fail("ownership.conflict", "The installed candidate differs from its approved file set.")
				}
				continue
			}
			src := filepath.Join(stage, f.RelativePath)
			h, e := databaseDigest(src)
			if e != nil || h != f.SHA256 {
				return Registry{}, model.Fail("ownership.conflict", "The staged candidate changed.")
			}
			if e = os.Rename(src, dst); e != nil {
				return Registry{}, e
			}
		}
		if err = files.SyncDir(dir); err != nil {
			return Registry{}, err
		}
		intent.Phase = "candidate_installed"
		if err = saveRestore(dir, intent); err != nil {
			return Registry{}, err
		}
	}
	if intent.Phase == "candidate_installed" {
		for _, f := range intent.CandidateFiles {
			h, e := databaseDigest(filepath.Join(dir, f.RelativePath))
			if e != nil || h != f.SHA256 {
				return Registry{}, model.Fail("ownership.conflict", "Candidate changed before finalization.")
			}
		}
		// Preview the exact SQL event on a private staging copy. No live writable
		// handle is opened until this complete finalization record is durable.
		preview := filepath.Join(base, "preview")
		if err = os.RemoveAll(preview); err != nil {
			return Registry{}, err
		}
		if err = files.PrivateDir(filepath.Join(preview, "authority", "events")); err != nil {
			return Registry{}, err
		}
		for _, name := range []string{"publication.lock", "namespace.lock"} {
			if err = files.WriteNew(filepath.Join(preview, name), nil); err != nil {
				return Registry{}, err
			}
		}
		body, e := files.Read(filepath.Join(dir, "state.sqlite3"), snapshotLimit)
		if e != nil {
			return Registry{}, e
		}
		if err = files.WriteNew(filepath.Join(preview, "state.sqlite3"), body); err != nil {
			return Registry{}, err
		}
		head, e := files.Read(filepath.Join(dir, "authority", "head.json"), 65536)
		if e != nil {
			return Registry{}, e
		}
		if err = files.WriteNew(filepath.Join(preview, "authority", "head.json"), head); err != nil {
			return Registry{}, err
		}
		d, e := openDB(ctx, filepath.Join(preview, "state.sqlite3"), "rw")
		if e != nil {
			return Registry{}, e
		}
		ps := &Store{Dir: preview, DB: d}
		sentinel := errors.New("preview complete")
		intent.Finalization = &Event{OperationID: model.NewID(), ID: model.NewID(), CreatedAt: model.Now()}
		change := Change{Kind: "restore", OperationID: intent.Finalization.OperationID, EventID: intent.Finalization.ID, CreatedAt: intent.Finalization.CreatedAt, Preview: func(e Event) error { intent.Finalization = &e; return sentinel }}
		e = ps.Change(ctx, change, finalRestoreSQL(ctx, intent))
		d.Close()
		if !errors.Is(e, sentinel) {
			return Registry{}, e
		}
		intent.Phase = "finalizing"
		if err = saveRestore(dir, intent); err != nil {
			return Registry{}, err
		}
	}
	if intent.Phase == "finalizing" {
		if intent.Finalization == nil {
			return Registry{}, model.Fail("record.invalid", "Restore finalization evidence is missing.")
		}
		d, e := openDB(ctx, filepath.Join(dir, "state.sqlite3"), "rw")
		if e != nil {
			return Registry{}, e
		}
		current := &Store{Dir: dir, DB: d}
		defer current.Close()
		if e = checkIntegrity(ctx, d); e != nil {
			return Registry{}, e
		}
		reg, e := current.Registry(ctx)
		if e != nil {
			return Registry{}, e
		}
		event := intent.Finalization
		if reg.LastEventID != nil && *reg.LastEventID == event.ID {
			committed, e := current.readEvent(ctx, event.ID)
			if e != nil {
				return Registry{}, e
			}
			if committed.EventHash != event.EventHash {
				return Registry{}, model.Fail("ownership.conflict", "Restore event content changed.")
			}
			e = current.ReconcilePublication(ctx)
			if e != nil {
				return Registry{}, e
			}
		} else {
			if reg.Generation != intent.ExpectedHead.Sequence || reg.Digest != intent.ExpectedHead.RecoveryDigest {
				return Registry{}, model.Fail("ownership.conflict", "Installed candidate is outside the frozen restore lineage.")
			}
			e = current.Change(ctx, Change{Kind: "restore", OperationID: event.OperationID, EventID: event.ID, CreatedAt: event.CreatedAt, Preview: func(actual Event) error {
				if actual.EventHash != event.EventHash {
					return model.Fail("ownership.conflict", "Restore finalization differs from its frozen event.")
				}
				return nil
			}}, finalRestoreSQL(ctx, intent))
			if e != nil {
				return Registry{}, e
			}
		}
		m, _, e := readBackupManifest(backup)
		if e != nil {
			return Registry{}, e
		}
		for rel, hash := range m.ArtifactHashes {
			src := filepath.Join(backup, rel)
			dst := filepath.Join(dir, rel)
			h, e := files.Digest(dst)
			if e != nil {
				return Registry{}, e
			}
			if h == hash {
				continue
			}
			if h != "" {
				return Registry{}, model.Fail("settings.edited", "An existing restore artifact conflicts with the backup.")
			}
			body, e := files.Read(src, 8<<20)
			if e != nil {
				return Registry{}, e
			}
			if files.Hash(body) != hash {
				return Registry{}, model.Fail("record.invalid", "Restore artifact changed.")
			}
			if e = files.PrivateDir(filepath.Dir(dst)); e != nil {
				return Registry{}, e
			}
			if e = files.WriteNew(dst, body); e != nil {
				return Registry{}, e
			}
		}
		if e = current.VerifyHead(ctx); e != nil {
			return Registry{}, e
		}
		if e = current.VerifyProjection(ctx); e != nil {
			return Registry{}, e
		}
		intent.Phase = "verified"
		if e = saveRestore(dir, intent); e != nil {
			return Registry{}, e
		}
		current.Close()
	}
	if intent.Phase != "verified" {
		return Registry{}, model.Fail("record.invalid", "Unknown restore phase.")
	}
	h, err := files.Digest(fence)
	if err != nil {
		return Registry{}, err
	}
	if err = files.RemoveExpected(fence, h); err != nil {
		return Registry{}, err
	}
	return Registry{ID: intent.RegistryID, Schema: 1, Generation: intent.Finalization.Sequence, Digest: intent.Finalization.AfterDigest, LastEventID: &intent.Finalization.ID}, nil
}
func physicalName(s string) bool {
	switch s {
	case "state.sqlite3", "state.sqlite3-wal", "state.sqlite3-shm", "state.sqlite3-journal":
		return true
	}
	return false
}
func readBackupManifest(path string) (BackupManifest, []byte, error) {
	var m BackupManifest
	b, e := files.Read(filepath.Join(path, "manifest.json"), 8<<20)
	if e == nil {
		e = files.Unwrap(b, &m)
	}
	return m, b, e
}

func (s *Store) PendingClaims(ctx context.Context, id string) ([]map[string]any, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT kind,resource_key,mode FROM resource_claims WHERE operation_id=? ORDER BY kind,resource_key", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var kind, key, mode string
		if err = rows.Scan(&kind, &key, &mode); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"kind": kind, "scope": model.Scope{Kind: kind, ID: &key}, "mode": mode})
	}
	return out, rows.Err()
}

var _ = fmt.Sprint

// VerifyLegacyRetirement keeps exclusive ownership checks shared by normal
// VM access and restore, including a damaged registry's backup candidate.
func (s *Store) VerifyLegacyRetirement(ctx context.Context, vmID string) error {
	query := "SELECT target,expected_digest,backup_ref FROM owned_settings WHERE field='legacy-retirement'"
	args := []any{}
	if vmID != "" {
		query += " AND owner_id=?"
		args = append(args, vmID)
	}
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var active, expected, archive string
		if err = rows.Scan(&active, &expected, &archive); err != nil {
			return err
		}
		if _, err = os.Lstat(active); !os.IsNotExist(err) {
			return model.Fail("ownership.conflict", "The legacy ownership record is active again or cannot be inspected.")
		}
		hash, e := files.Digest(archive)
		if e != nil || hash != expected {
			return model.Fail("ownership.conflict", "The legacy ownership retirement cannot be verified.")
		}
	}
	return rows.Err()
}

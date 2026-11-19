package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Change struct {
	Kind, OperationID              string
	Stage, ResultCode, EffectState string
	Namespace                      bool
	VMIDs                          []string
	EventID                        string
	CreatedAt                      string
	// Preview receives the fully resolved event before any publication. Returning
	// an error rolls back the tentative SQL projection. Maintenance uses this to
	// freeze an exact restore event before opening the installed candidate.
	Preview func(Event) error
}
type MarkerChange struct {
	RelativePath string `json:"relative_path"`
	OldDigest    string `json:"old_digest"`
	NewDigest    string `json:"new_digest"`
	NewPayload   string `json:"new_payload"`
}
type EventPayload struct {
	Stage              string         `json:"stage,omitempty"`
	ResultCode         string         `json:"result_code,omitempty"`
	EffectState        string         `json:"effect_state,omitempty"`
	SchemaVersion      int            `json:"schema_version"`
	RegistryID         string         `json:"registry_id"`
	RecoveryGeneration int64          `json:"recovery_generation"`
	SchemaVersionAfter int            `json:"schema_version_after"`
	NamespaceID        *string        `json:"namespace_id"`
	NamespaceEpoch     int64          `json:"namespace_epoch"`
	MarkerChanges      []MarkerChange `json:"marker_changes"`
	HandoffIDs         []string       `json:"handoff_ids"`
	RequiredLocal      bool           `json:"required_local_publication"`
	RequiredNamespace  bool           `json:"required_namespace_publication"`
}
type Event struct {
	ID                       string       `json:"id"`
	Sequence                 int64        `json:"sequence"`
	OperationID              string       `json:"operation_id"`
	NamespaceID              *string      `json:"namespace_id"`
	Kind                     string       `json:"kind"`
	PreviousEventID          *string      `json:"previous_event_id"`
	PreviousEventHash        *string      `json:"previous_event_hash"`
	PreviousNamespaceEventID *string      `json:"previous_namespace_event_id"`
	BeforeDigest             string       `json:"before_digest"`
	AfterDigest              string       `json:"after_digest"`
	Payload                  EventPayload `json:"payload"`
	EventHash                string       `json:"event_hash"`
	CreatedAt                string       `json:"created_at"`
}
type Head struct {
	SchemaVersion  int    `json:"schema_version"`
	RegistryID     string `json:"registry_id"`
	Sequence       int64  `json:"sequence"`
	EventID        string `json:"event_id"`
	EventHash      string `json:"event_hash"`
	RecoveryDigest string `json:"recovery_digest"`
}
type Intent struct {
	SchemaVersion int    `json:"schema_version"`
	RegistryID    string `json:"registry_id"`
	EventID       string `json:"event_id"`
	Sequence      int64  `json:"sequence"`
	BeforeDigest  string `json:"before_digest"`
	AfterDigest   string `json:"after_digest"`
	EventHash     string `json:"event_hash"`
	OperationID   string `json:"operation_id"`
}
type NamespaceMarker struct {
	SchemaVersion  int    `json:"schema_version"`
	RegistryID     string `json:"registry_id"`
	NamespaceID    string `json:"namespace_id"`
	VolumeIdentity string `json:"volume_identity"`
	Epoch          int64  `json:"epoch"`
	LastEventID    string `json:"last_event_id"`
	LastEventHash  string `json:"last_event_hash"`
}
type Owner struct {
	SchemaVersion     int     `json:"schema_version"`
	RegistryID        string  `json:"registry_id"`
	NamespaceID       string  `json:"namespace_id"`
	RegistrationID    string  `json:"registration_id"`
	InstanceName      string  `json:"instance_name"`
	OwnerGeneration   int64   `json:"owner_generation"`
	BindingGeneration int64   `json:"binding_generation"`
	OwnerState        string  `json:"owner_state"`
	HandoffID         *string `json:"handoff_id"`
	EventID           string  `json:"event_id"`
}

func (s *Store) Change(ctx context.Context, c Change, fn func(*sql.Tx) error) error {
	if s.ReadOnly {
		return model.Fail("state.unavailable", "A diagnostic connection cannot mutate state.")
	}
	lctx, cancel := bounded(ctx)
	defer cancel()
	g, err := files.Lock(lctx, filepath.Join(s.Dir, "publication.lock"), true, false)
	if err != nil {
		return model.Fail("operation.busy", "Another metadata publication is active.")
	}
	defer g.Close()
	if err = s.reconcileLocal(ctx); err != nil {
		return err
	}
	if c.Namespace {
		var pending int
		if err = s.DB.QueryRowContext(ctx, "SELECT count(*) FROM namespace_events WHERE namespace_id IS NOT NULL AND publication_state<>'published'").Scan(&pending); err != nil {
			return err
		}
		if pending > 0 {
			return model.Fail("state.publication_pending", "Namespace publication must be resumed before another VM mutation.")
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return wrapDB(err)
	}
	defer tx.Rollback()
	r, err := readRegistry(ctx, tx)
	if err != nil {
		return err
	}
	before, err := projection(ctx, tx)
	if err != nil {
		return err
	}
	if before != r.Digest {
		return model.Fail("ownership.conflict", "The authoritative projection does not match its recorded digest.")
	}
	if c.EventID == "" {
		c.EventID = model.NewID()
	}
	if c.OperationID == "" {
		c.OperationID = model.NewID()
	}
	e := Event{ID: c.EventID, Sequence: r.Generation + 1, OperationID: c.OperationID, Kind: c.Kind, PreviousEventID: r.LastEventID, PreviousNamespaceEventID: r.LastNamespaceEventID, BeforeDigest: before, CreatedAt: model.Now(), Payload: EventPayload{Stage: c.Stage, ResultCode: c.ResultCode, EffectState: c.EffectState, SchemaVersion: 1, RegistryID: r.ID, RecoveryGeneration: r.Generation + 1, SchemaVersionAfter: r.Schema, MarkerChanges: []MarkerChange{}, HandoffIDs: []string{}, RequiredLocal: true, RequiredNamespace: c.Namespace}}
	if c.CreatedAt != "" {
		e.CreatedAt = c.CreatedAt
	}
	if r.LastEventID != nil {
		var h string
		if err = tx.QueryRowContext(ctx, "SELECT event_hash FROM namespace_events WHERE id=?", *r.LastEventID).Scan(&h); err != nil {
			return err
		}
		e.PreviousEventHash = &h
	}
	if err = fn(tx); err != nil {
		return wrapDB(err)
	}
	if c.Namespace {
		n, er := readNamespace(ctx, tx)
		if er != nil {
			return er
		}
		e.NamespaceID = &n.ID
		e.Payload.NamespaceID = &n.ID
		e.Payload.NamespaceEpoch = n.Epoch + 1
		if _, err = tx.ExecContext(ctx, "UPDATE storage_namespace SET epoch=epoch+1,row_revision=row_revision+1"); err != nil {
			return err
		}
		for _, id := range c.VMIDs {
			if _, err = tx.ExecContext(ctx, "UPDATE registrations SET owner_event_id=?,publication_pending=1 WHERE id=?", e.ID, id); err != nil {
				return err
			}
			var o Owner
			o.SchemaVersion = 1
			o.RegistryID = r.ID
			o.EventID = e.ID
			if err = tx.QueryRowContext(ctx, "SELECT namespace_id,id,instance_name,owner_generation,binding_generation,ownership FROM registrations WHERE id=?", id).Scan(&o.NamespaceID, &o.RegistrationID, &o.InstanceName, &o.OwnerGeneration, &o.BindingGeneration, &o.OwnerState); err != nil {
				return err
			}
			var handoff string
			er = tx.QueryRowContext(ctx, "SELECT expected_digest FROM owned_settings WHERE owner_id=? AND field='handoff'", id).Scan(&handoff)
			if er == nil {
				o.HandoffID = &handoff
				e.Payload.HandoffIDs = append(e.Payload.HandoffIDs, handoff)
			} else if !errors.Is(er, sql.ErrNoRows) {
				return er
			}
			if o.OwnerState == "adopting" {
				o.OwnerState = "reserved"
			}
			if o.OwnerState == "unknown" || o.OwnerState == "conflict" {
				return model.Fail("ownership.conflict", "Cannot publish an uncertain owner.")
			}
			rel := filepath.Join(o.InstanceName, ".easy-lima-owner.json")
			var old string
			er = tx.QueryRowContext(ctx, "SELECT expected_digest FROM owned_settings WHERE owner_id=? AND target=? AND field='owner'", id, rel).Scan(&old)
			if er != nil && !errors.Is(er, sql.ErrNoRows) {
				return er
			}
			b, er := files.Wrap(o)
			if er != nil {
				return er
			}
			if o.OwnerState == "retired" {
				b = nil
			}
			m := MarkerChange{RelativePath: rel, OldDigest: old, NewPayload: string(b)}
			if b != nil {
				m.NewDigest = files.Hash(b)
			}
			e.Payload.MarkerChanges = append(e.Payload.MarkerChanges, m)
			if _, err = tx.ExecContext(ctx, "INSERT INTO owned_settings(owner_id,owner_kind,target,field,expected_digest) VALUES(?,'vm',?,'owner',?) ON CONFLICT(owner_id,target,field) DO UPDATE SET expected_digest=excluded.expected_digest", id, rel, m.NewDigest); err != nil {
				return err
			}
		}
	}
	e.AfterDigest, err = projection(ctx, tx)
	if err != nil {
		return err
	}
	hbytes, err := files.Canonical(e)
	if err != nil {
		return err
	}
	e.EventHash = files.Hash(hbytes)
	if c.Preview != nil {
		if err = c.Preview(e); err != nil {
			return err
		}
	}
	intent := Intent{1, r.ID, e.ID, e.Sequence, e.BeforeDigest, e.AfterDigest, e.EventHash, e.OperationID}
	b, err := files.Wrap(intent)
	if err != nil {
		return err
	}
	intentPath := filepath.Join(s.Dir, "authority", "intent.json")
	if err = files.WriteNew(intentPath, b); err != nil {
		return err
	}
	if err = s.fault("intent_durable"); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO namespace_events(id,sequence,operation_id,namespace_id,kind,previous_event_id,previous_event_hash,previous_namespace_event_id,before_digest,after_digest,payload,event_hash,publication_state,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,'pending_local',?)`, e.ID, e.Sequence, e.OperationID, e.NamespaceID, e.Kind, e.PreviousEventID, e.PreviousEventHash, e.PreviousNamespaceEventID, e.BeforeDigest, e.AfterDigest, JSON(e.Payload), e.EventHash, e.CreatedAt)
	if err != nil {
		return err
	}
	nsHead := r.LastNamespaceEventID
	if c.Namespace {
		nsHead = &e.ID
	}
	_, err = tx.ExecContext(ctx, "UPDATE registry SET generation=generation+1,recovery_generation=?,recovery_digest=?,last_event_id=?,last_namespace_event_id=?", e.Sequence, e.AfterDigest, e.ID, nsHead)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return model.Fail("operation.unknown", "The metadata commit result is unknown; reconcile its existing event before retrying.")
	}
	if err = s.fault("sql_committed"); err != nil {
		return err
	}
	if err = s.publishLocal(ctx, e); err != nil {
		return err
	}
	g.Close()
	g = nil
	if c.Namespace {
		return s.PublishNamespace(ctx)
	}
	return nil
}
func (s *Store) readEvent(ctx context.Context, id string) (Event, error) {
	var e Event
	var payload string
	err := s.DB.QueryRowContext(ctx, "SELECT id,sequence,operation_id,namespace_id,kind,previous_event_id,previous_event_hash,previous_namespace_event_id,before_digest,after_digest,payload,event_hash,created_at FROM namespace_events WHERE id=?", id).Scan(&e.ID, &e.Sequence, &e.OperationID, &e.NamespaceID, &e.Kind, &e.PreviousEventID, &e.PreviousEventHash, &e.PreviousNamespaceEventID, &e.BeforeDigest, &e.AfterDigest, &payload, &e.EventHash, &e.CreatedAt)
	if err != nil {
		return e, err
	}
	e.Payload, err = Decode[EventPayload](payload)
	if err != nil {
		return e, err
	}
	want := e.EventHash
	e.EventHash = ""
	body, err := files.Canonical(e)
	e.EventHash = want
	if err != nil {
		return e, err
	}
	if files.Hash(body) != want {
		return e, model.Fail("ownership.conflict", "Publication event contents differ from their retained hash.")
	}
	return e, nil
}
func (s *Store) reconcileLocal(ctx context.Context) error {
	p := filepath.Join(s.Dir, "authority", "intent.json")
	b, err := files.Read(p, 65536)
	if errors.Is(err, os.ErrNotExist) {
		return s.VerifyHead(ctx)
	}
	if err != nil {
		return err
	}
	var in Intent
	if err = files.Unwrap(b, &in); err != nil {
		return err
	}
	r, err := s.Registry(ctx)
	if err != nil {
		return err
	}
	if r.ID != in.RegistryID {
		return model.Fail("ownership.conflict", "The pending intent belongs to another registry.")
	}
	e, err := s.readEvent(ctx, in.EventID)
	if errors.Is(err, sql.ErrNoRows) && r.Digest == in.BeforeDigest {
		if err = s.VerifyHead(ctx); err != nil {
			return err
		}
		return files.RemoveExpected(p, files.Hash(b))
	}
	if err != nil {
		return model.Fail("state.publication_pending", "The pending publication cannot be reconciled.")
	}
	if e.EventHash != in.EventHash || r.Digest != in.AfterDigest {
		return model.Fail("ownership.conflict", "The pending publication contradicts the database.")
	}
	return s.publishLocal(ctx, e)
}
func (s *Store) VerifyHead(ctx context.Context) error {
	r, err := s.Registry(ctx)
	if err != nil {
		return err
	}
	if r.LastEventID == nil {
		return nil
	}
	b, err := files.Read(filepath.Join(s.Dir, "authority", "head.json"), 65536)
	if err != nil {
		return model.Fail("state.publication_pending", "The local publication witness is unavailable.")
	}
	var h Head
	if err = files.Unwrap(b, &h); err != nil {
		return err
	}
	if h.RegistryID != r.ID || h.EventID != *r.LastEventID || h.RecoveryDigest != r.Digest || h.Sequence != r.Generation {
		return model.Fail("ownership.conflict", "The database and independent publication witness disagree.")
	}
	return nil
}
func (s *Store) publishLocal(ctx context.Context, e Event) error {
	p := filepath.Join(s.Dir, "authority", "events", fmt.Sprintf("%020d.json", e.Sequence))
	b, err := files.Wrap(e)
	if err != nil {
		return err
	}
	old, err := files.Digest(p)
	if err != nil {
		return err
	}
	if old != "" && old != files.Hash(b) {
		return model.Fail("ownership.conflict", "The immutable publication event was changed.")
	}
	if err = files.Replace(p, b, old, 0600); err != nil {
		return err
	}
	head := Head{1, e.Payload.RegistryID, e.Sequence, e.ID, e.EventHash, e.AfterDigest}
	p = filepath.Join(s.Dir, "authority", "head.json")
	old, err = files.Digest(p)
	if err != nil {
		return err
	}
	if old != "" {
		var existing Head
		hb, er := files.Read(p, 65536)
		if er != nil {
			return er
		}
		if er = files.Unwrap(hb, &existing); er != nil {
			return er
		}
		if existing.EventID != e.ID && (e.PreviousEventID == nil || existing.EventID != *e.PreviousEventID) {
			return model.Fail("ownership.conflict", "The local head is neither the expected old nor target event.")
		}
	}
	b, err = files.Wrap(head)
	if err != nil {
		return err
	}
	if err = files.Replace(p, b, old, 0600); err != nil {
		return err
	}
	if err = s.fault("local_published"); err != nil {
		return err
	}
	state := "published"
	if e.NamespaceID != nil {
		state = "pending_namespace"
	}
	if _, err = s.DB.ExecContext(ctx, "UPDATE namespace_events SET publication_state=?,local_published_at=? WHERE id=?", state, model.Now(), e.ID); err != nil {
		return err
	}
	if err = s.publishLog(ctx, e.OperationID); err != nil {
		return err
	}
	p = filepath.Join(s.Dir, "authority", "intent.json")
	old, err = files.Digest(p)
	if err != nil {
		return err
	}
	if old != "" {
		return files.RemoveExpected(p, old)
	}
	return nil
}
func (s *Store) ReconcilePublication(ctx context.Context) error {
	if s.ReadOnly {
		return model.Fail("state.unavailable", "Publication is an explicit writing action.")
	}
	g, err := files.Lock(ctx, filepath.Join(s.Dir, "publication.lock"), true, false)
	if err != nil {
		return err
	}
	err = s.reconcileLocal(ctx)
	g.Close()
	if err != nil {
		return err
	}
	return s.PublishNamespace(ctx)
}
func (s *Store) PublishNamespace(ctx context.Context) error {
	g, err := files.Lock(ctx, filepath.Join(s.Dir, "namespace.lock"), true, false)
	if err != nil {
		return err
	}
	defer g.Close()
	n, err := s.Namespace(ctx)
	if err != nil {
		return err
	}
	if _, err = os.Stat(n.Root); err != nil {
		return model.Fail("storage.unavailable", "The configured VM volume is unavailable; publication remains pending.")
	}
	volume, err := files.RootIdentity(n.Root)
	if err != nil {
		return err
	}
	if volume != n.VolumeIdentity {
		return model.Fail("storage.unavailable", "The selected namespace root has a different filesystem identity; publication remains pending.")
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT id FROM namespace_events WHERE publication_state='pending_namespace' ORDER BY sequence")
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return s.VerifyNamespace(ctx)
	}
	for _, id := range ids {
		e, er := s.readEvent(ctx, id)
		if er != nil {
			return er
		}
		p := filepath.Join(n.Root, "_easy-lima", "namespace.json")
		old, er := files.Digest(p)
		if er != nil {
			return er
		}
		if old != "" {
			var prev NamespaceMarker
			b, er := files.Read(p, 65536)
			if er != nil {
				return er
			}
			if er = files.Unwrap(b, &prev); er != nil {
				return er
			}
			if prev.RegistryID != e.Payload.RegistryID || prev.NamespaceID != n.ID || (prev.LastEventID != e.ID && (e.PreviousNamespaceEventID == nil || prev.LastEventID != *e.PreviousNamespaceEventID)) {
				return model.Fail("ownership.conflict", "The VM namespace has a conflicting owner or epoch.")
			}
		} else if e.PreviousNamespaceEventID != nil {
			return model.Fail("ownership.conflict", "The existing namespace witness is missing.")
		}
		for _, m := range e.Payload.MarkerChanges {
			if filepath.IsAbs(m.RelativePath) || strings.Contains(m.RelativePath, "..") || filepath.Base(m.RelativePath) != ".easy-lima-owner.json" {
				return model.Fail("record.invalid", "Invalid ownership marker destination.")
			}
			mp := filepath.Join(n.Root, m.RelativePath)
			cur, er := files.Digest(mp)
			if er != nil {
				return er
			}
			if cur == m.NewDigest {
				continue
			}
			if cur != m.OldDigest {
				return model.Fail("ownership.conflict", "An instance owner marker has conflicting contents.")
			}
			if m.NewDigest == "" {
				if er = files.RemoveExpected(mp, m.OldDigest); er != nil {
					return er
				}
			} else {
				if files.Hash([]byte(m.NewPayload)) != m.NewDigest {
					return model.Fail("record.invalid", "Ownership marker digest mismatch.")
				}
				if er = files.PrivateDir(filepath.Dir(mp)); er != nil {
					return er
				}
				if er = files.Replace(mp, []byte(m.NewPayload), m.OldDigest, 0600); er != nil {
					return er
				}
			}
		}
		nm := NamespaceMarker{1, e.Payload.RegistryID, n.ID, n.VolumeIdentity, e.Payload.NamespaceEpoch, e.ID, e.EventHash}
		b, er := files.Wrap(nm)
		if er != nil {
			return er
		}
		if er = files.Replace(p, b, old, 0600); er != nil {
			return er
		}
		if er = s.fault("namespace_published"); er != nil {
			return er
		}
		lg, er := files.Lock(ctx, filepath.Join(s.Dir, "publication.lock"), true, false)
		if er != nil {
			return er
		}
		_, er = s.DB.ExecContext(ctx, "UPDATE namespace_events SET publication_state='published',namespace_published_at=? WHERE id=?", model.Now(), id)
		if er == nil {
			_, er = s.DB.ExecContext(ctx, "UPDATE registrations SET publication_pending=0 WHERE owner_event_id=?", id)
		}
		lg.Close()
		if er != nil {
			return er
		}
	}
	return nil
}

// VerifyNamespace is read-only and requires the exact committed volume witness.
func (s *Store) VerifyNamespace(ctx context.Context) error {
	n, err := s.Namespace(ctx)
	if err != nil {
		return err
	}
	volume, err := files.RootIdentity(n.Root)
	if err != nil || volume != n.VolumeIdentity {
		return model.Fail("storage.unavailable", "The selected VM storage volume is unavailable or changed.")
	}
	reg, err := s.Registry(ctx)
	if err != nil {
		return err
	}
	if reg.LastNamespaceEventID == nil {
		return model.Fail("ownership.conflict", "The registry has no namespace witness.")
	}
	e, err := s.readEvent(ctx, *reg.LastNamespaceEventID)
	if err != nil {
		return err
	}
	b, err := files.Read(filepath.Join(n.Root, "_easy-lima", "namespace.json"), 65536)
	if err != nil {
		return model.Fail("ownership.conflict", "The namespace owner witness is missing or unreadable.")
	}
	var m NamespaceMarker
	if err = files.Unwrap(b, &m); err != nil {
		return err
	}
	if m.RegistryID != reg.ID || m.NamespaceID != n.ID || m.VolumeIdentity != n.VolumeIdentity || m.Epoch != n.Epoch || m.LastEventID != e.ID || m.LastEventHash != e.EventHash {
		return model.Fail("ownership.conflict", "The namespace owner or epoch differs from the authoritative registry.")
	}
	return nil
}

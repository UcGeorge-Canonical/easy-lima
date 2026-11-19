package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/project"
)

type VMRecord struct {
	VM                                                                        model.VM
	RowRevision, BindingGeneration, OwnerGeneration, ProfileBindingGeneration int64
	AppliedProfileID                                                          *string
	AppliedProfileBindingGeneration                                           *int64
	PendingIdentityKind, PendingProfileID, IdentityPhase, IdentityOperationID *string
	ActiveTrustGeneration                                                     *int64
	PublicationPending                                                        bool
	Environment                                                               json.RawMessage
}

func readVM(ctx context.Context, q Queryer, id string) (VMRecord, error) {
	var r VMRecord
	v := &r.VM
	var env *string
	err := q.QueryRowContext(ctx, `SELECT r.id,r.namespace_id,r.instance_name,r.ssh_alias,r.source_path,r.profile_id,r.applied_environment_revision_id,r.applied_profile_revision_id,r.pending_profile_revision_id,r.ownership,r.lifecycle,r.row_revision,r.binding_generation,r.owner_generation,r.profile_binding_generation,r.applied_profile_id,r.applied_profile_binding_generation,r.pending_identity_kind,r.pending_profile_id,r.identity_transition_phase,r.identity_operation_id,r.active_trust_generation,r.publication_pending,n.root,e.inputs FROM registrations r JOIN storage_namespace n ON r.namespace_id=n.namespace_id LEFT JOIN environment_revisions e ON e.id=r.applied_environment_revision_id WHERE r.id=?`, id).Scan(&v.ID, &v.NamespaceID, &v.InstanceName, &v.SSHAlias, &v.SourcePath, &v.ProfileID, &v.AppliedEnvironmentRevisionID, &v.AppliedIdentityRevisionID, &v.PendingIdentityRevisionID, &v.Ownership, &v.State.Recorded, &r.RowRevision, &r.BindingGeneration, &r.OwnerGeneration, &r.ProfileBindingGeneration, &r.AppliedProfileID, &r.AppliedProfileBindingGeneration, &r.PendingIdentityKind, &r.PendingProfileID, &r.IdentityPhase, &r.IdentityOperationID, &r.ActiveTrustGeneration, &r.PublicationPending, &v.Storage.Root, &env)
	if errors.Is(err, sql.ErrNoRows) {
		return r, model.Fail("target.not_found", "No registration matches the exact selected project or VM ID.")
	}
	if err != nil {
		return r, err
	}
	if !model.ValidID(v.ID) {
		return r, model.Fail("record.invalid", "A registration has an invalid ID.")
	}
	v.OperationIDs = []string{}
	v.Storage.InstanceDirectory = filepath.Join(v.Storage.Root, v.InstanceName)
	if env != nil {
		r.Environment = json.RawMessage(*env)
		var e struct {
			Settings config.Settings `json:"settings"`
		}
		if json.Unmarshal([]byte(*env), &e) != nil {
			return r, model.Fail("record.invalid", "The registered environment is invalid.")
		}
		if e.Settings.Resources.CPUs > 0 {
			v.Resources.CPUs = &e.Settings.Resources.CPUs
		}
		if m, er := config.Quantity(e.Settings.Resources.Memory); er == nil {
			v.Resources.MemoryBytes = &m
		}
		if d, er := config.Quantity(e.Settings.Resources.Disk); er == nil {
			v.Resources.DiskMaxBytes = &d
		}
	}
	rows, err := q.QueryContext(ctx, "SELECT id FROM operations WHERE scope_id=? AND status NOT IN('completed','failed_known','cancelled_before_dispatch') ORDER BY created_at", v.ID)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return r, err
		}
		v.OperationIDs = append(v.OperationIDs, id)
	}
	return r, rows.Err()
}
func (s *Store) VM(ctx context.Context, id string) (VMRecord, error) { return readVM(ctx, s.DB, id) }
func (s *Store) ResolveVM(ctx context.Context, id, path string) (VMRecord, error) {
	if id != "" {
		return s.VM(ctx, id)
	}
	var found string
	err := s.DB.QueryRowContext(ctx, "SELECT id FROM registrations WHERE source_key=? AND retired_at IS NULL", path).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		rows, e := s.DB.QueryContext(ctx, "SELECT id,source_path FROM registrations WHERE retired_at IS NULL")
		if e != nil {
			return VMRecord{}, e
		}
		for rows.Next() {
			var candidate, source string
			if e = rows.Scan(&candidate, &source); e != nil {
				rows.Close()
				return VMRecord{}, e
			}
			if project.Same(source, path) {
				found = candidate
				break
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return VMRecord{}, e
		}
		if found == "" {
			return VMRecord{}, model.Fail("target.not_found", "The exact folder has no registered VM.")
		}
	} else if err != nil {
		return VMRecord{}, err
	}
	return s.VM(ctx, found)
}

type Cursor struct {
	Generation            int64 `json:"g"`
	Family, Filter, After string
}

func (s *Store) Page(ctx context.Context, family, filter, token string) (string, int64, error) {
	r, err := s.Registry(ctx)
	if err != nil {
		return "", 0, err
	}
	if token == "" {
		return "", r.Generation, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(b) > 4096 {
		return "", 0, model.Fail("input.invalid", "Invalid pagination cursor.")
	}
	var c Cursor
	if json.Unmarshal(b, &c) != nil || c.Family != family || c.Filter != filter || c.Generation != r.Generation {
		return "", 0, model.Fail("input.invalid", "The pagination cursor has expired or belongs to another listing.")
	}
	return c.After, r.Generation, nil
}
func nextCursor(gen int64, family, filter, after string) *string {
	s := base64.RawURLEncoding.EncodeToString([]byte(JSON(Cursor{gen, family, filter, after})))
	return &s
}

func LogCursor(gen int64, vmID, after string) *string { return nextCursor(gen, "logs", vmID, after) }
func (s *Store) ListVMs(ctx context.Context, profile string, limit int, cursor string) (model.ListData[model.VM], error) {
	out := model.NewList[model.VM]()
	after, gen, err := s.Page(ctx, "list", profile, cursor)
	if err != nil {
		return out, err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT id FROM registrations WHERE retired_at IS NULL AND id>? AND (?='' OR profile_id=?) ORDER BY id LIMIT ?", after, profile, profile, limit+1)
	if err != nil {
		return out, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(ids) > limit {
		ids = ids[:limit]
		out.NextCursor = nextCursor(gen, "list", profile, ids[len(ids)-1])
	}
	for _, id := range ids {
		r, err := s.VM(ctx, id)
		if err != nil {
			out.Complete = false
			out.InvalidRecords = append(out.InvalidRecords, model.RecordError{ID: &id, Source: "sqlite", Code: "record.invalid", Message: "Registration could not be decoded."})
			continue
		}
		out.Items = append(out.Items, r.VM)
	}
	return out, nil
}

type OperationRecord struct {
	Operation                   model.Operation
	Token                       string
	Inputs                      json.RawMessage
	Plan                        model.Plan
	RuntimePath, RuntimeVersion *string
}

func (s *Store) Operation(ctx context.Context, id string) (OperationRecord, error) {
	var r OperationRecord
	v := &r.Operation
	var inputs, plan string
	err := s.DB.QueryRowContext(ctx, `SELECT id,kind,status,scope_kind,scope_id,scope_path,parent_operation_id,retry_of_operation_id,last_confirmed_stage,created_at,updated_at,token,inputs,plan,runtime_path,runtime_version FROM operations WHERE id=?`, id).Scan(&v.ID, &v.Kind, &v.Status, &v.Scope.Kind, &v.Scope.ID, &v.Scope.Path, &v.ParentID, &v.RetryOfID, &v.LastConfirmedStage, &v.CreatedAt, &v.UpdatedAt, &r.Token, &inputs, &plan, &r.RuntimePath, &r.RuntimeVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return r, model.Fail("target.not_found", "Operation not found.")
	}
	if err != nil {
		return r, err
	}
	v.RecoveryRequired = !model.Terminal(v.Status) && v.Status != "prepared"
	v.LogPath = model.Ptr(filepath.Join(s.Dir, "logs", id+".jsonl"))
	r.Inputs = json.RawMessage(inputs)
	err = json.Unmarshal([]byte(plan), &r.Plan)
	return r, err
}
func (s *Store) ListOperations(ctx context.Context, limit int, cursor string) (model.ListData[model.Operation], error) {
	out := model.NewList[model.Operation]()
	after, gen, err := s.Page(ctx, "operations.list", "", cursor)
	if err != nil {
		return out, err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT id FROM operations WHERE id>? ORDER BY id LIMIT ?", after, limit+1)
	if err != nil {
		return out, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(ids) > limit {
		ids = ids[:limit]
		out.NextCursor = nextCursor(gen, "operations.list", "", ids[len(ids)-1])
	}
	for _, id := range ids {
		r, err := s.Operation(ctx, id)
		if err != nil {
			out.Complete = false
			out.InvalidRecords = append(out.InvalidRecords, model.RecordError{ID: &id, Source: "sqlite", Code: "record.invalid", Message: "Operation record is invalid."})
		} else {
			out.Items = append(out.Items, r.Operation)
		}
	}
	return out, nil
}

type Claim struct{ Kind, Key, Mode string }
type NewVM struct {
	ID, NamespaceID, InstanceName, Alias, SourcePath string
	ProfileID                                        *string
}

func (s *Store) Prepare(ctx context.Context, plan model.Plan, inputs any, claims []Claim, vm *NewVM) (OperationRecord, error) {
	id, token := model.NewID(), model.NewID()
	now := model.Now()
	scope := plan.Target
	err := s.Change(ctx, Change{Kind: "intent", OperationID: id, Namespace: scope.Kind == "vm"}, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO operations(id,kind,scope_kind,scope_id,scope_path,token,status,inputs,plan,created_at,updated_at) VALUES(?,?,?,?,?,?,'prepared',?,?,?,?)", id, plan.Action, scope.Kind, scope.ID, scope.Path, token, JSON(inputs), JSON(plan), now, now); err != nil {
			return err
		}
		if plan.Action == "adopt" && scope.Kind == "vm" {
			var adoption struct {
				VM                 NewVM  `json:"vm"`
				ProfileExisting    bool   `json:"profile_existing"`
				IdentityRevisionID string `json:"identity_revision_id"`
			}
			if e := json.Unmarshal([]byte(JSON(inputs)), &adoption); e != nil {
				return e
			}
			if e := checkOverlap(ctx, tx, adoption.VM.SourcePath, adoption.VM.ProfileID, adoption.VM.ID); e != nil {
				return e
			}
			if adoption.ProfileExisting && adoption.VM.ProfileID != nil {
				if err := RequireActiveProfile(ctx, tx, *adoption.VM.ProfileID); err != nil {
					return err
				}
				var revision string
				if err := tx.QueryRowContext(ctx, "SELECT default_identity_revision_id FROM profiles WHERE id=?", *adoption.VM.ProfileID).Scan(&revision); err != nil {
					return err
				}
				if revision != adoption.IdentityRevisionID {
					return model.Fail("binding.conflict", "The existing adopted profile changed after preview. Review adoption again.")
				}
			}
		}
		if plan.Action == "profile.assign" || plan.Action == "profile.unassign" || plan.Action == "profile.apply" || plan.Action == "profile.migrate-agent" {
			var transition struct {
				Targets []struct {
					VMID      string  `json:"vm_id"`
					ProfileID *string `json:"profile_id"`
				} `json:"targets"`
			}
			if err := json.Unmarshal([]byte(JSON(inputs)), &transition); err != nil {
				return err
			}
			targets := map[string]*string{}
			for _, target := range transition.Targets {
				targets[target.VMID] = target.ProfileID
			}
			for _, target := range transition.Targets {
				if target.ProfileID != nil {
					if err := RequireActiveProfile(ctx, tx, *target.ProfileID); err != nil {
						return err
					}
				}
				var source string
				if err := tx.QueryRowContext(ctx, "SELECT source_path FROM registrations WHERE id=? AND retired_at IS NULL", target.VMID).Scan(&source); err != nil {
					return err
				}
				if err := CheckProfileOverlapIn(ctx, tx, source, target.ProfileID, target.VMID, targets); err != nil {
					return err
				}
			}
		}
		for _, c := range claims {
			if c.Kind == "profile" && scope.Kind != "global" && plan.Action != "profile.create" && plan.Action != "profile.import" && plan.Action != "adopt" {
				if err := RequireActiveProfile(ctx, tx, c.Key); err != nil {
					return err
				}
			}
			if c.Kind == "profile" && plan.Action == "profile.remove" {
				if err := RequireUnusedProfile(ctx, tx, c.Key); err != nil {
					return err
				}
			}
			if c.Kind == "profile-label" {
				var owner string
				err := tx.QueryRowContext(ctx, "SELECT id FROM profiles WHERE label=? AND retired_at IS NULL", c.Key).Scan(&owner)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				if err == nil && (scope.ID == nil || owner != *scope.ID) {
					return model.Fail("binding.conflict", "Another profile already owns that label.")
				}
			}
			if c.Kind == "port" {
				if scope.ID == nil {
					return model.Fail("record.invalid", "A port reservation needs a VM owner.")
				}
				var occupied int
				if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM owned_settings WHERE field='port' AND target=? AND owner_id<>?", c.Key, *scope.ID).Scan(&occupied); err != nil {
					return err
				}
				if occupied > 0 {
					return model.Fail("port.unavailable", "Another VM reserves persistent endpoint "+c.Key+".")
				}
				if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM registrations r JOIN environment_revisions e ON e.id=r.applied_environment_revision_id JOIN json_each(e.inputs,'$.settings.ports') p WHERE r.retired_at IS NULL AND r.id<>? AND json_extract(p.value,'$.protocol')||'/'||json_extract(p.value,'$.host_address')||'/'||json_extract(p.value,'$.local')=?`, *scope.ID, c.Key).Scan(&occupied); err != nil {
					return err
				}
				if occupied > 0 {
					return model.Fail("port.unavailable", "Another registered VM uses persistent endpoint "+c.Key+".")
				}
				if _, err := tx.ExecContext(ctx, "INSERT INTO owned_settings(owner_id,owner_kind,target,field,expected_digest) VALUES(?,'vm',?,'port',?) ON CONFLICT(owner_id,target,field) DO NOTHING", *scope.ID, c.Key, id); err != nil {
					return err
				}
			}
			var count int
			if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM resource_claims WHERE kind=? AND resource_key=? AND (mode='exclusive' OR ?='exclusive')", c.Kind, c.Key, c.Mode).Scan(&count); err != nil {
				return err
			}
			if count > 0 {
				return model.Fail("operation.busy", "Another unresolved operation holds a conflicting resource claim.")
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO resource_claims VALUES(?,?,?,?,?)", id, c.Kind, c.Key, c.Mode, token); err != nil {
				return err
			}
		}
		if vm != nil {
			if vm.ProfileID != nil {
				if err := RequireActiveProfile(ctx, tx, *vm.ProfileID); err != nil {
					return err
				}
			}
			if err := checkOverlap(ctx, tx, vm.SourcePath, vm.ProfileID, vm.ID); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, "INSERT INTO registrations(id,namespace_id,instance_name,ssh_alias,source_path,source_key,profile_id,ownership,lifecycle,created_at) VALUES(?,?,?,?,?,?,?,'reserved','planned',?)", vm.ID, vm.NamespaceID, vm.InstanceName, vm.Alias, vm.SourcePath, vm.SourcePath, vm.ProfileID, now)
			if err == nil && vm.ProfileID != nil {
				_, err = tx.ExecContext(ctx, "UPDATE profiles SET membership_generation=membership_generation+1 WHERE id=?", *vm.ProfileID)
			}
			return err
		}
		return nil
	})
	if err != nil {
		return OperationRecord{}, err
	}
	if s.Progress != nil {
		s.Progress("Operation " + id + " · " + plan.Action)
	}
	return s.Operation(ctx, id)
}

const overlappingBindings = `SELECT id,source_path,profile_id FROM registrations WHERE retired_at IS NULL
UNION ALL SELECT registration_id,source_path,profile_id FROM binding_reservations
UNION ALL SELECT json_extract(inputs,'$.vm.ID'),json_extract(inputs,'$.vm.SourcePath'),json_extract(inputs,'$.vm.ProfileID') FROM operations WHERE kind='adopt' AND json_type(inputs,'$.vm.ID')='text' AND status NOT IN ('completed','failed_known','cancelled_before_dispatch')
UNION ALL SELECT r.id,r.source_path,json_extract(t.value,'$.profile_id') FROM operations o JOIN json_each(o.inputs,'$.targets') t JOIN registrations r ON r.id=json_extract(t.value,'$.vm_id') WHERE o.kind IN ('profile.assign','profile.unassign','profile.apply','profile.migrate-agent') AND o.status NOT IN ('completed','failed_known','cancelled_before_dispatch')`

func checkOverlap(ctx context.Context, q Queryer, path string, profile *string, except string) error {
	rows, err := q.QueryContext(ctx, overlappingBindings)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, p string
		var prof *string
		if err = rows.Scan(&id, &p, &prof); err != nil {
			return err
		}
		if id == except {
			continue
		}
		if project.Same(path, p) {
			return model.Fail("binding.conflict", "That folder is already registered or reserved by another operation.")
		}
		sameProfile := (profile == nil && prof == nil) || (profile != nil && prof != nil && *profile == *prof)
		if !sameProfile && project.Overlap(path, p) {
			return model.Fail("profile.scope_conflict", fmt.Sprintf("Project %q conflicts with %q (VM %s): parent/child projects must use the same profile, including profile-free.", path, p, id))
		}
	}
	return rows.Err()
}
func (s *Store) CheckOverlap(ctx context.Context, path string, profile *string, except string) error {
	return checkOverlap(ctx, s.DB, path, profile, except)
}

type Step struct {
	Stage         string          `json:"stage"`
	Attempt       int             `json:"attempt"`
	DispatchState string          `json:"dispatch_state"`
	EffectState   string          `json:"effect_state"`
	ResultCode    string          `json:"result_code"`
	Evidence      json.RawMessage `json:"-"`
}

func (s *Store) Steps(ctx context.Context, id string) ([]Step, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT stage,attempt,dispatch_state,effect_state,result_code,evidence FROM operation_steps WHERE operation_id=? ORDER BY rowid", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Step{}
	for rows.Next() {
		var st Step
		var b string
		if err = rows.Scan(&st.Stage, &st.Attempt, &st.DispatchState, &st.EffectState, &st.ResultCode, &b); err != nil {
			return out, err
		}
		st.Evidence = json.RawMessage(b)
		out = append(out, st)
	}
	return out, rows.Err()
}
func (s *Store) RecordStep(ctx context.Context, op OperationRecord, stage, dispatch, effect, code string, evidence any) error {
	err := s.Change(ctx, Change{Kind: "effect", OperationID: op.Operation.ID, Stage: stage, ResultCode: code, EffectState: effect}, func(tx *sql.Tx) error {
		var tok string
		if err := tx.QueryRowContext(ctx, "SELECT token FROM operations WHERE id=?", op.Operation.ID).Scan(&tok); err != nil {
			return err
		}
		if tok != op.Token {
			return model.Fail("operation.busy", "This controller has been superseded.")
		}
		var attempt int
		if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(attempt),0) FROM operation_steps WHERE operation_id=? AND stage=?", op.Operation.ID, stage).Scan(&attempt); err != nil {
			return err
		}
		if dispatch == "may_have_dispatched" || attempt == 0 {
			attempt++
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO operation_steps VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(operation_id,stage,attempt) DO UPDATE SET dispatch_state=excluded.dispatch_state,effect_state=excluded.effect_state,evidence=excluded.evidence,observed_at=excluded.observed_at,result_code=excluded.result_code", op.Operation.ID, stage, attempt, dispatch, effect, JSON(evidence), model.Now(), code)
		if err != nil {
			return err
		}
		status := "active"
		if effect == "unknown" {
			status = "needs_reconciliation"
		}
		var confirmed any
		if effect == "satisfied" {
			confirmed = stage
		}
		_, err = tx.ExecContext(ctx, "UPDATE operations SET status=?,last_confirmed_stage=COALESCE(?,last_confirmed_stage),updated_at=? WHERE id=? AND token=?", status, confirmed, model.Now(), op.Operation.ID, op.Token)
		return err
	})
	if err == nil && s.Progress != nil {
		s.Progress("Operation " + op.Operation.ID + " · " + stage + " · " + effect)
	}
	return err
}
func (s *Store) Finish(ctx context.Context, op OperationRecord, status string) error {
	return s.Change(ctx, Change{Kind: "effect", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, "UPDATE operations SET status=?,updated_at=? WHERE id=? AND token=?", status, model.Now(), op.Operation.ID, op.Token)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return model.Fail("operation.busy", "The executor token changed.")
		}
		if model.Terminal(status) {
			for _, table := range []string{"resource_claims", "binding_reservations"} {
				if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE operation_id=?", op.Operation.ID); err != nil {
					return err
				}
			}
			_, err = tx.ExecContext(ctx, "UPDATE capacity_claims SET state='released',remaining_bytes=0,released_at=? WHERE operation_id=?", model.Now(), op.Operation.ID)
		}
		return err
	})
}
func (s *Store) Takeover(ctx context.Context, id string) (OperationRecord, error) {
	token := model.NewID()
	err := s.Change(ctx, Change{Kind: "intent", OperationID: id}, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE operations SET token=?,status='reconciling',updated_at=? WHERE id=? AND status NOT IN('completed','failed_known','cancelled_before_dispatch')", token, model.Now(), id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE resource_claims SET token=? WHERE operation_id=?", token, id)
		return err
	})
	if err != nil {
		return OperationRecord{}, err
	}
	return s.Operation(ctx, id)
}
func (s *Store) UpdateVMState(ctx context.Context, op OperationRecord, id, lifecycle, ownership string) error {
	return s.Change(ctx, Change{Kind: "ownership", OperationID: op.Operation.ID, Namespace: true, VMIDs: []string{id}}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE registrations SET lifecycle=?,ownership=?,row_revision=row_revision+1 WHERE id=?", lifecycle, ownership, id)
		return err
	})
}
func (s *Store) SaveEnvironment(ctx context.Context, op OperationRecord, vmID, revID, digest string, inputs any, history string) error {
	return s.Change(ctx, Change{Kind: "effect", OperationID: op.Operation.ID, Namespace: true, VMIDs: []string{vmID}}, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM owned_settings WHERE owner_id=? AND field='port' AND target NOT IN (SELECT resource_key FROM resource_claims WHERE operation_id=? AND kind='port')", vmID, op.Operation.ID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO environment_revisions VALUES(?,?,1,?,?,'[]','v1',?,?)", revID, vmID, digest, JSON(inputs), history, model.Now())
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "UPDATE registrations SET applied_environment_revision_id=?,row_revision=row_revision+1 WHERE id=?", revID, vmID)
		return err
	})
}
func (s *Store) OwnSetting(ctx context.Context, opID, owner, kind, target, field, digest string) error {
	return s.Change(ctx, Change{Kind: "effect", OperationID: opID}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO owned_settings(owner_id,owner_kind,target,field,expected_digest) VALUES(?,?,?,?,?) ON CONFLICT(owner_id,target,field) DO UPDATE SET expected_digest=excluded.expected_digest", owner, kind, target, field, digest)
		return err
	})
}
func (s *Store) Setting(ctx context.Context, owner, target, field string) (string, error) {
	var d string
	err := s.DB.QueryRowContext(ctx, "SELECT expected_digest FROM owned_settings WHERE owner_id=? AND target=? AND field=?", owner, target, field).Scan(&d)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return d, err
}

func (s *Store) CheckProfileOverlap(ctx context.Context, source string, profile *string, id string, targets map[string]*string) error {
	return CheckProfileOverlapIn(ctx, s.DB, source, profile, id, targets)
}
func CheckProfileOverlapIn(ctx context.Context, q Queryer, source string, profile *string, id string, targets map[string]*string) error {
	rows, err := q.QueryContext(ctx, overlappingBindings)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var other, path string
		var current *string
		if err = rows.Scan(&other, &path, &current); err != nil {
			return err
		}
		if other == id {
			continue
		}
		if final, ok := targets[other]; ok {
			current = final
		}
		same := (profile == nil && current == nil) || (profile != nil && current != nil && *profile == *current)
		if !same && project.Overlap(source, path) {
			return model.Fail("profile.scope_conflict", fmt.Sprintf("Project %q conflicts with %q (VM %s): the final parent/child profiles would differ.", source, path, other))
		}
	}
	return rows.Err()
}

func RequireActiveProfile(ctx context.Context, q Queryer, id string) error {
	var count int
	if err := q.QueryRowContext(ctx, "SELECT count(*) FROM profiles WHERE id=? AND retired_at IS NULL", id).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return model.Fail("target.not_found", "The selected profile was removed or is unavailable. Review the request again.")
	}
	return nil
}
func RequireUnusedProfile(ctx context.Context, q Queryer, id string) error {
	var count int
	if err := q.QueryRowContext(ctx, "SELECT count(*) FROM registrations WHERE retired_at IS NULL AND (profile_id=? OR pending_profile_id=?)", id, id).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return model.Fail("binding.conflict", "The profile gained a member after preview and cannot be removed.")
	}
	return nil
}

func CheckOverlapIn(ctx context.Context, q Queryer, path string, profile *string, except string) error {
	return checkOverlap(ctx, q, path, profile, except)
}

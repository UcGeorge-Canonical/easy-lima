package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type KeyReference struct {
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	PublicPath string `json:"public_path"`
}

func (s *Store) SaveExternalProfile(ctx context.Context, op OperationRecord, p config.Profile, pub, fingerprint string, b agent.Binding) error {
	return s.Change(ctx, Change{Kind: "profile", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
		cred, revision := model.NewID(), model.NewID()
		desired, err := config.Encode(p)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO profiles(id,label,active_credential_revision_id,default_identity_revision_id,desired_digest) VALUES(?,?,?,?,?)", p.ID, p.Label, cred, revision, files.Hash(desired)); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO profile_credential_revisions VALUES(?,?,1,?,?,?,?,?,?,?,?,?,?,?,?)", cred, p.ID, files.Hash([]byte(pub)), pub, fingerprint, JSON(KeyReference{Kind: "file", Path: p.Key.Path, PublicPath: p.Key.PublicPath}), "external", "external", JSON(b), JSON(map[string]string{"kind": "keychain", "service": "canonical-local-work-git", "account": "work-ssh-key"}), "external", "github", p.GitHub.Username, model.Now())
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO profile_revisions VALUES(?,?,?,1,?,?,?)", revision, p.ID, cred, JSON(p.Git), files.Hash([]byte(JSON(p.Git))), model.Now()); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO profile_runtime(profile_id,credential_revision_id,agent_management,lock_state,remember_enabled) VALUES(?,?,'external',NULL,?)", p.ID, cred, p.Unlock.Remember)
		return err
	})
}

type ProfileRecord struct {
	Profile                                 model.Profile
	DesiredDigest                           string
	CredentialID, IdentityID                string
	Key                                     KeyReference
	KeyOwnership                            string
	Agent                                   agent.Binding
	StoreRef, StoreOwnership                *string
	RuntimeGeneration, MembershipGeneration int64
	TransitionOperationID                   *string
}

func (s *Store) Profile(ctx context.Context, labelOrID string) (ProfileRecord, error) {
	var r ProfileRecord
	v := &r.Profile
	var git, key, binding, lock string
	var lockNull *string
	var remember bool
	err := s.DB.QueryRowContext(ctx, `SELECT p.id,p.label,p.desired_digest,p.active_credential_revision_id,p.default_identity_revision_id,p.membership_generation,c.public_key,c.fingerprint,c.key_ref,c.key_ownership,c.agent_management,c.agent_binding,c.store_ref,c.store_ownership,c.expected_account,i.guest_identity,r.lock_state,r.remember_enabled,r.generation,r.transition_operation_id FROM profiles p JOIN profile_credential_revisions c ON c.id=p.active_credential_revision_id JOIN profile_revisions i ON i.id=p.default_identity_revision_id JOIN profile_runtime r ON r.profile_id=p.id WHERE (p.id=? OR p.label=?) AND p.retired_at IS NULL`, labelOrID, labelOrID).Scan(&v.ID, &v.Label, &r.DesiredDigest, &r.CredentialID, &r.IdentityID, &r.MembershipGeneration, &v.PublicKey, &v.Fingerprint, &key, &r.KeyOwnership, &v.AgentManagement, &binding, &r.StoreRef, &r.StoreOwnership, &v.GitHubUsername, &git, &lockNull, &remember, &r.RuntimeGeneration, &r.TransitionOperationID)
	if errors.Is(err, sql.ErrNoRows) {
		return r, model.Fail("target.not_found", "Profile not found.")
	}
	if err != nil {
		return r, err
	}
	r.Key, err = Decode[KeyReference](key)
	if err != nil {
		return r, err
	}
	r.Agent, err = Decode[agent.Binding](binding)
	if err != nil {
		return r, err
	}
	v.Git, err = Decode[model.GitIdentity](git)
	if err != nil {
		return r, err
	}
	lock = "managed_externally"
	if lockNull != nil {
		lock = *lockNull
	}
	v.LockState = lock
	v.RememberEnabled = remember
	v.MemberVMIDs = []string{}
	v.PendingOperationIDs = []string{}
	if r.TransitionOperationID != nil {
		v.PendingOperationIDs = append(v.PendingOperationIDs, *r.TransitionOperationID)
	}
	v.Capabilities = []string{"use", "check", "apply_guest", "remove"}
	if v.AgentManagement == "external" {
		v.Capabilities = append(v.Capabilities, "migrate_agent")
	} else {
		v.Capabilities = append(v.Capabilities, "lock", "unlock", "apply_credential", "forget")
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT id FROM registrations WHERE profile_id=? AND retired_at IS NULL ORDER BY id", v.ID)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return r, err
		}
		v.MemberVMIDs = append(v.MemberVMIDs, id)
	}
	return r, rows.Err()
}
func (s *Store) ListProfiles(ctx context.Context, limit int, cursor string) (model.ListData[model.Profile], error) {
	out := model.NewList[model.Profile]()
	after, gen, err := s.Page(ctx, "profile.list", "", cursor)
	if err != nil {
		return out, err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT id FROM profiles WHERE retired_at IS NULL AND id>? ORDER BY id LIMIT ?", after, limit+1)
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
		out.NextCursor = nextCursor(gen, "profile.list", "", ids[len(ids)-1])
	}
	for _, id := range ids {
		p, e := s.Profile(ctx, id)
		if e != nil {
			out.Complete = false
			out.InvalidRecords = append(out.InvalidRecords, model.RecordError{ID: &id, Source: "sqlite", Code: "record.invalid", Message: "Profile could not be decoded."})
		} else {
			out.Items = append(out.Items, p.Profile)
		}
	}
	return out, nil
}
func (s *Store) SaveProfile(ctx context.Context, op OperationRecord, p config.Profile, pub, fingerprint, digest, keyOwnership string, b agent.Binding) error {
	return s.Change(ctx, Change{Kind: "profile", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
		cred, identity := model.NewID(), model.NewID()
		now := model.Now()
		var account any
		if p.GitHub.Username != "" {
			account = p.GitHub.Username
		}
		var storeRef, storeOwnership any
		if p.Unlock.Remember {
			storeRef = JSON(map[string]string{"kind": "native", "item_reference": p.ID})
			storeOwnership = "owned"
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO profiles(id,label,active_credential_revision_id,default_identity_revision_id,desired_digest) VALUES(?,?,?,?,?)", p.ID, p.Label, cred, identity, digest); err != nil {
			return err
		}
		key := KeyReference{"file", p.Key.Path, p.Key.PublicPath}
		_, err := tx.ExecContext(ctx, `INSERT INTO profile_credential_revisions VALUES(?,?,1,?,?,?,?,?,?,?,?,?,?,?,?)`, cred, p.ID, files.Hash([]byte(pub)), pub, fingerprint, JSON(key), keyOwnership, "owned", JSON(b), storeRef, storeOwnership, "github", account, now)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO profile_revisions VALUES(?,?,?,1,?,?,?)", identity, p.ID, cred, JSON(p.Git), files.Hash([]byte(JSON(p.Git))), now)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO profile_runtime(profile_id,credential_revision_id,agent_management,lock_state,remember_enabled) VALUES(?,?,'owned','unlocked',?)", p.ID, cred, p.Unlock.Remember)
		return err
	})
}
func (s *Store) SetProfileRuntime(ctx context.Context, opID, id, lock string, remember *bool) error {
	return s.Change(ctx, Change{Kind: "profile", OperationID: opID}, func(tx *sql.Tx) error {
		if lock != "" {
			var management string
			if err := tx.QueryRowContext(ctx, "SELECT agent_management FROM profile_runtime WHERE profile_id=?", id).Scan(&management); err != nil {
				return err
			}
			if management != "owned" {
				return model.Fail("profile.external_managed", "Lock and unlock require an independently owned agent.")
			}
			if _, err := tx.ExecContext(ctx, "UPDATE profile_runtime SET lock_state=?,generation=generation+1,row_revision=row_revision+1 WHERE profile_id=?", lock, id); err != nil {
				return err
			}
		}
		if remember != nil {
			_, err := tx.ExecContext(ctx, "UPDATE profile_runtime SET remember_enabled=?,row_revision=row_revision+1 WHERE profile_id=?", *remember, id)
			return err
		}
		return nil
	})
}
func (s *Store) SetDesiredDigest(ctx context.Context, opID, id, label, digest string) error {
	return s.Change(ctx, Change{Kind: "profile", OperationID: opID}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE profiles SET label=?,desired_digest=?,row_revision=row_revision+1 WHERE id=?", label, digest, id)
		return err
	})
}
func (s *Store) BindInitialIdentity(ctx context.Context, opID, vmID, profileID, revisionID string) error {
	return s.Change(ctx, Change{Kind: "profile", OperationID: opID}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE registrations SET pending_identity_kind='profile',pending_profile_id=?,pending_profile_revision_id=?,identity_transition_phase='committed',identity_operation_id=?,profile_binding_generation=profile_binding_generation+1 WHERE id=? AND profile_id=?", profileID, revisionID, opID, vmID, profileID)
		return err
	})
}
func (s *Store) CompleteIdentity(ctx context.Context, opID, vmID string) error {
	return s.Change(ctx, Change{Kind: "profile", OperationID: opID}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE registrations SET applied_profile_id=pending_profile_id,applied_profile_revision_id=pending_profile_revision_id,applied_profile_binding_generation=profile_binding_generation,pending_identity_kind=NULL,pending_profile_id=NULL,pending_profile_revision_id=NULL,identity_transition_phase=NULL,identity_operation_id=NULL,row_revision=row_revision+1 WHERE id=? AND identity_operation_id=? AND identity_transition_phase='committed'`, vmID, opID)
		return err
	})
}

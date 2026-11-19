package app

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/connection"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/credential"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/identity"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

type IdentityTarget struct {
	VMID           string            `json:"vm_id"`
	OldProfileID   *string           `json:"old_profile_id"`
	ProfileID      *string           `json:"profile_id"`
	RevisionID     *string           `json:"revision_id"`
	ContinuationID string            `json:"continuation_id"`
	Git            model.GitIdentity `json:"git"`
	RowRevision    int64             `json:"row_revision"`
	PriorState     string            `json:"prior_state"`
}
type ProfileTransition struct {
	ProfileID            string           `json:"profile_id"`
	Desired              config.Profile   `json:"desired"`
	DesiredDigest        string           `json:"desired_digest"`
	OldCredentialID      string           `json:"old_credential_id"`
	CredentialID         string           `json:"credential_id"`
	DefaultRevisionID    string           `json:"default_revision_id"`
	PublicKey            string           `json:"public_key"`
	Fingerprint          string           `json:"fingerprint"`
	Agent                agent.Binding    `json:"agent"`
	CredentialChange     bool             `json:"credential_change"`
	Migration            bool             `json:"migration"`
	LabelOnly            bool             `json:"label_only"`
	MetadataChange       bool             `json:"metadata_change"`
	StoreRef             *string          `json:"store_ref"`
	StoreOwnership       *string          `json:"store_ownership"`
	KeyOwnership         string           `json:"key_ownership"`
	MembershipGeneration int64            `json:"membership_generation"`
	Targets              []IdentityTarget `json:"targets"`
	ProfileIDs           []string         `json:"profile_ids"`
}

func (s Service) profileTransition(ctx context.Context, db *store.Store, r Request) (model.MutationData, error) {
	input := ProfileTransition{Targets: []IdentityTarget{}, ProfileIDs: []string{}}
	scope := model.Global()
	var p store.ProfileRecord
	var err error
	if r.Command != "profile.unassign" {
		p, err = db.Profile(ctx, r.Label)
		if err != nil {
			return model.MutationData{}, err
		}
		input.ProfileID = p.Profile.ID
		input.CredentialID = p.CredentialID
		input.OldCredentialID = p.CredentialID
		input.DefaultRevisionID = p.IdentityID
		input.PublicKey = *p.Profile.PublicKey
		input.Fingerprint = *p.Profile.Fingerprint
		input.Agent = p.Agent
		input.MembershipGeneration = p.MembershipGeneration
		input.StoreRef, input.StoreOwnership, input.KeyOwnership = p.StoreRef, p.StoreOwnership, p.KeyOwnership
		scope = model.Scope{Kind: "profile", ID: &p.Profile.ID}
		input.ProfileIDs = append(input.ProfileIDs, p.Profile.ID)
	}
	plan := model.NewPlan(r.Command, scope)
	data := model.Mutation(plan)
	if p.Profile.ID != "" {
		data.Profile = &p.Profile
	}
	ids := append([]string{}, r.VMIDs...)
	if r.Command == "profile.apply" || r.Command == "profile.migrate-agent" {
		if p.TransitionOperationID != nil {
			return data, model.Fail("operation.busy", "Resume the existing profile transition before applying another target.")
		}
		f, e := config.LoadProfile(filepath.Join(s.Paths.Config, "profiles", p.Profile.ID+".toml"))
		if e != nil {
			return data, e
		}
		if f.Value.ID != p.Profile.ID {
			return data, model.Fail("ownership.conflict", "The desired profile ID differs from the registered profile.")
		}
		input.Desired = f.Value
		input.DesiredDigest = f.Digest
		if other, e := db.Profile(ctx, f.Value.Label); e == nil && other.Profile.ID != p.Profile.ID {
			return data, model.Fail("binding.conflict", "Another profile already uses label "+f.Value.Label+".")
		} else if e != nil && !isNotFound(e) {
			return data, e
		}
		expectedRef := ""
		if p.StoreOwnership != nil && *p.StoreOwnership == "external" {
			expectedRef = "keychain:canonical-local-work-git:work-ssh-key"
		} else if p.StoreRef != nil {
			expectedRef = p.Profile.ID
		}
		// The reference is opaque ownership metadata, not a user-selected secret source.
		if (p.StoreOwnership != nil && *p.StoreOwnership == "external" && f.Value.Unlock.CredentialRef != expectedRef) || ((p.StoreOwnership == nil || *p.StoreOwnership != "external") && f.Value.Unlock.CredentialRef != "" && f.Value.Unlock.CredentialRef != p.Profile.ID) {
			return data, model.Fail("profile.external_managed", "The credential store reference differs from its recorded owner. Preserve it; arbitrary store-reference changes are unsupported.")
		}
		if p.StoreOwnership != nil && *p.StoreOwnership == "external" && f.Value.Unlock.Remember != p.Profile.RememberEnabled {
			return data, model.Fail("profile.external_managed", "Remembering for this imported credential remains externally managed; preserve its policy.")
		}
		if f.Value.Unlock.Remember && p.Profile.AgentManagement != "external" {
			if _, e := identity.ParsePrivate(f.Value.Key.Path, nil); e == nil {
				return data, model.Fail("secret.input_invalid", "An unencrypted key has no passphrase to remember. Disable remember before applying.")
			} else if model.AsError(e).Code != "profile.unlock_required" {
				return data, e
			}
		}
		public, e := identity.ReadPublic(f.Value.Key.PublicPath)
		if e != nil {
			return data, e
		}
		input.PublicKey = string(ssh.MarshalAuthorizedKey(public))
		input.Fingerprint = ssh.FingerprintSHA256(public)
		if f.Value.Key.Fingerprint != "" && f.Value.Key.Fingerprint != input.Fingerprint {
			return data, model.Fail("identity.key_mismatch", "The desired public key differs from its configured fingerprint.")
		}
		input.MetadataChange = !reflect.DeepEqual(f.Value.Git, p.Profile.Git)
		expected := ""
		if p.Profile.GitHubUsername != nil {
			expected = *p.Profile.GitHubUsername
		}
		input.CredentialChange = input.Fingerprint != *p.Profile.Fingerprint || f.Value.Key.Path != p.Key.Path || f.Value.Key.PublicPath != p.Key.PublicPath || f.Value.GitHub.Username != expected || f.Value.Unlock.Remember != p.Profile.RememberEnabled
		// A migrated agent can still refer to an externally owned saved passphrase.
		// Metadata/account changes preserve that reference; replacing its key would
		// require a separate credential-store ownership transfer.
		if p.StoreOwnership != nil && *p.StoreOwnership == "external" && (input.Fingerprint != *p.Profile.Fingerprint || f.Value.Key.Path != p.Key.Path || f.Value.Key.PublicPath != p.Key.PublicPath) {
			return data, model.Fail("profile.external_managed", "This key uses an externally owned saved passphrase. Its key cannot be replaced by profile apply; create a new owned profile and explicitly assign it instead.")
		}
		input.Migration = r.Command == "profile.migrate-agent"
		if input.Migration {
			if p.Profile.AgentManagement != "external" {
				return data, nil
			}
			if input.Fingerprint != *p.Profile.Fingerprint || input.CredentialChange || input.MetadataChange {
				return data, model.Fail("profile.external_managed", "Agent migration preserves the same key and identity. Apply desired identity edits separately after migration.")
			}
			input.CredentialChange = true
			socket, e := agent.Socket(s.Paths.Cache, p.Profile.ID)
			if e != nil {
				return data, e
			}
			input.Agent = agent.Binding{SocketPath: socket}
			input.Agent, e = agent.Plan(ctx, p.Profile.ID, input.Agent)
			if e != nil {
				return data, e
			}
		}
		if input.CredentialChange && p.Profile.AgentManagement == "external" && !input.Migration {
			return data, model.Fail("profile.external_managed", "Migrate the external agent with the same key before changing shared credentials.")
		}
		input.LabelOnly = !input.CredentialChange && !input.MetadataChange && p.Profile.Label != input.Desired.Label
		if len(ids) == 0 && !input.LabelOnly {
			ids = append(ids, p.Profile.MemberVMIDs...)
		}
		sort.Strings(ids)
		members := append([]string{}, p.Profile.MemberVMIDs...)
		sort.Strings(members)
		if input.CredentialChange && !reflect.DeepEqual(ids, members) {
			return data, model.Fail("binding.conflict", "A shared credential change must include every member VM.")
		}
		if input.CredentialChange {
			input.CredentialID = model.NewID()
		}
		if input.CredentialChange || input.MetadataChange {
			input.DefaultRevisionID = model.NewID()
		}
	} else if len(ids) == 0 {
		return data, model.Fail("input.invalid", "Select at least one VM with --vm.")
	}
	if input.LabelOnly {
		ids = nil
	}
	if len(ids) > 1000 {
		return data, model.Fail("input.invalid", "A single transition supports at most 1000 VMs.")
	}
	sort.Strings(ids)
	seen := map[string]bool{}
	profiles := map[string]bool{}
	for _, id := range input.ProfileIDs {
		profiles[id] = true
	}
	var a lima.Adapter
	if len(ids) > 0 {
		u, e := s.user(r)
		if e != nil {
			return data, e
		}
		n, e := s.validateRoot(ctx, db, u.Value)
		if e != nil {
			return data, e
		}
		a, e = s.adapter(u.Value, n, r, "profile-plan")
		if e != nil {
			return data, e
		}
		if _, e = a.Probe(ctx); e != nil {
			return data, e
		}
	}
	finalProfiles := map[string]*string{}
	for _, id := range ids {
		if r.Command == "profile.unassign" {
			finalProfiles[id] = nil
		} else {
			finalProfiles[id] = &input.ProfileID
		}
	}
	for _, id := range ids {
		if seen[id] {
			return data, model.Fail("input.invalid", "A VM selector may not be repeated.")
		}
		seen[id] = true
		v, e := db.VM(ctx, id)
		if e != nil {
			return data, e
		}
		if v.PendingIdentityKind != nil || len(v.VM.OperationIDs) > 0 {
			same := v.PendingIdentityKind != nil && v.IdentityPhase != nil && *v.IdentityPhase == "committed" && !input.CredentialChange && !input.MetadataChange
			if r.Command == "profile.unassign" {
				same = same && *v.PendingIdentityKind == "none"
			} else {
				same = same && v.PendingProfileID != nil && *v.PendingProfileID == input.ProfileID && v.VM.PendingIdentityRevisionID != nil && *v.VM.PendingIdentityRevisionID == input.DefaultRevisionID
			}
			if same {
				data.MemberResults = append(data.MemberResults, model.MemberResult{VMID: id, Status: "pending", OperationID: v.IdentityOperationID})
				continue
			}
			return data, model.Fail("operation.busy", "A selected VM has pending work; inspect its saved operation before changing identity.")
		}
		if e = s.verifyOwnership(ctx, db, v); e != nil {
			return data, e
		}
		if v.VM.ProfileID != nil {
			profiles[*v.VM.ProfileID] = true
		}
		target := IdentityTarget{VMID: id, OldProfileID: v.VM.ProfileID, RowRevision: v.RowRevision, ContinuationID: model.NewID()}
		if r.Command != "profile.unassign" {
			target.ProfileID = &input.ProfileID
			revision := input.DefaultRevisionID
			target.RevisionID = &revision
			target.Git = p.Profile.Git
			if r.Command == "profile.apply" || input.Migration {
				if v.VM.ProfileID == nil || *v.VM.ProfileID != p.Profile.ID {
					return data, model.Fail("binding.conflict", "profile apply selectors must be members of this profile.")
				}
				target.Git = input.Desired.Git
				if (input.CredentialChange || input.MetadataChange) && v.VM.AppliedIdentityRevisionID != nil {
					var raw string
					e = db.DB.QueryRowContext(ctx, "SELECT guest_identity FROM profile_revisions WHERE id=?", *v.VM.AppliedIdentityRevisionID).Scan(&raw)
					if e != nil {
						return data, e
					}
					target.Git, e = store.Decode[model.GitIdentity](raw)
					if e != nil {
						return data, e
					}
					target.Git = mergeIdentity(p.Profile.Git, input.Desired.Git, target.Git)
					if !reflect.DeepEqual(target.Git, input.Desired.Git) {
						target.RevisionID = model.Ptr(model.NewID())
					}
				}
			}
		}
		if r.Command == "profile.assign" && v.VM.ProfileID != nil && *v.VM.ProfileID == p.Profile.ID {
			continue
		}
		if r.Command == "profile.unassign" && v.VM.ProfileID == nil {
			continue
		}
		if e = db.CheckProfileOverlap(ctx, v.VM.SourcePath, target.ProfileID, v.VM.ID, finalProfiles); e != nil {
			return data, e
		}
		obs, e := a.Observe(ctx, v.VM.InstanceName)
		if e != nil {
			return data, e
		}
		if obs.Status != "Running" && obs.Status != "Stopped" {
			return data, model.Fail("operation.unknown", "Identity changes require each VM to be observably running or stopped.")
		}
		target.PriorState = obs.Status
		input.Targets = append(input.Targets, target)
	}
	if r.Command == "profile.apply" && !input.CredentialChange && !input.MetadataChange {
		allCurrent := true
		for _, t := range input.Targets {
			v, _ := db.VM(ctx, t.VMID)
			if v.VM.AppliedIdentityRevisionID == nil || *v.VM.AppliedIdentityRevisionID != input.DefaultRevisionID {
				allCurrent = false
			}
		}
		if allCurrent && p.Profile.Label == input.Desired.Label {
			if len(data.MemberResults) > 0 {
				return data, model.Fail("operation.partial", "The same identity target is already saved. Open the stopped VM or resume its listed operation to complete it.")
			}
			return data, nil
		}
	}
	if len(input.Targets) == 0 && r.Command != "profile.apply" && r.Command != "profile.migrate-agent" {
		if len(data.MemberResults) > 0 {
			return data, model.Fail("operation.partial", "The same identity target remains pending; use its listed continuation.")
		}
		return data, nil
	}
	input.ProfileIDs = nil
	for id := range profiles {
		input.ProfileIDs = append(input.ProfileIDs, id)
	}
	sort.Strings(input.ProfileIDs)
	if input.Migration {
		plan.Add("vm_stop", "Stop every running member before same-key agent migration. Leave all members stopped with pending identity until explicitly opened.")
	}
	plan.Add("profile", "Fence selected identities, revoke owned forwarding, and commit the reviewed profile targets.")
	plan.Add("guest_stage", "Apply public identity settings to running VMs; preserve stopped VMs with durable pending targets.")
	if input.CredentialChange {
		plan.Add("profile", "Switch the complete profile to the exact selected key after revocation; preserve external agents during same-key migration.")
	}
	if input.LabelOnly {
		plan.Effects = nil
		plan.Add("metadata", "Rename this profile; preserve its key, agent, VM identities and running states.")
	}
	plan.Fields = append(plan.Fields, model.Field{Path: "profile.label", Value: map[string]string{"before": p.Profile.Label, "after": input.Desired.Label}, Changed: p.Profile.Label != input.Desired.Label})
	if input.CredentialChange {
		plan.Fields = append(plan.Fields, model.Field{Path: "key.fingerprint", Value: map[string]string{"before": *p.Profile.Fingerprint, "after": input.Fingerprint}, Changed: true})
	}
	if input.MetadataChange {
		plan.Fields = append(plan.Fields, model.Field{Path: "git", Value: map[string]model.GitIdentity{"before": p.Profile.Git, "after": input.Desired.Git}, Changed: true})
	}
	for _, t := range input.Targets {
		plan.Fields = append(plan.Fields, model.Field{Path: "member." + t.VMID, Value: fmt.Sprintf("%s; target author %s <%s>; stopped members remain pending", t.PriorState, t.Git.Name, t.Git.Email), Origin: "observed"})
	}
	for _, t := range input.Targets {
		plan.AffectedVMIDs = append(plan.AffectedVMIDs, t.VMID)
	}
	data.Plan = plan
	if r.DryRun {
		return data, nil
	}
	if err = s.approve(r, plan); err != nil {
		return data, err
	}
	// Decrypt before acquiring long-lived VM claims. Nothing is persisted except
	// nonsecret key references and the verified public fingerprint.
	var key identity.Key
	var pass []byte
	if input.CredentialChange {
		key, pass, err = s.transitionKey(ctx, p, input, r)
		if err != nil {
			return data, err
		}
		defer credential.Clear(pass)
	}
	claims := []store.Claim{}
	if input.Desired.Label != "" {
		claims = append(claims, store.Claim{Kind: "profile-label", Key: input.Desired.Label, Mode: "exclusive"})
	}
	for _, id := range input.ProfileIDs {
		claims = append(claims, store.Claim{Kind: "profile", Key: id, Mode: "exclusive"})
	}
	for _, t := range input.Targets {
		claims = append(claims, store.Claim{Kind: "vm", Key: t.VMID, Mode: "exclusive"})
	}
	op, err := db.Prepare(ctx, plan, input, claims, nil)
	if err != nil {
		return data, err
	}
	data.Operation = &op.Operation
	guard, err := operationGuard(ctx, db, op.Operation.ID)
	if err != nil {
		return data, err
	}
	defer guard.Close()
	members, err := s.runProfileTransition(ctx, db, op, input, r, key, pass)
	data.MemberResults = append(data.MemberResults, members...)
	data = s.operationResult(ctx, db, data, op)
	return data, err
}
func (s Service) transitionKey(ctx context.Context, p store.ProfileRecord, in ProfileTransition, r Request) (identity.Key, []byte, error) {
	if p.Profile.LockState == "locked" {
		return identity.Key{}, nil, model.Fail("profile.explicitly_locked", "Unlock the profile explicitly before switching its credential.")
	}
	key, err := identity.ParsePrivate(in.Desired.Key.Path, nil)
	var pass []byte
	if model.AsError(err).Code == "profile.unlock_required" && err != nil {
		if in.Desired.Key.Path == p.Key.Path && p.Profile.RememberEnabled && s.Credential != nil {
			pass, err = s.remembered(ctx, p, r.Interactive)
		}
		if len(pass) == 0 {
			pass, err = s.secret(r, "Passphrase for "+p.Profile.Label)
		}
		if err == nil {
			key, err = identity.ParsePrivate(in.Desired.Key.Path, pass)
		}
	}
	if err != nil {
		credential.Clear(pass)
		return key, nil, err
	}
	if key.Fingerprint != in.Fingerprint {
		credential.Clear(pass)
		return key, nil, model.Fail("identity.key_mismatch", "The selected private key does not match the frozen public key.")
	}
	return key, pass, nil
}
func (s Service) runProfileTransition(ctx context.Context, db *store.Store, op store.OperationRecord, in ProfileTransition, r Request, key identity.Key, pass []byte) ([]model.MemberResult, error) {
	results := []model.MemberResult{}
	if in.LabelOnly {
		if err := db.SetDesiredDigest(ctx, op.Operation.ID, in.ProfileID, in.Desired.Label, in.DesiredDigest); err != nil {
			return results, err
		}
		return results, db.Finish(ctx, op, "completed")
	}
	var prepared, committed bool
	steps, err := db.Steps(ctx, op.Operation.ID)
	if err != nil {
		return results, err
	}
	for _, step := range steps {
		if step.Stage == "identity.intent" && step.EffectState == "satisfied" {
			prepared = true
		}
		if step.Stage == "identity.commit" && step.EffectState == "satisfied" {
			committed = true
		}
	}
	var phase string
	if err = db.DB.QueryRowContext(ctx, "SELECT COALESCE(stop_reason,'') FROM operations WHERE id=?", op.Operation.ID).Scan(&phase); err != nil {
		return results, err
	}
	prepared = prepared || phase == "identity_prepared" || phase == "identity_committed"
	committed = committed || phase == "identity_committed"
	if !prepared {
		err = db.Change(ctx, store.Change{Kind: "profile", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
			if err := checkTransitionOverlaps(ctx, tx, in); err != nil {
				return err
			}
			if in.ProfileID != "" {
				var memberGen int64
				var credential string
				if e := tx.QueryRowContext(ctx, "SELECT membership_generation,active_credential_revision_id FROM profiles WHERE id=?", in.ProfileID).Scan(&memberGen, &credential); e != nil {
					return e
				}
				if memberGen != in.MembershipGeneration || credential != in.OldCredentialID {
					return model.Fail("binding.conflict", "Profile membership or credential changed after preview.")
				}
			}
			if in.CredentialChange {
				var ordinal int
				if e := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(ordinal),0)+1 FROM profile_credential_revisions WHERE profile_id=?", in.ProfileID).Scan(&ordinal); e != nil {
					return e
				}
				var account any
				if in.Desired.GitHub.Username != "" {
					account = in.Desired.GitHub.Username
				}
				var ref, ownership any
				if in.Desired.Unlock.Remember {
					ref = store.JSON(map[string]string{"kind": "native", "item_reference": in.ProfileID})
					ownership = "owned"
				}
				if in.Migration || (in.StoreOwnership != nil && *in.StoreOwnership == "external") {
					ref, ownership = in.StoreRef, in.StoreOwnership
				}
				keyOwnership := "external"
				if in.Desired.Key.Path != "" && in.Migration {
					keyOwnership = in.KeyOwnership
				}
				_, e := tx.ExecContext(ctx, "INSERT INTO profile_credential_revisions VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", in.CredentialID, in.ProfileID, ordinal, files.Hash([]byte(in.PublicKey)), in.PublicKey, in.Fingerprint, store.JSON(store.KeyReference{Kind: "file", Path: in.Desired.Key.Path, PublicPath: in.Desired.Key.PublicPath}), keyOwnership, "owned", store.JSON(in.Agent), ref, ownership, "github", account, model.Now())
				if e != nil {
					return e
				}
			}
			revisions := map[string]model.GitIdentity{}
			if in.MetadataChange || in.CredentialChange {
				revisions[in.DefaultRevisionID] = in.Desired.Git
			}
			for _, t := range in.Targets {
				if t.RevisionID != nil && *t.RevisionID != in.DefaultRevisionID {
					revisions[*t.RevisionID] = t.Git
				}
			}
			for id, git := range revisions {
				var ordinal int
				if e := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(ordinal),0)+1 FROM profile_revisions WHERE profile_id=?", in.ProfileID).Scan(&ordinal); e != nil {
					return e
				}
				if _, e := tx.ExecContext(ctx, "INSERT INTO profile_revisions VALUES(?,?,?,?,?,?,?)", id, in.ProfileID, in.CredentialID, ordinal, store.JSON(git), files.Hash([]byte(store.JSON(git))), model.Now()); e != nil {
					return e
				}
			}
			for _, id := range in.ProfileIDs {
				if _, e := tx.ExecContext(ctx, "UPDATE profile_runtime SET transition_operation_id=? WHERE profile_id=?", op.Operation.ID, id); e != nil {
					return e
				}
			}
			for _, t := range in.Targets {
				var revision int64
				var pending *string
				if e := tx.QueryRowContext(ctx, "SELECT row_revision,pending_identity_kind FROM registrations WHERE id=?", t.VMID).Scan(&revision, &pending); e != nil {
					return e
				}
				if revision != t.RowRevision || pending != nil {
					return model.Fail("binding.conflict", "A selected VM changed after preview.")
				}
				kind := "none"
				if t.ProfileID != nil {
					kind = "profile"
				}
				if _, e := tx.ExecContext(ctx, "UPDATE registrations SET pending_identity_kind=?,pending_profile_id=?,pending_profile_revision_id=?,identity_transition_phase='preparing',identity_operation_id=?,row_revision=row_revision+1 WHERE id=?", kind, t.ProfileID, t.RevisionID, op.Operation.ID, t.VMID); e != nil {
					return e
				}
			}
			_, e := tx.ExecContext(ctx, "UPDATE operations SET stop_reason='identity_prepared' WHERE id=?", op.Operation.ID)
			return e
		})
		if err != nil {
			return results, err
		}
		if err = db.RecordStep(ctx, op, "identity.intent", "exited", "satisfied", "identity.fenced", map[string]any{}); err != nil {
			return results, err
		}
	}
	if !committed {
		if err = s.revokeMembers(ctx, db, op, op.Plan.AffectedVMIDs); err != nil {
			return results, err
		}
		// Preflight each running guest's owned files before changing the shared key.
		for _, t := range in.Targets {
			v, e := db.VM(ctx, t.VMID)
			if e != nil {
				return results, e
			}
			u, e := s.user(r)
			if e != nil {
				return results, e
			}
			n, e := db.Namespace(ctx)
			if e != nil {
				return results, e
			}
			a, e := s.adapter(u.Value, n, r, op.Operation.ID)
			if e != nil {
				return results, e
			}
			obs, e := a.Observe(ctx, v.VM.InstanceName)
			if e != nil {
				return results, e
			}
			if obs.Status == "Stopped" {
				continue
			}
			if obs.Status != "Running" {
				return results, model.Fail("operation.unknown", "A selected VM's state is unknown; credential cutover remains fenced.")
			}
			c, e := s.pinned(ctx, db, v)
			if e != nil {
				return results, e
			}
			e = identity.Apply(ctx, c, identity.Bundle{Action: "check", Files: map[string]string{}})
			c.Close()
			if e != nil {
				return results, e
			}
		}
		if in.Migration {
			u, e := s.user(r)
			if e != nil {
				return results, e
			}
			n, e := db.Namespace(ctx)
			if e != nil {
				return results, e
			}
			a, e := s.adapter(u.Value, n, r, op.Operation.ID)
			if e != nil {
				return results, e
			}
			for _, t := range in.Targets {
				v, e := db.VM(ctx, t.VMID)
				if e != nil {
					return results, e
				}
				if e = s.vmEffect(ctx, db, op, v, a, "stop", ""); e != nil {
					return results, e
				}
				if _, e = a.ObserveStopped(ctx, v.VM.InstanceName, ""); e != nil {
					return results, e
				}
			}
		}
		binding := in.Agent
		if in.CredentialChange {
			if key.Private == nil {
				p, e := db.Profile(ctx, in.ProfileID)
				if e != nil {
					return results, e
				}
				key, pass, e = s.transitionKey(ctx, p, in, r)
				if e != nil {
					return results, e
				}
				defer credential.Clear(pass)
			}
			if err = db.RecordStep(ctx, op, "identity.agent", "may_have_dispatched", "unknown", "identity.switching", map[string]string{"fingerprint": in.Fingerprint}); err != nil {
				return results, err
			}
			binding, err = agent.Ensure(ctx, in.ProfileID, in.Agent)
			if err != nil {
				return results, err
			}
			c, e := agent.Connect(ctx, binding.SocketPath)
			if e != nil {
				return results, e
			}
			keys, e := c.Agent.List()
			if e != nil {
				c.Close()
				return results, e
			}
			for _, k := range keys {
				pub, e := ssh.ParsePublicKey(k.Blob)
				if e != nil {
					c.Close()
					return results, e
				}
				fp := ssh.FingerprintSHA256(pub)
				old, e := db.Profile(ctx, in.ProfileID)
				if e != nil {
					c.Close()
					return results, e
				}
				if fp != in.Fingerprint && fp != *old.Profile.Fingerprint {
					c.Close()
					return results, model.Fail("identity.key_mismatch", "An unexpected agent key blocks credential cutover.")
				}
			}
			if e = c.Agent.RemoveAll(); e == nil {
				e = c.Agent.Add(sshagent.AddedKey{PrivateKey: key.Private, Comment: "easy-lima:" + in.ProfileID})
			}
			if e == nil {
				e = c.Verify(in.Fingerprint)
			}
			c.Close()
			if e != nil {
				return results, e
			}
			if in.Desired.Unlock.Remember && !in.Migration && (in.StoreOwnership == nil || *in.StoreOwnership != "external") {
				if len(pass) == 0 {
					return results, model.Fail("profile.unlock_required", "Remembering requires the selected encryption passphrase.")
				}
				if e = s.Credential.Put(ctx, in.ProfileID, pass, r.Interactive); e != nil {
					return results, e
				}
			}
			if err = db.RecordStep(ctx, op, "identity.agent", "exited", "satisfied", "identity.ready", map[string]any{"binding": binding, "fingerprint": in.Fingerprint}); err != nil {
				return results, err
			}
		}
		err = db.Change(ctx, store.Change{Kind: "profile", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
			if err := checkTransitionOverlaps(ctx, tx, in); err != nil {
				return err
			}
			if in.CredentialChange {
				if binding != in.Agent {
					return model.Fail("ownership.conflict", "The target agent binding must be frozen before inserting its immutable credential revision.")
				}
				if _, e := tx.ExecContext(ctx, "UPDATE profiles SET active_credential_revision_id=? WHERE id=?", in.CredentialID, in.ProfileID); e != nil {
					return e
				}
				if _, e := tx.ExecContext(ctx, "UPDATE profile_runtime SET credential_revision_id=?,agent_management='owned',lock_state='unlocked',remember_enabled=?,generation=generation+1,row_revision=row_revision+1 WHERE profile_id=?", in.CredentialID, in.Desired.Unlock.Remember, in.ProfileID); e != nil {
					return e
				}
			}
			if in.ProfileID != "" && (op.Operation.Kind == "profile.apply" || in.Migration) {
				if _, e := tx.ExecContext(ctx, "UPDATE profiles SET default_identity_revision_id=?,label=?,desired_digest=?,row_revision=row_revision+1 WHERE id=?", in.DefaultRevisionID, in.Desired.Label, in.DesiredDigest, in.ProfileID); e != nil {
					return e
				}
			}
			for _, t := range in.Targets {
				plan := model.NewPlan("identity.complete", model.Scope{Kind: "vm", ID: &t.VMID})
				now := model.Now()
				if _, e := tx.ExecContext(ctx, "INSERT INTO operations(id,kind,scope_kind,scope_id,parent_operation_id,token,status,stop_reason,inputs,plan,created_at,updated_at) VALUES(?,'identity.complete','vm',?,?,?,'prepared','deferred_identity',?,?,?,?)", t.ContinuationID, t.VMID, op.Operation.ID, model.NewID(), store.JSON(t), store.JSON(plan), now, now); e != nil {
					return e
				}
				if _, e := tx.ExecContext(ctx, "UPDATE registrations SET profile_id=?,profile_binding_generation=profile_binding_generation+1,identity_transition_phase='committed',identity_operation_id=?,row_revision=row_revision+1 WHERE id=? AND identity_operation_id=?", t.ProfileID, t.ContinuationID, t.VMID, op.Operation.ID); e != nil {
					return e
				}
			}
			for _, id := range in.ProfileIDs {
				if _, e := tx.ExecContext(ctx, "UPDATE profiles SET membership_generation=membership_generation+1 WHERE id=?", id); e != nil {
					return e
				}
				if _, e := tx.ExecContext(ctx, "UPDATE profile_runtime SET transition_operation_id=NULL WHERE profile_id=? AND transition_operation_id=?", id, op.Operation.ID); e != nil {
					return e
				}
			}
			_, e := tx.ExecContext(ctx, "UPDATE operations SET stop_reason='identity_committed' WHERE id=?", op.Operation.ID)
			return e
		})
		if err != nil {
			return results, err
		}
		if err = db.RecordStep(ctx, op, "identity.commit", "exited", "satisfied", "identity.committed", map[string]any{}); err != nil {
			return results, err
		}
	}
	partial := false
	for _, t := range in.Targets {
		v, e := db.VM(ctx, t.VMID)
		if e != nil {
			return results, e
		}

		result := model.MemberResult{VMID: t.VMID, Status: "ready", OperationID: &t.ContinuationID}
		if v.PendingIdentityKind != nil {
			u, e := s.user(r)
			if e != nil {
				return results, e
			}
			n, e := db.Namespace(ctx)
			if e != nil {
				return results, e
			}
			a, e := s.adapter(u.Value, n, r, op.Operation.ID)
			if e != nil {
				return results, e
			}
			obs, e := a.Observe(ctx, v.VM.InstanceName)
			if e != nil {
				return results, e
			}
			if obs.Status == "Stopped" {
				result.Status = "pending"
				partial = true
			} else if obs.Status == "Running" {
				c, e := s.pinned(ctx, db, v)
				if e != nil {
					return results, e
				}
				e = s.completeGuestIdentity(ctx, db, op, v, c, r)
				if e != nil && (knownConnectionConflict(e) || model.AsError(e).Code == "profile.external_managed") {
					result.Status = "pending"
					partial = true
					e = nil
				}
				c.Close()
				if e != nil {
					return results, e
				}
			} else {
				return results, model.Fail("operation.unknown", "A member VM became unobservable during identity completion.")
			}
		}
		results = append(results, result)
	}
	if err = db.Finish(ctx, op, "completed"); err != nil {
		return results, err
	}
	if partial {
		return results, model.Fail("operation.partial", "The profile target is committed. Stopped VMs retain pending identity work for their next explicit start or open.")
	}
	return results, nil
}

var _ = connection.GuestSocket

func checkTransitionOverlaps(ctx context.Context, tx *sql.Tx, in ProfileTransition) error {
	targets := make(map[string]*string, len(in.Targets))
	for _, target := range in.Targets {
		targets[target.VMID] = target.ProfileID
	}
	for _, target := range in.Targets {
		if target.ProfileID != nil {
			if err := store.RequireActiveProfile(ctx, tx, *target.ProfileID); err != nil {
				return err
			}
		}
		var source string
		if err := tx.QueryRowContext(ctx, "SELECT source_path FROM registrations WHERE id=? AND retired_at IS NULL", target.VMID).Scan(&source); err != nil {
			return err
		}
		if err := store.CheckProfileOverlapIn(ctx, tx, source, target.ProfileID, target.VMID, targets); err != nil {
			return err
		}
	}
	return nil
}

// Apply only fields changed from the previous profile default to each VM snapshot.
func mergeIdentity(before, after, member model.GitIdentity) model.GitIdentity {
	if before.Name != after.Name {
		member.Name = after.Name
	}
	if before.Email != after.Email {
		member.Email = after.Email
	}
	if before.SignCommits != after.SignCommits {
		member.SignCommits = after.SignCommits
	}
	if before.SignTags != after.SignTags {
		member.SignTags = after.SignTags
	}
	return member
}

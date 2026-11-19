package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/credential"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/identity"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

func (s Service) reloadProfile(ctx context.Context, db *store.Store, p store.ProfileRecord) (*store.ProfileRecord, error) {
	v, err := db.Profile(ctx, p.Profile.ID)
	return &v, err
}
func (s Service) activateProfile(ctx context.Context, db *store.Store, p store.ProfileRecord, r Request, explicit bool) error {
	if p.TransitionOperationID != nil {
		return model.Fail("operation.busy", "The profile has an unfinished identity transition.")
	}
	if p.Profile.AgentManagement == "external" {
		c, err := agent.Connect(ctx, p.Agent.SocketPath)
		if err != nil {
			return externalAgentUnavailable(p, err)
		}
		defer c.Close()
		return externalAgentUnavailable(p, c.SignProbe(*p.Profile.Fingerprint))
	}
	if p.Profile.LockState == "locked" && !explicit {
		if !r.Interactive {
			return model.Fail("profile.explicitly_locked", "This profile was explicitly locked. Use profile unlock; --yes does not unlock it.")
		}
		plan := model.NewPlan("profile.unlock", model.Scope{Kind: "profile", ID: &p.Profile.ID})
		plan.Add("profile", "Unlock the explicitly locked profile for its VMs.")
		if s.Confirm == nil {
			return model.Fail("profile.explicitly_locked", "Explicit unlock consent is required.")
		}
		yes, err := s.Confirm(plan)
		if err != nil {
			return err
		}
		if !yes {
			return model.Fail("profile.explicitly_locked", "The selected profile remains locked.")
		}
		explicit = true
	}
	if c, err := agent.Connect(ctx, p.Agent.SocketPath); err == nil {
		if ownershipErr := agent.VerifyOwnership(ctx, p.Profile.ID, p.Agent); ownershipErr != nil {
			c.Close()
			return ownershipErr
		}
		verify := c.SignProbe(*p.Profile.Fingerprint)
		c.Close()
		if verify == nil && p.Profile.LockState != "locked" {
			return nil
		}
		if verify != nil && model.AsError(verify).Code == "identity.key_mismatch" {
			return verify
		}
	}
	plan := model.NewPlan("profile.unlock", model.Scope{Kind: "profile", ID: &p.Profile.ID})
	plan.Add("profile", "Activate this profile's existing key in its independent SSH agent.")
	op, err := db.Prepare(ctx, plan, map[string]string{"profile_id": p.Profile.ID, "credential_revision_id": p.CredentialID}, []store.Claim{{Kind: "profile", Key: p.Profile.ID, Mode: "exclusive"}}, nil)
	if err != nil {
		return err
	}
	guard, err := files.Lock(ctx, filepath.Join(db.Dir, "operations", op.Operation.ID+".lock"), true, true)
	if err != nil {
		return err
	}
	defer guard.Close()
	return s.completeActivation(ctx, db, op, p, r, explicit)
}

func (s Service) completeActivation(ctx context.Context, db *store.Store, op store.OperationRecord, p store.ProfileRecord, r Request, explicit bool) error {
	if err := s.loadProfileKey(ctx, db, op, p, r, explicit); err != nil {
		return err
	}
	return db.Finish(ctx, op, "completed")
}

// Loading the same committed key is safe under a shared profile claim. VM
// recovery must not request a second exclusive claim against its own retained
// claim, nor complete the enclosing VM operation as an agent activation.
func (s Service) loadProfileKey(ctx context.Context, db *store.Store, op store.OperationRecord, p store.ProfileRecord, r Request, explicit bool) error {
	key, err := identity.ParsePrivate(p.Key.Path, nil)
	var pass []byte
	if err != nil && model.AsError(err).Code == "profile.unlock_required" {
		if p.Profile.RememberEnabled && s.Credential != nil {
			pass, err = s.remembered(ctx, p, r.Interactive)
		}
		if len(pass) == 0 {
			pass, err = s.secret(r, "Passphrase for "+p.Profile.Label)
		}
		if err == nil {
			key, err = identity.ParsePrivate(p.Key.Path, pass)
		}
	}
	defer credential.Clear(pass)
	if err != nil {
		if op.Operation.Kind == "profile.unlock" {
			_ = db.Finish(ctx, op, "failed_known")
		}
		return err
	}
	if key.Fingerprint != *p.Profile.Fingerprint {
		if op.Operation.Kind == "profile.unlock" {
			_ = db.Finish(ctx, op, "failed_known")
		}
		return model.Fail("identity.key_mismatch", "The profile key file no longer matches its committed fingerprint.")
	}
	if err = db.RecordStep(ctx, op, "profile.agent", "may_have_dispatched", "unknown", "profile.activating", map[string]string{"profile_id": p.Profile.ID}); err != nil {
		return err
	}
	binding, err := agent.Ensure(ctx, p.Profile.ID, p.Agent)
	if err != nil {
		return err
	}
	if binding != p.Agent {
		return model.Fail("ownership.conflict", "The agent's supervisor binding changed; an explicit migration is required.")
	}
	client, err := agent.Connect(ctx, binding.SocketPath)
	if err != nil {
		return err
	}
	defer client.Close()
	keys, err := client.Agent.List()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		if err = client.Verify(key.Fingerprint); err != nil {
			return err
		}
	}
	if len(keys) == 0 {
		err = client.Agent.Add(sshagent.AddedKey{PrivateKey: key.Private, Comment: "easy-lima:" + p.Profile.ID})
	}
	if err != nil {
		return model.Fail("credential.denied", "The profile agent refused its selected key.")
	}
	if err = client.Verify(key.Fingerprint); err != nil {
		return err
	}
	if explicit || p.Profile.LockState == "locked" {
		if err = db.SetProfileRuntime(ctx, op.Operation.ID, p.Profile.ID, "unlocked", nil); err != nil {
			return err
		}
	}
	if err = db.RecordStep(ctx, op, "profile.agent", "exited", "satisfied", "profile.ready", map[string]string{"fingerprint": key.Fingerprint}); err != nil {
		return err
	}
	return nil
}

func (s Service) ensureProfileForConnection(ctx context.Context, db *store.Store, op store.OperationRecord, p store.ProfileRecord, r Request) error {
	if p.TransitionOperationID != nil {
		return model.Fail("operation.busy", "The profile has an unfinished identity transition.")
	}
	if p.Profile.LockState == "locked" {
		return model.Fail("profile.explicitly_locked", "The selected profile remains explicitly locked.")
	}
	if c, err := agent.Connect(ctx, p.Agent.SocketPath); err == nil {
		defer c.Close()
		if p.Profile.AgentManagement == "owned" {
			if err = agent.VerifyOwnership(ctx, p.Profile.ID, p.Agent); err != nil {
				return err
			}
		}
		err = c.SignProbe(*p.Profile.Fingerprint)
		if err == nil || model.AsError(err).Code == "identity.key_mismatch" {
			return err
		}
	}
	if p.Profile.AgentManagement != "owned" {
		return externalAgentUnavailable(p, model.Fail("profile.unlock_required", "External agent unavailable."))
	}
	return s.loadProfileKey(ctx, db, op, p, r, false)
}
func (s Service) profileMutation(ctx context.Context, db *store.Store, r Request) (model.MutationData, error) {
	if r.Command == "profile.create" || r.Command == "profile.import" {
		return s.createProfile(ctx, db, r)
	}
	if r.Command == "profile.assign" || r.Command == "profile.unassign" || r.Command == "profile.apply" || r.Command == "profile.migrate-agent" {
		return s.profileTransition(ctx, db, r)
	}
	p, err := db.Profile(ctx, r.Label)
	if err != nil {
		return model.MutationData{}, err
	}
	plan := model.NewPlan(r.Command, model.Scope{Kind: "profile", ID: &p.Profile.ID})
	data := model.Mutation(plan)
	data.Profile = &p.Profile
	plan.AffectedVMIDs = p.Profile.MemberVMIDs
	if r.Command == "profile.edit" {
		return s.editProfile(ctx, db, r, p)
	}
	if p.Profile.AgentManagement == "external" && r.Command != "profile.remove" {
		return data, model.Fail("profile.external_managed", "This agent or saved credential is managed externally. Use the separate same-key migration workflow first.")
	}
	switch r.Command {
	case "profile.unlock":
		plan.Add("profile", "Unlock this profile's owned agent using its existing key.")
	case "profile.lock":
		plan.Add("profile", "Revoke profile forwarding, unload its owned agent, and inhibit automatic reload.")
	case "profile.forget":
		if p.StoreOwnership != nil && *p.StoreOwnership != "owned" {
			return data, model.Fail("profile.external_managed", "The saved credential is owned externally and cannot be deleted.")
		}
		plan.Add("profile", "Disable remembering and remove only the easy-lima-owned saved passphrase; keep the key file and loaded key.")
	case "profile.remove":
		if len(p.Profile.MemberVMIDs) > 0 || p.TransitionOperationID != nil {
			return data, model.Fail("binding.conflict", "A bound or transitioning profile cannot be removed.")
		}
		plan.Add("profile", "Retire this unused profile while preserving imported keys and historical identity records.")
	default:
		return data, model.Fail("input.invalid", "Unknown profile action.")
	}
	data.Plan = plan
	if r.DryRun {
		return data, nil
	}
	if err = s.approve(r, plan); err != nil {
		return data, err
	}
	if r.Command == "profile.unlock" {
		err = s.activateProfile(ctx, db, p, r, true)
		if err == nil {
			fresh, _ := db.Profile(ctx, p.Profile.ID)
			data.Profile = &fresh.Profile
		}
		return data, err
	}
	input := ProfileFileInput{ProfileID: p.Profile.ID}
	if r.Command == "profile.forget" {
		f, e := config.LoadProfile(filepath.Join(s.Paths.Config, "profiles", p.Profile.ID+".toml"))
		if e != nil {
			return data, e
		}
		input.OldDigest = f.Digest
		input.Desired = f.Value
		input.Desired.Unlock.Remember = false
	}
	op, err := db.Prepare(ctx, plan, input, []store.Claim{{Kind: "profile", Key: p.Profile.ID, Mode: "exclusive"}}, nil)
	if err != nil {
		return data, err
	}
	data.Operation = &op.Operation
	guard, err := files.Lock(ctx, filepath.Join(db.Dir, "operations", op.Operation.ID+".lock"), true, true)
	if err != nil {
		return data, err
	}
	defer guard.Close()
	if r.Command == "profile.lock" {
		if err = db.SetProfileRuntime(ctx, op.Operation.ID, p.Profile.ID, "locked", nil); err != nil {
			return data, err
		}
		if err = s.revokeMembers(ctx, db, op, p.Profile.MemberVMIDs); err != nil {
			return data, err
		}
		if err = agent.UnloadOwned(ctx, p.Profile.ID, p.Agent, *p.Profile.Fingerprint); err != nil {
			return data, err
		}
	}
	if r.Command == "profile.forget" {
		if err = s.writeDesiredProfile(ctx, db, op, input); err != nil {
			return data, err
		}
		if s.Credential == nil {
			return data, model.Fail("credential.interaction_required", "The host credential store is unavailable.")
		}
		if err = s.Credential.Delete(ctx, p.Profile.ID, r.Interactive); err != nil {
			return data, err
		}
		remember := false
		if err = db.SetProfileRuntime(ctx, op.Operation.ID, p.Profile.ID, "", &remember); err != nil {
			return data, err
		}
	}
	if r.Command == "profile.remove" {
		if err = store.RequireUnusedProfile(ctx, db.DB, p.Profile.ID); err != nil {
			return data, err
		}
		if p.Profile.AgentManagement == "owned" {
			if err = agent.Remove(ctx, p.Profile.ID, p.Agent); err != nil {
				return data, err
			}
		}
		if p.Profile.RememberEnabled && p.StoreOwnership != nil && *p.StoreOwnership == "owned" {
			if err = s.Credential.Delete(ctx, p.Profile.ID, r.Interactive); err != nil {
				return data, err
			}
		}
		err = db.Change(ctx, store.Change{Kind: "profile", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
			if err := store.RequireUnusedProfile(ctx, tx, p.Profile.ID); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, "UPDATE profiles SET retired_at=?,row_revision=row_revision+1 WHERE id=?", model.Now(), p.Profile.ID)
			return err
		})
		if err != nil {
			return data, err
		}
	}
	if err = db.Finish(ctx, op, "completed"); err != nil {
		return data, err
	}
	if fresh, e := db.Profile(ctx, p.Profile.ID); e == nil {
		data.Profile = &fresh.Profile
	}
	return s.operationResult(ctx, db, data, op), nil
}

type ProfileCreation struct {
	Profile      config.Profile `json:"profile"`
	PublicKey    string         `json:"public_key"`
	Fingerprint  string         `json:"fingerprint"`
	Agent        agent.Binding  `json:"agent"`
	KeyOwnership string         `json:"key_ownership"`
}

func (s Service) createProfile(ctx context.Context, db *store.Store, r Request) (model.MutationData, error) {
	p := config.Profile{SchemaVersion: 1, ID: model.NewID(), Label: r.Label, Git: model.GitIdentity{Name: r.GitName, Email: r.GitEmail, SignCommits: true, SignTags: true}}
	p.GitHub.Username = r.GitHubUser
	if r.SignCommits != nil {
		p.Git.SignCommits = *r.SignCommits
	}
	if r.SignTags != nil {
		p.Git.SignTags = *r.SignTags
	}
	if r.Remember != nil {
		p.Unlock.Remember = *r.Remember
	}
	p.Key.Path = filepath.Join(s.Paths.Config, "keys", p.ID)
	p.Key.PublicPath = p.Key.Path + ".pub"
	var err error
	if r.Command == "profile.import" {
		p.Key.Path, err = config.ResolvePath(r.Key, s.CWD)
		if err != nil {
			return model.MutationData{}, err
		}
		p.Key.PublicPath = p.Key.Path + ".pub"
		if r.PublicKey != "" {
			p.Key.PublicPath, err = config.ResolvePath(r.PublicKey, s.CWD)
			if err != nil {
				return model.MutationData{}, err
			}
		}
	}
	if err = config.ValidateProfile(p); err != nil {
		return model.MutationData{}, err
	}
	if _, err = db.Profile(ctx, p.Label); err == nil {
		return model.MutationData{}, model.Fail("binding.conflict", "A live profile already uses that label.")
	}
	plan := model.NewPlan(r.Command, model.Scope{Kind: "profile", ID: &p.ID})
	plan.Add("profile", "Prepare one independent SSH identity with explicit Git name/email and signing defaults.")
	if r.Command == "profile.create" {
		plan.Add("file_publication", "Generate one encrypted Ed25519 key for reuse by this profile's VMs.")
	}
	if p.Unlock.Remember {
		plan.Add("profile", "Remember the passphrase in the host credential store for this profile only.")
	}
	plan.Fields = []model.Field{{Path: "git.name", Value: p.Git.Name, Origin: "flag"}, {Path: "git.email", Value: p.Git.Email, Origin: "flag"}, {Path: "github.username", Value: p.GitHub.Username, Origin: "flag"}, {Path: "unlock.remember", Value: p.Unlock.Remember, Origin: "flag"}}
	data := model.Mutation(plan)
	if r.DryRun {
		return data, nil
	}
	if err = s.approve(r, plan); err != nil {
		return data, err
	}
	var pass []byte
	var key identity.Key
	var encoded []byte
	if r.Command == "profile.create" {
		pass, err = s.secret(r, "New encrypted profile key passphrase")
		if err != nil {
			return data, err
		}
		if len(pass) == 0 {
			return data, model.Fail("secret.input_invalid", "Generated keys require a nonempty passphrase.")
		}
		key, encoded, err = identity.GenerateMaterial(pass)
		if err != nil {
			return data, err
		}
		defer credential.Clear(encoded)
	} else {
		key, err = identity.ParsePrivate(p.Key.Path, nil)
		if err != nil && model.AsError(err).Code == "profile.unlock_required" {
			pass, err = s.secret(r, "Passphrase for imported profile key")
			if err == nil {
				key, err = identity.ParsePrivate(p.Key.Path, pass)
			}
		}
		if err != nil {
			return data, err
		}
	}
	defer credential.Clear(pass)
	if r.Command == "profile.import" {
		if pub, e := identity.ReadPublic(p.Key.PublicPath); e == nil {
			if ssh.FingerprintSHA256(pub) != key.Fingerprint {
				return data, model.Fail("identity.key_mismatch", "The imported public and private keys do not match.")
			}
		} else if os.IsNotExist(e) {
			p.Key.PublicPath = filepath.Join(s.Paths.Config, "keys", p.ID+".pub")
		} else {
			return data, e
		}
	}
	p.Key.Fingerprint = key.Fingerprint
	if p.Unlock.Remember {
		if len(pass) == 0 {
			return data, model.Fail("secret.input_invalid", "An unencrypted key does not have a passphrase to remember.")
		}
		p.Unlock.CredentialRef = p.ID
	}
	socket, err := agent.Socket(s.Paths.Cache, p.ID)
	if err != nil {
		return data, err
	}
	binding, err := agent.Plan(ctx, p.ID, agent.Binding{SocketPath: socket})
	if err != nil {
		return data, err
	}
	ownership := "external"
	if r.Command == "profile.create" {
		ownership = "owned"
	}
	input := ProfileCreation{Profile: p, PublicKey: key.PublicText, Fingerprint: key.Fingerprint, Agent: binding, KeyOwnership: ownership}
	op, err := db.Prepare(ctx, plan, input, []store.Claim{{Kind: "profile", Key: p.ID, Mode: "exclusive"}, {Kind: "profile-label", Key: p.Label, Mode: "exclusive"}}, nil)
	if err != nil {
		return data, err
	}
	data.Operation = &op.Operation
	guard, err := files.Lock(ctx, filepath.Join(db.Dir, "operations", op.Operation.ID+".lock"), true, true)
	if err != nil {
		return data, err
	}
	defer guard.Close()
	if err = db.RecordStep(ctx, op, "profile.prepare", "may_have_dispatched", "unknown", "profile.preparing", map[string]string{"profile_id": p.ID}); err != nil {
		return data, err
	}
	if err = files.PrivateDir(filepath.Join(s.Paths.Config, "keys")); err != nil {
		return data, err
	}
	if r.Command == "profile.create" {
		err = files.WriteNew(p.Key.Path, encoded)
		if err != nil {
			return data, err
		}
		if err = files.WriteNew(p.Key.PublicPath, []byte(key.PublicText+"\n")); err != nil {
			return data, err
		}
	} else {
		if pub, e := identity.ReadPublic(p.Key.PublicPath); e == nil {
			if ssh.FingerprintSHA256(pub) != key.Fingerprint {
				return data, model.Fail("identity.key_mismatch", "The imported public and private keys do not match.")
			}
		} else if !os.IsNotExist(e) {
			return data, e
		} else {
			p.Key.PublicPath = filepath.Join(s.Paths.Config, "keys", p.ID+".pub")
			if err = files.WriteNew(p.Key.PublicPath, []byte(key.PublicText+"\n")); err != nil {
				return data, err
			}
		}
	}
	p.Key.Fingerprint = key.Fingerprint
	socket, err = agent.Socket(s.Paths.Cache, p.ID)
	if err != nil {
		return data, err
	}
	binding, err = agent.Ensure(ctx, p.ID, input.Agent)
	if err != nil {
		return data, err
	}
	client, err := agent.Connect(ctx, socket)
	if err != nil {
		return data, err
	}
	defer client.Close()
	if keys, e := client.Agent.List(); e != nil || len(keys) != 0 {
		return data, model.Fail("identity.key_mismatch", "A newly owned agent unexpectedly contains a key.")
	}
	if err = client.Agent.Add(sshagent.AddedKey{PrivateKey: key.Private, Comment: "easy-lima:" + p.ID}); err != nil {
		return data, model.Fail("credential.denied", "The owned agent refused the key.")
	}
	if err = client.Verify(key.Fingerprint); err != nil {
		return data, err
	}
	if p.Unlock.Remember {
		if len(pass) == 0 {
			return data, model.Fail("secret.input_invalid", "This imported key is unencrypted; remembering a passphrase is unnecessary.")
		}
		if err = s.Credential.Put(ctx, p.ID, pass, r.Interactive); err != nil {
			return data, err
		}
		p.Unlock.CredentialRef = p.ID
	}
	if err = files.PrivateDir(filepath.Join(s.Paths.Config, "profiles")); err != nil {
		return data, err
	}
	b, err := config.Encode(p)
	if err != nil {
		return data, err
	}
	if err = files.WriteNew(filepath.Join(s.Paths.Config, "profiles", p.ID+".toml"), b); err != nil {
		return data, err
	}
	if err = db.SaveProfile(ctx, op, p, key.PublicText, key.Fingerprint, files.Hash(b), ownership, binding); err != nil {
		return data, err
	}
	if err = db.RecordStep(ctx, op, "profile.prepare", "exited", "satisfied", "profile.ready", map[string]string{"fingerprint": key.Fingerprint}); err != nil {
		return data, err
	}
	if err = db.Finish(ctx, op, "completed"); err != nil {
		return data, err
	}
	pr, err := db.Profile(ctx, p.ID)
	if err == nil {
		data.Profile = &pr.Profile
	}
	return s.operationResult(ctx, db, data, op), err
}
func (s Service) editProfile(ctx context.Context, db *store.Store, r Request, p store.ProfileRecord) (model.MutationData, error) {
	path := filepath.Join(s.Paths.Config, "profiles", p.Profile.ID+".toml")
	f, err := config.LoadProfile(path)
	if err != nil {
		return model.MutationData{}, err
	}
	v := f.Value
	changed := false
	if r.GitName != "" {
		v.Git.Name = r.GitName
		changed = true
	}
	if r.GitEmail != "" {
		v.Git.Email = r.GitEmail
		changed = true
	}
	if r.GitHubUserSet || r.GitHubUser != "" {
		v.GitHub.Username = r.GitHubUser
		changed = true
	}
	if r.NewLabel != "" {
		v.Label = r.NewLabel
		changed = true
	}
	if r.SignCommits != nil {
		v.Git.SignCommits = *r.SignCommits
		changed = true
	}
	if r.SignTags != nil {
		v.Git.SignTags = *r.SignTags
		changed = true
	}
	if r.Remember != nil {
		v.Unlock.Remember = *r.Remember
		changed = true
	}
	if r.Key != "" {
		v.Key.Path, err = config.ResolvePath(r.Key, s.CWD)
		if err != nil {
			return model.MutationData{}, err
		}
		v.Key.PublicPath = v.Key.Path + ".pub"
		v.Key.Fingerprint = ""
		changed = true
	}
	if r.PublicKey != "" {
		v.Key.PublicPath, err = config.ResolvePath(r.PublicKey, s.CWD)
		if err != nil {
			return model.MutationData{}, err
		}
		changed = true
	}
	plan := model.NewPlan(r.Command, model.Scope{Kind: "profile", ID: &p.Profile.ID})
	data := model.Mutation(plan)
	data.Profile = &p.Profile
	if !changed {
		return data, model.Fail("input.invalid", "Supply at least one desired profile field to edit.")
	}
	if err = config.ValidateProfile(v); err != nil {
		return data, err
	}
	plan.Add("metadata", "Write desired profile intent; active agents and VMs change only after profile apply.")
	data.Plan = plan
	if r.DryRun {
		return data, nil
	}
	if err = s.approve(r, plan); err != nil {
		return data, err
	}
	op, err := db.Prepare(ctx, plan, ProfileFileInput{ProfileID: p.Profile.ID, OldDigest: f.Digest, Desired: v}, []store.Claim{{Kind: "profile", Key: p.Profile.ID, Mode: "exclusive"}}, nil)
	if err != nil {
		return data, err
	}
	data.Operation = &op.Operation
	guard, err := operationGuard(ctx, db, op.Operation.ID)
	if err != nil {
		return data, err
	}
	defer guard.Close()
	if err = s.writeDesiredProfile(ctx, db, op, ProfileFileInput{ProfileID: p.Profile.ID, OldDigest: f.Digest, Desired: v}); err != nil {
		return data, err
	}

	err = db.Finish(ctx, op, "completed")
	return s.operationResult(ctx, db, data, op), err
}

var _ = strings.TrimSpace

func (s Service) remembered(ctx context.Context, p store.ProfileRecord, interactive bool) ([]byte, error) {
	if s.Credential == nil {
		return nil, model.Fail("profile.unlock_required", "The host credential store is unavailable.")
	}
	if p.StoreOwnership != nil && *p.StoreOwnership == "external" {
		if p.StoreRef == nil {
			return nil, model.Fail("record.invalid", "The external credential reference is missing.")
		}
		ref, err := store.Decode[credential.Reference](*p.StoreRef)
		if err != nil {
			return nil, err
		}
		external, ok := s.Credential.(credential.ExternalReader)
		if !ok {
			return nil, model.Fail("credential.interaction_required", "The original credential store is unavailable on this host. Supply the existing key passphrase explicitly.")
		}
		return external.GetExternal(ctx, ref, interactive)
	}
	return s.Credential.Get(ctx, p.Profile.ID, interactive)
}

// ProfileFileInput freezes public desired intent before a profile file write.
// The private key and its passphrase never enter this record.
type ProfileFileInput struct {
	ProfileID string         `json:"profile_id"`
	OldDigest string         `json:"old_digest,omitempty"`
	Desired   config.Profile `json:"desired"`
}

func (s Service) writeDesiredProfile(ctx context.Context, db *store.Store, op store.OperationRecord, in ProfileFileInput) error {
	if !model.ValidID(in.ProfileID) || in.Desired.ID != in.ProfileID {
		return model.Fail("record.invalid", "The saved desired profile is invalid.")
	}
	b, err := config.Encode(in.Desired)
	if err != nil {
		return err
	}
	path := filepath.Join(s.Paths.Config, "profiles", in.ProfileID+".toml")
	current, err := files.Digest(path)
	if err != nil {
		return err
	}
	target := files.Hash(b)
	if current != target {
		if current != in.OldDigest {
			return model.Fail("settings.edited", "The desired profile changed while this operation was pending.")
		}
		if err = files.Replace(path, b, in.OldDigest, 0600); err != nil {
			return err
		}
	}
	p, err := db.Profile(ctx, in.ProfileID)
	if err != nil {
		return err
	}
	return db.SetDesiredDigest(ctx, op.Operation.ID, in.ProfileID, p.Profile.Label, target)
}

func externalAgentUnavailable(p store.ProfileRecord, err error) error {
	if err == nil {
		return nil
	}
	if model.AsError(err).Code != "profile.unlock_required" {
		return err
	}
	return model.Fail("profile.external_managed", "The external agent is unavailable or empty. Restore its recorded supervisor "+p.Agent.SupervisorRef+" and socket "+p.Agent.SocketPath+", or preview: easy-lima profile migrate-agent "+p.Profile.Label)
}

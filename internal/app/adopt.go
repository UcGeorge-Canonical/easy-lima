package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/connection"
	"github.com/UcGeorge-Canonical/easy-lima/internal/environment"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/identity"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/project"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"golang.org/x/crypto/ssh"
)

const PrototypeSHA256 = "55999dc949a226ad73cf3bd0084fa6eb4164162e9f296445c14f6b01e6d0333f"

type legacyRecord struct {
	Name      string           `json:"name"`
	Relative  string           `json:"relative"`
	Source    string           `json:"source"`
	Resources config.Resources `json:"resources"`
}
type legacyConfig struct {
	Defaults map[string]any            `toml:"defaults"`
	Projects map[string]map[string]any `toml:"projects"`
	Storage  struct {
		LimaHome string `toml:"lima_home"`
	} `toml:"storage"`
	Git struct {
		Enabled    bool   `toml:"enabled"`
		Name       string `toml:"name"`
		Email      string `toml:"email"`
		GitHubUser string `toml:"github_user"`
		KeyPath    string `toml:"key_path"`
	} `toml:"git"`
}
type Handoff struct {
	SchemaVersion  int              `json:"schema_version"`
	HandoffID      string           `json:"handoff_id"`
	OperationID    string           `json:"operation_id"`
	RegistryID     string           `json:"registry_id"`
	NamespaceID    string           `json:"namespace_id"`
	RegistrationID string           `json:"registration_id"`
	PrototypeRoot  string           `json:"prototype_root"`
	ActiveRecord   string           `json:"legacy_active_record"`
	ArchiveRecord  string           `json:"legacy_archive_record"`
	OldRecordHash  string           `json:"old_record_sha256"`
	NativeHash     string           `json:"native_sha256"`
	PriorRunning   bool             `json:"prior_running"`
	RestoreRunning bool             `json:"restore_running"`
	Backups        []map[string]any `json:"backups"`
	Quiescence     *lima.Quiescence `json:"quiescence_evidence"`
}
type AdoptInput struct {
	Handoff            Handoff           `json:"handoff"`
	VM                 store.NewVM       `json:"vm"`
	Environment        environment.Input `json:"environment"`
	Profile            *config.Profile   `json:"profile"`
	ProfileAgent       agent.Binding     `json:"profile_agent"`
	PublicKey          string            `json:"public_key"`
	Fingerprint        string            `json:"fingerprint"`
	ProfileExisting    bool              `json:"profile_existing"`
	IdentityRevisionID string            `json:"identity_revision_id"`
	NativeArtifact     string            `json:"native_artifact"`
	AliasBefore        string            `json:"alias_before"`
	AliasAfter         string            `json:"alias_after"`
	AliasDigest        string            `json:"alias_digest"`
}

func legacyName(root, relative string) string {
	slug := regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(strings.ToLower(filepath.Base(relative)), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 20 {
		slug = slug[:20]
	}
	if slug == "" {
		slug = "project"
	}
	h := sha256.Sum256([]byte(root + "\x00" + relative))
	return "cl-" + slug + "-" + hex.EncodeToString(h[:])[:10]
}
func (s Service) adopt(ctx context.Context, db *store.Store, r Request) (model.MutationData, error) {
	root, err := project.Resolve(r.FromPrototype, s.CWD, true)
	if err != nil {
		return model.MutationData{}, err
	}
	source, err := s.selected(r, true)
	if err != nil {
		return model.MutationData{}, err
	}
	relative, err := filepath.Rel(root, source)
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") {
		return model.MutationData{}, model.Fail("input.invalid", "Adoption requires the exact project path below the selected prototype root.")
	}
	hash, err := files.Digest(filepath.Join(root, ".local-launcher", "local.py"))
	if err != nil {
		return model.MutationData{}, err
	}
	if hash != PrototypeSHA256 {
		return model.MutationData{}, model.Fail("dependency.unsupported", "This prototype source is not the pinned supported ownership contract.")
	}
	n, err := db.Namespace(ctx)
	if err != nil {
		return model.MutationData{}, err
	}
	u, err := s.user(r)
	if err != nil {
		return model.MutationData{}, err
	}
	if _, err = s.validateRoot(ctx, db, u.Value); err != nil {
		return model.MutationData{}, err
	}
	name := legacyName(root, relative)
	recordPath := filepath.Join(root, ".local-state", "projects", name+".json")
	recordBytes, err := files.Read(recordPath, 65536)
	if err != nil {
		return model.MutationData{}, err
	}
	var record legacyRecord
	decoder := json.NewDecoder(strings.NewReader(string(recordBytes)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&record); err != nil {
		return model.MutationData{}, model.Fail("record.invalid", "The legacy project record has unsupported fields or values.")
	}
	if record.Name != name || record.Relative != relative || !project.Same(record.Source, source) {
		return model.MutationData{}, model.Fail("ownership.conflict", "The legacy record does not match this exact project and instance.")
	}
	configBytes, err := files.Read(filepath.Join(root, "local-vms.toml"), 1<<20)
	if err != nil {
		return model.MutationData{}, err
	}
	var lc legacyConfig
	if _, err = toml.Decode(string(configBytes), &lc); err != nil {
		return model.MutationData{}, model.Fail("config.invalid", "The legacy configuration could not be decoded.")
	}
	legacyRoot, err := config.ResolvePath(lc.Storage.LimaHome, root)
	if err != nil {
		return model.MutationData{}, err
	}
	if !project.Same(legacyRoot, n.Root) {
		return model.MutationData{}, model.Fail("ownership.conflict", "Adopted VMs must already be in the selected storage root. Adoption never moves storage.")
	}
	a, err := s.adapter(u.Value, n, r, "adopt-plan")
	if err != nil {
		return model.MutationData{}, err
	}
	if _, err = a.Probe(ctx); err != nil {
		return model.MutationData{}, err
	}
	obs, err := a.Observe(ctx, name)
	if err != nil {
		return model.MutationData{}, err
	}
	if obs.Status != "Running" && obs.Status != "Stopped" {
		return model.MutationData{}, model.Fail("operation.unknown", "The legacy instance must be observably running or stopped.")
	}
	if err = a.Existing(name); err != nil {
		return model.MutationData{}, err
	}
	id := model.NewID()
	plan := model.NewPlan("adopt", model.VMScope(id, source))
	data := model.Mutation(plan)
	nativePath := filepath.Join(n.Root, name, "lima.yaml")
	native, err := files.Read(nativePath, 8<<20)
	if err != nil {
		return data, err
	}
	base, e := a.PrototypeImageBase(ctx, nativePath, native)
	if e != nil {
		return data, e
	}
	normalized, settings, err := lima.AdoptNative(native, source, base)
	if err != nil {
		return data, err
	}
	if err = s.safeMounts(ctx, db, source, settings.Mounts); err != nil {
		return data, err
	}
	globals, err := lima.ReadGlobals(n.Root, false)
	if err != nil {
		return data, err
	}
	reg, err := db.Registry(ctx)
	if err != nil {
		return data, err
	}
	handoffID := model.NewID()
	input := AdoptInput{Handoff: Handoff{SchemaVersion: 1, HandoffID: handoffID, RegistryID: reg.ID, NamespaceID: n.ID, RegistrationID: id, PrototypeRoot: root, ActiveRecord: recordPath, ArchiveRecord: filepath.Join(root, ".local-state", "retired", handoffID, name+".json"), OldRecordHash: files.Hash(recordBytes), NativeHash: files.Hash(native), PriorRunning: obs.Status == "Running", RestoreRunning: obs.Status == "Running", Backups: []map[string]any{}}, VM: store.NewVM{ID: id, NamespaceID: n.ID, InstanceName: name, Alias: "lima-" + name, SourcePath: source}, Environment: environment.Input{SchemaVersion: 1, RevisionID: model.NewID(), Settings: settings, InstanceDigest: files.Hash(normalized), YAMLDigest: files.Hash(normalized), RuntimePath: a.Executable, RuntimeVersion: lima.ContractVersion, Globals: globals, CatalogRevision: "adopted-unknown", Scripts: []environment.ScriptRef{}}, NativeArtifact: filepath.Join(db.Dir, "artifacts", files.Hash(native)+".yaml")}
	input.Environment.LegacySetup = legacySetup(root, relative, lc)
	input.Environment.YAMLArtifact = filepath.Join(db.Dir, "artifacts", input.Environment.YAMLDigest+".yaml")
	if lc.Git.Enabled {
		p := config.Profile{SchemaVersion: 1, ID: model.NewID()}
		p.Label = "imported-" + p.ID[:8]
		p.Git = model.GitIdentity{Name: lc.Git.Name, Email: lc.Git.Email, SignCommits: true, SignTags: true}
		p.GitHub.Username = lc.Git.GitHubUser
		p.Key.Path, err = config.ResolvePath(lc.Git.KeyPath, root)
		if err != nil {
			return data, err
		}
		p.Key.PublicPath = p.Key.Path + ".pub"
		public, err := identity.ReadPublic(p.Key.PublicPath)
		if err != nil {
			return data, err
		}
		p.Key.Fingerprint = ssh.FingerprintSHA256(public)
		p.Unlock.Remember = true
		p.Unlock.CredentialRef = "keychain:canonical-local-work-git:work-ssh-key"
		if err = config.ValidateProfile(p); err != nil {
			return data, err
		}
		input.PublicKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(public)))
		input.Fingerprint = p.Key.Fingerprint
		input.ProfileAgent = agent.Binding{SocketPath: filepath.Join(s.Paths.Home, ".ssh", "canonical-work-agent.sock"), SupervisorKind: "external", SupervisorRef: "com.canonical.local.work-git-agent", OwnershipEvidenceDigest: files.Hash([]byte(root + "\x00" + input.PublicKey))}
		rows, e := db.DB.QueryContext(ctx, "SELECT p.id FROM profiles p JOIN profile_credential_revisions c ON c.id=p.active_credential_revision_id WHERE p.retired_at IS NULL AND c.fingerprint=? AND c.agent_management='external'", input.Fingerprint)
		if e != nil {
			return data, e
		}
		candidates := []string{}
		for rows.Next() {
			var pid string
			if e = rows.Scan(&pid); e != nil {
				rows.Close()
				return data, e
			}
			candidates = append(candidates, pid)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return data, e
		}
		for _, pid := range candidates {
			existing, e := db.Profile(ctx, pid)
			if e != nil {
				return data, e
			}
			if existing.Agent.SocketPath == input.ProfileAgent.SocketPath && existing.Key.Path == p.Key.Path && existing.Profile.Git == p.Git && existing.Profile.GitHubUsername != nil && *existing.Profile.GitHubUsername == p.GitHub.Username {
				p.ID = pid
				p.Label = existing.Profile.Label
				input.ProfileExisting = true
				input.IdentityRevisionID = existing.IdentityID
				input.ProfileAgent = existing.Agent
				break
			}
		}
		input.Profile = &p
		input.VM.ProfileID = &p.ID
		plan.Add("profile", "Preserve the complete legacy Git identity and references to its externally managed key, agent, and saved credential.")
	}
	if err = db.CheckOverlap(ctx, source, input.VM.ProfileID, id); err != nil {
		return data, err
	}
	var existing int
	if err = db.DB.QueryRowContext(ctx, "SELECT count(*) FROM registrations WHERE namespace_id=? AND instance_name=? AND retired_at IS NULL", n.ID, name).Scan(&existing); err != nil {
		return data, err
	}
	if existing != 0 {
		return data, model.Fail("ownership.conflict", "This instance is already registered by easy-lima.")
	}
	if d, e := files.Digest(filepath.Join(n.Root, name, ".easy-lima-owner.json")); e != nil || d != "" {
		return data, model.Fail("ownership.conflict", "An owner marker already occupies this instance.")
	}
	aliasPath := filepath.Join(root, ".local-state", "ssh", "config")
	alias, err := files.Read(aliasPath, 1<<20)
	if err != nil {
		return data, err
	}
	input.AliasBefore = string(alias)
	input.AliasDigest = files.Hash(alias)
	input.AliasAfter, err = retireLegacyAlias(string(alias), input.VM.Alias, filepath.Join(n.Root, name, "ssh.config"))
	if err != nil {
		return data, err
	}
	if input.Handoff.PriorRunning {
		plan.Add("vm_stop", "Gracefully stop the legacy VM and prove all old guest/forwarding sessions ended.")
	}
	plan.Add("ownership", "Back up exact metadata, atomically retire this project's active legacy record, and publish exclusive easy-lima ownership.")
	plan.Add("file_publication", "Transfer this unchanged SSH alias while preserving unrelated legacy aliases.")
	if input.Handoff.RestoreRunning {
		plan.Add("vm_start", "Restore the same previously running VM after exclusive ownership is verified; no tools or presets run.")
	}
	plan.Fields = []model.Field{{Path: "instance_name", Value: name, Origin: "recorded"}, {Path: "ssh_alias", Value: input.VM.Alias, Origin: "recorded"}, {Path: "storage.root", Value: n.Root, Origin: "recorded"}, {Path: "legacy_record", Value: recordPath, Origin: "recorded"}}
	data.Plan = plan
	if r.DryRun {
		return data, nil
	}
	if err = s.approve(r, plan); err != nil {
		return data, err
	}
	if err = environment.MaterializeSmall(map[string][]byte{input.NativeArtifact: native, input.Environment.YAMLArtifact: normalized}); err != nil {
		return data, err
	}
	claims := []store.Claim{{Kind: "instance", Key: n.ID + ":" + name, Mode: "exclusive"}, {Kind: "source", Key: source, Mode: "exclusive"}, {Kind: "alias", Key: input.VM.Alias, Mode: "exclusive"}, {Kind: "vm", Key: id, Mode: "exclusive"}}
	if input.Profile != nil {
		claims = append(claims, store.Claim{Kind: "profile", Key: input.Profile.ID, Mode: "exclusive"})
	}
	claims = append(claims, portClaims(input.Environment.Settings.Ports)...)
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
	err = s.runAdoption(ctx, db, op, input, r)
	return s.operationResult(ctx, db, data, op), err
}
func retireLegacyAlias(text, alias, nativeConfig string) (string, error) {
	lines := strings.SplitAfter(text, "\n")
	out := []string{}
	removed := 0
	include := `Include "` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(nativeConfig) + `"`
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if line == "Host "+alias {
			start := i
			i++
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "Include ") && !strings.HasPrefix(strings.TrimSpace(lines[i]), "Host ") {
				i++
			}
			if i >= len(lines) || strings.TrimSpace(lines[i]) != include {
				return "", model.Fail("settings.edited", "The selected legacy alias block was edited.")
			}
			for _, l := range lines[start+1 : i] {
				f := strings.Fields(l)
				if len(f) != 2 || !(f[0] == "ForwardAgent" || f[0] == "ControlMaster" || f[0] == "ControlPath" || f[0] == "ServerAliveInterval" || f[0] == "ServerAliveCountMax") {
					return "", model.Fail("settings.edited", "The legacy alias has unsupported edits.")
				}
			}
			i++
			removed++
			continue
		}
		if line == include {
			i++
			removed++
			continue
		}
		out = append(out, lines[i])
		i++
	}
	if removed != 1 {
		return "", model.Fail("settings.edited", "The exact legacy alias entry was not found.")
	}
	return strings.Join(out, ""), nil
}
func (s Service) runAdoption(ctx context.Context, db *store.Store, op store.OperationRecord, in AdoptInput, r Request) error {
	h := in.Handoff
	h.OperationID = op.Operation.ID
	legacyGuard, err := files.Lock(ctx, filepath.Join(h.PrototypeRoot, ".local-state", "launcher.lock"), true, false)
	if err != nil {
		return model.Fail("operation.busy", "The legacy launcher is active or its lock is unavailable.")
	}
	defer legacyGuard.Close()
	u, err := s.user(r)
	if err != nil {
		return err
	}
	n, err := s.validateRoot(ctx, db, u.Value)
	if err != nil {
		return err
	}
	a, err := s.adapter(u.Value, n, r, op.Operation.ID)
	if err != nil {
		return err
	}
	if a.Executable != in.Environment.RuntimePath {
		return model.Fail("dependency.unsupported", "Adoption resume requires its frozen runtime executable.")
	}
	if _, err = a.Probe(ctx); err != nil {
		return err
	}
	if err = lima.CheckGlobals(n.Root, in.Environment.Globals, false); err != nil {
		return err
	}
	aliasBefore, err := files.Digest(filepath.Join(h.PrototypeRoot, ".local-state", "ssh", "config"))
	if err != nil {
		return err
	}
	if aliasBefore != in.AliasDigest && aliasBefore != files.Hash([]byte(in.AliasAfter)) {
		return model.Fail("settings.edited", "The legacy SSH fragment changed after the adoption preview; ownership was not transferred.")
	}
	v, err := db.VM(ctx, in.VM.ID)
	registered := err == nil
	if err != nil && !isNotFound(err) {
		return err
	}
	if !registered {
		v = store.VMRecord{VM: model.VM{ID: in.VM.ID, InstanceName: in.VM.InstanceName, SSHAlias: in.VM.Alias, SourcePath: in.VM.SourcePath, NamespaceID: in.VM.NamespaceID, Storage: model.Storage{Root: n.Root, InstanceDirectory: filepath.Join(n.Root, in.VM.InstanceName)}}}
	}
	active, err := files.Digest(h.ActiveRecord)
	if err != nil {
		return err
	}
	archive, err := files.Digest(h.ArchiveRecord)
	if err != nil {
		return err
	}
	if active != "" && active != h.OldRecordHash {
		return model.Fail("ownership.conflict", "The active legacy owner record changed.")
	}
	if archive != "" && archive != h.OldRecordHash {
		return model.Fail("ownership.conflict", "The retired legacy owner record changed.")
	}
	if active == "" && archive == "" {
		return model.Fail("ownership.conflict", "Neither the active nor retired ownership record can be verified.")
	}
	if registered && active != "" {
		return model.Fail("ownership.conflict", "The legacy owner is active again; automatic handoff cannot continue.")
	}
	handoffDir := filepath.Join(db.Dir, "handoffs", h.HandoffID)
	if err = files.PrivateDir(handoffDir); err != nil {
		return err
	}
	if !registered {
		if active == "" {
			proof, e := a.ObserveStopped(ctx, v.VM.InstanceName, h.PrototypeRoot)
			if e != nil {
				return e
			}
			if proof.ConfigDigest != h.NativeHash {
				return model.Fail("config.drift", "Retired legacy inputs changed before owner publication.")
			}
		}
		if active != "" {
			obs, e := a.Observe(ctx, v.VM.InstanceName)
			if e != nil {
				return e
			}
			if obs.Status == "Running" {
				if !h.PriorRunning {
					return model.Fail("operation.unknown", "The legacy VM was started after the adoption preview.")
				}
				if e = s.vmEffect(ctx, db, op, v, a, "stop", ""); e != nil {
					return e
				}
			}
			proof, e := a.ObserveStopped(ctx, v.VM.InstanceName, h.PrototypeRoot)
			if e != nil {
				return e
			}
			if proof.ConfigDigest != h.NativeHash {
				return model.Fail("config.drift", "Legacy native inputs changed before retirement.")
			}
			h.Quiescence = &proof
			for _, pair := range [][2]string{{h.ActiveRecord, "project.json"}, {filepath.Join(v.VM.Storage.InstanceDirectory, "lima.yaml"), "lima.yaml"}, {filepath.Join(h.PrototypeRoot, ".local-state", "ssh", "config"), "ssh-config"}} {
				body, e := files.Read(pair[0], 8<<20)
				if e != nil {
					return e
				}
				dest := filepath.Join(handoffDir, pair[1])
				d, e := files.Digest(dest)
				if e != nil {
					return e
				}
				if d != "" && d != files.Hash(body) {
					return model.Fail("settings.edited", "A handoff metadata backup changed.")
				}
				if d == "" {
					if e = files.WriteNew(dest, body); e != nil {
						return e
					}
				}
				h.Backups = append(h.Backups, map[string]any{"path": dest, "sha256": files.Hash(body), "mode": 384})
			}
			manifest, err := files.Wrap(h)
			if err != nil {
				return err
			}
			manifestPath := filepath.Join(handoffDir, "manifest.json")
			d, err := files.Digest(manifestPath)
			if err != nil {
				return err
			}
			if d == "" {
				if err = files.WriteNew(manifestPath, manifest); err != nil {
					return err
				}
			}
			if err = files.PrivateDir(filepath.Dir(h.ArchiveRecord)); err != nil {
				return err
			}
			if archive != "" {
				return model.Fail("ownership.conflict", "Both active and archived legacy records exist.")
			}
			if d, err = files.Digest(h.ActiveRecord); err != nil || d != h.OldRecordHash {
				return model.Fail("ownership.conflict", "Legacy record changed immediately before retirement.")
			}
			if err = lima.CheckGlobals(n.Root, in.Environment.Globals, false); err != nil {
				return err
			}
			aliasNow, e := files.Digest(filepath.Join(h.PrototypeRoot, ".local-state", "ssh", "config"))
			if e != nil {
				return e
			}
			if aliasNow != in.AliasDigest {
				return model.Fail("settings.edited", "The legacy SSH fragment changed before retirement. The original owner is preserved.")
			}
			if err = os.Rename(h.ActiveRecord, h.ArchiveRecord); err != nil {
				return err
			}
			if err = files.SyncDir(filepath.Dir(h.ActiveRecord)); err != nil {
				return err
			}
			if err = files.SyncDir(filepath.Dir(h.ArchiveRecord)); err != nil {
				return err
			}
		}
		if in.Profile != nil && !in.ProfileExisting {
			if _, e := db.Profile(ctx, in.Profile.ID); isNotFound(e) {
				if e = db.SaveExternalProfile(ctx, op, *in.Profile, in.PublicKey, in.Fingerprint, in.ProfileAgent); e != nil {
					return e
				}
			} else if e != nil {
				return e
			}
			body, e := config.Encode(*in.Profile)
			if e != nil {
				return e
			}
			path := filepath.Join(s.Paths.Config, "profiles", in.Profile.ID+".toml")
			if e = files.PrivateDir(filepath.Dir(path)); e != nil {
				return e
			}
			got, e := files.Digest(path)
			if e != nil {
				return e
			}
			if got != "" && got != files.Hash(body) {
				return model.Fail("settings.edited", "The adopted desired profile was edited during handoff.")
			}
			if got == "" {
				if e = files.WriteNew(path, body); e != nil {
					return e
				}
			}
		}
		err = db.Change(ctx, store.Change{Kind: "ownership", OperationID: op.Operation.ID, Namespace: true}, func(tx *sql.Tx) error {
			if e := store.CheckOverlapIn(ctx, tx, in.VM.SourcePath, in.VM.ProfileID, in.VM.ID); e != nil {
				return e
			}
			_, e := tx.ExecContext(ctx, "INSERT INTO registrations(id,namespace_id,instance_name,ssh_alias,source_path,source_key,profile_id,ownership,lifecycle,created_at) VALUES(?,?,?,?,?,?,?,'adopting','stopped',?)", in.VM.ID, in.VM.NamespaceID, in.VM.InstanceName, in.VM.Alias, in.VM.SourcePath, in.VM.SourcePath, in.VM.ProfileID, model.Now())
			if e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, "UPDATE registrations SET pending_trust_operation_id=? WHERE id=?", op.Operation.ID, in.VM.ID); e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO owned_settings(owner_id,owner_kind,target,field,expected_digest) VALUES(?,'vm',?,'handoff',?)", in.VM.ID, handoffDir, h.HandoffID); e != nil {
				return e
			}
			_, e = tx.ExecContext(ctx, "INSERT INTO owned_settings(owner_id,owner_kind,target,field,expected_digest,backup_ref) VALUES(?,'vm',?,'legacy-retirement',?,?)", in.VM.ID, h.ActiveRecord, h.OldRecordHash, h.ArchiveRecord)
			return e
		})
		if err != nil {
			return err
		}
		v, err = db.VM(ctx, in.VM.ID)
		if err != nil {
			return err
		}
	}
	nativePath := filepath.Join(v.VM.Storage.InstanceDirectory, "lima.yaml")
	cur, err := files.Digest(nativePath)
	if err != nil {
		return err
	}
	if cur != h.NativeHash && cur != in.Environment.InstanceDigest {
		return model.Fail("settings.edited", "The adopted native configuration changed.")
	}
	if cur != in.Environment.InstanceDigest {
		body, e := environment.Read(in.Environment)
		if e != nil {
			return e
		}
		if e = files.Replace(nativePath, body, h.NativeHash, 0600); e != nil {
			return e
		}
	}
	aliasPath := filepath.Join(h.PrototypeRoot, ".local-state", "ssh", "config")
	cur, err = files.Digest(aliasPath)
	if err != nil {
		return err
	}
	if cur != in.AliasDigest && cur != files.Hash([]byte(in.AliasAfter)) {
		return model.Fail("settings.edited", "The legacy SSH fragment changed during adoption.")
	}
	if cur == in.AliasDigest {
		if err = files.Replace(aliasPath, []byte(in.AliasAfter), cur, 0600); err != nil {
			return err
		}
	}
	if in.Profile != nil && v.PendingIdentityKind == nil && v.VM.AppliedIdentityRevisionID == nil {
		p, e := db.Profile(ctx, in.Profile.ID)
		if e != nil {
			return e
		}
		if e = db.BindInitialIdentity(ctx, op.Operation.ID, v.VM.ID, p.Profile.ID, p.IdentityID); e != nil {
			return e
		}
		if e = db.OwnSetting(ctx, op.Operation.ID, v.VM.ID, "vm", "guest:canonical-local", "legacy-identity", files.Hash([]byte(in.PublicKey))); e != nil {
			return e
		}
	}
	if err = db.UpdateVMState(ctx, op, v.VM.ID, "stopped", "owned"); err != nil {
		return err
	}
	legacyGuard.Close()
	v, err = db.VM(ctx, v.VM.ID)
	if err != nil {
		return err
	}
	endpoint, e := connection.ReadEndpoint(n.Root, v.VM.InstanceName, v.VM.ID)
	if e != nil {
		return e
	}
	if e = connection.PublishAlias(ctx, db, op.Operation.ID, s.Paths.Home, s.Binary, v, endpoint); e != nil {
		return e
	}
	if h.RestoreRunning {
		if err = s.vmEffect(ctx, db, op, v, a, "start", ""); err != nil {
			return err
		}
		e, err := connection.ReadEndpoint(n.Root, v.VM.InstanceName, v.VM.ID)
		if err != nil {
			return err
		}
		if _, err = connection.Enroll(ctx, db, op, v, e); err != nil {
			return err
		}
		v, err = db.VM(ctx, v.VM.ID)
		if err != nil {
			return err
		}
		if err = s.prepareConnection(ctx, db, op, v, in.Environment, r, true); err != nil {
			return err
		}
		if err = db.UpdateVMState(ctx, op, v.VM.ID, "running", "owned"); err != nil {
			return err
		}
	}
	if v.VM.AppliedEnvironmentRevisionID == nil {
		history := "unknown"
		if h.RestoreRunning && in.Environment.LegacySetup != nil && in.Environment.LegacySetup.ExpectedDigest != "" {
			c, e := s.pinned(ctx, db, v)
			if e == nil {
				output, readErr := c.Output(ctx, "python3", "-c", "from pathlib import Path; p=Path('/var/lib/canonical-local/provision.sha256'); print(p.read_text()[:256].strip() if p.is_file() else 'missing')")
				c.Close()
				if readErr == nil {
					receipt := strings.TrimSpace(string(output))
					if receipt == in.Environment.LegacySetup.ExpectedDigest {
						in.Environment.LegacySetup.Status = "completed"
						in.Environment.LegacySetup.ObservedDigest = receipt
						history = "known"
					} else if receipt == "missing" || regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(receipt) {
						in.Environment.LegacySetup.Status = "incomplete"
						if receipt != "missing" {
							in.Environment.LegacySetup.ObservedDigest = receipt
						}
					}
				}
			}
		}
		if err = db.SaveEnvironment(ctx, op, v.VM.ID, in.Environment.RevisionID, files.Hash([]byte(store.JSON(in.Environment))), in.Environment, history); err != nil {
			return err
		}
	}
	if err = db.Finish(ctx, op, "completed"); err != nil {
		return err
	}
	_ = db.RefreshInventory(ctx, v.VM.ID)
	return nil
}
func (s Service) verifyLegacyRetirement(ctx context.Context, db *store.Store, vmID string) error {
	return db.VerifyLegacyRetirement(ctx, vmID)
}

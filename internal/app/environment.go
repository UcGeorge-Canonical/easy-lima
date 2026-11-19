package app

import (
	"context"
	"database/sql"
	"github.com/UcGeorge-Canonical/easy-lima/internal/connection"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/environment"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/project"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

type UpdateInput struct {
	VMID               string            `json:"vm_id"`
	Old                environment.Input `json:"old_environment"`
	Target             environment.Input `json:"target_environment"`
	OldSource          string            `json:"old_source"`
	NewSource          string            `json:"new_source"`
	PriorState         string            `json:"prior_state"`
	OldNativeDigest    string            `json:"old_native_digest"`
	TargetNativeDigest string            `json:"target_native_digest"`
	Provision          bool              `json:"provision"`
	RefreshTools       bool              `json:"refresh_tools"`
	History            string            `json:"history"`
	RowRevision        int64             `json:"row_revision"`
}

func (s Service) environmentUpdate(ctx context.Context, db *store.Store, r Request) (model.MutationData, error) {
	return s.planUpdate(ctx, db, r, false)
}
func (s Service) rebind(ctx context.Context, db *store.Store, r Request) (model.MutationData, error) {
	return s.planUpdate(ctx, db, r, true)
}
func (s Service) planUpdate(ctx context.Context, db *store.Store, r Request, rebind bool) (model.MutationData, error) {
	v, err := s.resolveVM(ctx, db, r)
	if err != nil {
		return model.MutationData{}, err
	}
	plan := model.NewPlan(r.Command, model.VMScope(v.VM.ID, v.VM.SourcePath))
	data := model.Mutation(plan)
	data.VM = &v.VM
	if len(v.VM.OperationIDs) > 0 {
		return data, model.Fail("operation.busy", "Resume existing VM work before changing its environment or binding.")
	}
	if err = s.verifyOwnership(ctx, db, v); err != nil {
		return data, err
	}
	old, err := decodeEnvironment(v)
	if err != nil {
		return data, err
	}
	u, err := s.user(r)
	if err != nil {
		return data, err
	}
	n, err := s.validateRoot(ctx, db, u.Value)
	if err != nil {
		return data, err
	}
	a, err := s.adapter(u.Value, n, r, "update-plan")
	if err != nil {
		return data, err
	}
	if _, err = a.Probe(ctx); err != nil {
		return data, err
	}
	b, err := files.Read(filepath.Join(v.VM.Storage.InstanceDirectory, "lima.yaml"), 8<<20)
	if err != nil {
		return data, err
	}
	currentHash := files.Hash(b)
	if currentHash != old.InstanceDigest && !r.AcceptNative {
		return data, model.Fail("config.drift", "Native Lima inputs changed; review --accept-native-config in an explicit environment update.")
	}
	if err = lima.CheckGlobals(n.Root, old.Globals, false); err != nil && !r.AcceptNative {
		return data, err
	}
	if err = lima.CheckNative(b); err != nil {
		return data, err
	}
	target := old
	target.RevisionID = model.NewID()
	if r.AcceptNative {
		target.Globals, err = lima.ReadGlobals(n.Root, false)
		if err != nil {
			return data, err
		}
	}
	var history string
	if err = db.DB.QueryRowContext(ctx, "SELECT history FROM environment_revisions WHERE id=?", old.RevisionID).Scan(&history); err != nil {
		return data, err
	}
	settings := old.Settings
	newSource := v.VM.SourcePath
	if rebind {
		if r.VMID == "" || r.To == "" {
			return data, model.Fail("input.invalid", "rebind requires --vm ID and --to PATH.")
		}
		newSource, err = project.Resolve(r.To, s.CWD, true)
		if err != nil {
			return data, err
		}
		if project.Same(newSource, v.VM.SourcePath) {
			return data, nil
		}
		if err = db.CheckOverlap(ctx, newSource, v.VM.ProfileID, v.VM.ID); err != nil {
			return data, err
		}
		plan.Fields = append(plan.Fields, model.Field{Path: "source_path", Value: newSource, Origin: "flag", Changed: true})
	} else {
		if r.AcceptNative {
			native, images, e := lima.NativeSettings(b)
			if e != nil {
				return data, e
			}
			baseline, oldImages, e := lima.NativeSettings(mustReadEnvironment(old))
			if e != nil {
				return data, e
			}
			if native.Resources.Disk != baseline.Resources.Disk || !reflect.DeepEqual(images, oldImages) {
				return data, model.Fail("resource.unsupported", "Image and disk changes cannot be accepted on an existing VM.")
			}
			settings = config.Merge(settings, native)
		}
		if r.FromProject {
			path := r.ProjectConfig
			if path == "" {
				path = filepath.Join(v.VM.SourcePath, "easy-lima.toml")
			} else {
				path, err = config.ResolvePath(path, s.CWD)
				if err != nil {
					return data, err
				}
			}
			projectFile, e := config.LoadProject(path)
			if e != nil && !(os.IsNotExist(e) && r.ProjectConfig == "") {
				return data, e
			}
			p := projectFile.Value
			settings = config.Merge(settings, config.Settings{Environment: p.Environment, Resources: p.Resources, Mounts: p.Mounts, Ports: p.Ports})
		}
		settings = config.Merge(settings, r.Settings)
		if settings.Resources.Disk != old.Settings.Resources.Disk || settings.Environment.Template != old.Settings.Environment.Template {
			return data, model.Fail("resource.unsupported", "Changing an existing VM's disk maximum or image is outside an environment update.")
		}
		if err = config.NormalizeSettings(&settings, s.CWD, true); err != nil {
			return data, err
		}
		if err = environment.ValidateResources(settings.Resources); err != nil {
			return data, err
		}
		for _, preset := range old.Settings.Environment.Presets {
			if !lima.HasPreset(settings, preset) {
				return data, model.Fail("resource.unsupported", "Preset removal is not supported; installed tools and user workloads are preserved.")
			}
		}
		plan.Fields = environment.Diff(old.Settings, settings)
		if len(plan.Fields) == 0 && !r.RefreshTools && !r.AcceptNative {
			return data, nil
		}
	}
	if err = s.safeMounts(ctx, db, newSource, settings.Mounts); err != nil {
		return data, err
	}
	target.Settings = settings
	var patched []byte
	if rebind {
		patched, err = lima.PatchWorkspace(b, newSource)
	} else {
		patched, err = lima.Patch(b, settings, newSource)
	}
	if err != nil {
		return data, err
	}
	target.YAMLDigest = files.Hash(patched)
	target.InstanceDigest = target.YAMLDigest
	target.YAMLArtifact = filepath.Join(db.Dir, "artifacts", target.YAMLDigest+".yaml")
	provision := !rebind && (r.RefreshTools || !reflect.DeepEqual(settings.Environment.Presets, old.Settings.Environment.Presets))
	if provision && history == "unknown" && !r.RefreshTools {
		return data, model.Fail("decision.required", "This adopted VM has unknown setup history. Review --refresh-tools explicitly before installing the current preset tools.")
	}
	if provision {
		c, e := environment.LoadCatalog()
		if e != nil {
			return data, e
		}
		if r.RefreshTools || (!lima.HasPreset(old.Settings, "development") && lima.HasPreset(settings, "development")) {
			target.Snaps = c.Snaps[environment.HostArch()]
		}
		if r.RefreshTools || (!lima.HasPreset(old.Settings, "docker") && lima.HasPreset(settings, "docker")) {
			target.Docker = c.Docker[environment.HostArch()]
		}
		if lima.HasPreset(settings, "development") || lima.HasPreset(settings, "docker") {
			if e = environment.PrepareNetworks(ctx, &target); e != nil {
				return data, e
			}
		}
		target.CatalogRevision = c.Revision
		target.PreviousRevisionID = old.ProvisionRevisionID
		if target.PreviousRevisionID == "" {
			target.PreviousRevisionID = old.RevisionID
		}
		target.PreviousPresets = old.Settings.Environment.Presets
		target.RefreshTools = r.RefreshTools
		target.Scripts = nil
	}
	material := map[string][]byte{target.YAMLArtifact: patched}
	if provision {
		frozen, e := environment.Freeze(&target, db.Dir)
		if e != nil {
			return data, e
		}
		for path, body := range frozen {
			material[path] = body
		}
	}
	obs, err := a.Observe(ctx, v.VM.InstanceName)
	if err != nil {
		return data, err
	}
	if obs.Status != "Running" && obs.Status != "Stopped" {
		return data, model.Fail("operation.unknown", "A configuration change requires a known running or stopped VM.")
	}
	// Existing running forwards belong to this VM. New endpoints must be free
	// before approval and before stopping an otherwise usable workspace.
	ports := settings.Ports
	if obs.Status == "Running" {
		ports = nil
		for _, candidate := range settings.Ports {
			present := false
			for _, previous := range old.Settings.Ports {
				if portKey(candidate) == portKey(previous) {
					present = true
					break
				}
			}
			if !present {
				ports = append(ports, candidate)
			}
		}
	}
	if err = availablePorts(ports); err != nil {
		return data, err
	}
	if obs.Status == "Running" {
		plan.Add("vm_stop", "Gracefully stop this VM to update its reviewed native inputs.")
	}
	plan.Add("file_publication", "Back up the native configuration and publish only the reviewed field changes.")
	if rebind {
		plan.Add("metadata", "Move the project binding while preserving the VM ID, disk, instance name, and SSH alias.")
		plan.Add("connection", "Verify /workspace using a temporary owned probe file in the new checkout, then remove it. For a stopped VM, verification occurs on its next explicit start/open.")
	}
	if obs.Status == "Running" || provision {
		plan.Add("vm_start", "Start the same VM after the reviewed configuration is installed.")
	}
	if provision {
		appendEnvironmentPreview(&plan, target)
	}
	if obs.Status == "Stopped" && provision {
		plan.Add("vm_stop", "Restore the VM's previous stopped state after the update.")
	}
	data.Plan = plan
	if r.DryRun || (!rebind && !r.Apply) {
		return data, nil
	}
	if err = s.approve(r, plan); err != nil {
		return data, err
	}

	if err = environment.MaterializeSmall(material); err != nil {
		return data, err
	}
	input := UpdateInput{VMID: v.VM.ID, Old: old, Target: target, OldSource: v.VM.SourcePath, NewSource: newSource, PriorState: obs.Status, OldNativeDigest: currentHash, TargetNativeDigest: target.InstanceDigest, Provision: provision, RefreshTools: r.RefreshTools, RowRevision: v.RowRevision, History: history}
	op, err := db.Prepare(ctx, plan, input, append(portClaims(settings.Ports), store.Claim{Kind: "vm", Key: v.VM.ID, Mode: "exclusive"}), nil)
	if err != nil {
		return data, err
	}
	data.Operation = &op.Operation
	g, err := operationGuard(ctx, db, op.Operation.ID)
	if err != nil {
		return data, err
	}
	defer g.Close()
	err = s.runUpdate(ctx, db, op, input, r)
	return s.operationResult(ctx, db, data, op), err
}
func mustReadEnvironment(in environment.Input) []byte { b, _ := environment.Read(in); return b }
func (s Service) runUpdate(ctx context.Context, db *store.Store, op store.OperationRecord, in UpdateInput, r Request) error {
	raw, e := capture(db, op, r)
	if e != nil {
		return e
	}
	if raw != nil {
		defer raw.Close()
	}
	v, err := db.VM(ctx, in.VMID)
	if err != nil {
		return err
	}
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

	a.Capture = raw
	if err = s.verifyOwnership(ctx, db, v); err != nil {
		return err
	}
	if err = lima.CheckGlobals(n.Root, in.Target.Globals, false); err != nil {
		return err
	}
	if _, err = environment.Read(in.Target); err != nil {
		return err
	}
	if err = a.Validate(ctx, in.Target.YAMLArtifact); err != nil {
		return err
	}
	if a.Executable != in.Old.RuntimePath {
		return model.Fail("dependency.unsupported", "Resume requires the recorded Lima executable.")
	}
	if _, err = a.Probe(ctx); err != nil {
		return err
	}
	var bindingCount int
	if err = db.DB.QueryRowContext(ctx, "SELECT count(*) FROM binding_changes WHERE operation_id=?", op.Operation.ID).Scan(&bindingCount); err != nil {
		return err
	}
	if bindingCount == 0 {
		if err = db.CheckOverlap(ctx, in.NewSource, v.VM.ProfileID, in.VMID); err != nil {
			return err
		}
		err = db.Change(ctx, store.Change{Kind: "rebind", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
			if e := store.CheckOverlapIn(ctx, tx, in.NewSource, v.VM.ProfileID, in.VMID); e != nil {
				return e
			}
			var revision int64
			if e := tx.QueryRowContext(ctx, "SELECT row_revision FROM registrations WHERE id=?", in.VMID).Scan(&revision); e != nil {
				return e
			}
			if revision != in.RowRevision {
				return model.Fail("binding.conflict", "The selected VM changed after the update preview.")
			}
			if _, e := tx.ExecContext(ctx, "INSERT INTO binding_changes(operation_id,registration_id,kind,old_binding,proposed_binding,stage) VALUES(?,?,?,?,?,'prepared')", op.Operation.ID, in.VMID, op.Operation.Kind, store.JSON(map[string]string{"source_path": in.OldSource, "native_digest": in.OldNativeDigest}), store.JSON(map[string]string{"source_path": in.NewSource, "native_digest": in.TargetNativeDigest})); e != nil {
				return e
			}
			if in.NewSource != in.OldSource {
				_, e := tx.ExecContext(ctx, "INSERT INTO binding_reservations VALUES(?,?,?,?,?,?)", op.Operation.ID, in.VMID, in.NewSource, in.NewSource, v.VM.ProfileID, v.VM.ProfileID != nil)
				return e
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	configPath := filepath.Join(v.VM.Storage.InstanceDirectory, "lima.yaml")
	cur, err := files.Digest(configPath)
	if err != nil {
		return err
	}
	if cur != in.OldNativeDigest && cur != in.TargetNativeDigest {
		return model.Fail("settings.edited", "Native settings differ from both the old and approved target; preserve them for recovery.")
	}
	var stage string
	if err = db.DB.QueryRowContext(ctx, "SELECT stage FROM binding_changes WHERE operation_id=? AND registration_id=?", op.Operation.ID, in.VMID).Scan(&stage); err != nil {
		return err
	}
	if stage == "prepared" {
		obs, e := a.Observe(ctx, v.VM.InstanceName)
		if e != nil {
			return e
		}
		if obs.Status == "Running" {
			if e = s.vmEffect(ctx, db, op, v, a, "stop", ""); e != nil {
				return e
			}
		} else if obs.Status != "Stopped" {
			return model.Fail("operation.unknown", "The VM is not observably stopped for configuration publication.")
		}
		if e = connectionRetireStopped(ctx, db, op, v); e != nil {
			return e
		}
		if cur == in.OldNativeDigest {
			old, e := files.Read(configPath, 8<<20)
			if e != nil {
				return e
			}
			backup := filepath.Join(db.Dir, "backups", "native-"+op.Operation.ID+".yaml")
			hash, e := files.Digest(backup)
			if e != nil {
				return e
			}
			if hash == "" {
				if e = files.WriteNew(backup, old); e != nil {
					return e
				}
			} else if hash != in.OldNativeDigest {
				return model.Fail("settings.edited", "The update's native backup changed.")
			}
			b, e := environment.Read(in.Target)
			if e != nil {
				return e
			}
			if e = db.RecordStep(ctx, op, "config.publish", "may_have_dispatched", "unknown", "config.publishing", map[string]string{"old_digest": in.OldNativeDigest, "target_digest": in.TargetNativeDigest}); e != nil {
				return e
			}
			if e = files.Replace(configPath, b, in.OldNativeDigest, 0600); e != nil {
				return e
			}
		}
		if err = db.Change(ctx, store.Change{Kind: "rebind", OperationID: op.Operation.ID, Namespace: true, VMIDs: []string{in.VMID}}, func(tx *sql.Tx) error {
			if _, e := tx.ExecContext(ctx, "UPDATE registrations SET lifecycle='stopped',row_revision=row_revision+1 WHERE id=?", in.VMID); e != nil {
				return e
			}
			_, e := tx.ExecContext(ctx, "UPDATE binding_changes SET stage='installed' WHERE operation_id=?", op.Operation.ID)
			return e
		}); err != nil {
			return err
		}
		if err = db.RecordStep(ctx, op, "config.publish", "exited", "satisfied", "config.published", map[string]string{"target_digest": in.TargetNativeDigest}); err != nil {
			return err
		}
	}
	v, err = db.VM(ctx, in.VMID)
	if err != nil {
		return err
	}
	if stage != "restored" && stage != "restoring" && (in.PriorState == "Running" || in.Provision) {
		if err = s.vmEffect(ctx, db, op, v, a, "start", ""); err != nil {
			return err
		}
		if in.Provision {
			c, e := s.pinned(ctx, db, v)
			if e != nil {
				return e
			}
			e = environment.Provision(ctx, db, op, v, in.Target, c, s.Progress, s.stageOutput(raw))
			c.Close()
			if e != nil {
				return e
			}
		}
	}
	if stage != "restored" && stage != "restoring" && (in.PriorState == "Running" || in.Provision) {
		c, e := s.pinned(ctx, db, v)
		if e != nil {
			return e
		}
		e = verifyWorkspace(ctx, c, in.NewSource, op)
		c.Close()
		if e != nil {
			return e
		}
	}
	if err = lima.CheckGlobals(n.Root, in.Target.Globals, false); err != nil {
		return err
	}
	if in.NewSource != v.VM.SourcePath {
		if err = db.Change(ctx, store.Change{Kind: "rebind", OperationID: op.Operation.ID, Namespace: true, VMIDs: []string{in.VMID}}, func(tx *sql.Tx) error {
			if e := store.CheckOverlapIn(ctx, tx, in.NewSource, v.VM.ProfileID, in.VMID); e != nil {
				return e
			}
			_, e := tx.ExecContext(ctx, "UPDATE registrations SET source_path=?,source_key=?,binding_generation=binding_generation+1,row_revision=row_revision+1 WHERE id=?", in.NewSource, in.NewSource, in.VMID)
			return e
		}); err != nil {
			return err
		}
	}
	if v.VM.AppliedEnvironmentRevisionID == nil || *v.VM.AppliedEnvironmentRevisionID != in.Target.RevisionID {
		if err = db.SaveEnvironment(ctx, op, in.VMID, in.Target.RevisionID, files.Hash([]byte(store.JSON(in.Target))), in.Target, firstNonempty(in.History, "unknown")); err != nil {
			return err
		}
	}
	if stage != "restored" && in.PriorState == "Stopped" && in.Provision {
		// Persist the restoration boundary before dispatching stop. A retry must
		// reconcile the final stop, never start the VM again to repeat completed work.
		if stage != "restoring" {
			if err = db.Change(ctx, store.Change{Kind: "rebind", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
				_, e := tx.ExecContext(ctx, "UPDATE binding_changes SET stage='restoring' WHERE operation_id=?", op.Operation.ID)
				return e
			}); err != nil {
				return err
			}
		}
		if err = s.vmEffect(ctx, db, op, v, a, "stop", ""); err != nil {
			return err
		}
	}
	if stage != "restored" {
		if err = db.Change(ctx, store.Change{Kind: "rebind", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
			_, e := tx.ExecContext(ctx, "UPDATE binding_changes SET stage='restored' WHERE operation_id=?", op.Operation.ID)
			return e
		}); err != nil {
			return err
		}
	}
	state := "stopped"
	if in.PriorState == "Running" {
		state = "running"
	}
	if err = db.UpdateVMState(ctx, op, in.VMID, state, "owned"); err != nil {
		return err
	}
	if state == "running" {
		v, err = db.VM(ctx, in.VMID)
		if err != nil {
			return err
		}
		if err = s.prepareConnection(ctx, db, op, v, in.Target, r, false); err != nil {
			return err
		}
	}
	if err = db.Finish(ctx, op, "completed"); err != nil {
		return err
	}
	_ = db.RefreshInventory(ctx, in.VMID)
	return nil
}

// A short owned nonce proves /workspace resolves to the selected host source.
func verifyWorkspace(ctx context.Context, c *connection.Client, source string, op store.OperationRecord) error {
	name := ".easy-lima-mount-" + op.Operation.ID
	p := filepath.Join(source, name)
	body := []byte(op.Token + "\n")
	digest, err := files.Digest(p)
	if err != nil {
		return err
	}
	if digest != "" && digest != files.Hash(body) {
		return model.Fail("settings.edited", "The temporary workspace verification path is occupied.")
	}
	if digest == "" {
		if err = files.WriteNew(p, body); err != nil {
			return err
		}
	}
	defer files.RemoveExpected(p, files.Hash(body))
	// Shared filesystems can briefly cache a negative lookup. Poll the exact
	// nonce in the guest with a short bound; never infer success from mount presence.
	observed, err := c.Output(ctx, "python3", "-c", `import pathlib,sys,time
p=pathlib.Path(sys.argv[1]); expected=sys.argv[2]
for _ in range(50):
 try:
  value=p.read_text().strip()
  if value == expected:
   print(value);sys.exit(0)
 except (FileNotFoundError,PermissionError): pass
 time.sleep(.1)
`, "/workspace/"+name, op.Token)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(observed)) != op.Token {
		return model.Fail("binding.conflict", "The live /workspace mount does not expose the selected project. Inspect its native mount, then resume this setup or rebind.")
	}
	return nil
}

func appendEnvironmentPreview(plan *model.Plan, in environment.Input) {
	stages := []string{}
	for _, st := range environment.Stages(in) {
		stages = append(stages, st.ID)
	}
	plan.Fields = append(plan.Fields, model.Field{Path: "setup.stages", Value: stages, Origin: "frozen"})
	if in.PreviousRevisionID == "" || in.RefreshTools {
		plan.Fields = append(plan.Fields, model.Field{Path: "setup.ubuntu_packages", Origin: "shipped", Value: []string{"git", "openssh-client", "ca-certificates", "curl", "build-essential", "python3", "python3-venv", "jq", "rsync", "tar", "gzip", "bzip2", "xz-utils", "unzip", "zip", "iptables", "uidmap", "dbus-user-session", "slirp4netns", "pigz", "libltdl7"}})
	}
	if lima.HasPreset(in.Settings, "development") {
		plan.Fields = append(plan.Fields, model.Field{Path: "setup.snap_revisions", Value: in.Snaps, Origin: "frozen"})
	}
	if lima.HasPreset(in.Settings, "docker") {
		packages := []string{}
		for _, p := range in.Docker.Packages {
			packages = append(packages, p.Name+"="+p.Version)
		}
		plan.Fields = append(plan.Fields, model.Field{Path: "setup.docker_packages", Value: packages, Origin: "frozen"})
	}
	plan.Add("guest_stage", "Install only the listed stages. Ubuntu packages use trusted publisher candidates frozen per operation; retries reuse receipts and artifacts.")
	if lima.HasPreset(in.Settings, "development") {
		plan.Add("guest_stage", "Development tools: snapd, LXD 5.21/stable and Juju 3/stable; record and hold Snap revisions. Add the guest user to lxd, which grants guest root-equivalent access.")
	}
	if lima.HasPreset(in.Settings, "docker") {
		plan.Add("guest_stage", "Docker Engine, Buildx and Compose use the frozen publisher packages. Add the guest user to docker, which grants guest root-equivalent access.")
	}
}

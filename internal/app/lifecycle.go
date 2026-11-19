package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/connection"
	"github.com/UcGeorge-Canonical/easy-lima/internal/environment"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/project"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"github.com/UcGeorge-Canonical/easy-lima/internal/storage"
)

type LifecycleInput struct {
	VMID              string            `json:"vm_id"`
	Environment       environment.Input `json:"environment"`
	ProfileID         *string           `json:"profile_id"`
	ProfileRevisionID *string           `json:"profile_revision_id"`
	Create            bool              `json:"create"`
	Ledger            storage.Ledger    `json:"ledger"`
}

func (s Service) creationSettings(ctx context.Context, r Request, u config.User, source string) (config.Settings, config.Project, []model.Field, error) {
	settings := config.Shipped()
	p := config.Project{}
	projectPath := r.ProjectConfig
	if projectPath == "" {
		projectPath = filepath.Join(source, "easy-lima.toml")
	} else {
		var e error
		projectPath, e = config.ResolvePath(projectPath, s.CWD)
		if e != nil {
			return settings, p, nil, e
		}
	}
	file, err := config.LoadProject(projectPath)
	if err == nil {
		p = file.Value
	} else if r.ProjectConfig != "" || !os.IsNotExist(err) {
		return settings, p, nil, err
	}
	projectSettings := config.Settings{Environment: p.Environment, Resources: p.Resources, Mounts: p.Mounts, Ports: p.Ports}
	flags := r.Settings
	if err = config.NormalizeSettings(&flags, s.CWD, false); err != nil {
		return settings, p, nil, err
	}
	selected := config.Merge(config.Merge(config.Merge(settings, u.Defaults), projectSettings), flags).Environment.Template
	origins := map[string]string{}
	sources := map[string]*string{}
	fieldsOf := func(v config.Settings) map[string]any {
		return map[string]any{"environment.template": v.Environment.Template, "environment.presets": v.Environment.Presets, "resources.cpus": v.Resources.CPUs, "resources.memory": v.Resources.Memory, "resources.disk": v.Resources.Disk, "mounts": v.Mounts, "ports": v.Ports}
	}
	mark := func(v config.Settings, origin string, path *string) {
		for key, value := range fieldsOf(v) {
			set := false
			switch x := value.(type) {
			case string:
				set = x != ""
			case int:
				set = x != 0
			case []string:
				set = x != nil
			case []config.Mount:
				set = x != nil
			case []config.Port:
				set = x != nil
			}
			if set {
				origins[key] = origin
				sources[key] = path
			}
		}
	}
	mark(settings, "shipped", nil)
	var native config.Settings
	if selected != "ubuntu-24.04" {
		executable, e := config.Executable(firstNonempty(r.Limactl, u.Runtime.Limactl), s.CWD)
		if e != nil {
			return settings, p, nil, e
		}
		b, e := lima.Template(ctx, executable, selected)
		if e != nil {
			return settings, p, nil, e
		}
		native, _, e = lima.NativeSettings(b)
		if e != nil {
			return settings, p, nil, e
		}
		if e = config.NormalizeSettings(&native, filepath.Dir(selected), false); e != nil {
			return settings, p, nil, e
		}
		settings = config.Merge(settings, native)
		mark(native, "native", &selected)
	}
	userPath := r.UserConfig
	if userPath == "" {
		userPath = filepath.Join(s.Paths.Config, "config.toml")
	}
	for _, layer := range []struct {
		value  config.Settings
		origin string
		path   *string
	}{{u.Defaults, "user", &userPath}, {projectSettings, "project", &projectPath}, {flags, "flag", nil}} {
		settings = config.Merge(settings, layer.value)
		mark(layer.value, layer.origin, layer.path)
	}
	// Native list entries are part of the template, independent of wrapper
	// whole-list precedence. Clearing a wrapper list never erases them.
	if u.Defaults.Mounts != nil || p.Mounts != nil || flags.Mounts != nil {
		settings.Mounts = append(append([]config.Mount{}, native.Mounts...), settings.Mounts...)
	}
	if u.Defaults.Ports != nil || p.Ports != nil || flags.Ports != nil {
		settings.Ports = append(append([]config.Port{}, native.Ports...), settings.Ports...)
	}
	if err = config.NormalizeSettings(&settings, s.CWD, true); err != nil {
		return settings, p, nil, err
	}
	values := fieldsOf(settings)
	fields := []model.Field{}
	for _, key := range []string{"environment.template", "environment.presets", "resources.cpus", "resources.memory", "resources.disk", "mounts", "ports"} {
		fields = append(fields, model.Field{Path: key, Value: values[key], Origin: origins[key], SourcePath: sources[key]})
	}
	if p.Identity.ProfileLabel != "" {
		fields = append(fields, model.Field{Path: "identity.requested_profile", Value: p.Identity.ProfileLabel, Origin: "project", SourcePath: &projectPath})
	}
	return settings, p, fields, nil
}
func (s Service) safeMounts(ctx context.Context, db *store.Store, source string, mounts []config.Mount) error {
	protected := []string{s.Paths.State, filepath.Join(s.Paths.Config, "keys")}
	rows, err := db.DB.QueryContext(ctx, "SELECT key_ref FROM profile_credential_revisions")
	if err != nil {
		return err
	}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		ref, e := store.Decode[store.KeyReference](raw)
		if e != nil {
			rows.Close()
			return model.Fail("record.invalid", "Credential references cannot be checked for mount exposure.")
		}
		protected = append(protected, ref.Path)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	sources := []string{source}
	for _, m := range mounts {
		p, err := project.Resolve(m.Source, s.CWD, true)
		if err != nil {
			return err
		}
		sources = append(sources, p)
	}
	for _, p := range sources {
		for _, secret := range protected {
			if project.Overlap(p, secret) {
				return model.Fail("binding.conflict", fmt.Sprintf("Mount source %q overlaps protected state or private key path %q. Select a narrower directory.", p, secret))
			}
		}
	}
	return nil
}
func (s Service) lifecycle(ctx context.Context, db *store.Store, r Request) (model.MutationData, error) {
	u, err := s.user(r)
	if err != nil {
		return model.MutationData{}, err
	}
	n, err := s.validateRoot(ctx, db, u.Value)
	if err != nil {
		return model.MutationData{}, err
	}
	a, err := s.adapter(u.Value, n, r, "preflight")
	if err != nil {
		return model.MutationData{}, err
	}
	if _, err = a.Probe(ctx); err != nil {
		return model.MutationData{}, err
	}
	v, err := s.resolveVM(ctx, db, r)
	if isNotFound(err) && r.Command == "up" && r.VMID == "" {
		return s.create(ctx, db, r, u.Value, n, a)
	}
	if err != nil {
		return model.MutationData{}, err
	}
	if r.Command == "up" {
		for _, id := range v.VM.OperationIDs {
			pending, e := db.Operation(ctx, id)
			if e != nil {
				return model.MutationData{}, e
			}
			if pending.Operation.Kind == "up" {
				resume := r
				resume.Command = "operations.resume"
				resume.ID = id
				return s.resume(ctx, db, resume)
			}
		}
	}
	if r.CreationFlags || r.Profile != "" || r.NoProfile {
		return model.MutationData{}, model.Fail("input.invalid", "Existing environments retain their creation settings and profile; use an explicit update or profile assignment.")
	}
	plan := model.NewPlan(r.Command, model.VMScope(v.VM.ID, v.VM.SourcePath))
	data := model.Mutation(plan)
	data.VM = &v.VM
	if r.Command == "delete" && v.VM.Ownership == "reserved" {
		return s.deleteReservation(ctx, db, r, v, a)
	}
	for _, id := range v.VM.OperationIDs {
		pending, e := db.Operation(ctx, id)
		if e != nil {
			return data, e
		}
		if pending.Operation.Kind != "identity.complete" || v.IdentityPhase == nil || *v.IdentityPhase != "committed" {
			return data, model.Fail("operation.busy", "This VM has an unfinished attempt; use operations resume with its recorded ID.")
		}
	}
	if err = s.verifyOwnership(ctx, db, v); err != nil {
		return data, err
	}
	in, err := decodeEnvironment(v)
	if err != nil {
		return data, err
	}
	if err = lima.CheckGlobals(n.Root, in.Globals, false); err != nil && r.Command != "stop" && r.Command != "delete" {
		return data, err
	}
	if err = s.verifyNative(v, in); err != nil {
		return data, err
	}
	obs, err := a.Observe(ctx, v.VM.InstanceName)
	if err != nil {
		return data, err
	}
	action := r.Command
	if action == "up" {
		action = "start"
	}
	switch action {
	case "start":
		if obs.Status == "Running" {
			plan.Add("connection", "Reconcile the selected profile and pinned connection on the running VM.")
		} else {
			plan.Add("vm_start", "Start the recorded Ubuntu VM without changing its environment.")
		}
	case "stop":
		if obs.Status == "Stopped" {
			return data, nil
		}
		plan.Add("vm_stop", "Gracefully stop the selected VM; active SSH/editor sessions disconnect.")
	case "restart":
		plan.Add("vm_stop", "Gracefully stop the selected VM.")
		plan.Add("vm_start", "Start the same disk and recorded environment.")
	case "delete":
		if r.ConfirmVM != v.VM.ID && !r.DryRun {
			return data, model.Fail("decision.required", "Deletion requires --confirm-vm with this VM's full immutable ID.")
		}
		plan.Add("vm_delete", "Delete this VM and its disk; preserve the host checkout and profile keys.")
		plan.Effects[0].Destructive = true
	default:
		return data, model.Fail("input.invalid", "Unknown lifecycle action.")
	}
	plan.AffectedVMIDs = []string{v.VM.ID}
	data.Plan = plan
	if r.DryRun {
		return data, nil
	}
	if (action == "start" || action == "restart") && obs.Status == "Stopped" {
		if err = availablePorts(in.Settings.Ports); err != nil {
			return data, err
		}
	}
	if action == "delete" {
		if err = s.approve(r, plan); err != nil {
			return data, err
		}
	}
	if r.Command == "up" && v.VM.ProfileID != nil {
		p, e := db.Profile(ctx, *v.VM.ProfileID)
		if e != nil {
			return data, e
		}
		if e = s.activateProfile(ctx, db, p, r, false); e != nil {
			return data, e
		}
	}
	claims := []store.Claim{{Kind: "vm", Key: v.VM.ID, Mode: "exclusive"}}
	if v.VM.ProfileID != nil {
		claims = append(claims, store.Claim{Kind: "profile", Key: *v.VM.ProfileID, Mode: "shared"})
	}
	op, err := db.Prepare(ctx, plan, LifecycleInput{VMID: v.VM.ID, Environment: in}, claims, nil)
	if err != nil {
		return data, err
	}
	data.Operation = &op.Operation
	guard, err := files.Lock(ctx, filepath.Join(db.Dir, "operations", op.Operation.ID+".lock"), true, true)
	if err != nil {
		return data, err
	}
	defer guard.Close()
	raw, e := capture(db, op, r)
	if e != nil {
		return data, e
	}
	if raw != nil {
		defer raw.Close()
		a.Capture = raw
	}
	a.Temp = filepath.Join(n.Root, "_easy-lima", "tmp", op.Operation.ID)
	if action == "restart" || action == "delete" {
		if obs.Status == "Running" || obs.Status == "Stopped" {
			if err = s.vmEffect(ctx, db, op, v, a, "stop", ""); err != nil {
				return s.operationResult(ctx, db, data, op), err
			}
		}
	}
	if action == "delete" {
		err = s.vmEffect(ctx, db, op, v, a, "delete", "")
		if err == nil {
			err = s.retireVM(ctx, db, op, v)
		}
	} else {
		next := action
		if action == "restart" {
			next = "start"
		}
		err = s.vmEffect(ctx, db, op, v, a, next, "")
		if err == nil {
			state := "running"
			if next == "stop" {
				state = "stopped"
				if e := connection.RetireStoppedTunnel(ctx, db, op.Operation.ID, v.VM.ID); e != nil {
					return s.operationResult(ctx, db, data, op), e
				}
			}
			err = db.UpdateVMState(ctx, op, v.VM.ID, state, "owned")
		}
		if err == nil && next == "start" {
			v, _ = db.VM(ctx, v.VM.ID)
			err = s.prepareConnection(ctx, db, op, v, in, r, r.Command == "up")
			if err != nil && knownConnectionConflict(err) {
				_ = db.Finish(context.WithoutCancel(ctx), op, "failed_known")
			}
		}
	}
	if err == nil {
		err = db.Finish(ctx, op, "completed")
	}
	return s.operationResult(ctx, db, data, op), err
}
func (s Service) create(ctx context.Context, db *store.Store, r Request, u config.User, n store.Namespace, a lima.Adapter) (model.MutationData, error) {
	source, err := s.selected(r, true)
	if err != nil {
		return model.MutationData{}, err
	}
	settings, p, fields, err := s.creationSettings(ctx, r, u, source)
	if err != nil {
		return model.MutationData{}, err
	}
	if err = s.safeMounts(ctx, db, source, settings.Mounts); err != nil {
		return model.MutationData{}, err
	}
	if p.Identity.ProfileLabel != "" && r.Profile == "" && !r.NoProfile {
		return model.MutationData{}, model.Fail("profile.binding_required", "The project requests a local profile. Select --profile explicitly or choose --no-profile.")
	}
	id := model.NewID()
	name, err := lima.InstanceName(n.Root, project.Slug(source), id)
	if err != nil {
		return model.MutationData{}, err
	}
	var profileID, revisionID *string
	var profile *store.ProfileRecord
	if r.Profile != "" {
		pr, e := db.Profile(ctx, r.Profile)
		if e != nil {
			return model.MutationData{}, e
		}
		profile = &pr
		profileID = &pr.Profile.ID
		revisionID = &pr.IdentityID
	}
	if err = db.CheckOverlap(ctx, source, profileID, ""); err != nil {
		return model.MutationData{}, err
	}
	if err = availablePorts(settings.Ports); err != nil {
		return model.MutationData{}, err
	}
	input, material, err := environment.Resolve(ctx, n.Root, db.Dir, source, name, a.Executable, settings)
	if err != nil {
		return model.MutationData{}, err
	}
	ledger, err := storage.Plan(n.Root, db.Dir, input.Image.MaxBytes, mustBytes(settings.Resources.Disk)+input.Image.ExpandedBound)
	if err != nil {
		return model.MutationData{}, err
	}
	plan := model.NewPlan("up", model.VMScope(id, source))
	plan.Fields = fields
	plan.Fields = append(plan.Fields, model.Field{Path: "storage.root", Value: n.Root, Origin: "recorded"}, model.Field{Path: "storage.creation_ledger", Value: ledger, Origin: "recorded"}, model.Field{Path: "identity.profile_id", Value: profileID, Origin: "flag"})
	plan.AffectedVMIDs = []string{id}
	plan.Add("vm_create", "Create an Ubuntu 24.04 VM using the pinned image on the selected volume.")
	plan.Add("vm_start", "Boot and enroll the local VM SSH key by recorded trust on first use.")
	plan.Add("connection", "Verify /workspace using a temporary owned probe file in the selected checkout; remove the probe afterwards.")
	appendEnvironmentPreview(&plan, input)
	plan.Fields = append(plan.Fields, model.Field{Path: "instance.name", Value: name, Origin: "resolved"}, model.Field{Path: "instance.yaml", Value: filepath.Join(n.Root, name, "lima.yaml"), Origin: "resolved"}, model.Field{Path: "workspace", Value: source + " → /workspace (read/write)", Origin: "selected"})
	if profile != nil {
		plan.Fields = append(plan.Fields, model.Field{Path: "identity.profile", Value: profile.Profile.Label, Origin: "selected"})
		plan.Add("profile", "Reuse this host-held key for Git authentication and SSH commit/tag signing. In the guest, HTTPS GitHub remotes map to SSH; host Git settings stay untouched.")
	}
	plan.Add("connection", "Publish a stable SSH alias and prepare /workspace access.")
	data := model.Mutation(plan)
	if profile != nil {
		data.Profile = &profile.Profile
	}
	if r.DryRun {
		return data, nil
	}
	if err = s.approve(r, plan); err != nil {
		return data, err
	}
	if profile != nil {
		if err = s.activateProfile(ctx, db, *profile, r, false); err != nil {
			return data, err
		}
		profile, err = s.reloadProfile(ctx, db, *profile)
		if err != nil {
			return data, err
		}
	}
	if err = environment.MaterializeSmall(material); err != nil {
		return data, err
	}
	creation := LifecycleInput{VMID: id, Environment: input, ProfileID: profileID, ProfileRevisionID: revisionID, Create: true, Ledger: ledger}
	claims := append(portClaims(settings.Ports), []store.Claim{{Kind: "vm", Key: id, Mode: "exclusive"}, {Kind: "source", Key: source, Mode: "exclusive"}, {Kind: "namespace-config", Key: n.ID, Mode: "shared"}}...)
	if profileID != nil {
		claims = append(claims, store.Claim{Kind: "profile", Key: *profileID, Mode: "shared"})
	}
	op, err := db.Prepare(ctx, plan, creation, claims, &store.NewVM{ID: id, NamespaceID: n.ID, InstanceName: name, Alias: "easy-lima-" + name[3:], SourcePath: source, ProfileID: profileID})
	if err != nil {
		return data, err
	}
	data.Operation = &op.Operation
	guard, err := files.Lock(ctx, filepath.Join(db.Dir, "operations", op.Operation.ID+".lock"), true, true)
	if err != nil {
		return data, err
	}
	defer guard.Close()
	if err = storage.Reserve(ctx, db, op, n, ledger); err != nil {
		return s.operationResult(ctx, db, data, op), err
	}
	v, err := db.VM(ctx, id)
	if err != nil {
		return data, err
	}
	err = s.runCreation(ctx, db, op, v, creation, r)
	return s.operationResult(ctx, db, data, op), err
}
func mustBytes(s string) int64 { b, _ := config.Quantity(s); return b }
func (s Service) runCreation(ctx context.Context, db *store.Store, op store.OperationRecord, v store.VMRecord, input LifecycleInput, r Request) error {
	n, err := db.Namespace(ctx)
	if err != nil {
		return err
	}
	if err = storage.CheckRoot(n); err != nil {
		return err
	}
	if err = db.VerifyNamespace(ctx); err != nil {
		return err
	}
	if v.VM.Ownership == "owned" {
		if err = s.verifyOwnership(ctx, db, v); err != nil {
			return err
		}
	}
	if _, err = environment.Read(input.Environment); err != nil {
		return err
	}
	if v.VM.Ownership == "owned" {
		if v.VM.AppliedEnvironmentRevisionID != nil {
			applied, e := decodeEnvironment(v)
			if e != nil {
				return e
			}
			if err = s.verifyNative(v, applied); err != nil {
				return err
			}
		} else {
			frozen, e := environment.Read(input.Environment)
			if e != nil {
				return e
			}
			actual, e := files.Read(filepath.Join(n.Root, v.VM.InstanceName, "lima.yaml"), 8<<20)
			if e != nil {
				return e
			}
			if e = lima.SameCreationConfig(frozen, actual); e != nil {
				return e
			}
		}
	}
	var held int
	if err = db.DB.QueryRowContext(ctx, "SELECT count(*) FROM capacity_claims WHERE operation_id=? AND state='held'", op.Operation.ID).Scan(&held); err != nil {
		return err
	}
	if held == 0 {
		ledger, e := storage.Plan(n.Root, db.Dir, input.Environment.Image.MaxBytes, mustBytes(input.Environment.Settings.Resources.Disk)+input.Environment.Image.ExpandedBound)
		if e != nil {
			return e
		}
		if e = storage.Reserve(ctx, db, op, n, ledger); e != nil {
			return e
		}
	}

	raw, e := capture(db, op, r)
	if e != nil {
		return e
	}
	if raw != nil {
		defer raw.Close()
	}
	a := lima.Adapter{Capture: raw, Output: s.Err, Executable: input.Environment.RuntimePath, Root: n.Root, Temp: filepath.Join(n.Root, "_easy-lima", "tmp", op.Operation.ID), Runner: s.Runner}
	if err = lima.CheckGlobals(n.Root, input.Environment.Globals, v.VM.Ownership == "reserved"); err != nil {
		return err
	}
	if _, err = storage.Materialize(ctx, n.Root, op.Operation.ID, input.Environment.Image); err != nil {
		return err
	}
	if v.VM.Ownership == "reserved" {
		if err = a.Validate(ctx, input.Environment.YAMLArtifact); err != nil {
			return err
		}
		if err = s.vmEffect(ctx, db, op, v, a, "create", input.Environment.YAMLArtifact); err != nil {
			return err
		}
		frozen, e := environment.Read(input.Environment)
		if e != nil {
			return e
		}
		actual, e := files.Read(filepath.Join(n.Root, v.VM.InstanceName, "lima.yaml"), 8<<20)
		if e != nil {
			return e
		}
		if e = lima.SameCreationConfig(frozen, actual); e != nil {
			return e
		}
		if err = db.UpdateVMState(ctx, op, v.VM.ID, "stopped", "owned"); err != nil {
			return err
		}
	}
	v, err = db.VM(ctx, v.VM.ID)
	if err != nil {
		return err
	}
	a.InitialBoot = true
	steps, err := db.Steps(ctx, op.Operation.ID)
	if err != nil {
		return err
	}
	for _, st := range steps {
		if st.Stage == "vm.start" && st.DispatchState != "none" {
			a.InitialBoot = false
		}
	}
	if !a.InitialBoot {
		if err = a.Existing(v.VM.InstanceName); err != nil {
			return err
		}
	}
	if err = s.vmEffect(ctx, db, op, v, a, "start", ""); err != nil {
		return err
	}
	if err = db.UpdateVMState(ctx, op, v.VM.ID, "running", "owned"); err != nil {
		return err
	}
	v, err = db.VM(ctx, v.VM.ID)
	if err != nil {
		return err
	}
	endpoint, err := connection.ReadEndpoint(n.Root, v.VM.InstanceName, v.VM.ID)
	if err != nil {
		return err
	}
	trust, err := connection.Enroll(ctx, db, op, v, endpoint)
	if err != nil {
		return err
	}
	key, err := trust.Key()
	if err != nil {
		return err
	}
	client, err := connection.Dial(ctx, endpoint, key, false)
	if err != nil {
		return err
	}
	defer client.Close()
	if v.VM.AppliedEnvironmentRevisionID == nil {
		if err = environment.Provision(ctx, db, op, v, input.Environment, client, s.Progress, s.stageOutput(raw)); err != nil {
			return err
		}
		b, err := files.Read(filepath.Join(v.VM.Storage.InstanceDirectory, "lima.yaml"), 8<<20)
		if err != nil {
			return err
		}
		frozen, err := environment.Read(input.Environment)
		if err != nil {
			return err
		}
		if err = lima.SameCreationConfig(frozen, b); err != nil {
			return err
		}
		input.Environment.InstanceDigest = files.Hash(b)
		if err = db.SaveEnvironment(ctx, op, v.VM.ID, input.Environment.RevisionID, files.Hash([]byte(store.JSON(input.Environment))), input.Environment, "known"); err != nil {
			return err
		}
	}
	v, err = db.VM(ctx, v.VM.ID)
	if err != nil {
		return err
	}
	if input.ProfileID != nil && v.VM.AppliedIdentityRevisionID == nil && v.PendingIdentityKind == nil {
		if err = db.BindInitialIdentity(ctx, op.Operation.ID, v.VM.ID, *input.ProfileID, *input.ProfileRevisionID); err != nil {
			return err
		}
		v, err = db.VM(ctx, v.VM.ID)
		if err != nil {
			return err
		}
	}
	if err = s.prepareConnection(ctx, db, op, v, input.Environment, r, true); err != nil {
		if knownConnectionConflict(err) {
			_ = db.Finish(context.WithoutCancel(ctx), op, "failed_known")
		}
		return err
	}
	if err = db.Finish(ctx, op, "completed"); err != nil {
		return err
	}
	_ = db.RefreshInventory(ctx, v.VM.ID)
	return nil
}
func (s Service) vmEffect(ctx context.Context, db *store.Store, op store.OperationRecord, v store.VMRecord, a lima.Adapter, action, configPath string) error {
	stage := "vm." + action
	if len(op.Plan.AffectedVMIDs) > 1 {
		stage += "." + v.VM.ID
	}
	steps, err := db.Steps(ctx, op.Operation.ID)
	if err != nil {
		return err
	}
	var prior *store.Step
	for i := range steps {
		if steps[i].Stage == stage {
			prior = &steps[i]
		}
	}
	obs, observeErr := a.Observe(ctx, v.VM.InstanceName)
	satisfied := observeErr == nil && ((action == "create" && obs.Status != "Absent") || (action == "start" && obs.Status == "Running") || (action == "stop" && obs.Status == "Stopped") || (action == "delete" && obs.Status == "Absent"))
	if prior != nil && prior.DispatchState != "none" {
		if satisfied {
			if action == "delete" {
				if _, e := os.Lstat(filepath.Join(a.Root, v.VM.InstanceName)); !os.IsNotExist(e) {
					return model.Fail("operation.unknown", "Instance files remain after deletion.")
				}
			}
			if action == "stop" {
				if _, err = a.ObserveStopped(ctx, v.VM.InstanceName, ""); err != nil {
					return err
				}
			}
			return db.RecordStep(ctx, op, stage, "exited", "satisfied", "vm."+strings.ToLower(obs.Status), map[string]string{"state": obs.Status})
		}
		if observeErr != nil {
			return model.Fail("operation.unknown", "The previous VM action cannot yet be reconciled; no replacement was dispatched.")
		}
		if action == "create" {
			if _, err = os.Lstat(filepath.Join(a.Root, v.VM.InstanceName)); err == nil {
				return model.Fail("operation.unknown", "The reserved instance has partial files; preserve them for explicit recovery.")
			}
		} else if action != "start" && action != "stop" {
			return model.Fail("operation.unknown", "The previous VM effect has no conclusive replay predicate.")
		}
		if action == "start" {
			if _, err = a.ObserveStopped(ctx, v.VM.InstanceName, ""); err != nil {
				return err
			}
		}
		if action == "stop" {
			if err = a.NoController(ctx, v.VM.InstanceName); err != nil {
				return err
			}
		}

	}
	if action == "delete" && satisfied {
		if _, e := os.Lstat(filepath.Join(a.Root, v.VM.InstanceName)); !os.IsNotExist(e) {
			return model.Fail("operation.unknown", "Lima reports absence but instance files remain; deletion is not complete.")
		}
	}
	if prior == nil && action != "create" && satisfied {
		return db.RecordStep(ctx, op, stage, "exited", "satisfied", "vm."+strings.ToLower(obs.Status), map[string]string{"state": obs.Status})
	}
	if observeErr != nil {
		return observeErr
	}
	if action == "create" && prior == nil && satisfied {
		return model.Fail("ownership.conflict", "An instance already occupies the reserved name.")
	}
	if err = db.RecordStep(ctx, op, stage, "may_have_dispatched", "unknown", "vm.dispatch", map[string]string{"action": action, "instance": v.VM.InstanceName}); err != nil {
		return err
	}
	if action == "start" || action == "create" {
		var frozen LifecycleInput
		if json.Unmarshal(op.Inputs, &frozen) == nil && len(frozen.Environment.Globals) > 0 {
			if e := lima.CheckGlobals(a.Root, frozen.Environment.Globals, action == "create"); e != nil {
				return e
			}
		}
	}
	out, callErr := a.Mutate(ctx, action, v.VM.InstanceName, configPath)
	observeCtx, cancelObserve := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelObserve()
	obs, observeErr = a.Observe(observeCtx, v.VM.InstanceName)
	if action == "start" || action == "create" {
		var frozen LifecycleInput
		if json.Unmarshal(op.Inputs, &frozen) == nil && len(frozen.Environment.Globals) > 0 {
			if e := lima.CheckGlobals(a.Root, frozen.Environment.Globals, action == "create"); e != nil {
				return e
			}
		}
	}
	satisfied = observeErr == nil && ((action == "create" && obs.Status != "Absent") || (action == "start" && obs.Status == "Running") || (action == "stop" && obs.Status == "Stopped") || (action == "delete" && obs.Status == "Absent"))
	if action == "delete" && satisfied {
		if _, e := os.Lstat(filepath.Join(a.Root, v.VM.InstanceName)); !os.IsNotExist(e) {
			satisfied = false
		}
	}
	if !satisfied {
		_ = db.RecordStep(context.WithoutCancel(ctx), op, stage, "exited", "unknown", "operation.unknown", map[string]any{"process_exit": out.ExitCode, "started": out.Started})
		if ctx.Err() != nil {
			return model.Fail("operation.cancelled", "Cancelled; the dispatched VM action is not yet confirmed. Resume this operation to reconcile it.")
		}
		if callErr != nil && !out.Started {
			return callErr
		}
		return model.Fail("operation.unknown", "The VM action's postcondition is not confirmed. Resume the same operation to reconcile it.")
	}
	if action == "stop" {
		if _, err = a.ObserveStopped(ctx, v.VM.InstanceName, ""); err != nil {
			return err
		}
	}
	if err = db.RecordStep(observeCtx, op, stage, "exited", "satisfied", "vm."+strings.ToLower(obs.Status), map[string]any{"state": obs.Status, "process_exit": out.ExitCode}); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return model.Fail("operation.cancelled", "Cancelled after the VM action was confirmed; resume the operation to finish remaining work.")
	}
	return nil
}
func (s Service) verifyOwnership(ctx context.Context, db *store.Store, v store.VMRecord) error {
	if v.VM.Ownership != "owned" || v.PublicationPending {
		return model.Fail("ownership.conflict", "The VM does not have a fully published exclusive owner.")
	}
	if err := db.VerifyNamespace(ctx); err != nil {
		return err
	}
	if err := db.VerifyHead(ctx); err != nil {
		return err
	}
	b, err := files.Read(filepath.Join(v.VM.Storage.InstanceDirectory, ".easy-lima-owner.json"), 65536)
	if err != nil {
		return model.Fail("ownership.conflict", "The VM owner marker is unavailable.")
	}
	var o store.Owner
	if err = files.Unwrap(b, &o); err != nil {
		return err
	}
	reg, err := db.Registry(ctx)
	if err != nil {
		return err
	}
	if o.RegistryID != reg.ID || o.RegistrationID != v.VM.ID || o.InstanceName != v.VM.InstanceName || o.OwnerGeneration != v.OwnerGeneration || o.BindingGeneration != v.BindingGeneration || o.OwnerState != "owned" {
		return model.Fail("ownership.conflict", "The instance and registry ownership evidence disagree.")
	}
	expected, e := db.Setting(ctx, v.VM.ID, filepath.Join(v.VM.InstanceName, ".easy-lima-owner.json"), "owner")
	if e != nil {
		return e
	}
	if o.NamespaceID != v.VM.NamespaceID || expected == "" || expected != files.Hash(b) {
		return model.Fail("ownership.conflict", "The owner marker differs from its committed publication.")
	}
	return s.verifyLegacyRetirement(ctx, db, v.VM.ID)
}
func (s Service) verifyNative(v store.VMRecord, in environment.Input) error {
	p := filepath.Join(v.VM.Storage.InstanceDirectory, "lima.yaml")
	b, err := files.Read(p, 8<<20)
	if err != nil {
		return err
	}
	if in.InstanceDigest == "" || files.Hash(b) != in.InstanceDigest {
		return model.Fail("config.drift", "Native VM configuration changed; preview environment update --accept-native-config.")
	}
	return lima.CheckNative(b)
}
func decodeEnvironment(v store.VMRecord) (environment.Input, error) {
	var in environment.Input
	if len(v.Environment) == 0 || json.Unmarshal(v.Environment, &in) != nil {
		return in, model.Fail("operation.partial", "This VM has no complete applied environment; resume its setup operation.")
	}
	return in, nil
}
func (s Service) operationResult(ctx context.Context, db *store.Store, data model.MutationData, op store.OperationRecord) model.MutationData {
	if r, err := db.Operation(context.WithoutCancel(ctx), op.Operation.ID); err == nil {
		data.Operation = &r.Operation
		data.RecoveryRequired = r.Operation.RecoveryRequired
	}
	if data.Plan.Target.ID != nil && data.Plan.Target.Kind == "vm" {
		if v, err := db.VM(context.WithoutCancel(ctx), *data.Plan.Target.ID); err == nil {
			data.VM = &v.VM
			if err := db.RefreshInventory(context.WithoutCancel(ctx), v.VM.ID); err != nil {
				data.Checks = append(data.Checks, model.NewCheck("inventory.publication", "unavailable", "The authoritative operation is recorded, but its diagnostic inventory could not be refreshed. Inspect the VM for inventory status; do not repeat the VM action.", data.Plan.Target))
			}
		}
	}
	if data.Plan.Target.ID != nil && data.Plan.Target.Kind == "profile" {
		if p, e := db.Profile(context.WithoutCancel(ctx), *data.Plan.Target.ID); e == nil {
			data.Profile = &p.Profile
		}
	}
	if data.Operation != nil && data.Operation.Status == "completed" {
		if steps, e := db.Steps(context.WithoutCancel(ctx), op.Operation.ID); e == nil {
			for _, step := range steps {
				if step.Stage == "connection.identity" && step.EffectState == "not_applied" {
					data.Checks = append(data.Checks, model.NewCheck("identity.readiness", "unavailable", "VM is running; Git identity remains unavailable ("+step.ResultCode+"). Unlock or restore the selected profile, then open this VM to complete any pending identity change.", data.Plan.Target))
				}
			}
		}
		for _, e := range data.Plan.Effects {
			data.ConfirmedEffects = append(data.ConfirmedEffects, e.Kind)
		}
	}
	return data
}
func (s Service) retireVM(ctx context.Context, db *store.Store, op store.OperationRecord, v store.VMRecord) error {
	if err := connection.RemoveAlias(ctx, db, v.VM.ID); err != nil {
		return err
	}
	return db.Change(ctx, store.Change{Kind: "retirement", OperationID: op.Operation.ID, Namespace: true, VMIDs: []string{v.VM.ID}}, func(tx *sql.Tx) error {
		var retired *string
		if err := tx.QueryRowContext(ctx, "SELECT retired_at FROM registrations WHERE id=?", v.VM.ID).Scan(&retired); err != nil {
			return err
		}
		if retired != nil {
			return nil
		}
		_, err := tx.ExecContext(ctx, "UPDATE registrations SET ownership='retired',lifecycle='retired',retired_at=?,row_revision=row_revision+1 WHERE id=?", model.Now(), v.VM.ID)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM owned_settings WHERE owner_id=? AND field='port'", v.VM.ID)
		}
		if err == nil && v.VM.ProfileID != nil {
			_, err = tx.ExecContext(ctx, "UPDATE profiles SET membership_generation=membership_generation+1 WHERE id=?", *v.VM.ProfileID)
		}
		return err
	})
}

func knownConnectionConflict(err error) bool {
	switch model.AsError(err).Code {
	case "connection.repair_required", "profile.external_managed", "trust.mismatch", "settings.edited", "profile.unlock_required", "profile.explicitly_locked", "identity.key_mismatch", "dependency.missing":
		return true
	}
	return false
}

func firstNonempty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

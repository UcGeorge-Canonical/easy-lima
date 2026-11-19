// Package app coordinates durable operations shared by the CLI and TUI.
package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/credential"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/project"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"github.com/UcGeorge-Canonical/easy-lima/internal/storage"
)

type Request struct {
	Command, Path, VMID, Label, ID                                                 string
	UserConfig, ProjectConfig, Limactl                                             string
	GitHubUserSet                                                                  bool
	JSON, DryRun, Yes, CaptureOutput, Interactive                                  bool
	Apply, NoStart, Start, Online, Follow, FromProject, RefreshTools, AcceptNative bool
	Profile, StorageRoot, OpenAction, Editor, To, FromPrototype, ConfirmVM         string
	NoProfile                                                                      bool
	Settings                                                                       config.Settings
	CreationFlags                                                                  bool
	VMIDs, Repositories, GuestArgs                                                 []string
	GitName, GitEmail, GitHubUser, Key, PublicKey, NewLabel                        string
	SignCommits, SignTags, Remember                                                *bool
	PassphraseFD                                                                   int
	Limit                                                                          int
	Cursor, RepairKind, HostKeyFile, CheckpointMode, Timeout                       string
	GuestPort, LocalPort                                                           int
	Bind                                                                           string
}
type Service struct {
	Paths       config.Paths
	CWD, Binary string
	In          io.Reader
	Out, Err    io.Writer
	Runner      process.Runner
	Credential  credential.Store
	Confirm     func(model.Plan) (bool, error)
	Secret      func(string) ([]byte, error)
	Progress    func(string)
}

func New() (Service, error) {
	p, err := config.UserPaths()
	if err != nil {
		return Service{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return Service{}, err
	}
	binary, err := os.Executable()
	if err != nil {
		return Service{}, err
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return Service{}, err
	}
	return Service{Paths: p, CWD: cwd, Binary: binary, In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Runner: process.OS{}, Credential: credential.Native(), Secret: credential.Prompt}, nil
}
func (s Service) user(r Request) (config.File[config.User], error) {
	p := r.UserConfig
	if p == "" {
		p = filepath.Join(s.Paths.Config, "config.toml")
	} else {
		var err error
		p, err = config.ResolvePath(p, s.CWD)
		if err != nil {
			return config.File[config.User]{}, err
		}
	}
	return config.LoadUser(p)
}
func (s Service) selected(r Request, required bool) (string, error) {
	p := r.Path
	if p == "" {
		p = s.CWD
	}
	return project.Resolve(p, s.CWD, required)
}
func (s Service) approve(r Request, p model.Plan) error {
	if r.DryRun {
		return nil
	}
	if r.Yes {
		return nil
	}
	if r.Interactive && s.Confirm != nil {
		yes, err := s.Confirm(p)
		if err != nil {
			return err
		}
		if yes {
			return nil
		}
		return model.Fail("operation.cancelled", "The effect plan was declined.")
	}
	return model.Fail("decision.required", "Review the effect plan and supply --yes to accept it.")
}
func (s Service) adapter(u config.User, n store.Namespace, r Request, opID string) (lima.Adapter, error) {
	path := r.Limactl
	if path == "" {
		path = u.Runtime.Limactl
	}
	p, err := config.Executable(path, s.CWD)
	if err != nil {
		return lima.Adapter{}, err
	}
	return lima.Adapter{Executable: p, Root: n.Root, Temp: filepath.Join(n.Root, "_easy-lima", "tmp", opID), Runner: s.Runner, Output: s.Err}, nil
}
func (s Service) Execute(ctx context.Context, r Request) (result model.Result) {
	result = model.NewResult(r.Command)
	defer func() {
		if ctx.Err() != nil && len(result.Errors) > 0 {
			result.ExitCode = 130
			result.Outcome = "cancelled"
		}
		if r.Command == "exec" && len(result.Errors) > 0 {
			result.ExitCode = 255
		}
	}()
	if r.Limit == 0 {
		r.Limit = 100
	}
	if r.PassphraseFD == 0 {
		r.PassphraseFD = -1
	}
	if r.Command == "version" {
		result.Data = map[string]any{"binary_version": model.Version, "db_schema_versions": []int{1}, "config_schema_versions": []int{1}, "result_schema_version": 1, "dependencies": []map[string]string{{"name": "limactl", "version": lima.ContractVersion}}, "lima_contracts": []map[string]any{{"version": lima.ContractVersion, "host": runtime.GOOS, "arch": runtime.GOARCH, "driver": driver(), "qualified": false}}}
		return result
	}
	if r.Command == "init" {
		data, err := s.initialize(ctx, r)
		result.Data = data
		if err != nil {
			result.SetError(err)
		} else if r.DryRun {
			result.Outcome = "planned"
		}
		return result
	}
	if r.Command == "state.restore" {
		return s.restore(ctx, r)
	}
	write := !readOnly(r.Command) && !r.DryRun
	if (r.Command == "environment.update" || r.Command == "repair" || r.Command == "state.restore" || r.Command == "state.migrate") && !r.Apply {
		write = false
	}
	db, err := store.Open(ctx, s.Paths.State, write)
	if err != nil {
		if r.Command == "list" || r.Command == "inspect" {
			inventory := store.ReadInventory(s.Paths.State, r.Limit)
			if r.Command == "inspect" {
				if r.VMID != "" {
					inventory = store.ReadInventoryVM(s.Paths.State, r.VMID)
				} else {
					path, e := s.selected(r, false)
					filtered := []model.VM{}
					if e == nil {
						for _, v := range inventory.Items {
							if project.Same(v.SourcePath, path) {
								filtered = append(filtered, v)
							}
						}
					}
					inventory.Items = filtered
				}
			}
			result.Data = inventory
			result.SetError(model.Fail("state.unavailable", "The database is unavailable; these inventory entries are diagnostic only and may be incomplete."))
			return result
		}
		if r.Command == "doctor" {
			return s.doctorUnavailable(ctx, r, err)
		}
		result.SetError(err)
		return result
	}
	defer db.Close()
	db.Progress = s.Progress
	if write {
		if err = db.ReconcilePublication(ctx); err != nil {
			profileOnly := strings.HasPrefix(r.Command, "profile.")
			if r.Command == "operations.resume" {
				if op, e := db.Operation(ctx, r.ID); e == nil && op.Operation.Scope.Kind == "profile" {
					profileOnly = true
				}
			}
			if !profileOnly {
				result.SetError(err)
				return result
			}
			if model.AsError(err).Code != "storage.unavailable" {
				result.SetError(err)
				return result
			}
		}
	}
	var data any
	switch {
	case r.Command == "list":
		filter := ""
		if r.Profile != "" {
			p, e := db.Profile(ctx, r.Profile)
			if e != nil {
				err = e
				break
			}
			filter = p.Profile.ID
		}
		var list model.ListData[model.VM]
		list, err = db.ListVMs(ctx, filter, r.Limit, r.Cursor)
		data = list
		if !list.Complete {
			result.ExitCode = 6
			result.Outcome = "partial"
		}
	case r.Command == "profile.list":
		var list model.ListData[model.Profile]
		list, err = db.ListProfiles(ctx, r.Limit, r.Cursor)
		data = list
		if !list.Complete {
			result.ExitCode = 6
			result.Outcome = "partial"
		}
	case r.Command == "profile.show":
		var p store.ProfileRecord
		p, err = db.Profile(ctx, r.Label)
		if err == nil {
			data = map[string]any{"profile": p.Profile}
		}
	case r.Command == "profile.check":
		return s.profileCheck(ctx, db, r)
	case strings.HasPrefix(r.Command, "profile."):
		data, err = s.profileMutation(ctx, db, r)
	case r.Command == "operations.list":
		data, err = db.ListOperations(ctx, r.Limit, r.Cursor)
	case r.Command == "operations.inspect":
		var op store.OperationRecord
		op, err = db.Operation(ctx, r.ID)
		if err == nil {
			var steps []store.Step
			steps, err = db.Steps(ctx, r.ID)
			claims, e := db.PendingClaims(ctx, r.ID)
			if e != nil {
				err = e
				break
			}
			data = map[string]any{"operation": op.Operation, "steps": steps, "claims": claims, "plan": op.Plan}
		}
	case r.Command == "operations.resume":
		data, err = s.resume(ctx, db, r)
	case r.Command == "inspect":
		var v store.VMRecord
		v, err = s.resolveVM(ctx, db, r)
		if err == nil {
			ops := []model.Operation{}
			for _, id := range v.VM.OperationIDs {
				if op, e := db.Operation(ctx, id); e == nil {
					ops = append(ops, op.Operation)
				}
			}
			var p *model.Profile
			if v.VM.ProfileID != nil {
				if rec, e := db.Profile(ctx, *v.VM.ProfileID); e == nil {
					p = &rec.Profile
				}
			}
			data = map[string]any{"vm": v.VM, "profile": p, "operations": ops, "identity_ready": inspectedIdentityReady(v, p), "identity_readiness_source": "recorded binding only; use doctor for live signing and forwarding", "inventory_status": db.InventoryStatus(ctx, v.VM.ID)}
			if in, e := decodeEnvironment(v); e == nil {
				var history string
				_ = db.DB.QueryRowContext(ctx, "SELECT history FROM environment_revisions WHERE id=?", in.RevisionID).Scan(&history)
				data.(map[string]any)["environment"] = map[string]any{"revision_id": in.RevisionID, "settings": in.Settings, "catalog_revision": in.CatalogRevision, "history": history, "legacy_setup": in.LegacySetup}
			}
		}
	case r.Command == "doctor":
		return s.doctor(ctx, db, r)
	case r.Command == "storage.inspect":
		data, err = s.storageInspect(ctx, db)
	case r.Command == "config.explain":
		data, err = s.explain(ctx, db, r)
	case strings.HasPrefix(r.Command, "state."):
		data, err = s.maintenance(ctx, db, r)
	case r.Command == "logs":
		data, err = s.logs(ctx, db, r)
	case r.Command == "environment.update":
		data, err = s.environmentUpdate(ctx, db, r)
	case r.Command == "rebind":
		data, err = s.rebind(ctx, db, r)
	case r.Command == "adopt":
		data, err = s.adopt(ctx, db, r)
	case r.Command == "repair":
		data, err = s.repair(ctx, db, r)
	case r.Command == "code" || r.Command == "ssh" || r.Command == "exec" || r.Command == "forward":
		var exit int
		data, exit, err = s.connect(ctx, db, r)
		if err == nil {
			result.ExitCode = exit
		}
	default:
		data, err = s.lifecycle(ctx, db, r)
	}
	result.Data = data
	if m, ok := data.(model.MutationData); ok {
		result.Checks = append(result.Checks, m.Checks...)
		if m.Operation != nil {
			result.OperationID = &m.Operation.ID
		}
		if m.Operation == nil && err == nil && len(m.Plan.Effects) > 0 && r.Command != "profile.unlock" {
			result.Outcome = "planned"
		}
	}
	if err != nil {
		result.SetError(err)
	} else if r.DryRun || ((r.Command == "environment.update" || r.Command == "repair" || r.Command == "state.migrate") && !r.Apply) {
		result.Outcome = "planned"
	}
	return result
}
func readOnly(cmd string) bool {
	switch cmd {
	case "list", "inspect", "doctor", "logs", "profile.list", "profile.show", "profile.check", "operations.list", "operations.inspect", "config.explain", "storage.inspect", "version":
		return true
	}
	return false
}
func driver() string {
	if runtime.GOOS == "darwin" {
		return "vz"
	}
	return "qemu"
}
func (s Service) resolveVM(ctx context.Context, db *store.Store, r Request) (store.VMRecord, error) {
	if r.VMID != "" {
		return db.VM(ctx, r.VMID)
	}
	p, err := s.selected(r, false)
	if err != nil {
		return store.VMRecord{}, err
	}
	return db.ResolveVM(ctx, "", p)
}
func identityReady(v store.VMRecord) *bool {
	if v.AppliedProfileBindingGeneration == nil {
		return nil
	}
	ready := v.PendingIdentityKind == nil && *v.AppliedProfileBindingGeneration == v.ProfileBindingGeneration
	return &ready
}
func (s Service) initialize(ctx context.Context, r Request) (any, error) {
	root, err := config.ResolvePath(r.StorageRoot, s.CWD)
	if err != nil {
		return nil, model.Fail("input.invalid", "init requires an explicitly selected storage directory; use the interactive picker or --storage-root PATH.")
	}
	if r.OpenAction != "" && r.OpenAction != "code" && r.OpenAction != "ssh" {
		return nil, model.Invalid("open-action", "must be code or ssh")
	}
	configPath := filepath.Join(s.Paths.Config, "config.toml")
	if r.UserConfig != "" {
		configPath, err = config.ResolvePath(r.UserConfig, s.CWD)
		if err != nil {
			return nil, err
		}
		// An explicit configuration is an existing input, never a new registry.
		if _, err = config.LoadUser(configPath); err != nil {
			return nil, err
		}
	}
	limactl, err := config.Executable(r.Limactl, s.CWD)
	if err != nil {
		return nil, model.Fail("dependency.missing", "Lima "+lima.ContractVersion+" is required. Install that Lima release (macOS: brew install lima; Linux: follow https://lima-vm.io/docs/installation/), check limactl --version, then retry. Use --limactl /path/to/limactl to select an installed binary.")
	}
	if _, err = (lima.Adapter{Executable: limactl, Runner: s.Runner}).Probe(ctx); err != nil {
		return nil, err
	}
	if r.Editor != "" {
		if r.Editor, err = config.Executable(r.Editor, s.CWD); err != nil {
			return nil, err
		}
	}
	if _, err = lima.InstanceName(root, "project", model.NewID()); err != nil {
		return nil, err
	}
	vol, err := files.InspectVolume(root)
	if err != nil {
		return nil, err
	}
	if _, err = files.InspectVolume(s.Paths.State); err != nil {
		return nil, err
	}
	var existingID string
	if existing, e := store.Open(ctx, s.Paths.State, false); e == nil {
		defer existing.Close()
		n, e := existing.Namespace(ctx)
		if e != nil {
			return nil, e
		}
		if !project.Same(n.Root, root) {
			return nil, model.Fail("ownership.conflict", "This registry is bound to another storage root. V1 does not move or replace it.")
		}
		if e = storage.CheckRoot(n); e != nil {
			return nil, e
		}
		reg, e := existing.Registry(ctx)
		if e != nil {
			return nil, e
		}
		existingID = reg.ID
	} else if _, statErr := os.Stat(filepath.Join(s.Paths.State, "state.sqlite3")); !os.IsNotExist(statErr) {
		return nil, e
	}
	if info, e := os.Lstat(root); e == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return nil, model.Invalid("storage.root", "choose a real directory, not a file or symbolic link")
	}
	plan := model.NewPlan("init", model.Scope{Kind: "storage", Path: &root})
	plan.Add("metadata", "Create or complete the private configuration and registry; preserve existing settings.")
	plan.Add("file_publication", "Bind the selected directory on this volume. Existing directory permissions remain unchanged.")
	plan.Fields = []model.Field{
		{Path: "storage.root", Value: root, Origin: "selected"}, {Path: "storage.volume", Value: vol.MountPoint, Origin: "observed"},
		{Path: "storage.available_GiB", Value: float64(vol.AvailableBytes) / (1 << 30), Origin: "observed"},
		{Path: "storage.image_cache", Value: filepath.Join(root, "_easy-lima", "cache"), Origin: "resolved"},
		{Path: "storage.images", Value: filepath.Join(root, "_easy-lima", "assets"), Origin: "resolved"},
		{Path: "storage.temporary", Value: filepath.Join(root, "_easy-lima", "tmp"), Origin: "resolved"},
		{Path: "state_directory", Value: s.Paths.State, Origin: "shipped"}, {Path: "config_file", Value: configPath, Origin: "resolved"},
		{Path: "runtime.limactl", Value: limactl, Origin: "verified"}, {Path: "ui.open_action", Value: r.OpenAction, Origin: "selected"},
	}
	data := map[string]any{"plan": plan, "registry_id": existingID, "config_directory": s.Paths.Config, "state_directory": s.Paths.State, "storage_root": root}
	if r.DryRun {
		return data, nil
	}
	if err = s.approve(r, plan); err != nil {
		return data, err
	}
	current, err := files.InspectVolume(root)
	if err != nil {
		return data, err
	}
	if current.Identity != vol.Identity || current.MountPoint != vol.MountPoint {
		return data, model.Fail("storage.unavailable", "The selected volume changed while the preview was open. Reconnect it and retry; no fallback was created.")
	}
	// Do not chmod a directory selected by the user. Only owned children are private.
	if err = os.MkdirAll(root, 0700); err != nil {
		return data, model.Fail("storage.unavailable", "The selected VM storage directory cannot be created. Choose a writable directory on the intended drive, or correct that directory's permissions, then retry.")
	}
	if err = files.PrivateDir(filepath.Join(root, "_easy-lima")); err != nil {
		return data, model.Fail("storage.unavailable", "The selected storage directory cannot hold easy-lima metadata. Choose a writable directory on the intended drive; no storage setting was saved.")
	}
	id, err := files.RootIdentity(root)
	if err != nil {
		return data, err
	}
	if err = files.PrivateDir(s.Paths.Config); err != nil {
		return data, err
	}
	u := config.DefaultUser()
	u.Storage.Root = root
	u.Runtime.Limactl = limactl
	if r.OpenAction != "" {
		u.UI.OpenAction = r.OpenAction
	}
	u.Editor.Executable = r.Editor
	b, err := config.Encode(u)
	if err != nil {
		return data, err
	}
	if d, e := files.Digest(configPath); e != nil {
		return data, e
	} else if d != "" {
		existing, e := config.LoadUser(configPath)
		if e != nil {
			return data, e
		}
		if existing.Value.Storage.Root != root {
			return data, model.Fail("settings.edited", "Existing configuration selects another storage root.")
		}
	} else if err = files.WriteNew(configPath, b); err != nil {
		return data, err
	}
	if existingID != "" {
		return data, nil
	}
	db, err := store.Init(ctx, s.Paths.State, root, id)
	if err != nil {
		return data, err
	}
	defer db.Close()
	reg, err := db.Registry(ctx)
	if err == nil {
		data["registry_id"] = reg.ID
	}
	return data, err
}
func (s Service) validateRoot(ctx context.Context, db *store.Store, u config.User) (store.Namespace, error) {
	n, err := db.Namespace(ctx)
	if err != nil {
		return n, err
	}
	if u.Storage.Root != n.Root {
		return n, model.Fail("ownership.conflict", "Configuration cannot change the registry's selected storage root.")
	}
	return n, storage.CheckRoot(n)
}
func isNotFound(err error) bool { return err != nil && model.AsError(err).Code == "target.not_found" }
func (s Service) secret(r Request, label string) ([]byte, error) {
	if r.PassphraseFD >= 3 {
		return credential.ReadFD(r.PassphraseFD)
	}
	if r.Interactive && s.Secret != nil {
		return s.Secret(label)
	}
	return nil, model.Fail("profile.unlock_required", "The selected key needs a passphrase; supply --passphrase-fd or use an interactive command.")
}
func classified(err error) error {
	var e *model.Error
	if errors.As(err, &e) {
		return e
	}
	return model.Fail("operation.failed", "The operation could not complete its required postconditions.")
}

func inspectedIdentityReady(v store.VMRecord, p *model.Profile) *bool {
	if v.VM.ProfileID == nil {
		return nil
	}
	if p != nil && p.LockState == "locked" {
		return model.Ptr(false)
	}
	if v.VM.State.Recorded != "running" || v.PendingIdentityKind != nil {
		return model.Ptr(false)
	}
	return nil // A matching saved binding is not proof of current forwarding/signing.
}

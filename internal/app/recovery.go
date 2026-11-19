package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/UcGeorge-Canonical/easy-lima/internal/connection"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/identity"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

func (s Service) resume(ctx context.Context, db *store.Store, r Request) (model.MutationData, error) {
	op, err := db.Operation(ctx, r.ID)
	if err != nil {
		return model.MutationData{}, err
	}
	data := model.Mutation(op.Plan)
	data.Operation = &op.Operation
	if model.Terminal(op.Operation.Status) {
		return data, nil
	}
	if r.DryRun {
		return data, nil
	}
	g, err := operationGuard(ctx, db, op.Operation.ID)
	if err != nil {
		return data, err
	}
	defer g.Close()
	op, err = db.Takeover(ctx, op.Operation.ID)
	if err != nil {
		return data, err
	}
	if model.Terminal(op.Operation.Status) {
		return s.operationResult(ctx, db, data, op), nil
	}
	switch op.Operation.Kind {
	case "up", "start", "stop", "restart", "delete":
		var in LifecycleInput
		if err = json.Unmarshal(op.Inputs, &in); err != nil {
			return data, model.Fail("record.invalid", "The saved VM attempt cannot be decoded.")
		}
		v, e := db.VM(ctx, in.VMID)
		if e != nil {
			return data, e
		}
		if in.Create {
			err = s.runCreation(ctx, db, op, v, in, r)
			break
		}
		err = s.resumeLifecycle(ctx, db, op, v, in, r)
	case "environment.update", "rebind":
		var in UpdateInput
		if err = json.Unmarshal(op.Inputs, &in); err == nil {
			err = s.runUpdate(ctx, db, op, in, r)
		}
	case "adopt":
		var in AdoptInput
		if err = json.Unmarshal(op.Inputs, &in); err == nil {
			err = s.runAdoption(ctx, db, op, in, r)
		}
	case "profile.apply", "profile.assign", "profile.unassign", "profile.migrate-agent":
		var in ProfileTransition
		if err = json.Unmarshal(op.Inputs, &in); err == nil {
			data.MemberResults, err = s.runProfileTransition(ctx, db, op, in, r, identity.Key{}, nil)
		}
	case "identity.complete":
		var in IdentityTarget
		if err = json.Unmarshal(op.Inputs, &in); err != nil {
			break
		}
		v, e := db.VM(ctx, in.VMID)
		if e != nil {
			return data, e
		}
		if v.PendingIdentityKind == nil {
			err = db.Finish(ctx, op, "completed")
			break
		}
		c, e := s.pinned(ctx, db, v)
		if e != nil {
			return data, model.Fail("operation.partial", "This saved identity update remains pending. Start or open the VM explicitly to complete it.")
		}
		err = s.completeGuestIdentity(ctx, db, op, v, c, r)
		c.Close()
	case "code", "ssh", "exec", "forward":
		var in LifecycleInput
		if err = json.Unmarshal(op.Inputs, &in); err != nil {
			break
		}
		v, e := db.VM(ctx, in.VMID)
		if e != nil {
			return data, e
		}
		err = s.prepareConnection(ctx, db, op, v, in.Environment, r, true)
		if err == nil {
			err = db.Finish(ctx, op, "completed")
		}
	case "repair":
		var in RepairInput
		if err = json.Unmarshal(op.Inputs, &in); err == nil {
			err = s.runRepair(ctx, db, op, in, r)
		}
	case "profile.unlock", "profile.lock", "profile.forget", "profile.remove", "profile.edit":
		var in ProfileFileInput
		if err = json.Unmarshal(op.Inputs, &in); err != nil {
			break
		}
		p, e := db.Profile(ctx, in.ProfileID)
		if e != nil {
			return data, e
		}
		switch op.Operation.Kind {
		case "profile.unlock":
			err = s.completeActivation(ctx, db, op, p, r, true)

		case "profile.lock":
			if err = db.SetProfileRuntime(ctx, op.Operation.ID, p.Profile.ID, "locked", nil); err == nil {
				err = s.revokeMembers(ctx, db, op, p.Profile.MemberVMIDs)
			}
			if err == nil {
				err = agent.UnloadOwned(ctx, p.Profile.ID, p.Agent, *p.Profile.Fingerprint)
			}
		case "profile.edit":
			err = s.writeDesiredProfile(ctx, db, op, in)
		case "profile.forget":
			if err = s.writeDesiredProfile(ctx, db, op, in); err != nil {
				return data, err
			}
			if s.Credential == nil {
				return data, model.Fail("credential.interaction_required", "The credential store is unavailable.")
			}
			err = s.Credential.Delete(ctx, p.Profile.ID, r.Interactive)
			if err == nil {
				err = db.SetProfileRuntime(ctx, op.Operation.ID, p.Profile.ID, "", model.Ptr(false))
			}
		case "profile.remove":
			if err = store.RequireUnusedProfile(ctx, db.DB, p.Profile.ID); err != nil {
				return data, err
			}
			if p.Profile.AgentManagement == "owned" {
				if err = agent.Remove(ctx, p.Profile.ID, p.Agent); err != nil {
					return data, err
				}
			}
			if p.Profile.RememberEnabled && p.StoreOwnership != nil && *p.StoreOwnership == "owned" {
				if s.Credential == nil {
					return data, model.Fail("credential.interaction_required", "The credential store is unavailable.")
				}
				if err = s.Credential.Delete(ctx, p.Profile.ID, r.Interactive); err != nil {
					return data, err
				}
			}
			err = db.Change(ctx, store.Change{Kind: "profile", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
				if err := store.RequireUnusedProfile(ctx, tx, p.Profile.ID); err != nil {
					return err
				}
				_, e := tx.ExecContext(ctx, "UPDATE profiles SET retired_at=? WHERE id=?", model.Now(), p.Profile.ID)
				return e
			})
		}
		if err == nil {
			err = db.Finish(ctx, op, "completed")
		}
	case "profile.create", "profile.import":
		var in ProfileCreation
		if err = json.Unmarshal(op.Inputs, &in); err != nil {
			break
		}
		err = s.resumeProfileCreation(ctx, db, op, in, r)
	default:
		err = model.Fail("operation.unknown", "This attempt has no conclusive automated continuation. Its evidence and claims were retained for explicit recovery.")
	}
	return s.operationResult(ctx, db, data, op), err
}
func (s Service) resumeLifecycle(ctx context.Context, db *store.Store, op store.OperationRecord, v store.VMRecord, in LifecycleInput, r Request) error {
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
		return model.Fail("dependency.unsupported", "Resume requires the saved Lima executable.")
	}
	if op.Operation.Kind == "delete" {
		if err = db.VerifyHead(ctx); err != nil {
			return err
		}
		if err = db.VerifyNamespace(ctx); err != nil {
			return err
		}
		obs, e := a.Observe(ctx, v.VM.InstanceName)
		if e != nil {
			return e
		}
		if obs.Status == "Absent" {
			if _, e = os.Lstat(v.VM.Storage.InstanceDirectory); !os.IsNotExist(e) {
				return model.Fail("operation.unknown", "Instance files remain; deletion cannot yet be retired.")
			}
			if e = a.NoController(ctx, v.VM.InstanceName); e != nil {
				return e
			}
			steps, e := db.Steps(ctx, op.Operation.ID)
			if e != nil {
				return e
			}
			admitted := false
			for _, st := range steps {
				if st.Stage == "vm.delete" && st.DispatchState != "none" {
					admitted = true
				}
			}
			if !admitted {
				return model.Fail("operation.unknown", "No dispatched deletion authorizes retirement of this absent VM.")
			}
			if err = s.retireVM(ctx, db, op, v); err != nil {
				return err
			}
			return db.Finish(ctx, op, "completed")
		}
	}
	if err = s.verifyOwnership(ctx, db, v); err != nil {
		return err
	}
	if err = s.verifyNative(v, in.Environment); err != nil {
		return err
	}
	if op.Operation.Kind != "stop" && op.Operation.Kind != "delete" {
		if err = lima.CheckGlobals(n.Root, in.Environment.Globals, false); err != nil {
			return err
		}
	}
	action := op.Operation.Kind
	if action == "up" {
		action = "start"
	}
	if action == "restart" || action == "delete" {
		steps, e := db.Steps(ctx, op.Operation.ID)
		if e != nil {
			return e
		}
		stopped := false
		for _, st := range steps {
			if st.Stage == "vm.stop" && st.EffectState == "satisfied" {
				stopped = true
			}
		}
		if !stopped {
			if err = s.vmEffect(ctx, db, op, v, a, "stop", ""); err != nil {
				return err
			}
		}
		if action == "restart" {
			action = "start"
		}
	}
	if err = s.vmEffect(ctx, db, op, v, a, action, ""); err != nil {
		return err
	}
	if action == "delete" {
		err = s.retireVM(ctx, db, op, v)
	} else {
		state := "stopped"
		if action == "start" {
			state = "running"
		}
		err = db.UpdateVMState(ctx, op, v.VM.ID, state, "owned")
		if err == nil && action == "start" {
			v, err = db.VM(ctx, v.VM.ID)
			if err == nil {
				err = s.prepareConnection(ctx, db, op, v, in.Environment, r, op.Operation.Kind == "up")
				if err != nil && knownConnectionConflict(err) {
					_ = db.Finish(context.WithoutCancel(ctx), op, "failed_known")
				}
			}
		}
	}
	if err != nil {
		return err
	}
	return db.Finish(ctx, op, "completed")
}

type RepairInput struct {
	VMID      string `json:"vm_id"`
	Kind      string `json:"kind"`
	PublicKey string `json:"public_key"`
}

func (s Service) repair(ctx context.Context, db *store.Store, r Request) (model.MutationData, error) {
	v, err := s.resolveVM(ctx, db, r)
	if err != nil {
		return model.MutationData{}, err
	}
	plan := model.NewPlan("repair", model.VMScope(v.VM.ID, v.VM.SourcePath))
	data := model.Mutation(plan)
	data.VM = &v.VM
	data.AvailableRepairs = []string{"alias", "connection", "identity", "trust-export", "host-key", "ownership-publication"}
	if r.RepairKind == "" {
		return data, nil
	}
	valid := false
	for _, kind := range data.AvailableRepairs {
		if r.RepairKind == kind {
			valid = true
		}
	}
	if !valid {
		return data, model.Fail("input.invalid", "Unknown repair kind.")
	}
	input := RepairInput{VMID: v.VM.ID, Kind: r.RepairKind}
	if r.RepairKind == "host-key" {
		if r.HostKeyFile == "" {
			return data, model.Fail("input.invalid", "Host-key repair requires an independently verified --host-key-file.")
		}
		key, e := identity.ReadPublic(r.HostKeyFile)
		if e != nil {
			return data, e
		}
		input.PublicKey = connection.PublicText(key)
		plan.Fields = append(plan.Fields, model.Field{Path: "ssh.host_key", Value: connection.Fingerprint(key), Origin: "flag", Changed: true})
	}
	plan.Add("file_publication", "Repair only the selected owned setting after checking its current contents and saved evidence.")
	data.Plan = plan
	if r.DryRun || !r.Apply {
		return data, nil
	}
	if err = s.approve(r, plan); err != nil {
		return data, err
	}
	op, err := db.Prepare(ctx, plan, input, []store.Claim{{Kind: "vm", Key: v.VM.ID, Mode: "exclusive"}}, nil)
	if err != nil {
		return data, err
	}
	data.Operation = &op.Operation
	g, err := operationGuard(ctx, db, op.Operation.ID)
	if err != nil {
		return data, err
	}
	defer g.Close()
	err = s.runRepair(ctx, db, op, input, r)
	if err != nil && knownConnectionConflict(err) {
		_ = db.Finish(context.WithoutCancel(ctx), op, "failed_known")
	}
	return s.operationResult(ctx, db, data, op), err
}
func (s Service) runRepair(ctx context.Context, db *store.Store, op store.OperationRecord, in RepairInput, r Request) error {
	v, err := db.VM(ctx, in.VMID)
	if err != nil {
		return err
	}
	if in.Kind == "ownership-publication" {
		if err = db.ReconcilePublication(ctx); err != nil {
			return err
		}
		if err = s.verifyOwnership(ctx, db, v); err != nil {
			return err
		}
		return db.Finish(ctx, op, "completed")
	}
	if err = s.verifyOwnership(ctx, db, v); err != nil {
		return err
	}
	endpoint, err := connection.ReadEndpoint(v.VM.Storage.Root, v.VM.InstanceName, v.VM.ID)
	if err != nil {
		return err
	}
	switch in.Kind {
	case "alias":
		p := filepath.Join(db.Dir, "ssh", "hosts", v.VM.ID+".conf")
		if _, e := os.Lstat(p); os.IsNotExist(e) {
			if err = db.OwnSetting(ctx, op.Operation.ID, v.VM.ID, "vm", p, "alias", ""); err != nil {
				return err
			}
		}
		err = connection.PublishAlias(ctx, db, op.Operation.ID, s.Paths.Home, s.Binary, v, endpoint)
	case "trust-export":
		p := connection.TrustPath(db.Dir, v.VM.ID)
		if _, e := os.Lstat(p); os.IsNotExist(e) {
			if err = db.OwnSetting(ctx, op.Operation.ID, v.VM.ID, "vm", p, "trust", ""); err != nil {
				return err
			}
		}
		err = connection.PublishTrust(ctx, db, op.Operation.ID, v.VM.ID)
	case "host-key":
		err = s.revokeMembers(ctx, db, op, []string{v.VM.ID})
		if err == nil {
			err = connection.ReplaceTrust(ctx, db, op, v, endpoint, in.PublicKey)
		}
	case "connection":
		c, e := s.pinned(ctx, db, v)
		if e != nil {
			return e
		}
		defer c.Close()
		if err = connection.RepairTunnel(ctx, db, op.Operation.ID, v, endpoint, c); err != nil {
			return err
		}
		env, e := decodeEnvironment(v)
		if e != nil {
			return e
		}
		err = s.prepareConnection(ctx, db, op, v, env, r, true)
	case "identity":
		if v.PendingIdentityKind == nil {
			return model.Fail("settings.edited", "No identity update is pending. Review profile apply to change owned identity settings; user edits are not reset.")
		}
		c, e := s.pinned(ctx, db, v)
		if e != nil {
			return e
		}
		defer c.Close()
		err = s.completeGuestIdentity(ctx, db, op, v, c, r)
	}
	if err != nil {
		return err
	}
	return db.Finish(ctx, op, "completed")
}

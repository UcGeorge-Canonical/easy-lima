package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

type absentRunner struct{}

func (absentRunner) Run(_ context.Context, r process.Request) (process.Result, error) {
	if r.Executable == "ps" || len(r.Args) > 0 && r.Args[0] == "list" {
		return process.Result{Exited: true, ExitCode: 0}, nil
	}
	return process.Result{}, fmt.Errorf("unexpected mutation: %s", r.Executable)
}

func TestDeleteUncreatedReservationRequiresAbsenceAndReleasesClaims(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	vmroot := filepath.Join(root, "vms")
	if err := os.Mkdir(vmroot, 0700); err != nil {
		t.Fatal(err)
	}
	volume, err := files.RootIdentity(vmroot)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Init(ctx, filepath.Join(root, "state"), vmroot, volume)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	n, err := db.Namespace(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id := model.NewID()
	source := filepath.Join(root, "project")
	op, err := db.Prepare(ctx, model.NewPlan("up", model.VMScope(id, source)), nil, []store.Claim{{Kind: "vm", Key: id, Mode: "exclusive"}, {Kind: "source", Key: source, Mode: "exclusive"}}, &store.NewVM{ID: id, NamespaceID: n.ID, InstanceName: "el-project", Alias: "easy-lima-project", SourcePath: source})
	if err != nil {
		t.Fatal(err)
	}
	v, err := db.VM(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	service := Service{Paths: config.Paths{State: db.Dir}}
	adapter := lima.Adapter{Root: vmroot, Runner: absentRunner{}}
	request := Request{Command: "delete", VMID: id, ConfirmVM: id, Yes: true}
	partial := filepath.Join(vmroot, v.VM.InstanceName)
	if err = os.Mkdir(partial, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = service.deleteReservation(ctx, db, request, v, adapter); err == nil {
		t.Fatal("retired partial instance files")
	}
	if err = os.Remove(partial); err != nil {
		t.Fatal(err)
	}
	data, err := service.deleteReservation(ctx, db, request, v, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if data.Operation == nil || data.Operation.Status != "completed" {
		t.Fatal("retirement not recorded")
	}
	claims, err := db.PendingClaims(ctx, op.Operation.ID)
	if err != nil || len(claims) != 0 {
		t.Fatalf("claims retained: %+v %v", claims, err)
	}
	old, err := db.Operation(ctx, op.Operation.ID)
	if err != nil || old.Operation.Status != "failed_known" {
		t.Fatalf("original attempt lost: %+v %v", old, err)
	}
	if err = db.VerifyProjection(ctx); err != nil {
		t.Fatal(err)
	}
}

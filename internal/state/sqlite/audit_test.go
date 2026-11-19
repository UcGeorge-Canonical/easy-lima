package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func testRegistration(t *testing.T, s *Store, source string, profile *string, claims []Claim) OperationRecord {
	t.Helper()
	ctx := context.Background()
	n, err := s.Namespace(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id := model.NewID()
	op, err := s.Prepare(ctx, model.NewPlan("up", model.VMScope(id, source)), nil, claims, &NewVM{ID: id, NamespaceID: n.ID, InstanceName: "el-" + id, Alias: "easy-lima-" + id, SourcePath: source, ProfileID: profile})
	if err != nil {
		t.Fatal(err)
	}
	return op
}
func testProfile(t *testing.T, s *Store) string {
	t.Helper()
	ctx := context.Background()
	p := config.Profile{SchemaVersion: 1, ID: model.NewID(), Label: "fixture", Git: model.GitIdentity{Name: "Fixture", Email: "fixture@example.invalid"}}
	op, err := s.Prepare(ctx, model.NewPlan("profile.import", model.Global()), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveExternalProfile(ctx, op, p, "public", "fingerprint", agent.Binding{}); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, op, "completed"); err != nil {
		t.Fatal(err)
	}
	return p.ID
}
func TestNamespaceTamperingWithoutPendingPublication(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	n, err := s.Namespace(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(n.Root, "_easy-lima", "namespace.json")
	b, err := files.Read(p, 65536)
	if err != nil {
		t.Fatal(err)
	}
	var marker NamespaceMarker
	if err = files.Unwrap(b, &marker); err != nil {
		t.Fatal(err)
	}
	marker.Epoch++
	b, err = files.Wrap(marker)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.ReconcilePublication(ctx); err == nil {
		t.Fatal("accepted a foreign namespace epoch without a pending publication")
	}
}
func TestPersistentPortOwnershipSurvivesCompletionAndReleasesOnUpdate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	claim := Claim{Kind: "port", Key: "tcp/127.0.0.1/18500", Mode: "exclusive"}
	first := testRegistration(t, s, "/project-one", nil, []Claim{claim})
	id := *first.Operation.Scope.ID
	if err := s.Finish(ctx, first, "completed"); err != nil {
		t.Fatal(err)
	}
	secondID := model.NewID()
	if _, err := s.Prepare(ctx, model.NewPlan("up", model.VMScope(secondID, "/project-two")), nil, []Claim{claim}, nil); err == nil {
		t.Fatal("completed VM lost its persistent port reservation")
	}
	update, err := s.Prepare(ctx, model.NewPlan("environment.update", model.VMScope(id, "/project-one")), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveEnvironment(ctx, update, id, model.NewID(), "digest", map[string]any{"settings": config.Settings{Ports: []config.Port{}}}, "known"); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, update, "completed"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Prepare(ctx, model.NewPlan("up", model.VMScope(secondID, "/project-two")), nil, []Claim{claim}, nil); err != nil {
		t.Fatal("removed endpoint stayed reserved", err)
	}
}
func TestPendingProfileMembershipFencesOverlapsAndRemoval(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	profile := testProfile(t, s)
	parent := testRegistration(t, s, "/group", nil, nil)
	id := *parent.Operation.Scope.ID
	if err := s.Finish(ctx, parent, "completed"); err != nil {
		t.Fatal(err)
	}
	inputs := map[string]any{"targets": []map[string]any{{"vm_id": id, "profile_id": profile}}}
	if _, err := s.Prepare(ctx, model.NewPlan("profile.assign", model.Global()), inputs, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckOverlap(ctx, "/group/child", nil, ""); err == nil {
		t.Fatal("ignored proposed membership before cutover")
	}
	// Simulate the committed assignment; removal admission must see the new member.
	if err := s.Change(ctx, Change{Kind: "profile"}, func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE registrations SET profile_id=? WHERE id=?", profile, id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Prepare(ctx, model.NewPlan("profile.remove", model.Scope{Kind: "profile", ID: &profile}), nil, []Claim{{Kind: "profile", Key: profile, Mode: "exclusive"}}, nil); err == nil {
		t.Fatal("retired a profile that gained a member")
	}
}
func TestLogRotationPreservesActiveAndAuthoritativeHistory(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	done, err := s.Prepare(ctx, model.NewPlan("fixture", model.Global()), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, done, "completed"); err != nil {
		t.Fatal(err)
	}
	active, err := s.Prepare(ctx, model.NewPlan("fixture", model.Global()), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RecordStep(ctx, active, "fixture", "may_have_dispatched", "unknown", "fixture", nil); err != nil {
		t.Fatal(err)
	}
	if err = s.pruneCompletedLogs(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(s.Dir, "logs", done.Operation.ID+".jsonl")); !os.IsNotExist(err) {
		t.Fatal("completed export not rotated")
	}
	if _, err = os.Stat(filepath.Join(s.Dir, "logs", active.Operation.ID+".jsonl")); err != nil {
		t.Fatal("active log removed", err)
	}
	if _, err = s.Operation(ctx, done.Operation.ID); err != nil {
		t.Fatal("rotation deleted authoritative history", err)
	}
}

func TestTransitionAdmissionConflictLeavesNoClaims(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	profile := testProfile(t, s)
	parent := testRegistration(t, s, "/group", nil, nil)
	child := testRegistration(t, s, "/group/child", nil, nil)
	for _, op := range []OperationRecord{parent, child} {
		if err := s.Finish(ctx, op, "completed"); err != nil {
			t.Fatal(err)
		}
	}
	var before int
	if err := s.DB.QueryRow("SELECT count(*) FROM operations").Scan(&before); err != nil {
		t.Fatal(err)
	}
	id := *parent.Operation.Scope.ID
	inputs := map[string]any{"targets": []map[string]any{{"vm_id": id, "profile_id": profile}}}
	_, err := s.Prepare(ctx, model.NewPlan("profile.assign", model.Global()), inputs, []Claim{{Kind: "profile", Key: profile, Mode: "exclusive"}, {Kind: "vm", Key: id, Mode: "exclusive"}}, nil)
	if err == nil {
		t.Fatal("accepted overlapping profiles")
	}
	var after, claims int
	if err = s.DB.QueryRow("SELECT count(*) FROM operations").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if err = s.DB.QueryRow("SELECT count(*) FROM resource_claims").Scan(&claims); err != nil {
		t.Fatal(err)
	}
	if after != before || claims != 0 {
		t.Fatal("preflight conflict retained impossible recovery claims", before, after, claims)
	}
}

func TestAdoptionCannotReuseRetiredProfile(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	profile := testProfile(t, s)
	p, err := s.Profile(ctx, profile)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Change(ctx, Change{Kind: "profile"}, func(tx *sql.Tx) error {
		_, e := tx.Exec("UPDATE profiles SET retired_at=? WHERE id=?", model.Now(), profile)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	n, err := s.Namespace(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id := model.NewID()
	input := map[string]any{"vm": NewVM{ID: id, NamespaceID: n.ID, SourcePath: "/adopted", ProfileID: &profile}, "profile_existing": true, "identity_revision_id": p.IdentityID}
	if _, err = s.Prepare(ctx, model.NewPlan("adopt", model.VMScope(id, "/adopted")), input, nil, nil); err == nil {
		t.Fatal("retired profile accepted for adoption")
	}
}

package sqlite

import (
	"context"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func TestConnectionClaimFencesRotationAndSurvivesTakeover(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id := model.NewID()
	op, err := s.Prepare(ctx, model.NewPlan("exec", model.Global()), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.HoldProfile(ctx, op, id); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Prepare(ctx, model.NewPlan("profile.lock", model.Global()), nil, []Claim{{Kind: "profile", Key: id, Mode: "exclusive"}}, nil); err == nil {
		t.Fatal("profile lock raced connection establishment")
	}
	resumed, err := s.Takeover(ctx, op.Operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.HoldProfile(ctx, op, id); err == nil {
		t.Fatal("stale claimant continued")
	}
	if err = s.HoldProfile(ctx, resumed, id); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, resumed, "completed"); err != nil {
		t.Fatal(err)
	}
	lock, err := s.Prepare(ctx, model.NewPlan("profile.lock", model.Global()), nil, []Claim{{Kind: "profile", Key: id, Mode: "exclusive"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Prepare(ctx, model.NewPlan("exec", model.Global()), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.HoldProfile(ctx, other, id); err == nil {
		t.Fatal("connection bypassed an active profile lock")
	}
	if err = s.Finish(ctx, lock, "completed"); err != nil {
		t.Fatal(err)
	}
	if err = s.HoldProfile(ctx, other, id); err != nil {
		t.Fatal(err)
	}
}

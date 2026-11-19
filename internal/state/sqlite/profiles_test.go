package sqlite

import (
	"context"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func TestAdoptedProfilePreservesExternalIdentity(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	p := config.Profile{SchemaVersion: 1, ID: model.NewID(), Label: "work", Git: model.GitIdentity{Name: "Work Developer", Email: "work@example.invalid", SignCommits: true, SignTags: true}}
	p.Key.Path = "/external/key"
	p.Key.PublicPath = "/external/key.pub"
	p.Unlock.Remember = true
	p.GitHub.Username = "work-account"
	plan := model.NewPlan("adopt", model.Scope{Kind: "profile", ID: &p.ID})
	op, err := s.Prepare(ctx, plan, map[string]string{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	binding := agent.Binding{SocketPath: "/external/socket", SupervisorKind: "external", SupervisorRef: "/external/service", OwnershipEvidenceDigest: "external"}
	if err = s.SaveExternalProfile(ctx, op, p, "public", "fingerprint", binding); err != nil {
		t.Fatal(err)
	}
	got, err := s.Profile(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile.Git != p.Git || got.Profile.GitHubUsername == nil || *got.Profile.GitHubUsername != p.GitHub.Username || got.Agent != binding || got.Profile.AgentManagement != "external" || !got.Profile.RememberEnabled || got.StoreOwnership == nil || *got.StoreOwnership != "external" {
		t.Fatalf("identity lost: %+v", got)
	}
	if err = s.VerifyProjection(ctx); err != nil {
		t.Fatal(err)
	}
}

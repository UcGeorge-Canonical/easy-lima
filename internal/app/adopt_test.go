package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacySetupReceiptMatchesPrototypeJSONAndHierarchy(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".local-launcher"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".local-launcher", "provision.sh"), []byte("fixture-script\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := legacyConfig{Defaults: map[string]any{"go_channel": "latest/stable"}, Projects: map[string]map[string]any{"group": {"rockcraft": true}, "group/project": {"go_channel": "1.23/stable"}}}
	got := legacySetup(root, "group/project", cfg)
	// Produced independently with Python hashlib(script + json.dumps(settings)).
	if got.ExpectedDigest != "4c463b225feb107cd8c36d6b0fc07d933eb8c147db31536690937fd7355fe7fa" || got.Status != "unknown" {
		t.Fatal(got)
	}
	cfg.Defaults["charmcraft"] = "invalid"
	if got := legacySetup(root, "group/project", cfg); got.ExpectedDigest != "" {
		t.Fatal("invalid legacy input received trusted history", got)
	}
}

func TestAdoptionRetiresOnlyOneExactLegacyAlias(t *testing.T) {
	path := "/volume with spaces/vm/ssh.config"
	entry := "Include \"" + path + "\"\n"
	unrelated := "Include \"" + path + ".backup\"\nHost *\n"
	got, err := retireLegacyAlias(entry+unrelated, "lima-test", path)
	if err != nil || got != unrelated {
		t.Fatalf("unrelated alias was changed: %q %v", got, err)
	}
	for _, edited := range []string{entry + entry, unrelated, "Host lima-test\n  ForwardAgent /agent\n" + unrelated, entry[:len(entry)-1] + " /another/include\n"} {
		if _, err = retireLegacyAlias(edited, "lima-test", path); err == nil {
			t.Fatalf("accepted edited legacy include: %q", edited)
		}
	}
}

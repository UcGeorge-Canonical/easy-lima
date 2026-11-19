package environment

import (
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"strings"
	"testing"
)

func TestAddingPresetDoesNotReinstallAppliedTools(t *testing.T) {
	in := Input{Settings: config.Shipped(), PreviousRevisionID: "previous", PreviousPresets: []string{"base", "development"}}
	in.Settings.Environment.Presets = []string{"base", "development", "docker"}
	foundDocker := false
	for _, stage := range Stages(in) {
		if strings.HasPrefix(stage.ID, "base.") || strings.HasPrefix(stage.ID, "dev.") {
			t.Fatal("existing tool scheduled", stage.ID)
		}
		if strings.HasPrefix(stage.ID, "docker.") {
			foundDocker = true
		}
	}
	if !foundDocker {
		t.Fatal("new preset omitted")
	}
	in.RefreshTools = true
	foundBase := false
	for _, stage := range Stages(in) {
		if stage.ID == "base.install.v1" {
			foundBase = true
		}
	}
	if !foundBase {
		t.Fatal("explicit tool refresh omitted")
	}
}

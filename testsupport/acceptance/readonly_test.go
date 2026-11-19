//go:build acceptance

package acceptance

import (
	"context"
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/app"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func verifyReadOnly(t *testing.T, s app.Service, vm *model.VM) {
	t.Helper()
	snapshot := func() map[string][32]byte {
		found := map[string][32]byte{}
		for _, root := range []string{s.Paths.Config, s.Paths.State, filepath.Join(s.Paths.Home, ".ssh")} {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if os.IsNotExist(err) {
					return nil
				}
				if err != nil {
					return err
				}
				if d.Type().IsRegular() {
					b, e := os.ReadFile(path)
					if e != nil {
						return e
					}
					found[path] = sha256.Sum256(b)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		return found
	}
	before := snapshot()
	for _, command := range []string{"list", "inspect", "doctor", "storage.inspect", "operations.list", "logs", "config.explain"} {
		r := s.Execute(context.Background(), app.Request{Command: command, Path: vm.SourcePath, PassphraseFD: -1})
		if r.ExitCode != 0 {
			t.Fatalf("read-only %s: %+v checks=%+v", command, r.Errors, r.Checks)
		}
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("diagnostics changed durable files, WAL, trust, or aliases")
	}
}

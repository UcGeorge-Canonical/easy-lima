// Package lima is the only package that knows limactl command syntax.
package lima

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

const ContractVersion = "2.2.1"

type Adapter struct {
	Executable, Root, Temp string
	Runner                 process.Runner
	Capture                io.Writer
	Output                 io.Writer
	InitialBoot            bool
}
type Observation struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Dir           string   `json:"dir"`
	Arch          string   `json:"arch"`
	VMType        string   `json:"vmType"`
	CPUs          int      `json:"cpus"`
	Memory        int64    `json:"memory"`
	Disk          int64    `json:"disk"`
	SSHLocalPort  int      `json:"sshLocalPort"`
	SSHConfigFile string   `json:"sshConfigFile"`
	SSHAddress    string   `json:"sshAddress"`
	Errors        []string `json:"errors"`
}
type Observations struct {
	Items    []Observation
	Complete bool
}

func (a Adapter) runner() process.Runner {
	if a.Runner != nil {
		return a.Runner
	}
	return process.OS{}
}
func (a Adapter) env() []string {
	return process.Environment(map[string]string{"LIMA_HOME": a.Root, "TMPDIR": a.Temp, "TMP": a.Temp, "TEMP": a.Temp, "LIMA_SSH_PORT_FORWARDER": "false", "XDG_CACHE_HOME": filepath.Join(a.Root, "_easy-lima", "cache")})
}
func (a Adapter) Probe(ctx context.Context) (string, error) {
	r, err := a.runner().Run(ctx, process.Request{Executable: a.Executable, Args: []string{"--version"}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 4096})
	if err != nil {
		return "", err
	}
	if r.ExitCode != 0 || r.Truncated {
		return "", model.Fail("dependency.unsupported", "Could not determine the installed Lima version.")
	}
	v := strings.TrimSpace(strings.TrimPrefix(string(r.Stdout), "limactl version "))
	if v != ContractVersion {
		return v, model.Fail("dependency.unsupported", "This build implements the Lima 2.2.1 contract; the installed version is different.")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return v, model.Fail("dependency.unsupported", "Only native macOS and Linux hosts are supported.")
	}
	return v, nil
}
func (a Adapter) Observe(ctx context.Context, name string) (Observation, error) {
	var empty Observation
	if !validName(name) {
		return empty, model.Fail("input.invalid", "Invalid Lima instance name.")
	}
	if _, err := os.Stat(a.Root); err != nil {
		return empty, model.Fail("storage.unavailable", "Lima storage is unavailable.")
	}
	// Read-only observations require an existing namespace. Contract tests guard
	// against list initializing it or changing any instance files.
	r, err := a.runner().Run(ctx, process.Request{Executable: a.Executable, Args: []string{"list", "--format=json"}, Env: a.env(), Timeout: 10 * time.Second, Limit: 1 << 20})
	if err != nil {
		return empty, err
	}
	if r.Truncated {
		return empty, model.Fail("lima.output_unsupported", "Lima observation exceeded its bound.")
	}
	obs, err := Decode(r.Stdout)
	if err != nil {
		return empty, err
	}
	for _, o := range obs.Items {
		if o.Name == name {
			if !obs.Complete || len(o.Errors) > 0 {
				return o, model.Fail("lima.output_unsupported", "The instance observation is incomplete.")
			}
			if o.Dir != filepath.Join(a.Root, name) {
				return o, model.Fail("ownership.conflict", "Lima reported an instance outside the configured namespace.")
			}
			return o, nil
		}
	}
	if r.ExitCode != 0 || !obs.Complete {
		return empty, model.Fail("lima.output_unsupported", "Lima could not establish complete instance absence.")
	}
	return Observation{Name: name, Status: "Absent", Dir: filepath.Join(a.Root, name)}, nil
}
func Decode(b []byte) (Observations, error) {
	out := Observations{Items: []Observation{}, Complete: true}
	d := json.NewDecoder(bytes.NewReader(b))
	for {
		var raw json.RawMessage
		err := d.Decode(&raw)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return out, model.Fail("lima.output_unsupported", "Lima returned malformed JSON.")
		}
		entries := []json.RawMessage{raw}
		if len(raw) > 0 && raw[0] == '[' {
			if json.Unmarshal(raw, &entries) != nil {
				return out, model.Fail("lima.output_unsupported", "Lima returned an invalid instance list.")
			}
		}
		for _, entry := range entries {
			var o Observation
			if json.Unmarshal(entry, &o) != nil || o.Name == "" || o.Status == "" || o.Dir == "" {
				out.Complete = false
				continue
			}
			switch o.Status {
			case "Running", "Stopped", "Broken", "Uninitialized", "Installing":
			default:
				out.Complete = false
			}
			out.Items = append(out.Items, o)
		}
	}
	return out, nil
}

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,79}$`)

func validName(s string) bool { return namePattern.MatchString(s) }
func (a Adapter) Validate(ctx context.Context, path string) error {
	r, err := a.runner().Run(ctx, process.Request{Executable: a.Executable, Args: []string{"validate", path}, Env: a.env(), Timeout: 15 * time.Second, Limit: 64 << 10})
	if err != nil {
		return err
	}
	if r.ExitCode != 0 {
		return model.Fail("config.invalid", "Lima rejected the prepared native configuration.")
	}
	return nil
}
func (a Adapter) Mutate(ctx context.Context, action, name, configPath string) (process.Result, error) {
	if !validName(name) {
		return process.Result{}, model.Fail("input.invalid", "Invalid instance name.")
	}
	if _, err := a.Probe(ctx); err != nil {
		return process.Result{}, err
	}
	if action == "create" || action == "start" {
		if _, err := ReadGlobals(a.Root, action == "create"); err != nil {
			return process.Result{}, err
		}
	}
	args := []string{action, "--tty=false"}
	switch action {
	case "create":
		if _, err := os.Lstat(filepath.Join(a.Root, name)); err == nil {
			return process.Result{}, model.Fail("ownership.conflict", "The reserved instance path already exists; reconcile it before creation.")
		}
		args = append(args, "--name", name, configPath)
	case "start", "stop", "delete":
		var existingErr error
		if action == "start" && a.InitialBoot {
			_, existingErr = files.Read(filepath.Join(a.Root, name, "lima.yaml"), 8<<20)
		} else {
			existingErr = a.Existing(name)
		}
		if err := existingErr; err != nil {
			return process.Result{}, err
		}
		args = append(args, name)
	default:
		return process.Result{}, model.Fail("input.invalid", "Unsupported Lima mutation.")
	}
	if err := files.PrivateDir(a.Temp); err != nil {
		return process.Result{}, err
	}
	return a.runner().Run(ctx, process.Request{Executable: a.Executable, Args: args, Env: a.env(), Timeout: 20 * time.Minute, Capture: a.Capture, Stdout: a.Output, Stderr: a.Output, Limit: 128 << 10})
}
func (a Adapter) Existing(name string) error {
	if !validName(name) {
		return model.Fail("input.invalid", "Invalid instance name.")
	}
	dir := filepath.Join(a.Root, name)
	for _, p := range []string{"lima.yaml", "disk"} {
		i, err := os.Lstat(filepath.Join(dir, p))
		if err != nil || !i.Mode().IsRegular() {
			return model.Fail("operation.unknown", "The registered instance configuration or disk is missing; start will not recreate it.")
		}
	}
	return nil
}

package environment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/connection"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"path/filepath"
	"strconv"
)

type Stage struct{ ID, Script string }

func Stages(in Input) []Stage {
	if in.StageManifest != nil {
		return append([]Stage{}, in.StageManifest...)
	}
	return stageManifest(in, false)
}
func stageManifest(in Input, networks bool) []Stage {
	out := []Stage{{"env.inspect.v1", "base"}, {"base.resolve.v1", "base"}, {"base.install.v1", "base"}, {"base.runtime.v1", "base"}}
	if networks && (lima.HasPreset(in.Settings, "development") || lima.HasPreset(in.Settings, "docker")) {
		out = append(out, Stage{"network.resolve.v1", "base"})
	}
	if lima.HasPreset(in.Settings, "development") {
		for _, id := range []string{"dev.resolve.v1", "dev.snapd.v1", "dev.lxd.v1", "dev.juju.v1", "dev.pool.v1", "dev.bridge.v1", "dev.profile.v1"} {
			out = append(out, Stage{id, "development"})
		}
	}
	if lima.HasPreset(in.Settings, "docker") {
		for _, id := range []string{"docker.source.v1", "docker.config.v1", "docker.install.v1", "docker.service.v1"} {
			out = append(out, Stage{id, "docker"})
		}
	}
	for i, script := range in.Scripts {
		id := "native." + script.Digest[:48] + ".v1"
		if networks {
			id = fmt.Sprintf("native.%d.%s.v1", i, script.Digest[:48])
		}
		out = append(out, Stage{id, fmt.Sprintf("native-%d", i)})
	}
	out = append(out, Stage{"access.groups.v1", "base"}, Stage{"env.verify.v1", "base"})
	if in.PreviousRevisionID != "" && !in.RefreshTools {
		existing := map[string]bool{}
		for _, preset := range in.PreviousPresets {
			existing[preset] = true
		}
		filtered := []Stage{}
		for _, st := range out {
			if strings.HasPrefix(st.ID, "base.") || strings.HasPrefix(st.ID, "dev.") && existing["development"] || strings.HasPrefix(st.ID, "docker.") && existing["docker"] {
				continue
			}
			filtered = append(filtered, st)
		}
		out = filtered
	}
	return out
}

type StageInput struct {
	SchemaVersion         int            `json:"schema_version"`
	RegistrationID        string         `json:"registration_id"`
	OperationID           string         `json:"operation_id"`
	EnvironmentRevisionID string         `json:"environment_revision_id"`
	StageID               string         `json:"stage_id"`
	StageOrdinal          int            `json:"stage_ordinal"`
	Attempt               int            `json:"attempt"`
	ScriptSHA256          string         `json:"script_sha256"`
	RunnerSHA256          string         `json:"runner_sha256"`
	HelperSHA256          string         `json:"helper_sha256"`
	Parameters            map[string]any `json:"parameters"`
}
type Receipt struct {
	SchemaVersion         int     `json:"schema_version"`
	RegistrationID        string  `json:"registration_id"`
	OperationID           string  `json:"operation_id"`
	EnvironmentRevisionID string  `json:"environment_revision_id"`
	StageID               string  `json:"stage_id"`
	StageOrdinal          int     `json:"stage_ordinal"`
	Attempt               int     `json:"attempt"`
	InputSHA256           string  `json:"input_sha256"`
	ScriptSHA256          string  `json:"script_sha256"`
	RunnerSHA256          string  `json:"runner_sha256"`
	BootID                string  `json:"boot_id"`
	Unit                  string  `json:"unit"`
	InvocationID          string  `json:"invocation_id"`
	Phase                 string  `json:"phase"`
	StartedAt             string  `json:"started_at"`
	FinishedAt            *string `json:"finished_at"`
	ExitCode              *int    `json:"exit_code"`
	ResultCode            string  `json:"result_code"`
	Substeps              []any   `json:"substeps"`
	Postconditions        []any   `json:"postconditions"`
	OutputDroppedBytes    int64   `json:"output_dropped_bytes"`
}

const installArtifact = `import os,sys,hashlib,tempfile
p,expected=sys.argv[1:]
b=sys.stdin.buffer.read(2*1024*1024+1)
if len(b)>2*1024*1024 or hashlib.sha256(b).hexdigest()!=expected: sys.exit(75)
os.makedirs(os.path.dirname(p),mode=0o700,exist_ok=True)
if os.path.lexists(p):
 if os.path.islink(p) or hashlib.sha256(open(p,'rb').read()).hexdigest()!=expected: sys.exit(76)
 sys.exit(0)
fd,tmp=tempfile.mkstemp(dir=os.path.dirname(p))
os.fchmod(fd,0o600)
with os.fdopen(fd,'wb') as f: f.write(b);f.flush();os.fsync(f.fileno())
os.rename(tmp,p)
fd=os.open(os.path.dirname(p),os.O_DIRECTORY);os.fsync(fd);os.close(fd)
`

func PutArtifact(ctx context.Context, c *connection.Client, p string, b []byte) error {
	var out, stderr bytes.Buffer
	code, err := c.Run(ctx, []string{"sudo", "python3", "-c", installArtifact, p, files.Hash(b)}, bytes.NewReader(b), &out, &stderr)
	if err != nil {
		return err
	}
	if code != 0 {
		return model.Fail("settings.edited", "The guest artifact could not be published without replacing changed contents.")
	}
	return nil
}
func Provision(ctx context.Context, s *store.Store, op store.OperationRecord, v store.VMRecord, in Input, c *connection.Client, progress func(string), capture io.Writer) error {
	runner, err := frozen(in, "guest/stage-runner.py")
	if err != nil {
		return err
	}
	helper, err := frozen(in, "guest/setup.py")
	if err != nil {
		return err
	}
	bundle := "/var/lib/easy-lima/bundles/" + in.RevisionID
	for p, b := range map[string][]byte{path.Join(bundle, "runner.py"): runner, path.Join(bundle, "setup.py"): helper} {
		if err = PutArtifact(ctx, c, p, b); err != nil {
			return err
		}
	}
	steps, err := s.Steps(ctx, op.Operation.ID)
	if err != nil {
		return err
	}
	latest := map[string]store.Step{}
	for _, st := range steps {
		latest[st.Stage] = st
	}
	for ordinal, stage := range Stages(in) {
		if err = ctx.Err(); err != nil {
			return model.Fail("operation.cancelled", "Setup was interrupted; its recorded guest attempt remains available for reconciliation.")
		}
		if progress != nil {
			progress(stage.ID)
		}
		script, err := frozen(in, "presets/"+stage.Script+".sh")
		nativeMode := ""
		if strings.HasPrefix(stage.Script, "native-") {
			index, e := strconv.Atoi(strings.TrimPrefix(stage.Script, "native-"))
			if e != nil || index < 0 || index >= len(in.Scripts) {
				return model.Fail("record.invalid", "Invalid native stage ordinal.")
			}
			ref := in.Scripts[index]
			script, err = files.Read(ref.Artifact, 65536)
			if err == nil && files.Hash(script) != ref.Digest {
				err = model.Fail("config.drift", "The frozen native script changed.")
			}
			nativeMode = ref.Mode
		}
		if err != nil {
			return err
		}
		scriptPath := path.Join(bundle, stage.Script+".sh")
		if err = PutArtifact(ctx, c, scriptPath, script); err != nil {
			return err
		}
		input := StageInput{1, v.VM.ID, op.Operation.ID, in.RevisionID, stage.ID, ordinal + 1, 1, files.Hash(script), files.Hash(runner), files.Hash(helper), map[string]any{"native_mode": nativeMode, "previous_revision_id": in.PreviousRevisionID, "previous_presets": in.PreviousPresets, "refresh_tools": in.RefreshTools, "user": c.Endpoint.User, "presets": in.Settings.Environment.Presets, "snaps": in.Snaps, "docker": in.Docker, "architecture": guestArch(), "lxd_subnet": in.LXDSubnet, "docker_subnet": in.DockerSubnet, "docker_pool": in.DockerPool}}
		if in.StageManifest != nil {
			input.Parameters["network_candidates"] = in.NetworkCandidates
		}
		if old, ok := latest[stage.ID]; ok && old.Attempt > 0 {
			input.Attempt = old.Attempt
		}
		inputBytes, err := files.Canonical(input)
		if err != nil {
			return err
		}
		inputPath := stageInputPath(bundle, input)
		if err = PutArtifact(ctx, c, inputPath, inputBytes); err != nil {
			return err
		}
		receiptPath := stageReceiptPath(input)
		if old, ok := latest[stage.ID]; ok && old.DispatchState != "none" {
			r, err := readReceipt(ctx, c, receiptPath)
			if err != nil {
				return model.Fail("operation.unknown", "The prior guest stage has no conclusive receipt; do not repeat the script. Inspect the existing unit and package state.")
			}
			if r.InputSHA256 != files.Hash(inputBytes) || r.OperationID != op.Operation.ID || r.StageID != stage.ID {
				return model.Fail("ownership.conflict", "The guest receipt differs from the frozen attempt.")
			}
			if r.Phase == "verified" {
				if _, err = c.Output(ctx, "sudo", "python3", path.Join(bundle, "setup.py"), "verify", inputPath); err != nil {
					return err
				}
				if err = captureResolution(ctx, s, c, in, stage); err != nil {
					return err
				}
				if err = s.RecordStep(ctx, op, stage.ID, "exited", "satisfied", "stage.verified", r); err != nil {
					return err
				}
				continue
			}
			// Only a quiescent, independently inspected builtin may continue
			// its missing owned substeps. Native scripts are never replayed.
			waitErr := waitStage(ctx, c, receiptPath, input, files.Hash(inputBytes))
			if waitErr == nil {
				r, err = readReceipt(ctx, c, receiptPath)
				if err != nil {
					return err
				}
				proof, err := c.Output(ctx, "sudo", "python3", path.Join(bundle, "setup.py"), "verify", inputPath)
				if err != nil {
					return err
				}
				if err = captureResolution(ctx, s, c, in, stage); err != nil {
					return err
				}
				if err = s.RecordStep(ctx, op, stage.ID, "exited", "satisfied", "stage.reconciled", map[string]any{"receipt": r, "postconditions": json.RawMessage(proof)}); err != nil {
					return err
				}
				continue
			}
			if model.AsError(waitErr).Code != "operation.partial" || strings.HasPrefix(stage.ID, "native.") {
				return waitErr
			}
			proof, err := c.Output(ctx, "sudo", "python3", path.Join(bundle, "setup.py"), "reconcile", inputPath)
			if err != nil {
				return waitErr
			}
			var decision struct {
				Disposition string `json:"disposition"`
				Stage       string `json:"stage_id"`
			}
			if json.Unmarshal(proof, &decision) != nil || decision.Disposition != "resume_owned_substeps" || decision.Stage != stage.ID {
				return waitErr
			}
			if input.Attempt >= 100 {
				return model.Fail("operation.partial", "The bounded stage retry limit was reached; inspect the retained attempts.")
			}
			if err = s.RecordStep(ctx, op, stage.ID, "exited", "partial", "stage.resume_permitted", json.RawMessage(proof)); err != nil {
				return err
			}
			input.Attempt++
			inputBytes, err = files.Canonical(input)
			if err != nil {
				return err
			}
			inputPath = stageInputPath(bundle, input)
			receiptPath = stageReceiptPath(input)
			if err = PutArtifact(ctx, c, inputPath, inputBytes); err != nil {
				return err
			}
		}

		if err = s.RecordStep(ctx, op, stage.ID, "may_have_dispatched", "unknown", "stage.dispatch", map[string]any{"input_sha256": files.Hash(inputBytes), "unit": fmt.Sprintf("el-%s-s%d-a%d.service", op.Operation.ID, ordinal+1, input.Attempt)}); err != nil {
			return err
		}
		unit := fmt.Sprintf("el-%s-s%d-a%d", op.Operation.ID, ordinal+1, input.Attempt)
		args := []string{"sudo", "systemd-run", "--unit", unit, "--service-type=exec", "--property=Restart=no", "--property=KillMode=control-group", "--property=RemainAfterExit=yes", "--property=StandardInput=null", "--property=StandardOutput=null", "--property=StandardError=null", "/usr/bin/python3", path.Join(bundle, "runner.py"), "run", inputPath, scriptPath, path.Join(bundle, "setup.py")}
		if _, err = c.Output(ctx, args...); err != nil {
			return model.Fail("operation.unknown", "Guest stage dispatch is uncertain; resume this operation to inspect the same unit.")
		}
		streamCtx, cancelStream := context.WithCancel(ctx)
		streamDone := make(chan struct{})
		if capture != nil {
			go func() {
				defer close(streamDone)
				_, _ = c.Run(streamCtx, []string{"sudo", "python3", path.Join(bundle, "runner.py"), "stream", inputPath}, nil, capture, io.Discard)
			}()
		} else {
			close(streamDone)
		}
		if err = waitStage(ctx, c, receiptPath, input, files.Hash(inputBytes)); err != nil {
			cancelStream()
			<-streamDone
			return err
		}
		cancelStream()
		<-streamDone
		receipt, err := readReceipt(ctx, c, receiptPath)
		if err != nil {
			return err
		}
		if _, err = c.Output(ctx, "sudo", "python3", path.Join(bundle, "setup.py"), "verify", inputPath); err != nil {
			return err
		}
		if err = captureResolution(ctx, s, c, in, stage); err != nil {
			return err
		}

		if err = s.RecordStep(ctx, op, stage.ID, "exited", "satisfied", "stage.verified", receipt); err != nil {
			return err
		}
	}
	return nil
}
func readReceipt(ctx context.Context, c *connection.Client, p string) (Receipt, error) {
	var r Receipt
	b, err := c.Output(ctx, "sudo", "cat", p)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(b, &r)
	return r, err
}
func waitStage(ctx context.Context, c *connection.Client, p string, in StageInput, inputHash string) error {
	deadline := time.NewTimer(30 * time.Minute)
	defer deadline.Stop()
	for {
		r, err := readReceipt(ctx, c, p)
		if err == nil {
			if r.InputSHA256 != inputHash || r.OperationID != in.OperationID || r.ScriptSHA256 != in.ScriptSHA256 || r.RunnerSHA256 != in.RunnerSHA256 {
				return model.Fail("ownership.conflict", "Guest stage evidence does not match the admitted inputs.")
			}
			if r.Phase == "verified" {
				return nil
			}
			status, e := c.Output(ctx, "sudo", "systemctl", "show", r.Unit, "--property=ActiveState", "--property=MainPID", "--property=ControlGroup")
			if e == nil {
				fields := map[string]string{}
				for _, line := range strings.Split(string(status), "\n") {
					k, v, ok := strings.Cut(line, "=")
					if ok {
						fields[k] = v
					}
				}
				inactive := (fields["ActiveState"] == "failed" || fields["ActiveState"] == "inactive") && fields["MainPID"] == "0"
				if inactive {
					// Empty or removed cgroups are required before verification.
					cg := fields["ControlGroup"]
					if cg != "" {
						_, e = c.Output(ctx, "sudo", "python3", "-c", `import pathlib,sys
p=pathlib.Path('/sys/fs/cgroup')/sys.argv[1].lstrip('/')/'cgroup.procs'
if p.exists() and p.read_text().strip(): sys.exit(75)`, cg)
					}
					if e != nil {
						return model.Fail("operation.unknown", "The stopped runner's cgroup is still active or unreadable; recovery cannot replay its effects.")
					}
					if e == nil && !strings.HasPrefix(in.StageID, "native.") {
						bundle := "/var/lib/easy-lima/bundles/" + in.EnvironmentRevisionID
						b, verify := c.Output(ctx, "sudo", "python3", bundle+"/setup.py", "verify", stageInputPath(bundle, in))
						var conditions []any
						if verify == nil && json.Unmarshal(b, &conditions) == nil {
							return nil
						}
					}
					return model.Fail("operation.partial", "The guest runner stopped with incomplete postconditions. Its recorded VM and receipt were preserved; no script was replayed.")
				}
			}
			if r.Phase == "failed" || r.Phase == "interrupted" {
				return model.Fail("operation.unknown", "Guest setup failed, but its entire unit has not been proven inactive. Inspect the retained receipt before recovery.")
			}

		}
		select {
		case <-ctx.Done():
			return model.Fail("operation.unknown", "The setup observer ended; the guest job may still be running.")
		case <-deadline.C:
			return model.Fail("operation.unknown", "The guest stage exceeded the observation deadline.")
		case <-time.After(2 * time.Second):
		}
	}
}
func guestArch() string {
	c, _ := LoadCatalog()
	_ = c
	return strings.ReplaceAll(hostArch(), "x86_64", "amd64")
}

func captureResolution(ctx context.Context, s *store.Store, c *connection.Client, in Input, stage Stage) error {
	if stage.ID == "base.resolve.v1" || stage.ID == "dev.resolve.v1" {
		group := strings.Split(stage.ID, ".")[0]
		body, e := c.Output(ctx, "sudo", "cat", "/var/lib/easy-lima/environments/"+in.RevisionID+"/"+group+".packages.json")
		if e != nil {
			return e
		}
		var resolution map[string]any
		if json.Unmarshal(body, &resolution) != nil {
			return model.Fail("record.invalid", "Invalid package resolution evidence.")
		}
		if e = MaterializeSmall(map[string][]byte{filepath.Join(s.Dir, "artifacts", files.Hash(body)+".packages.json"): body}); e != nil {
			return e
		}
	}
	return nil
}

func stageInputPath(bundle string, in StageInput) string {
	name := in.StageID + ".json"
	if in.Attempt > 1 {
		name = fmt.Sprintf("%s.a%d.json", in.StageID, in.Attempt)
	}
	return path.Join(bundle, in.OperationID, name)
}
func stageReceiptPath(in StageInput) string {
	return fmt.Sprintf("/var/lib/easy-lima/operations/%s/%s/%d/receipt.json", in.OperationID, in.StageID, in.Attempt)
}

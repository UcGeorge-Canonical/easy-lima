package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/assets"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/storage"
)

type Catalog struct {
	SchemaVersion int                        `json:"schema_version"`
	Revision      string                     `json:"revision"`
	ImageDate     string                     `json:"image_date"`
	Images        map[string]storage.Asset   `json:"images"`
	SnapChannels  map[string]string          `json:"snap_channels"`
	Snaps         map[string]map[string]Snap `json:"snaps"`
	Docker        map[string]Docker          `json:"docker"`
	Subnets       []string                   `json:"subnets"`
	Qualification []map[string]any           `json:"qualification"`
}
type Snap struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Channel  string `json:"channel"`
}
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}
type Docker struct {
	Key       string    `json:"key"`
	KeySHA256 string    `json:"key_sha256"`
	Packages  []Package `json:"packages"`
}
type Artifact struct {
	Path   string `json:"path"`
	Digest string `json:"sha256"`
}
type LegacySetup struct {
	Status         string `json:"status"`
	ExpectedDigest string `json:"expected_digest,omitempty"`
	ObservedDigest string `json:"observed_digest,omitempty"`
}

type Input struct {
	LegacySetup         *LegacySetup        `json:"legacy_setup,omitempty"`
	StageManifest       []Stage             `json:"stage_manifest,omitempty"`
	NetworkCandidates   []Networks          `json:"network_candidates,omitempty"`
	ProvisionRevisionID string              `json:"provision_revision_id"`
	Bundle              map[string]Artifact `json:"bundle"`
	PreviousRevisionID  string              `json:"previous_revision_id,omitempty"`
	PreviousPresets     []string            `json:"previous_presets,omitempty"`
	RefreshTools        bool                `json:"refresh_tools"`
	SchemaVersion       int                 `json:"schema_version"`
	RevisionID          string              `json:"revision_id"`
	Settings            config.Settings     `json:"settings"`
	Image               storage.Asset       `json:"image"`
	YAMLArtifact        string              `json:"yaml_artifact"`
	YAMLDigest          string              `json:"yaml_digest"`
	InstanceDigest      string              `json:"instance_digest"`
	Globals             []lima.GlobalInput  `json:"globals"`
	RuntimePath         string              `json:"runtime_path"`
	RuntimeVersion      string              `json:"runtime_version"`
	CatalogRevision     string              `json:"catalog_revision"`
	Scripts             []ScriptRef         `json:"scripts"`
	Snaps               map[string]Snap     `json:"snaps"`
	Docker              Docker              `json:"docker"`
	LXDSubnet           string              `json:"lxd_subnet"`
	DockerSubnet        string              `json:"docker_subnet"`
	DockerPool          string              `json:"docker_pool"`
}
type ScriptRef struct{ Mode, Artifact, Digest string }

func LoadCatalog() (Catalog, error) {
	var c Catalog
	b, err := assets.Read("catalog.json")
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return c, err
	}
	if c.SchemaVersion != 1 {
		return c, model.Fail("dependency.unsupported", "The embedded environment catalogue has an unsupported schema.")
	}
	return c, nil
}
func Resolve(ctx context.Context, root, state, source, instance, runtimePath string, s config.Settings) (Input, map[string][]byte, error) {
	c, err := LoadCatalog()
	if err != nil {
		return Input{}, nil, err
	}
	in := Input{SchemaVersion: 1, RevisionID: model.NewID(), Settings: s, RuntimePath: runtimePath, RuntimeVersion: lima.ContractVersion, CatalogRevision: c.Revision, Scripts: []ScriptRef{}, Snaps: c.Snaps[runtime.GOARCH], Docker: c.Docker[runtime.GOARCH], LXDSubnet: "10.210.0.0/24", DockerSubnet: "10.211.0.0/24", DockerPool: "10.212.0.0/16"}
	if lima.HasPreset(s, "development") || lima.HasPreset(s, "docker") {
		in.NetworkCandidates, err = networkCandidates(ctx, c.Subnets)
		if err != nil {
			return in, nil, err
		}
	}
	in.Globals, err = lima.ReadGlobals(root, true)
	if err != nil {
		return in, nil, err
	}
	template, err := assets.Read("ubuntu-24.04.yaml")
	if err != nil {
		return in, nil, err
	}
	in.Image = c.Images[runtime.GOARCH]
	if s.Environment.Template != "ubuntu-24.04" {
		template, err = lima.Template(ctx, runtimePath, s.Environment.Template)
		if err != nil {
			return in, nil, err
		}
		_, images, err := lima.NativeSettings(template)
		if err != nil {
			return in, nil, err
		}
		arch := "x86_64"
		if runtime.GOARCH == "arm64" {
			arch = "aarch64"
		}
		found := false
		for _, i := range images {
			if i.Arch != "" && i.Arch != arch {
				continue
			}
			if !strings.HasPrefix(i.Digest, "sha256:") {
				return in, nil, model.Invalid("images.digest", "custom images require a pinned SHA-256")
			}
			u, er := url.Parse(i.Location)
			if er != nil {
				return in, nil, er
			}
			if u.Scheme == "" {
				path, er := config.ResolvePath(i.Location, filepath.Dir(s.Environment.Template))
				if er != nil {
					return in, nil, er
				}
				u = &url.URL{Scheme: "file", Path: path}
			}
			if u.Scheme != "file" {
				return in, nil, model.Fail("resource.unsupported", "Custom remote images need a release catalogue with verified allocation bounds; use a local pinned image.")
			}
			st, er := os.Stat(u.Path)
			if er != nil {
				return in, nil, er
			}
			in.Image = storage.Asset{URL: u.String(), SHA256: strings.TrimPrefix(i.Digest, "sha256:"), MaxBytes: st.Size(), ExpandedBound: mustQuantity(s.Resources.Disk)}
			found = true
			break
		}
		if !found {
			return in, nil, model.Fail("resource.unsupported", "No native-architecture image is available in the template.")
		}
	}
	if in.Image.SHA256 == "" {
		return in, nil, model.Fail("dependency.unsupported", "The release has no image for this host architecture.")
	}
	if err = ValidateResources(s.Resources); err != nil {
		return in, nil, err
	}
	if lima.HasPreset(s, "development") && (len(in.Snaps) != 2 || in.Snaps["lxd"].Revision == "" || in.Snaps["juju"].Revision == "") {
		return in, nil, model.Fail("dependency.unsupported", "The development preset requires a complete pinned snap catalogue for this architecture.")
	}
	if lima.HasPreset(s, "docker") && (len(in.Docker.Packages) < 5 || in.Docker.KeySHA256 == "") {
		return in, nil, model.Fail("dependency.unsupported", "The Docker preset requires a complete pinned publisher catalogue.")
	}
	b, scripts, err := lima.Prepare(template, s, source, storage.AssetPath(root, in.Image), in.Image.SHA256)
	if err != nil {
		return in, nil, err
	}
	in.YAMLDigest = files.Hash(b)
	in.YAMLArtifact = filepath.Join(state, "artifacts", in.YAMLDigest+".yaml")
	material := map[string][]byte{in.YAMLArtifact: b}
	for _, script := range scripts {
		digest := files.Hash([]byte(script.Script))
		p := filepath.Join(state, "artifacts", digest+".sh")
		in.Scripts = append(in.Scripts, ScriptRef{script.Mode, p, digest})
		material[p] = []byte(script.Script)
	}
	frozen, err := Freeze(&in, state)
	if err != nil {
		return in, nil, err
	}
	for p, b := range frozen {
		material[p] = b
	}
	return in, material, nil
}

// ValidateResources is shared by creation and explicit resource updates.
// It validates one VM's limits, without pretending to reserve host capacity.
func ValidateResources(r config.Resources) error {
	if r.CPUs < 1 || r.CPUs > runtime.NumCPU() {
		return model.Fail("resource.unsupported", "The requested CPU count is outside host capacity.")
	}
	memory, err := config.Quantity(r.Memory)
	if err != nil {
		return err
	}
	disk, err := config.Quantity(r.Disk)
	if err != nil {
		return err
	}
	if memory < 2<<30 || disk < 16<<30 {
		return model.Fail("resource.unsupported", "Ubuntu requires at least 2 GiB of memory and a 16 GiB disk maximum.")
	}
	total, err := hostMemory()
	if err != nil {
		return model.Fail("check.incomplete", "Host memory capacity could not be inspected.")
	}
	if uint64(memory) > total {
		return model.Fail("resource.unsupported", "The requested guest memory exceeds the host's physical memory.")
	}
	return nil
}

// Freeze snapshots executable setup inputs in the admission record. A newer
// binary never substitutes its embedded runner into a previously started VM.
func Freeze(in *Input, state string) (map[string][]byte, error) {
	in.StageManifest = stageManifest(*in, true)
	in.Bundle = map[string]Artifact{}
	in.ProvisionRevisionID = in.RevisionID
	out := map[string][]byte{}
	for _, name := range []string{"guest/stage-runner.py", "guest/setup.py", "presets/base.sh", "presets/development.sh", "presets/docker.sh"} {
		b, e := assets.Read(name)
		if e != nil {
			return nil, e
		}
		h := files.Hash(b)
		p := filepath.Join(state, "artifacts", h+filepath.Ext(name))
		in.Bundle[name] = Artifact{Path: p, Digest: h}
		out[p] = b
	}
	return out, nil
}
func frozen(in Input, name string) ([]byte, error) {
	ref, ok := in.Bundle[name]
	if !ok {
		return nil, model.Fail("record.invalid", "The frozen setup artifact is missing; a newer binary cannot replace it.")
	}
	b, e := files.Read(ref.Path, 2<<20)
	if e != nil {
		return nil, e
	}
	if files.Hash(b) != ref.Digest {
		return nil, model.Fail("config.drift", "A frozen setup artifact changed.")
	}
	return b, nil
}
func mustQuantity(s string) int64 { v, _ := config.Quantity(s); return v }
func MaterializeSmall(material map[string][]byte) error {
	for path, b := range material {
		if err := files.PrivateDir(filepath.Dir(path)); err != nil {
			return err
		}
		d, err := files.Digest(path)
		if err != nil {
			return err
		}
		if d != "" && d != files.Hash(b) {
			return model.Fail("config.drift", "An immutable environment artifact changed.")
		}
		if err = files.Replace(path, b, d, 0600); err != nil {
			return err
		}
	}
	return nil
}
func Read(in Input) ([]byte, error) {
	b, err := files.Read(in.YAMLArtifact, 8<<20)
	if err != nil {
		return nil, err
	}
	if files.Hash(b) != in.YAMLDigest {
		return nil, model.Fail("config.drift", "The frozen native template changed.")
	}
	return b, nil
}
func Diff(old, new config.Settings) []model.Field {
	out := []model.Field{}
	a := map[string]any{"resources.cpus": old.Resources.CPUs, "resources.memory": old.Resources.Memory, "environment.presets": old.Environment.Presets, "mounts": old.Mounts, "ports": old.Ports}
	b := map[string]any{"resources.cpus": new.Resources.CPUs, "resources.memory": new.Resources.Memory, "environment.presets": new.Environment.Presets, "mounts": new.Mounts, "ports": new.Ports}
	for _, key := range []string{"resources.cpus", "resources.memory", "environment.presets", "mounts", "ports"} {
		if fmt.Sprint(a[key]) != fmt.Sprint(b[key]) {
			out = append(out, model.Field{Path: key, Value: b[key], Origin: "flag", Changed: true})
		}
	}
	return out
}

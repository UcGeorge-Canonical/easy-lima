# Configuration

[Documentation home](README.md) · [Complete TOML reference](reference/toml.md) · [Native YAML reference](reference/lima-yaml.md)

All easy-lima TOML files require `schema_version = 1`. Unknown fields and malformed values are rejected. Native Lima configuration remains YAML.

## Locations

| Data | macOS | Linux |
| --- | --- | --- |
| User configuration | `~/Library/Application Support/easy-lima/config.toml` | `${XDG_CONFIG_HOME:-~/.config}/easy-lima/config.toml` |
| Desired profiles and generated keys | `profiles/` and `keys/` beside user configuration | Same |
| Authoritative state | `~/Library/Application Support/easy-lima/state/` | `${XDG_STATE_HOME:-~/.local/state}/easy-lima/` |
| Small runtime cache | `~/Library/Caches/easy-lima/` | `${XDG_CACHE_HOME:-~/.cache}/easy-lima/` |
| Project configuration | `easy-lima.toml` in the exact selected folder | Same |
| VM data and image cache | Explicit `storage.root` | Explicit `storage.root` |

XDG overrides must be absolute. Long home paths may require a private short socket directory under the operating system's temporary directory. This contains sockets, not VM disks.

`--config FILE` and `--project-config FILE` select configuration inputs for one invocation. They do not create a different registry or change its storage ownership. Relative paths in a file resolve beside that file; CLI paths resolve from the original CWD. Only a leading `~/` expands. Shell variables and command substitutions are not evaluated.

## User defaults

```toml
schema_version = 1
[storage]
root = "/chosen/volume/vms"
[runtime]
limactl = "/chosen/bin/limactl"
[defaults.environment]
template = "ubuntu-24.04"
presets = ["base"]
[defaults.resources]
cpus = 4
memory = "8GiB"
disk = "80GiB"
[ui]
open_action = "code"
show_hidden = false
show_dependencies = false
```

`init` writes the resolved Lima executable. Keep that generated path or select a verified installation explicitly with `--limactl`.

CPU and memory requests are checked against host capability. Disk is a creation-time maximum. Quantities use whole `MiB`, `GiB`, or `TiB`, not `GB`. Requests are rejected rather than silently reduced. Base guests require at least 2 GiB RAM and a 16 GiB maximum disk.

## Project configuration

```toml
schema_version = 1
[environment]
template = "ubuntu-24.04"
presets = ["development", "docker"]
[resources]
cpus = 4
memory = "8GiB"
disk = "80GiB"
[identity]
profile_label = "work"
[[mounts]]
source = "../shared-data"
target = "/data"
writable = false
[[ports]]
guest = 8080
local = 18080
protocol = "tcp"
host_address = "127.0.0.1"
guest_address = "127.0.0.1"
```

The profile label is a request to resolve locally. It cannot silently assign a key from a repository-controlled file. Confirm it by selecting the profile in the browser or using `--profile work`. `--no-profile` explicitly chooses a VM without a Git profile.

For new VMs, resource precedence is native template → user defaults → project → flags. Template and preset selection is shipped defaults → user → project → flags. Lists replace lower-priority wrapper lists; `[]` explicitly clears one. Supported custom native entries are retained and shown in the effective plan.

Existing VMs use their recorded inputs. Editing creation defaults does not change them. Request a preview with:

```sh
easy-lima config explain ./service
easy-lima environment update ./service --from-project
easy-lima environment update ./service --from-project --apply
```

`--from-project` cannot be mixed with explicit setting overrides. Resource updates can change CPU and RAM but not resize the disk in v1. Removing an already applied preset requires a new VM; easy-lima does not guess how to uninstall user-modified software.

## Presets and native YAML

`base` is always included. `development` adds Snap, AppArmor utilities, LXD on `5.21/stable`, and Juju on `3/stable`. `docker` adds Docker Engine, Buildx, and Compose. The embedded catalog records resolved image hashes, Snap revisions, and Docker package versions. Installation and refresh use recorded artifacts and publisher checks. Ubuntu package dependency resolution is recorded per operation; this is not a fully reproducible OS package lock.

Use `--template /path/to/template.yaml` for a local Lima template. Local YAML `base` includes are frozen and merged using the installed Lima CLI. Remote includes, arbitrary host networks, additional disks, kernel/firmware overrides, and unsupported automatic hooks are rejected. Inline one-time `system` and `user` provisioning scripts become recorded guest stages. They are never automatically replayed after an uncertain attempt.

Custom templates must satisfy the same mount, transport, resource, and exposure rules. They do not gain the tested Ubuntu preset's compatibility guarantee. Details are in the [native boundary contract](design/LIMA_BOUNDARIES.md).

## Mounts and ports

The selected directory is writable at `/workspace`. Extra mounts are read-only unless requested otherwise. Overlapping guest targets, replacing `/workspace`, and exposing easy-lima state or configured private keys are rejected. Do not select a host folder containing secrets that you do not want the guest to read.

Persistent mappings accept TCP or UDP, literal guest IPs, and host `127.0.0.1` or `::1` only. Automatic broad forwarding is suppressed. The host address is not implicitly dual-stack. Structured `--ports-from FILE` and `--mounts-from FILE` inputs use `schema_version = 1` plus the respective arrays shown above.

A temporary TCP tunnel runs in the foreground:

```sh
easy-lima forward ./service 8080 --local 18080
```

It requires an already running VM and closes when cancelled. `exec` similarly requires a running VM unless `--start` is given.

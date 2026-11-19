# TOML reference

[Documentation home](../README.md) · [Configuration overview](../configuration.md) · [Examples](../examples/README.md)

Every easy-lima TOML file starts with `schema_version = 1`. Unknown keys, duplicate keys, unsupported schema versions and invalid values are errors. Files are limited to 1 MiB and individual strings to 64 KiB. Native Lima templates are separate YAML files; see the [YAML reference](lima-yaml.md).

## Files and discovery

| File | Location and purpose |
| --- | --- |
| User | macOS: `~/Library/Application Support/easy-lima/config.toml`; Linux: `${XDG_CONFIG_HOME:-~/.config}/easy-lima/config.toml`. Created by `init`. |
| Project | **`easy-lima.toml`**, in the exact selected directory. No leading dot. Parent directories are not searched. Optional. |
| Profile | `profiles/IMMUTABLE_PROFILE_ID.toml` beside the user configuration. Created/imported with `profile` commands. |
| Mount input | Any file passed with `--mounts-from FILE`; requires `schema_version` and `mounts`. |
| Port input | Any file passed with `--ports-from FILE`; requires `schema_version` and `ports`. |

`--config FILE` and `--project-config FILE` select existing inputs for an invocation. They do not establish another registry. An explicitly supplied missing file is an error. Linux XDG location overrides must be absolute. Paths inside a file resolve beside that declaring file, after resolving its parent directory symlinks. CLI paths resolve from the invocation CWD. A leading `~/` expands to the host home; environment variables, globs and shell substitutions are not expanded. Configuration inputs must be regular files, not symlinks.

## User file

```toml
schema_version = 1

[storage]
root = "/Volumes/Work/easy-lima-vms"

[runtime]
limactl = "/opt/homebrew/bin/limactl"

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

[editor]
executable = "code"
args = ["--remote", "ssh-remote+{alias}", "{workspace}"]
```

The paths above are examples. Keep the paths generated for your own system by setup.

| Key | Type / default | Meaning |
| --- | --- | --- |
| `schema_version` | integer; required `1` | Configuration format. |
| `storage.root` | path; explicitly chosen at init | Sole VM namespace/root. Editing it does not move or rebind storage. |
| `runtime.limactl` | executable name or path | Installed Lima CLI. Init records its resolved path; `--limactl` overrides this invocation. |
| `defaults.environment.template` | string; `ubuntu-24.04` | Shipped template name or local native YAML path. |
| `defaults.environment.presets` | string array; `["base"]` | `base`, `development`, `docker`. Base is always included. |
| `defaults.resources.cpus` | integer; `4` | Creation request, 1–1024 and within host capability. |
| `defaults.resources.memory` | quantity; `8GiB` | Creation RAM request, within host capability. |
| `defaults.resources.disk` | quantity; `80GiB` | Sparse maximum disk size, not a reservation. Immutable after creation in v1. |
| `defaults.mounts` | mount array; `[]` | Creation default mounts in addition to `/workspace`. |
| `defaults.ports` | port array; `[]` | Creation default persistent port mappings. |
| `ui.open_action` | `code` or `ssh`; `code` | Preferred open action in the browser. |
| `ui.show_hidden` | boolean; `false` | Initial visibility of hidden folders. |
| `ui.show_dependencies` | boolean; `false` | Initial visibility of `node_modules`, `.venv`, `venv`, `vendor`, `__pycache__`, `.git`, `.cache`. |
| `editor.executable` | executable name or path; `code` when empty | VS Code-compatible launcher. |
| `editor.args` | string array; example above | Arguments passed directly, never through a shell. Only `{alias}` and `{workspace}` placeholders are supported. |

Quantities use a positive whole number followed by `MiB`, `GiB`, or `TiB`; `8GB`, fractions and bare numbers are invalid. Base requires at least 2 GiB RAM and a 16 GiB disk maximum. Resource requests are rejected rather than silently reduced. Use `[[defaults.mounts]]` and `[[defaults.ports]]` for user defaults, with the fields below.

## Project file

Save as `easy-lima.toml`:

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
```

| Key | Type | Meaning |
| --- | --- | --- |
| `schema_version` | required integer `1` | Format version. |
| `environment.template` | string | Same rules as user template. |
| `environment.presets` | string array | Same preset rules as user defaults. |
| `resources.cpus`, `.memory`, `.disk` | integer / quantity strings | Override creation resource defaults. |
| `identity.profile_label` | string | Locally resolved profile request. Must be confirmed with explicit selection or `--profile LABEL`; repository content cannot silently select credentials. |
| `mounts` | array | Additional host mounts. |
| `ports` | array | Persistent explicit loopback mappings. |

All fields except `schema_version` are optional. Template/preset precedence for creation: shipped defaults → user → project → CLI. Resources: selected native template → user → project → CLI. Later layers replace an entire wrapper list; `[]` clears it. The required base environment and `/workspace` mount remain. Existing VMs use recorded values until an explicit environment update. A project-based update changes supplied project fields, preserving omitted recorded settings; it does not adopt unrelated new user defaults.

`identity.profile_label` follows the label pattern below. `--no-profile` overrides a project identity request explicitly. Parent/child folders with different profiles are rejected because their mounted checkouts overlap.

## Mounts and ports

```toml
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

| Mount field | Type / default | Constraint |
| --- | --- | --- |
| `source` | required host path | Existing directory. Relative to declaring file. Exposing managed state/private key directories is rejected. |
| `target` | required guest path | Absolute directory other than `/`, outside `/workspace`; mount targets cannot overlap. |
| `writable` | boolean; `false` | Guest writes affect the host when true. |

The selected project itself is always mounted writable at `/workspace`. There are at most 256 extra mounts. Selecting a broad host folder exposes its contents to the guest; review the resolved sources in the creation plan.

| Port field | Type / default | Constraint |
| --- | --- | --- |
| `guest` | required integer | 1–65535; guest SSH port 22 is reserved. |
| `local` | required integer | 1–65535; exact host port, no automatic substitution. |
| `protocol` | string; `tcp` | `tcp` or `udp`. |
| `host_address` | string; `127.0.0.1` | Literal `127.0.0.1` or `::1`; IPv4 and IPv6 are distinct bindings. |
| `guest_address` | string; `127.0.0.1` | Literal guest IP, not a DNS name. |

There are at most 256 mappings. Duplicate endpoints are rejected; broad automatic forwarding is disabled. Ports use the same guest IP in the forwarding request, so match the address your application listens on.

A separate `--mounts-from`/`--ports-from` file contains `schema_version = 1` and only that array. To clear one, use `mounts = []` or `ports = []` at the TOML root, before any table header.

## Profile file

Prefer `profile create`, `profile import`, and `profile edit`; these write and validate the desired file. `profile apply LABEL` previews and applies desired changes. Editing the file alone never rotates an active key or changes existing VM identity.

```toml
schema_version = 1
id = "0123456789abcdef0123456789abcdef"
label = "work"

[git]
name = "Example Developer"
email = "developer@example.com"
sign_commits = true
sign_tags = true

[github]
username = "ExampleWork"

[key]
path = "/absolute/host/path/work_ed25519"
public_path = "/absolute/host/path/work_ed25519.pub"
fingerprint = "SHA256:RECORDED_FINGERPRINT"

[unlock]
remember = false
credential_ref = ""
```

This illustrates the schema, not a profile to import verbatim. Keep the real generated ID, fingerprint and credential reference.

| Key | Constraint / behavior |
| --- | --- |
| `id` | Immutable 32 lowercase hex characters, matching the filename. |
| `label` | 1–48 lowercase letters, digits or hyphens, starting with a letter; unique locally. |
| `git.name` | Explicit trimmed author name, 1–256 bytes, no control characters. |
| `git.email` | Explicit single email address, at most 254 bytes. Never inferred from host Git config. |
| `git.sign_commits`, `git.sign_tags` | Booleans, default true; use SSH signatures with the profile key. |
| `github.username` | Optional expected GitHub username. Online checks compare the authenticated account against it. |
| `key.path` | Host private key path; never copied into the guest. |
| `key.public_path` | Public key file; defaults to private path plus `.pub`. |
| `key.fingerprint` | Recorded public fingerprint; the observed key must match. |
| `unlock.remember` | Opt-in native credential storage for an encrypted owned key. An explicit lock inhibits automatic unlock. |
| `unlock.credential_ref` | Opaque, app-managed store reference; not a passphrase. Preserve imported references; arbitrary replacement is rejected. |

An adopted external agent has restricted capabilities until an explicit migration. See [profiles](../profiles.md) before changing ownership, forgetting credentials, assigning a new profile or rotating keys.

# Native Lima YAML reference

[Documentation home](../README.md) · [TOML reference](toml.md)

Use easy-lima TOML for ordinary resource, preset, mount and port settings. Choose a local native Lima YAML file when you need template-level configuration:

```sh
easy-lima up ./service --template /path/to/ubuntu.yaml --profile work --dry-run
```

Or set `template = "./ubuntu.yaml"` under `[environment]` in the project's `easy-lima.toml`. The path resolves relative to that TOML file.

## Minimal template

Use an image for the host's native architecture, and supply its independently obtained SHA-256:

```yaml
cpus: 4
memory: 8GiB
disk: 80GiB
images:
  - location: ./ubuntu-24.04-arm64.img
    arch: aarch64
    digest: sha256:REPLACE_WITH_THE_VERIFIED_IMAGE_DIGEST
```

This is an illustrative schema: supply an existing local image and its verified digest. Custom remote images are rejected in v1 because their allocation bounds are not known; the shipped template has a verified download and allocation contract. For a ready-to-use Ubuntu image, select the shipped `ubuntu-24.04` template instead. It records verified publisher image metadata. Custom templates must support the required Ubuntu provisioning contracts; arbitrary guest operating systems are not supported in v1.

## Supported and managed fields

| Native input | easy-lima behavior |
| --- | --- |
| `images` | Selects an existing local native-architecture image with its frozen digest; staging stays under the selected storage root. |
| `cpus`, `memory`, `disk` | Supply initial resources. User/project/CLI wrapper overrides take precedence; disk cannot be resized by an update. |
| Local `base` includes | Resolved and frozen through the installed Lima CLI; remote includes are rejected. |
| `mounts` | Supported directory entries become explicit mount requests; the owned `/workspace` source is the selected project. |
| `portForwards` | Supported exact TCP/UDP loopback entries become explicit requests. Ranges, broad exposure and socket mappings are not application-port rules. |
| Inline `provision` scripts | Only one-time `system` or `user` scripts, with only `mode` and `script` keys. Frozen as explicit guest stages; maximum 64 KiB per script. |
| Other supported native nodes | Preserved where they do not contradict managed fields or the boundary checks; Lima also validates the resulting YAML. |

A script example:

```yaml
provision:
  - mode: user
    script: |
      #!/bin/sh
      set -eu
      mkdir -p "$HOME/example-tools"
```

Custom scripts run only as admitted setup stages. If their outcome is unknown, recovery observes the durable receipt and stops for instructions when needed. V1 does not invent a rollback or blindly rerun arbitrary scripts. Never put secrets in templates: frozen inputs are retained for recovery.

The following are deliberately owned by easy-lima: native architecture and VM driver (`vz` on macOS, `qemu` on Linux), downloaded image path, `/workspace`, explicit resource overrides, SSH transport, agent isolation, forwarding suppression, and the namespace paths. Ambient host SSH keys are not injected. Lima's broad containerd defaults and proxy propagation are disabled. Use the Docker preset if you need Docker.

`networks`, `additionalDisks`, `firmware`, `kernel`, `initrd`, `param`, `probes`, and `copyToHost` with semantic content are rejected where unsupported by the asset/execution contract. YAML anchors, aliases, duplicate keys, multiple documents and mappings deeper than 64 levels are rejected. Template input is bounded to 8 MiB.

## Namespace files and manual edits

The chosen storage root is the `LIMA_HOME` used by easy-lima. Its `_config/default.yaml`, `_config/override.yaml`, and creation-time `_config/base.yaml` may be absent, empty or comment-only. Semantic global defaults are rejected so they cannot silently change a VM's reviewed configuration. Consumed bytes are recorded; even comment changes require explicit acceptance:

```sh
easy-lima environment update ./service --accept-native-config
easy-lima environment update ./service --accept-native-config --apply
```

Native edits must still pass the managed boundary rules. Normal start never silently accepts drift. Use `environment update` for planned resource/mount/port changes and `rebind` for moving a project. The [boundary specification](../design/LIMA_BOUNDARIES.md) describes the exact supported subset for maintainers.

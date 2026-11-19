# Command reference

[Documentation home](../README.md) · [Everyday examples](../usage.md) · [TOML reference](toml.md)

Run `easy-lima` without a command to open the browser in the current directory. An existing directory as the first argument changes the browsing root. Shared browser inputs are `--config`, `--project-config`, and `--limactl`.

Run any command with `--help` for its exact grammar. `PATH` defaults to the original CWD. Select by exact folder or `--vm FULL_VM_ID`, never both. Operation and VM IDs are full immutable 32-character hex identifiers.

## Shared flags

| Flag | Behavior |
| --- | --- |
| `--config FILE` | Existing user TOML input; keeps the same registry. |
| `--project-config FILE` | Existing project TOML input. |
| `--limactl FILE` | Explicit installed Lima CLI override, version checked. |
| `--json` | One result object for metadata commands; noninteractive. Not accepted by raw terminal commands. |
| `--dry-run` | Preview a mutation without applying it. |
| `--yes` | Accept a concrete effect plan; does not bypass explicit profile lock, key authentication, or delete ID confirmation. |
| `--capture-output` | Opt in to saving raw command output, with known secret patterns excluded. Default operational logs contain no raw output. |

Flags can precede or follow the command, before `--`. Scalar flags cannot repeat. Booleans accept `true` or `false`; a bare boolean means true. `--vm` repeats only for profile apply/assign/unassign; `--repository` repeats for doctor.

## Commands

| Syntax | Specific flags |
| --- | --- |
| `easy-lima adopt --from-prototype PATH --project PATH` | `--from-prototype`, `--project`, `--passphrase-fd` |
| `easy-lima code [PATH]` | `--vm`, `--no-start`, `--passphrase-fd` |
| `easy-lima config explain [PATH]` |  |
| `easy-lima delete [PATH] --confirm-vm ID` | `--vm`, `--confirm-vm` |
| `easy-lima doctor [PATH] [--online]` | `--vm`, `--online`, `--repository` |
| `easy-lima environment update [PATH] [--apply]` | `--vm`, `--apply`, `--template`, `--presets`, `--cpus`, `--memory`, `--disk`, `--mounts-from`, `--ports-from`, `--refresh-tools`, `--from-project`, `--accept-native-config`, `--passphrase-fd` |
| `easy-lima exec [PATH] [--start] -- COMMAND ARG…` | `--vm`, `--start`, `--passphrase-fd` |
| `easy-lima forward [PATH] GUEST_PORT --local HOST_PORT` | `--vm`, `--local`, `--bind`, `--passphrase-fd` |
| `easy-lima init --storage-root PATH` | `--storage-root`, `--open-action`, `--editor` |
| `easy-lima inspect [PATH]` | `--vm` |
| `easy-lima list [--profile LABEL]` | `--profile`, `--limit`, `--cursor` |
| `easy-lima logs [PATH]` | `--vm`, `--follow`, `--limit`, `--cursor` |
| `easy-lima operations inspect ID` |  |
| `easy-lima operations list` | `--limit`, `--cursor` |
| `easy-lima operations resume ID` | `--passphrase-fd` |
| `easy-lima profile apply LABEL [--vm ID]` | `--vm`, `--passphrase-fd` |
| `easy-lima profile assign LABEL --vm ID` | `--vm`, `--passphrase-fd` |
| `easy-lima profile check LABEL` | `--online` |
| `easy-lima profile create LABEL --git-name NAME --git-email EMAIL` | `--git-name`, `--git-email`, `--github-user`, `--sign-commits`, `--sign-tags`, `--remember`, `--passphrase-fd` |
| `easy-lima profile edit LABEL` | `--key`, `--public-key`, `--git-name`, `--git-email`, `--github-user`, `--sign-commits`, `--sign-tags`, `--remember`, `--label` |
| `easy-lima profile forget LABEL` |  |
| `easy-lima profile import LABEL --key PATH --git-name NAME --git-email EMAIL` | `--key`, `--public-key`, `--git-name`, `--git-email`, `--github-user`, `--sign-commits`, `--sign-tags`, `--remember`, `--passphrase-fd` |
| `easy-lima profile list` | `--limit`, `--cursor` |
| `easy-lima profile lock LABEL` |  |
| `easy-lima profile migrate-agent LABEL` | `--passphrase-fd` |
| `easy-lima profile remove LABEL` |  |
| `easy-lima profile show LABEL` |  |
| `easy-lima profile unassign --vm ID` | `--vm` |
| `easy-lima profile unlock LABEL` | `--passphrase-fd` |
| `easy-lima rebind --vm ID --to PATH` | `--vm`, `--to`, `--passphrase-fd` |
| `easy-lima repair [PATH] [--kind KIND --apply]` | `--vm`, `--kind`, `--apply`, `--host-key-file`, `--passphrase-fd` |
| `easy-lima restart [PATH]` | `--vm`, `--passphrase-fd` |
| `easy-lima ssh [PATH]` | `--vm`, `--no-start`, `--passphrase-fd` |
| `easy-lima start [PATH]` | `--vm`, `--passphrase-fd` |
| `easy-lima state backup [--to PATH]` | `--to` |
| `easy-lima state checkpoint [--mode passive\|full\|restart\|truncate]` | `--mode`, `--timeout` |
| `easy-lima state inventory refresh [--vm ID]` | `--vm` |
| `easy-lima state migrate [--apply]` | `--apply` |
| `easy-lima state restore BACKUP_PATH [--apply]` | `--apply` |
| `easy-lima stop [PATH]` | `--vm` |
| `easy-lima storage inspect` |  |
| `easy-lima up [PATH]` | `--vm`, `--profile`, `--no-profile`, `--template`, `--presets`, `--cpus`, `--memory`, `--disk`, `--mounts-from`, `--ports-from`, `--passphrase-fd` |
| `easy-lima version` |  |

`--presets` is a comma-separated list. `--mounts-from` and `--ports-from` read schema-versioned TOML arrays. CPU is a positive integer; memory and disk use `MiB`, `GiB` or `TiB`. `--repository` takes an absolute **guest** directory. `--passphrase-fd` is an inherited descriptor of at least 3; never pass a secret as a command argument or environment variable.

`environment update` and `repair` preview by default; add `--apply` to request execution. `profile edit` changes desired TOML; `profile apply` changes the effective profile. `rebind` uses `--vm` and `--to` and preserves disk/alias. Adoption accepts only the supported prototype under the same storage root. See [recovery](../recovery.md).

`ssh`, `exec`, `forward`, and `code` do not support `--json`. `exec` requires a literal `--` followed by the guest command; other commands use `--` to end option parsing for paths such as `./-project`. `logs --follow` streams until cancelled and is incompatible with JSON. Lists accept `--limit` from 1 to 1000 and return `next_cursor` when another page exists; cursors expire when registry generation changes.

## Exit statuses

| Code | Meaning |
| --- | --- |
| 0 | Completed, or a successful preview. |
| 2 | Invalid input or configuration. |
| 3 | Missing or unsupported dependency. |
| 4 | Decision, ownership, settings or binding conflict. |
| 5 | Unlock or credential interaction required. |
| 6 | Partial, unknown, unavailable state, or unverified provider result. |
| 7 | A check or operation failed. |
| 8 | Requested target not found. |
| 9 | Storage, resource or port availability prevents the request. |
| 130 | Cancelled. Already dispatched effects can still need reconciliation. |
| 255 | `exec` could not establish the guest exit status. |

When `exec` obtains a guest exit status, it returns that exact status. Read JSON `errors`, `checks`, `operation_id` and each `next_action` rather than inferring a cause from an exit number alone.

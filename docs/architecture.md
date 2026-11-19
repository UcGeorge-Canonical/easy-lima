# Architecture

easy-lima is a foreground application with shared CLI and TUI orchestration. Lima owns VM runtime processes. The host's service manager, or a verified detached process on Linux without a user manager, keeps each owned SSH agent available after command exit.

```text
cmd/easy-lima
    └── internal/ui/{cli,tui}
            └── internal/app
                ├── config / project / model
                ├── state/sqlite ── witnessed metadata + diagnostic inventory
                ├── host/lima ──── installed limactl
                ├── environment ── frozen inputs + guest stage receipts
                ├── identity / connection ── pinned SSH + per-profile agent
                └── storage / diagnostics / events
```

## Boundaries

`internal/app` validates intent, previews effects, acquires durable resource claims, records an operation, and dispatches effects. Frontends submit the same request types. `internal/model` owns public results and classified errors; human rendering is separate from JSON metadata.

`internal/host/lima` is the only component that knows `limactl` syntax. It checks the exact supported version, interprets bounded JSON observations, freezes local YAML base graphs, inventories runtime assets, and rejects unsupported global/native inputs. It does not replace Lima's runtime or automatically install it.

`internal/environment` freezes content-addressed templates, guest helpers, stage manifests, catalog selections, and package resolution. The Python runner in `assets/guest` writes durable receipts around systemd jobs. Recovery reconciles job identity and postconditions before deciding whether continuation is safe. Native scripts keep an explicit unknown outcome when replay cannot be justified.

`internal/connection` separates VM login keys from profile Git keys. VM host keys are pinned at authorized creation/adoption. Guest Git authentication uses one dedicated profile agent and strict provider host keys. Managed SSH aliases invoke the installed binary to check ownership and pending identity before opening a connection.

`internal/host/credential` uses native macOS Keychain APIs or Linux Secret Service. Passphrases are never sent as process arguments. `internal/host/agent` verifies owned supervisors and exact offered keys; externally managed agents cannot be administered implicitly.

## Persistence and concurrency

SQLite is authoritative. Transactions record project bindings, revisions, profiles, claims, operation attempts, steps, owned settings, and publication evidence. WAL and `synchronous=FULL` are explicit. Checkpointing is an explicit maintenance action; closing the database does not silently checkpoint it.

Critical changes have an integrity witness and publication protocol for external owner markers. The read-only recovery inventory is a derived diagnostic view. Restore validates the database and witness/publication relationship instead of reconstructing ownership from directory names or inventory files.

Filesystem locks coordinate local callers; durable claims retain exclusivity across process crashes. Resource claims cover VMs, profile transitions, project bindings, storage capacity, and ownership. Slow external operations happen outside short SQLite transactions. Failed observations never imply absence.

## Repository layout

| Path | Purpose |
| --- | --- |
| `cmd/` | Executable entry point. |
| `internal/` | Application logic and host adapters; no public Go SDK. |
| `assets/` | Embedded presets, templates, pinned catalog, and guest scripts. |
| `testdata/` | Public interface fixtures and the pinned legacy ownership fixture. |
| `testsupport/` | Guest-script contracts and opt-in real VM acceptance. |
| `packaging/` | Shell completions. |
| `scripts/` | License inventory, release packaging, and documentation checks. |
| `docs/*.md` | Current user/developer guides. |
| `docs/design/` | Approved design and decision records. |
| `docs/reviews/` | Historical EGM review evidence. |
| `.github/` | CI and contributor/security guidance. |

Generated `bin/`, `dist/`, `.cache/`, and `test-results/` are ignored. No private keys, local runtime state, or VM disks belong in the repository.

## Limits

V1 uses one storage root and one optional profile per VM. It does not provide storage moves, live disk resize, arbitrary VM import, multi-profile guests, remote VM hosts, automatic rollback of scripts, or a background easy-lima manager. Windows/WSL, cross-architecture emulation, nested hardware virtualization, and GPU passthrough are outside this release's guarantees.

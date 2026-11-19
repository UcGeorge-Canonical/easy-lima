# Operations and recovery

## Observe first

```sh
easy-lima inspect ./service
easy-lima doctor ./service
easy-lima doctor ./service --online
easy-lima operations list
easy-lima operations inspect OPERATION_ID
easy-lima logs ./service
easy-lima storage inspect
```

Diagnostics are read-only: they do not start a VM or agent, load a credential, repair an alias, enroll a host key, initialize storage, or checkpoint SQLite. Checks preserve separate `passed`, `failed`, `unavailable`, `unknown`, `unverified`, and `skipped` statuses. Recorded state is distinguished from a fresh observation.

Read-only listings remain useful when Lima is unavailable. If SQLite cannot be read, the recovery inventory can supply diagnostic recorded information with an incomplete result. It cannot authorize changes, recover ownership by itself, or replace the database.

Operational events are kept by default. Eligible raw command output is stored only when `--capture-output` is explicitly requested. Paths to raw captures and existing Lima logs are reported separately. Treat raw captures as potentially sensitive user output; automatic filtering cannot recognize every secret printed by an arbitrary script.

## Resume interrupted work

```sh
easy-lima operations resume OPERATION_ID
```

An operation records its intended target, frozen environment inputs, claims, and dispatch evidence before effects. Resume observes the previous attempt first and reuses the same VM. A live guest runner is observed rather than duplicated. Unknown dispatch state keeps the claim and returns recovery instructions.

Builtin stages have independent postconditions and restricted recovery checks. Arbitrary native scripts are not automatically rolled back or replayed. If their outcome remains unknown, inspect the retained receipt and fix the underlying cause before resuming. Ctrl-C ends observation; it does not prove a dispatched guest job stopped.

SSH session or command timeouts report an incomplete check or unknown command result, not a workspace mismatch. Check the guest's SSH/login services and the recorded operation before retrying. If the guest itself needs a restart while an unfinished operation holds its claim, inspect the exact storage root and instance name with `inspect` first. An explicit `LIMA_HOME=/recorded/root limactl stop INSTANCE_NAME` stops that instance without deleting its disk; then resume the saved operation. Never use `limactl delete` to recover an interrupted setup. Native recovery is an operator action, not an automatic diagnostic repair.

A failed creation with **no instance files and confirmed Lima absence** can be retired with `delete --vm ID --confirm-vm ID`. If partial instance files exist, deletion does not guess ownership; use the reported recovery action.

## Explicit repairs

```sh
easy-lima repair ./service
easy-lima repair ./service --kind alias --apply
```

The first command lists supported repairs. Repairs have their own previews and owned-file checks. There is no general force bypass. A changed SSH host key is a trust conflict, never an invitation to accept the next observed key silently. Host-key repair requires an independently verified public-key file via `--host-key-file`.

## Move a project

Move the host checkout, then explicitly rebind its existing registration:

```sh
easy-lima rebind --vm VM_ID --to /new/path/service
```

The preview identifies the changed source and any stop/restart. VM identity, disk, and SSH alias are preserved. Conflicting bindings and profile overlaps are rejected. An interrupted rebind is resumed through its operation ID; do not create a replacement VM to work around it.

## Adopt the supported prototype

```sh
easy-lima adopt --from-prototype /path/to/legacy/local --project /path/to/legacy/local/service --dry-run
easy-lima adopt --from-prototype /path/to/legacy/local --project /path/to/legacy/local/service
```

Adoption is a narrowly supported import of the pinned `canonical-local` prototype, not a general import of arbitrary Lima VMs. The source hash and metadata must match the adapter's supported format. The prototype fixture is retained only for tests; no legacy CLI or `make local` compatibility entry point is installed.

The existing VM must already be in easy-lima's selected storage root. Adoption never moves disks. The handoff backs up metadata, acquires the original launcher lock, proves quiescence, retires its active record, and publishes exclusive ownership. The old launcher then refuses to manage the existing VM. Running VMs are gracefully stopped for the handoff and their prior runtime state is restored only after ownership is established.

Identity setup and public settings are preserved. Existing external key, saved-credential, and agent references remain external until an explicit supported migration. Known completed setup is not provisioned again. Unproven setup remains recorded as incomplete or unknown and is not silently replayed. Conflicting edits block the handoff with retained recovery evidence; changing an environment with unknown setup history requires an explicit refresh.

## Metadata backup and maintenance

```sh
easy-lima state backup --to /private/backups/easy-lima
easy-lima state checkpoint --mode passive
easy-lima state inventory refresh
```

Backups cover authoritative metadata and its integrity evidence, **not VM disk contents or private-key backups**. Back up those separately using an appropriate stopped-VM/filesystem procedure. Backup paths must not already exist.

SQLite uses WAL with full durability and explicit checkpoints. Passive is the default; `full`, `restart`, and `truncate` are explicit maintenance choices. A busy checkpoint reports partial/busy instead of escalating automatically.

**Advanced restore is deferred and is not qualified for normal use in this version.** The command exists, but namespace-epoch finalization and interrupted restore paths remain open in the [audit register](reviews/2026-11-19_REQUIREMENTS.md#explicitly-deferred-or-outside-this-completion-claim). A successful backup is not proof that every restore path is complete. Do not copy an old SQLite file over live state or edit owner markers to force a downgrade.

## Scripted result handling

`--json` returns one versioned result object for metadata commands; raw SSH/exec streams do not mix with it. Common exit classes are 2 for invalid input/configuration, 3 for dependencies, 4 for conflicts/decisions, 5 for credentials, 6 for incomplete results, 7 for confirmed failure, 8 for absent targets, 9 for resources, and 130 for cancellation. Guest `exec` preserves a known guest exit status and uses 255 when no guest status could be established.

Use error/check codes and structured fields rather than parsing prose. [The v1 interface reference](design/INTERFACE_V1.md) describes the complete envelope, precedence, and command grammar.

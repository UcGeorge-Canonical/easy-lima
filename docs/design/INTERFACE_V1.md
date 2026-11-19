> Design record: this document predates implementation. For current behavior and validation, see the [user guides](../../README.md) and [test record](../testing.md). Historical status statements remain part of the decision trail.

# Version 1 command, configuration and result contract

**Status:** Approved by George Uche-Umeh on 2026-11-19 ("Approved"), resolving the public decisions for ELD-11 and ELD-12. The exact field/flag/result contract and the previously approved experience and ELD-01/02/08 policies are accepted. Independent design re-review is complete; runtime qualification remains an implementation gate. These examples describe the future application, not commands executed in this repository.

## Consistent command behavior

`easy-lima` opens the browser at CWD; `easy-lima ROOT` opens that exact root. Use `easy-lima ./list` to browse a folder whose name collides with a command. A non-terminal browser invocation is a usage error. The TUI and CLI submit the same plans and use the same validation, confirmations and recovery operations.

`[PATH]` means an exact selected folder, defaulting to the invocation's original CWD. No parent/Git-root inference occurs. An existing-VM command may instead use `--vm ID`, which is mutually exclusive with PATH; accept the full immutable ID, not a fuzzy name. Paths after `--` are positional, except that `exec` reserves `--` for the guest command. Missing source paths can still address recorded inspection/stop/delete operations; creation/rebind require an existing source directory. IDs, instance names and SSH aliases are distinct displayed values.

All long options take the documented value type; repeatable flags are explicitly marked. Unknown options, repeated scalar flags, conflicting selectors and extra arguments are usage errors. Parse/validate inputs before reading secret FDs or dispatching effects. No general `--force` bypass exists.

| Shared option | Meaning and availability |
| --- | --- |
| `--config FILE`, `--project-config FILE` | Select the user/project TOML file for this invocation. File-relative paths anchor at that file; CLI paths anchor at original CWD. Neither changes the authoritative registry location/root binding. |
| `--limactl FILE` | Explicit executable selection for a runtime-using request; absolute resolution and compatibility checks still apply. No automatic install/version switch on retry. |
| `--json` | Metadata commands only; emit one final version-1 object and force noninteractive behavior. Progress, if any, stays on stderr. Never mix guest/raw output into JSON. |
| `--dry-run` | For mutating commands: produce a read-only plan without saving an operation, downloading, unlocking, starting, repairing or resolving inputs by running scripts. Report unresolved prerequisites explicitly. |
| `--yes` | Accept the resolved effect plan for the requested action. It supplies no missing value, identity binding, unlock consent or destructive target confirmation. Noninteractive actions needing acceptance return decision-required with their plan unless this flag is present. |
| `--capture-output` | Capture eligible raw output only for this explicit operation; credential interactions are excluded. Incompatible with read-only/dry-run commands. Existing OS/Lima logs remain separate. |
| `--limit N`, `--cursor TOKEN` | Paginated list/log/operation listings: default 100, range 1–1000, opaque next cursor bound to command/filter/snapshot. Report an expired cursor; never silently restart at page one. |

Interactive mutating actions show their effect preview and gather acceptance when required; healthy ordinary start/open does not acquire an extra confirmation screen. `environment update` is preview-only until `--apply`, even with `--yes`. `--dry-run` always prevents effects. JSON/noninteractive commands never open a picker, editor, OS credential dialog or passphrase prompt.

## TOML v1

All three file types require `schema_version = 1`. Unknown/duplicate keys, wrong types, dotted-key/table collisions, unsupported versions and TOML null-like values are errors with source/field/line. Limits: 1 MiB per wrapper file, 256 mounts/ports per file, 64 KiB per string unless a narrower bound follows. Only the enumerated fields are accepted. No shell/environment interpolation; leading `~/` expands for the current user. File locations and per-field precedence remain those in [the accepted configuration design](TECHNICAL_DETAILS.md#t5-configuration-syntax-locations-and-precedence).

| User configuration field | Type, default and rule |
| --- | --- |
| `storage.root` | Required directory path after explicit init; no implicit default. Must match committed namespace/volume. Ordinary file edits cannot move it. |
| `runtime.limactl` | Executable path, discovered/selected and stored at init. An absent dependency is reported, not installed. |
| `defaults.environment.template` | String, default `ubuntu-24.04`; that exact built-in ID or a native local YAML path. No moving URL or arbitrary template-name resolution. |
| `defaults.environment.presets` | String array, default `["base"]`; unique members `base`, `development`, `docker`. Base is always required and is prepended when omitted; optional stage order follows dependencies. |
| `defaults.resources.cpus` | Optional positive integer; baseline is native template, or 4 for shipped template. Cannot exceed detected host capability silently. |
| `defaults.resources.memory`, `defaults.resources.disk` | Optional quantity strings; native-template baseline, or `8GiB` / `80GiB` shipped defaults. Disk is a creation-time maximum. |
| `defaults.mounts`, `defaults.ports` | Optional arrays with the element schemas below; absence supplies no wrapper override, `[]` explicitly clears the lower-priority wrapper list. |
| `ui.open_action` | `"code"` or `"ssh"`, default `"code"`. |
| `ui.show_hidden`, `ui.show_dependencies` | Booleans, both default false; browsing toggles affect the current session unless the user explicitly saves preferences. |
| `editor.executable` | Executable path, default discovery of `code`; absence leaves SSH available and shows setup guidance. No implicit editor installation. |
| `editor.args` | String array, default `["--remote", "ssh-remote+{alias}", "{workspace}"]`. Only literal `{alias}`/`{workspace}` substitutions; no shell expansion or arbitrary command template. Workspace is `/workspace`. |

Project files accept only `schema_version`, `[environment]` with `template`/`presets`, `[resources]` with `cpus`/`memory`/`disk`, `[identity] profile_label`, `mounts`, and `ports`. Their shapes match the corresponding user defaults. Missing project fields make no override. `identity.profile_label` requests a local label; it cannot supply a key or authorize a new binding. Selecting `--profile LABEL` or `--no-profile` explicitly resolves it. Omitting both is profile-free only when no unresolved project identity request exists; existing VMs retain their committed binding.

Creation resources resolve native template → user defaults → project → explicit flags. Template/preset selection resolves shipped → user → project → explicit flags. Lists replace complete wrapper lists; supported custom native entries remain visible and are not silently erased. Existing environments start from recorded applied values. Update uses explicit flags or an explicit `--from-project` request, not changed user creation defaults.

Quantity strings match `[1-9][0-9]*(MiB|GiB|TiB)` and must fit signed 64-bit bytes; no fractional, bare-number, SI `GB`, negative or overflow values. Values below the qualified image/preset's minimum or above host capacity fail with the relevant bound; they are never rounded or downsized. Wrapper CPU values must be integers 1–1024, then pass the real host check. Zero is not automatic sizing.

| Array element | Exact fields and validation |
| --- | --- |
| `[[mounts]]` / `[[defaults.mounts]]` | Required `source` host directory and `target` absolute guest directory; optional `writable` bool defaults false. Resolve/check symlinks and filesystem equivalence. Duplicate/overlapping target entries, `/workspace` replacement, authoritative-state exposure and configured private-key exposure are rejected. Native unsupported mount types remain precise compatibility errors. |
| `[[ports]]` / `[[defaults.ports]]` | Required `guest` and `local` integers 1–65535; `protocol="tcp"` or `"udp"` (default tcp), `host_address="127.0.0.1"` or `"::1"` (default IPv4), `guest_address` literal IP (default `127.0.0.1`). No ranges, port zero, DNS name or wildcard host exposure. Reject conflicting host address/port/protocol tuples and the reserved VM SSH mapping. A privileged bind unsupported by the current user fails; no automatic sudo or port replacement. |

The effective native catch-all suppression, guest-address semantics and custom/adoption restrictions in [LIMA_BOUNDARIES.md](LIMA_BOUNDARIES.md) remain mandatory. TOML precedence cannot waive them.

### Profile files

`profiles/<id>.toml` holds desired intent. Runtime ownership, lock inhibition, applied identity and agent capabilities are authoritative state, not editable fields. File edits alone change no VM or active key.

| Profile field | Type, default and validation |
| --- | --- |
| `id` | Required immutable 32-character lowercase hex ID; generated by profile create/import. Must match the filename and existing registry profile. |
| `label` | Required `[a-z][a-z0-9-]{0,47}`, unique among live profiles. Rename preserves ID/bindings. |
| `git.name` | Required trimmed nonempty Unicode string, at most 256 UTF-8 bytes, no newline/control characters. Never inferred from host Git. |
| `git.email` | Required single address, at most 254 bytes, no display-name/comment/whitespace/control syntax; validate as a single mailbox. Preserve its spelling. |
| `git.sign_commits`, `git.sign_tags` | Booleans, both default true. `user.useConfigOnly` and SSH format remain fixed owned behavior. |
| `github.username` | Optional expected account string: 1–39 ASCII letters/digits/hyphens, beginning/ending alphanumeric. Compare authenticated accounts case-insensitively while preserving display spelling. Absence skips account-match verification, not SSH key isolation. |
| `key.path` | Required path to the generated/imported host private-key file. Generated keys use the private user configuration `keys/<profile-id>` location. Never embed key bytes in TOML. |
| `key.public_path` | Optional public-key-file path, defaults to `key.path + ".pub"`. Its parsed key must match the private key/agent. |
| `key.fingerprint` | Optional expected SHA-256 fingerprint; if present, mismatch is a conflict. Authoritative applied fingerprints are always computed/verified, not trusted from this field. |
| `unlock.remember` | Boolean, default false; explicit choice shared across VMs. |
| `unlock.credential_ref` | Optional supported adapter's opaque nonsecret reference. An imported reference is preserved. Changing it requires explicit apply and ownership/profile checks; it cannot point at another profile's owned item or grant permission to delete an external one. |

V1 generated keys are encrypted Ed25519 OpenSSH keys. Import supports qualified software Ed25519, RSA (at least 2048 bits with SHA-2 signatures) and ECDSA P-256/P-384/P-521 keys; hardware-backed keys/certificates are deferred. Unsupported formats fail precisely. Existing unencrypted supported imports are permitted and remain unencrypted; the application never removes a passphrase to enable automatic reuse.

### Valid and rejected examples

This project file requests one locally confirmed profile and two explicitly exposed resources:

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
```

A minimal user file after explicit setup can be:

```toml
schema_version = 1
[storage]
root = "/chosen/volume/vms"
[runtime]
limactl = "/chosen/bin/limactl"
[ui]
open_action = "ssh"
```

A generated profile's desired file has this shape; the ID and paths below are placeholders, not an existing profile:

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
path = "../keys/0123456789abcdef0123456789abcdef"
[unlock]
remember = false
```

Reject `memory="8GB"`, `cpus="4"`, an unknown `storage` table in a project, `ports=[{guest=8080}]` without `local`, `host_address="0.0.0.0"`, duplicate host tuples, and `identity.key="..."`. Reject disk-size edits on an existing VM even if their TOML syntax is valid. These cases become schema/CLI golden fixtures; hypothetical example paths are not expected to exist on this machine.

Reject a profile without explicit Git name/email, an ID that differs from its filename, duplicate labels, and a desired key/fingerprint mismatch. An unchanged valid imported key does not require a new key per VM. Field edits remain desired intent until an explicit apply.

## Exact command inputs

Creation/update value flags are `--template FILE_OR_ID`, `--presets CSV`, `--cpus N`, `--memory QUANTITY`, `--disk QUANTITY`. `--presets ''` explicitly requests base only. Extra mounts/ports use `--mounts-from TOML_FILE` / `--ports-from TOML_FILE`, each containing only `schema_version=1` and that array; an empty array clears wrapper requests. This avoids delimiter parsing for paths with spaces/colons. These selectors are mutually exclusive with `--from-project` for the same field. Profile selection is `--profile LABEL` or `--no-profile`, mutually exclusive. Existing-environment lifecycle rejects creation-setting flags instead of ignoring them.

Examples of complete requests, assuming the referenced profile/files already exist:

```sh
easy-lima up ./project --profile work --presets development,docker --yes
easy-lima environment update ./project --cpus 6 --memory 12GiB
easy-lima environment update ./project --cpus 6 --memory 12GiB --apply --yes
easy-lima profile apply work --vm 0123456789abcdef0123456789abcdef --dry-run
```

The first update command only previews; the second explicitly applies the same requested resource changes after revalidation. The profile example is valid for a guest-only change; a shared-key change with omitted members is rejected. `environment update --disk 100GiB`, a preset-removal request and `up` creation flags on an existing registration all refuse before effects.

| Command grammar | Additional inputs and effect |
| --- | --- |
| `init --storage-root PATH` | Interactive form may gather the missing root; noninteractive requires it. `--open-action code\|ssh` and `--editor FILE` set preferences. Profiles remain optional and use their own workflow. Existing root/registry mismatches refuse; no moves. |
| `up [PATH]` | Creation flags/profile selection apply only to an unregistered folder. Create or start/resume existing setup. CLI returns readiness; the TUI's Create and open chains the chosen open action after success. |
| `start [PATH]`, `stop [PATH]`, `restart [PATH]` | Existing registration only. Graceful stop/restart; never auto-force or recreate. |
| `code [PATH]`, `ssh [PATH]` | `--no-start` requires running. Prepare the recorded profile; an unavailable external agent is not restarted. `code` reports launch acceptance separately from an observed editor connection; `ssh` owns terminal streams. No metadata JSON. |
| `exec [PATH] [--start] -- COMMAND ARG…` | At least one guest command argument; pass argv safely to a `/workspace` invocation. No implicit host shell. Preserve known guest exit/stdout/stderr; no `--json`. |
| `list` | `--profile LABEL` optional filter plus pagination. List all registrations independently of CWD. |
| `inspect [PATH]`, `doctor [PATH]` | `doctor --online` requests provider checks. Both remain read-only; no agent/trust enrollment or repair. `doctor --repository PATH` may repeat for explicit guest repository checks; bounded discovery reports examined/unexamined paths. |
| `logs [PATH]` | `--follow` streams text events; incompatible with `--json`. A bounded non-follow page supports JSON and pagination. Never enable raw capture retroactively. |
| `forward [PATH] GUEST_PORT --local HOST_PORT` | `--bind 127.0.0.1\|::1`, default IPv4 loopback. Running VM required; foreground TCP tunnel, no JSON or background service. UDP uses reviewed persistent configuration. |
| `environment update [PATH]` | Default preview; `--apply` enables execution after acceptance. Accept changed CPU/RAM/mount/port values, `--presets` for additions, `--refresh-tools`, `--from-project`, or `--accept-native-config`. Last flag explicitly reviews supported externally changed native inputs; it cannot accept unsupported namespace globals. An unchanged request returns a no-change plan. Disk/image/driver change and preset removal refuse. |
| `rebind --vm ID --to PATH` | Preview/confirm or `--dry-run`; preserve VM/disk/alias and show stop/restoration. No fuzzy source discovery. |
| `adopt --from-prototype PATH --project PATH` | Preview/confirm the exact supported handoff, graceful stop and eligible prior-state restoration. No optional safety-bypass mode; unknown provenance/work follows ADOPTION.md. |
| `delete [PATH]` | Interactive typed displayed VM-ID confirmation. Noninteractive requires `--yes --confirm-vm ID` matching the selected full ID. Checkout and profile key survive. |
| `profile list`, `profile show LABEL`, `profile check LABEL` | Read-only; `check --online` additionally verifies provider/account. List supports pagination. Show includes public key, ownership and capabilities, never a passphrase/private key. |
| `profile create LABEL` | Required explicit `--git-name TEXT`, `--git-email ADDRESS`; optional `--github-user NAME`, `--sign-commits BOOL`, `--sign-tags BOOL`, `--remember` (default false). Generate once under the chosen profile ID. |
| `profile import LABEL --key PATH` | Same metadata/options as create; optional `--public-key PATH`. Reuse the existing supported key, never replace it. |
| `profile edit LABEL` | Interactive desired-field editor, or at least one explicit metadata/key/unlock flag above; `--label NEW_LABEL`, `--remember BOOL` supported here. Writes desired TOML with conflict checks only; does not apply to VMs or load a key. |
| `profile apply LABEL` | Preview and apply desired settings. Repeatable `--vm ID` selects existing member VMs for guest-only changes; default all members. Any shared credential change requires the complete member set. External-agent credential changes refuse. |
| `profile assign LABEL --vm ID` | Repeatable VM selector; explicit reassignment through old/new profile claims and overlap checks. Keep each VM/disk/alias; preview revocation/any stop and pending guest changes. No VM start from profile administration. |
| `profile unassign --vm ID` | Repeatable VM selector; explicit transition to profile-free. Remove only unchanged owned identity settings/forwards; never delete host keys. |
| `profile migrate-agent LABEL` | Explicit same-key whole-profile migration from external to owned agent, following ADOPTED_AGENTS.md. Preserve external users; show stops and pending guest targets. |
| `profile unlock LABEL`, `profile lock LABEL` | Owned agents only. Unlock/in-flow consent never substitutes another key. Lock inhibits reload and unloads the owned agent's key. External agents return a capability conflict. |
| `profile forget LABEL` | Disable remembering and delete only its easy-lima-owned saved item; leave the loaded key and key file alone. External-owned item/policy changes refuse. |
| `profile remove LABEL` | Require no VM bindings or unfinished transitions; preview owned cleanup. Never delete imported/external key or credential assets. |
| `operations list`, `operations inspect ID`, `operations resume ID` | List supports pagination. Resume first reconciles the same saved attempt; unchanged approved effects need no second blanket confirmation. Changed scope or required unlock is reported/handled explicitly. |
| `repair [PATH]` | Default list/preview supported repairs; `--kind alias\|connection\|identity\|trust-export\|host-key\|ownership-publication --apply` selects one. Host-key repair requires `--host-key-file FILE` containing the independently verified public key. Preserve conflicting user edits; no generic reset/force/package rollback. |
| `state backup` | `--to PATH` optional new backup directory, otherwise the private backups location with generated ID. Never overwrite an existing backup. Preview/dry-run available. |
| `state migrate`, `state restore BACKUP_PATH` | Preview-only until `--apply`; explicit acceptance. Known incompatible downgrade/stale backup refuses. Preserve unresolved claims and use the publication/swap protocol. |
| `state checkpoint` | `--mode passive\|full\|restart\|truncate`, default passive; `--timeout DURATION` positive Go duration up to 60s, default 5s. No automatic mode escalation. This is explicit maintenance. |
| `state inventory refresh` | Optional `--vm ID`; explicit projection refresh from readable authoritative SQLite, not from other inventory files. No ownership reconstruction. |
| `config explain [PATH]`, `storage inspect`, `version` | Read-only resolved fields/provenance, capacity/paths, or binary/schema/compatibility data. No initialization, download or checkpoint. |

Creation/import/unlock and profile-aware operations that need a secret accept **`--passphrase-fd N`**, where N is an explicitly inherited readable descriptor ≥3. Consume at most 64 KiB, remove one terminal LF/CRLF, reject NUL and empty new-key passphrases, close the duplicate and clear mutable buffers after use. The shell/CI caller supplies the descriptor securely; no secret value appears in argv, environment, config, service files or capture. A generated encrypted key requires this FD in noninteractive mode. Import requires it only if encrypted and no already-authorized usable credential exists. Interactive mode uses a non-echoing prompt only when needed. `--yes` never means "unlock an explicitly locked profile"; interactive consent or explicit `profile unlock` supplies that decision.

Examples of intended input contracts (paths/identities are placeholders):

```text
easy-lima up ./service --profile work --cpus 4 --memory 8GiB --yes
easy-lima profile import work --key /private/work-key --git-name "Example Developer" --git-email developer@example.invalid --github-user example-work --passphrase-fd 3 --remember --yes
easy-lima environment update --vm 11111111111111111111111111111111 --memory 12GiB --apply --yes
easy-lima environment update --vm 11111111111111111111111111111111 --refresh-tools
```

The last command only previews. Reject a noninteractive profile creation missing explicit name/email, `--passphrase SECRET`, FD 0, `--vm` together with PATH, incomplete credential-change subsets, `--yes` as a substitute for `--confirm-vm`, or `--disk` on start/update.

## Version-1 metadata results

Every metadata response contains all envelope fields: `schema_version` (1), `command` (canonical dotted command such as `profile.show`), `request_id` (ID), `operation_id` (ID or null), `outcome`, `exit_code` (integer), `data` (family payload or null), `checks` (array), `warnings` (array) and `errors` (array). No JSON stream/progress records are interleaved. Parser failures use the identified command or `unknown`, null data and a classified error.

Outcome is `planned` (complete read-only effect preview), `completed`, `blocked` (known unmet prerequisite/decision before progress), `partial` (incomplete or unknown required result), `failed` (confirmed failure) or `cancelled` (requested cancellation, with effects separately represented). A completed VM step does not make the whole identity operation completed. Empty arrays are emitted; null is explicit for specified unknown/absent fields, never guessed as zero/false. Clients must accept additional nonbreaking fields in schema version 1; changed meanings/types require a new result version. Error prose is not a parsing interface.

Shared objects:

| Type | Required fields |
| --- | --- |
| `Scope` | `{kind:"global"\|"vm"\|"profile"\|"operation"\|"storage"\|"record", id:ID\|null, path:string\|null}`. |
| `Check` | `{code,status,scope,checked_at:Time,evidence_time:Time\|null,source:"recorded"\|"observed"\|"mixed",message,next_action,suggested_command:string[]\|null}`. Status: `passed`, `failed`, `unavailable`, `skipped`, `unknown`, `unverified`. Messages use controlled fields; commands contain safe identifiers/paths, never secrets or arbitrary scripts. |
| `Issue` | `{code,scope,message,next_action,suggested_command:string[]\|null}` for errors/warnings. |
| `VM` | `{id,source_path,proposed_source_path:string\|null,instance_name,ssh_alias,namespace_id,profile_id:ID\|null,applied_environment_revision_id:ID\|null,applied_identity_revision_id:ID\|null,pending_identity_revision_id:ID\|null,ownership,vm_state,resources,storage,operation_ids:ID[]}`. |
| `vm_state` | `{recorded:"planned"\|"running"\|"stopped"\|"retired"\|"unknown",observed:"running"\|"stopped"\|"absent"\|"unavailable"\|"unknown"\|null,observed_at:Time\|null}`. Recorded is explicitly last-known. |
| `resources` | `{cpus:integer\|null,memory_bytes:integer\|null,disk_max_bytes:integer\|null}`. |
| `storage` | `{root,instance_directory,allocated_bytes:integer\|null,measurement_time:Time\|null,future_growth_reserved:false}`. |
| `Profile` | `{id,label,git:{name,email,sign_commits,sign_tags},github_username:string\|null,public_key:string\|null,fingerprint:string\|null,agent_management:"owned"\|"external"\|"unknown",lock_state:"locked"\|"unlocked"\|"managed_externally"\|"unknown",remember_enabled:bool,capabilities:string[],member_vm_ids:ID[],pending_operation_ids:ID[]}`. Capability names are the relevant profile action suffixes (`use`, `check`, `lock`, `unlock`, `apply_guest`, `apply_credential`, `forget`, `remove`, `migrate_agent`); availability is still checked at effect time. |
| `Operation` | `{id,kind,status,scope,parent_operation_id:ID\|null,retry_of_operation_id:ID\|null,last_confirmed_stage:string\|null,recovery_required:bool,created_at:Time,updated_at:Time,log_path:string\|null}`. Status uses the eight coordinator states in the design, not inferred process liveness. |
| `Plan` | `{action,target:Scope,complete:bool,effects:[{kind,scope,description,requires_restart:bool,destructive:bool}],fields:[Field],required_confirmations:string[],affected_vm_ids:ID[],unresolved_codes:string[],future_growth_reserved:false}`. Effects are ordered; kind is `metadata`, `vm_create`, `vm_start`, `vm_stop`, `vm_delete`, `guest_stage`, `profile`, `connection`, `ownership`, `checkpoint` or `file_publication`. |
| `Field` | `{path,value:JSON scalar/array/object\|null,origin:"shipped"\|"native"\|"user"\|"project"\|"flag"\|"recorded",source_path:string\|null,changed:bool}`. Only allowlisted public fields are emitted; never raw scripts, secret references/values or unrestricted native YAML. |
| `RecordError` | `{id:ID\|null,source:"sqlite"\|"inventory"\|"config",code:"record.invalid",message}`. Invalid rows are separate, not coerced into VM/Profile objects. |

`VM.ownership` is `reserved`, `adopting`, `owned`, `retired`, `conflict` or `unknown`. Action names use the public dotted command grammar; no incidental Go/SQL type names become public values. `next_action` is one of `none`, `init`, `install_dependency`, `resolve_config`, `unlock_profile`, `migrate_agent`, `resume_operation`, `repair`, `rebind`, `state_recover`, `connect_storage`, `register_key` or `inspect`.

### Command-family payloads

| Commands | `data` shape |
| --- | --- |
| `list`, `profile.list`, `operations.list` | `{items:[VM\|Profile\|Operation],invalid_records:[RecordError],complete:bool,next_cursor:string\|null}`; one typed family per response. Malformed rows do not hide valid rows. |
| `inspect` | `{vm:VM,profile:Profile\|null,operations:[Operation],identity_ready:bool\|null,inventory_status:"current"\|"stale"\|"missing"\|"invalid"\|"unknown"}`. |
| `profile.show` | `{profile:Profile}`. |
| `doctor`, `profile.check` | `{target:Scope,summary:{passed,failed,unavailable,skipped,unknown,unverified}}`, each count a nonnegative integer; all detailed results remain in checks. |
| `operations.inspect` | `{operation:Operation,steps:[{stage,attempt,dispatch_state,effect_state,result_code}],claims:[{kind,scope,mode}],plan:Plan}`. dispatch_state is `none`, `may_have_dispatched`, `exited`; effect_state is `satisfied`, `active`, `not_applied`, `partial`, `unknown`; mode shared/exclusive. |
| Lifecycle, adoption, rebind, environment update, profile mutations, operations.resume and repair | `{plan:Plan,operation:Operation\|null,vm:VM\|null,profile:Profile\|null,member_results:[{vm_id,status:"ready"\|"pending"\|"blocked"\|"unknown",operation_id:ID\|null}],confirmed_effects:string[],recovery_required:bool,available_repairs:string[]}`. Preview has null operation and no confirmed effects. Confirmed effects use ordered Plan effect kinds, never raw output. `available_repairs` lists the supported repair kinds for the selected target and is empty for other actions. |
| `init` | `{plan:Plan,registry_id:ID\|null,config_directory,state_directory,storage_root}`. |
| `logs` | `{entries:[{time:Time,operation_id:ID,stage:string\|null,code,message}],next_cursor:string\|null,raw_capture_paths:string[],runtime_log_paths:string[]}`. No raw log file content is embedded. |
| `config.explain` | `{target:Scope,fields:[Field],differences:[Field],complete:bool}`. |
| `storage.inspect` | `{root,namespace_id:ID\|null,available:bool,vm_paths:[{vm_id,path,allocated_bytes:integer\|null,disk_max_bytes:integer\|null}],capacity_groups:[{id,available_bytes:integer\|null,pending_bytes:integer,quota_known:bool}],cache_paths:string[],temporary_paths:string[],future_growth_reserved:false}`. |
| `state.backup` | `{plan:Plan,backup_id:ID\|null,path:string\|null,recovery_generation:integer\|null,verified:bool}`. |
| `state.migrate`, `state.restore`, `state.inventory.refresh` | `{plan:Plan,operation:Operation\|null,registry_id:ID,old_generation:integer,new_generation:integer\|null,publication_pending:bool,inventory_pending:bool,recovery_required:bool}`. |
| `state.checkpoint` | `{plan:Plan,mode,frames_total:integer\|null,frames_checkpointed:integer\|null,status:"complete"\|"partial"\|"busy"\|"error"\|"not_run"}`. Preview never executes a checkpoint to fill counts. |
| `version` | `{binary_version,db_schema_versions:integer[],config_schema_versions:integer[],result_schema_version:1,dependencies:[{name,version}],lima_contracts:[{version,host,arch,driver,qualified:bool}]}`. Only actually qualified builds are advertised as supported. |

Preflight failure may use null data when no valid target/plan exists; useful partial payloads are preferred when their required fields can be represented honestly. Path strings in state are escaped for text display; JSON retains their string identity. A missing/unreadable DB yields inventory-derived recorded items with observed runtime unknown and incomplete collection status, not an authoritative empty list.

### Stable checks and errors

Check codes identify the check independently of result: `registry.readability`, `registry.schema`, `registry.publication`, `storage.presence`, `storage.capacity`, `ownership.consistency`, `lima.compatibility`, `vm.state`, `vm.ssh`, `vm.host_key`, `profile.agent`, `profile.capabilities`, `identity.forwarding`, `identity.git_settings`, `identity.signing`, `provider.authentication`, `provider.account`, `inventory.freshness`, `repository.coverage`, `environment.stages`, `ports.readiness`. Scoped repetitions are allowed; each retains its own result. Provider unreachability uses unverified; missing local prerequisites use unavailable; a confirmed wrong account/key uses failed. Unrequested/inapplicable checks are skipped and do not make results partial.

| Error/warning codes | Exit class when this is the controlling error |
| --- | --- |
| `input.invalid`, `config.invalid`, `config.unsupported_schema`, `config.unknown_field`, `secret.input_invalid` | 2 usage/configuration. |
| `dependency.missing`, `dependency.unsupported`, `lima.output_unsupported` | 3 prerequisite/compatibility before effects. |
| `decision.required`, `operation.busy`, `ownership.conflict`, `binding.conflict`, `config.drift`, `settings.edited`, `profile.scope_conflict`, `profile.external_managed`, `profile.binding_required`, `trust.mismatch`, `state.backup_stale` | 4 conflict/busy/explicit decision. |
| `profile.unlock_required`, `profile.explicitly_locked`, `credential.interaction_required` | 5 necessary credential action. |
| `operation.unknown`, `operation.partial`, `state.unavailable`, `state.publication_pending`, `inventory.stale`, `record.invalid`, `check.incomplete`, `provider.unverified`, `output.truncated` | 6 when required results are incomplete; an ancillary warning alone does not change a successful action's exit. |
| `operation.failed`, `credential.denied`, `identity.key_mismatch`, `provider.account_mismatch`, `check.failed`, `internal.error` | 7 confirmed failure. |
| `target.not_found` | 8 confirmed complete lookup says absent. |
| `storage.unavailable`, `storage.insufficient`, `resource.unsupported`, `port.unavailable` | 9 resource/storage preflight failure. |
| `operation.cancelled` | 130 requested cancellation; required recovery remains explicitly represented. |

For mutating commands, validate syntax/config first (2). Otherwise cancellation returns 130; a potentially dispatched effect whose outcome cannot be established returns 6 before other failure classes. With known/no effects, multiple controlling errors use fixed precedence **4 → 5 → 9 → 3 → 8 → 7** and all errors remain present. Planning without a required decision is blocked/4; a complete dry-run is planned/0. Successful no-change operations are completed/0 with no new operation record. Ancillary provider or inventory warnings never trigger another start or false failure of completed requested infrastructure work.

For diagnostic commands (including list/inspect/check), an established failed requested check or corrupt ownership consistency wins with **7**; otherwise any required unavailable/unknown/unverified result, invalid row or incomplete collection yields **6**; otherwise **0**. This settles doctor with failed ownership and unreachable GitHub: exit 7, preserving both checks. A confirmed target-not-found with no partial target results is 8. Diagnostic commands never return unlock-needed as an invitation to perform a hidden unlock.

`exec` preserves a known guest exit status (including nonzero) and raw streams. Before a guest exit is known it returns 255 with a classified stderr diagnostic. Interactive SSH likewise retains SSH stream semantics; `code` reports launcher errors separately, and a launched editor is not asserted to have connected. Ctrl-C of setup does not imply that an already dispatched guest job stopped.

### Representative JSON fixtures

The synthetic IDs/timestamps below are fixture values. An empty successful operation listing:

```json
{"schema_version":1,"command":"operations.list","request_id":"11111111111111111111111111111111","operation_id":null,"outcome":"completed","exit_code":0,"data":{"items":[],"invalid_records":[],"complete":true,"next_cursor":null},"checks":[],"warnings":[],"errors":[]}
```

A list retains a malformed row separately and reports an incomplete collection:

```json
{"schema_version":1,"command":"list","request_id":"11111111111111111111111111111111","operation_id":null,"outcome":"partial","exit_code":6,"data":{"items":[],"invalid_records":[{"id":"22222222222222222222222222222222","source":"sqlite","code":"record.invalid","message":"Registration payload is invalid."}],"complete":false,"next_cursor":null},"checks":[],"warnings":[],"errors":[]}
```

An unavailable agent stays a diagnostic result; no activation occurs:

```json
{"schema_version":1,"command":"profile.check","request_id":"11111111111111111111111111111111","operation_id":null,"outcome":"partial","exit_code":6,"data":{"target":{"kind":"profile","id":"22222222222222222222222222222222","path":null},"summary":{"passed":0,"failed":0,"unavailable":1,"skipped":0,"unknown":0,"unverified":0}},"checks":[{"code":"profile.agent","status":"unavailable","scope":{"kind":"profile","id":"22222222222222222222222222222222","path":null},"checked_at":"2026-11-19T12:00:00Z","evidence_time":null,"source":"observed","message":"External profile agent is unavailable.","next_action":"inspect","suggested_command":null}],"warnings":[],"errors":[]}
```

A confirmed ownership failure outranks provider uncertainty without hiding it:

```json
{"schema_version":1,"command":"doctor","request_id":"11111111111111111111111111111111","operation_id":null,"outcome":"failed","exit_code":7,"data":{"target":{"kind":"vm","id":"22222222222222222222222222222222","path":null},"summary":{"passed":0,"failed":1,"unavailable":0,"skipped":0,"unknown":0,"unverified":1}},"checks":[{"code":"ownership.consistency","status":"failed","scope":{"kind":"vm","id":"22222222222222222222222222222222","path":null},"checked_at":"2026-11-19T12:00:00Z","evidence_time":null,"source":"observed","message":"The legacy ownership record is active again.","next_action":"repair","suggested_command":null},{"code":"provider.account","status":"unverified","scope":{"kind":"vm","id":"22222222222222222222222222222222","path":null},"checked_at":"2026-11-19T12:00:00Z","evidence_time":null,"source":"observed","message":"GitHub could not be reached.","next_action":"inspect","suggested_command":null}],"warnings":[],"errors":[]}
```

## Review and implementation evidence

This approved contract preserves the everyday flow and makes its scripting interface explicit. The accepted additions are the precise configuration/flag spellings, structured mount/port files, explicit profile assign/unassign/migrate-agent commands, FD input grammar and mixed-result precedence. It does not authorize a live operation.

Implement the schema/grammar in the planned config/model/CLI packages; add `testdata/interface/v1/cases.json`, `testdata/interface/v1/expected-results.json`, `testdata/interface/v1/user-valid.toml` and `testdata/interface/v1/project-valid.toml` golden input/output cases and `internal/ui/cli/contracts_test.go`. Cover every command family, valid and rejected TOML, noninteractive missing input, encrypted/unencrypted import, FD handling without capture, malformed rows, mixed diagnostic results, no-change/preview outcomes and exact guest stdout/exit passthrough. Examples here are parse-checked documentation; application/runtime tests remain future work. ELD-11/ELD-12 are applied to the design following author approval; the final targeted review and focused correction verification are complete.

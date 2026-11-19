> Design record: this document predates implementation. For current behavior and validation, see the [user guides](../../README.md) and [test record](../testing.md). Historical status statements remain part of the decision trail.

# State additions, ownership publication and restore admission

**Status:** Normative ELD-13 internal design correction, selected on 2026-11-19. Complements the core relational/operation model in [the detailed contracts](TECHNICAL_DETAILS.md). Independent design review and focused correction verification are complete; executable migrations and fault qualification remain pending.

SQLite is authoritative. Marker files and the local publication witness prevent unsafe action when durable evidence disagrees; neither can create a registration, recover lost tables, or authorize mutation without an accepted authoritative database. The diagnostic recovery inventory is a separate projection and participates in none of the admission predicates below.

## Types and constraints

All new tables use SQLite STRICT tables, explicit non-null columns unless `?` is shown, foreign keys enabled on every connection, and restricted deletion of referenced history. `ID` is a 32-character lowercase hexadecimal identifier from 128 random bits; `Seq` is a nonnegative signed-64-bit integer; `Hash` is lowercase 64-character SHA-256 hex; `Time` is a UTC RFC3339 string; `Path` is a validated absolute path with no NUL/control character, bounded to 8192 UTF-8 bytes. `JSON` is schema-validated UTF-8 text with `json_valid` and a 64 KiB bound unless a specific contract sets another bound. Enum sets below are closed; unknown persisted values produce an invalid-record result, never a default.

SQL CHECKs enforce local types/enums/nullability/ranges; composite foreign keys enforce identity/ownership relationships. Cross-row predicates described below run inside the same immediate admission transaction and have corruption tests. Deferrable references permit the profile transition's atomic multi-row commit, not inconsistent committed state. Record observed timestamps separately from integer generations; clocks and PIDs are never leases.

### Existing-table additions

| Table | Required addition/constraint |
| --- | --- |
| `registry` | `recovery_generation Seq`, `recovery_digest Hash`, `critical_projection_version INTEGER=1`, `last_event_id ID?`, `last_namespace_event_id ID?`. Generation increments once per recovery-relevant commit; publication-only acknowledgments do not recursively advance it. |
| `profiles` | `active_credential_revision_id ID?`, `default_identity_revision_id ID?`, `membership_generation Seq`. Null active/default is allowed only before initial commit or after confirmed retirement. Composite FKs `(id,active_credential_revision_id)` and `(id,default_identity_revision_id)` bind the selected revisions to this profile; unique `(id,active_credential_revision_id)` supports runtime binding. |
| `profile_credential_revisions` | `id ID` PK, `profile_id ID` FK, `ordinal Seq>0`, `content_digest Hash`, `public_key TEXT`, `fingerprint TEXT`, `key_ref JSON`, `key_ownership ENUM(owned,external)`, `agent_management ENUM(owned,external)`, `agent_binding JSON`, `store_ref JSON?`, `store_ownership ENUM(owned,external)?`, `provider TEXT?`, `expected_account TEXT?`, `created_at Time`. Unique `(profile_id,ordinal)`, `(profile_id,id)` and `(profile_id,id,agent_management)`. Key/store references contain no secret; store ref/ownership are jointly null or present. Immutable after insertion. |
| `profile_revisions` | `id ID` PK, `profile_id ID`, `credential_revision_id ID`, `ordinal Seq>0`, `guest_identity JSON`, `content_digest Hash`, `created_at Time`. Composite FK `(profile_id,credential_revision_id)` to the credential revision; unique `(profile_id,id)` and `(profile_id,id,credential_revision_id)`. Immutable complete Git/signing/provider-public-key snapshot, not a mutable TOML pointer. |
| `registrations` | Selected `profile_id ID?` FK; `profile_binding_generation Seq`; `applied_profile_id ID?`, `applied_profile_revision_id ID?`, `applied_profile_binding_generation Seq?`; `pending_identity_kind ENUM(profile,none)?`, `pending_profile_id ID?`, `pending_profile_revision_id ID?`, `identity_transition_phase ENUM(preparing,committed)?`, `identity_operation_id ID?`; `binding_generation Seq`, `owner_generation Seq`, `owner_event_id ID?`, `publication_pending INTEGER IN(0,1)`. Applied and pending profile/revision pairs have independent composite FKs to immutable profile_revisions. No history FK uses the registration's mutable current-profile tuple. Exact pending/phase constraints are below. |
| `operations` | `parent_operation_id ID?`, `retry_of_operation_id ID?`, `requested_postconditions JSON`, `ancillary_checks JSON`, `user_decisions JSON`. References stay within this registry; cycles/self-parent/retry are rejected at admission. |

For registration identity state, enforce these local CHECKs and immediate-transaction predicates:

- Applied profile/revision IDs are jointly null or present. A present pair requires `applied_profile_binding_generation`; a null pair with a verified generation records completed profile-free identity, while a null generation records unverified initial state.
- No pending kind means pending profile/revision, phase and identity operation are all null. Pending kind `profile` requires both target IDs plus phase and operation; kind `none` requires null target IDs but non-null phase and operation. Thus pending profile-free cleanup cannot be mistaken for no work.
- In phase `preparing`, selected membership remains the recorded old membership; the pending tuple may name a different profile or none. In phase `committed`, selected `profile_id` must equal the non-null profile target, or be null for kind `none`. Check this against the immutable operation intent and generation in the same transaction. Do not rely on SQL NULL comparison to enforce equality.
- An ordinary identity-ready registration has no pending tuple and its verified applied binding generation equals current `profile_binding_generation`. If selected profile is present, the applied profile must match it and its revision/credential must pass current capability/lock checks. If selected profile is null, both applied identity IDs must be null. Historical applied data during a pending transition grants no connection authority.
- The registration's selected profile, independently owned applied/pending revisions, transition phase, operation link and binding generations are recovery-critical projection fields. Profile removal uses tombstones for retained history; never cascade new membership into old revisions/connections.

`key_ref` is `{kind:"file",path,public_path}`. `agent_binding` is `{socket_path,supervisor_kind,supervisor_ref,ownership_evidence_digest}`; supervisor kind is `launchd`, `systemd_user`, `detached` or `external`. `store_ref` is `{kind:"keychain"|"secret_service",item_reference,access_policy}` using a supported adapter's nonsecret opaque reference. `guest_identity` contains exact Git name/email, signing booleans, allowed-signers principal, provider known-hosts artifact hash and mapping policy. Extra fields are rejected by version-1 decoders.

### `profile_runtime`

| Column | Type/constraint |
| --- | --- |
| `profile_id` | ID PK/FK to profiles. |
| `credential_revision_id`, `agent_management` | ID and owned/external enum; composite FK to committed profile and credential revision. |
| `generation` | Seq; increment for an admitted runtime/binding change. |
| `lock_state` | Nullable enum `locked`, `unlocked`; NULL exactly when agent_management=external. External availability never implies either state. |
| `remember_enabled` | Boolean integer; an imported preference does not grant ownership of an external item. |
| `transition_operation_id` | ID? FK; non-null blocks ordinary use/membership until its protocol permits continuation. |
| `service_evidence` | JSON? `{supervisor_ref,process_id,process_start_identity,host_boot_id,socket_file_identity,observed_at}`; unknown fields nullable, values are observations not authority. |
| `last_verified_fingerprint`, `last_verified_at` | TEXT?/Time?, jointly null or present; historical readiness only. |
| `last_result_code`, `row_revision` | Safe TEXT? and Seq. |

Migration commits active credential/runtime capability changes atomically with per-VM pending targets. A fresh process must check current lock/transition state before using a historical ready observation. Read-only diagnostics never update these fields. Externally managed profiles reject unsupported lifecycle/Lock methods before any runtime mutation.

### `vm_trust` and `connection_bindings`

`vm_trust` is immutable history keyed by `(registration_id,trust_generation)`: `namespace_id ID`, `instance_name TEXT`, `algorithm TEXT`, `public_key TEXT`, `fingerprint TEXT`, `source ENUM(imported_pin,local_first_use,explicit_repair)`, `enrollment_operation_id ID`, `evidence_digest Hash`, `created_at Time`. FK registration/namespace/instance must match. The registration has nullable `active_trust_generation Seq` and nullable `pending_trust_operation_id ID`; applied trust changes only through [the VM trust protocol](LIMA_BOUNDARIES.md#vm-ssh-host-key-trust-eld-04).

| `connection_bindings` column | Type/constraint |
| --- | --- |
| `id`, `registration_id` | ID PK; registration FK. Records represent profile tunnels; profile-free aliases use owned_settings without a profile tunnel row. |
| `profile_id`, `credential_revision_id`, `identity_revision_id` | Historical IDs with composite FKs to that profile's immutable credential and identity revisions. `registration_id` independently references the immutable registration ID. No FK or cascade targets current registrations.profile_id. |
| `generation`, `profile_binding_generation`, `profile_runtime_generation`, `trust_generation` | Seq; retain the registration membership generation used for this tunnel. Trust FK includes registration ID. |
| `vm_boot_id`, `guest_uid` | TEXT? and nonnegative INTEGER; boot ID required before ready. |
| `host_agent_socket`, `host_control_socket`, `guest_socket` | Paths; ownership checked against immutable references/owned settings before use. |
| `endpoint_address`, `endpoint_port` | Loopback literal TEXT and INTEGER 1–65535. |
| `state` | Enum `prepared`, `establishing`, `ready`, `revoking`, `unknown`, `retired`. |
| `operation_id`, `process_evidence`, `last_probe` | Operation FK, JSON? process/start/socket evidence, JSON? typed probe/evidence/time. |
| `created_at`, `retired_at`, `row_revision` | Time, Time?, Seq. Retired time is present exactly for retired state. |

Unique live tunnel per registration; unique live host control/guest socket bindings in their respective host/VM scopes. A connection may become ready only when selected registration/profile, verified applied identity and profile-binding/credential/trust/runtime generations all match, the pending identity tuple is cleared, ownership and lock checks pass, and endpoint/guest-agent probes succeed. The owning completion operation may probe an establishing target tunnel behind the pending gate; ordinary proxy use is refused until the atomic ready transition. Retiring a tunnel changes state/timestamps only and preserves its original identity references. A new socket at an old pathname does not satisfy that predicate. Persistent tunnels do not keep an unbounded shared SQL claim; mutation protocols explicitly revoke them.

### `capacity_claims`

Columns: `(operation_id ID, capacity_group_id Hash, phase_id TEXT)` PK; `namespace_id ID`, `destination_ledger_digest Hash`, `filesystem_evidence JSON`, `peak_bytes Seq`, `remaining_bytes Seq`, `state ENUM(held,released)`, `admitted_generation Seq`, `released_at Time?`. FKs to operation and namespace; `remaining_bytes<=peak_bytes`; released implies remaining=0 and release time present. `filesystem_evidence` includes actual device/container group identity, relevant volume quota limits and the bounded list of destinations; its digest must match the frozen ledger.

Admission groups shared APFS/container/filesystem capacity and subtracts other active **remaining** budgets once per group. Allocation already reflected in observed free space is not charged twice. Reduce a claim only after the corresponding phase's allocation/effect is observed; unknown work retains a conservative remainder. Terminal closure releases it only after that work is reconciled. These rows reserve no physical filesystem space and promise nothing about future guest growth.

### `namespace_events` and publication rows

Despite its retained name, `namespace_events` stores all recovery-critical publication events; profile-only events have `namespace_id=NULL` and publish locally only. Columns: `id ID` PK, `sequence Seq>0` UNIQUE, `operation_id ID`, `namespace_id ID?`, `kind ENUM(initialize,intent,dispatch,effect,ownership,rebind,profile,trust,schema,restore,retirement)`, `previous_event_id ID?`, `previous_event_hash Hash?`, `previous_namespace_event_id ID?`, `before_digest Hash`, `after_digest Hash`, `payload JSON`, `event_hash Hash`, `publication_state ENUM(pending_local,pending_namespace,published,conflict)`, `created_at Time`, `local_published_at Time?`, `namespace_published_at Time?`. Namespace publication time is NULL for local-only events; published requires the local time and, when requested, the namespace time. Prior global/namespace references must match the registry's current heads in the admission transaction. No deletion/rewrite of historical event content.

`payload` is versioned `{schema_version:1,registry_id,recovery_generation,schema_version_after,namespace_id,namespace_epoch,marker_changes:[{relative_path,old_digest,new_digest,new_payload}],handoff_ids:[ID],required_local_publication:true,required_namespace_publication:bool}`. Marker paths are derived/validated against the fixed locations below, never arbitrary payload filesystem destinations. The event hash covers canonical content excluding acknowledgment fields and event_hash itself. `owned_settings` records the expected published digest for trust/alias/config artifacts; the marker change record supplies the same expected-old/target comparison during recovery.

All new composite FK parent tuples have explicit UNIQUE indexes. Index active runtime transitions, live connections by profile/registration, held budgets by capacity group and pending events by sequence/namespace. Bounded list readers decode each record independently; corrupt JSON in one logical row is not silently treated as empty.

## Exact external representations

Use version-1 canonical JSON: UTF-8, lexically sorted object keys, no insignificant whitespace, integers only for numeric values, preserved array order and a terminal newline. Hash the payload bytes excluding the envelope's `sha256`; JSON digests are integrity checks, not protection against an authorized local user modifying files.

| Location | Version-1 payload and role |
| --- | --- |
| `<state>/authority/head.json` | `{schema_version:1,registry_id,sequence,event_id,event_hash,recovery_digest}`. Last fully published local critical event; a comparison witness only. |
| `<state>/authority/intent.json` | `{schema_version:1,registry_id,event_id,sequence,before_digest,after_digest,event_hash,operation_id}`. Durable fence around a critical SQL commit/publication; no VM effects may use the new generation while pending. |
| `<state>/authority/events/<20-digit-sequence>.json` | Full immutable event payload plus chain metadata; hash-checked copy of the SQL event, not a replay log that can reconstruct authority. |
| `<root>/_easy-lima/namespace.json` | `{schema_version:1,registry_id,namespace_id,volume_identity,epoch,last_event_id,last_event_hash}`. Latest published namespace-affecting event; epochs strictly increase. Profile-only local events may occur between namespace events. |
| `<root>/<instance>/.easy-lima-owner.json` | `{schema_version:1,registry_id,namespace_id,registration_id,instance_name,owner_generation,binding_generation,owner_state,handoff_id,event_id}`. owner_state is `reserved`, `owned` or `retired`; handoff_id nullable only outside adoption. |
| `<state>/handoffs/<handoff-id>/manifest.json` | `{schema_version:1,handoff_id,operation_id,registry_id,namespace_id,registration_id,legacy_active_record,legacy_archive_record,old_record_sha256,backups:[{path,sha256,mode}],prior_running,restore_running,quiescence_evidence}`. Every nonsecret backup uses expected hashes. |
| `<prototype-root>/.local-state/easy-lima-handoffs/<handoff-id>/project.json` | Exact retired legacy project-record bytes. The active `projects/<name>.json` must be absent; unrelated legacy records are not moved. A sibling manifest binds the archived bytes to the same handoff ID and new registry. |
| `<state>/restore.intent.json` | See the restore swap protocol below. Its presence blocks ordinary registry opening, including diagnostics that would otherwise open a mixed DB generation. |

Each JSON publication has envelope `{payload:<above>,sha256:<hash>}` and is bounded to 64 KiB. Use restrictive permissions, same-directory temporary files, file synchronization, expected-old digest check, rename and directory synchronization. A second process must take the corresponding publication guard. Unknown file contents or a foreign registry are conflicts, not stale files to overwrite. Do not follow unvalidated symlinks or infer an absent external volume from a same-named directory.

### Recovery digest and publication order

`recovery_digest` covers the canonical projection of all registrations/tombstones, immutable applied/pending environment/profile/trust revisions, profiles/runtime locks, operations and dispatch/step evidence, claims/reservations, owned-setting expectations, binding changes and capacity holds. Include record/schema/namespace identities. Exclude the digest field itself, event/outbox acknowledgment fields, registration publication_pending acknowledgments, inventory/log publication progress, purely diagnostic cached timestamps and SQL physical page layout. Desired owner/binding generations and marker target digests remain included; acknowledgment flags cannot change those targets. Immutable artifact **digests** are included; referenced bytes must be verified before use. Changing any authority/effect/recovery field advances the recovery generation.

Every critical commit first uses one short local publication guard, with no external VM/credential call or VM-volume access while held:

1. Read/recheck the expected authoritative generation, compute the intended critical projection and event. Write/fsync `authority/intent.json` before the SQL commit. Only one pending critical publication is admitted; unrelated long VM work can continue under its existing dispatch claims, but a new dispatch waits for this short publication step.
2. In an immediate SQL transaction compare the generation and executor token, apply the changes, insert the event and set pending publication. The intent/event/hash must match. Commit before any external effect authorized by that change. An ambiguous commit is reconciled by ID/token, not replayed.
3. Publish and verify the immutable local event file and local head. Acknowledge local publication in SQL, remove/fsync the local intent fence and release the local guard. Local-only profile effects can now proceed. An event needing VM-volume publication remains pending_namespace; it cannot yet authorize its VM effect or owner.
4. Under a separate namespace-publication guard, publish pending namespace events in order: recorded owner/handoff markers followed by the namespace epoch, using exact old/target hashes. Namespace events link the previous namespace event even when global sequence numbers have gaps. Verify and acknowledge each required publication without another critical generation. Only then can the corresponding VM effect dispatch or newly published owner become usable. Read-only diagnostics report pending evidence but do not complete it.

A missing VM volume can leave a namespace publication pending without retaining the local publication fence. Independent profile administration can still commit/publish locally. Later namespace-affecting mutation admission waits for that namespace's pending prefix; already-running external work may still record its observations and queue further ordered publication. No command drops evidence merely because the volume became unavailable.

A crash after intent but before SQL leaves no authority for a new effect. A crash after SQL but before the required local/namespace publication leaves authoritative pending work, not permission to dispatch the affected effect. A crash after publication but before acknowledgment is recoverable by verifying the matching target bytes and finishing the acknowledgment. Filesystem publication never silently rolls back committed SQL. The separate serialized local and namespace publications avoid two writers overwriting one another's witness; no lock spans downloads, guest jobs or user input.

| SQL/event and file evidence | Explicit writer reconciliation |
| --- | --- |
| SQL at old digest, pending intent identifies an uncommitted event, all markers/head still old | Confirm no committed event/dispatch, then remove the orphan intent; do not replay a VM effect. |
| SQL contains the matching new event, local head/intent publication incomplete | Finish the matching local event/head and clear only the local fence. Dispatch still requires all publications for its scope. |
| Local event is published; required namespace markers remain old/absent at their expected-old state | Publish the recorded namespace prefix in order and acknowledge it. Affected VM effects remain blocked; independent local-only profiles can proceed. |
| SQL contains the event, all targets/head match, acknowledgment/intent cleanup incomplete | Acknowledge and remove the fence. No external effect is repeated to establish metadata state. |
| Any third digest, foreign owner/registry, event-chain break, unexplained newer head or epoch | Conflict; retain evidence and refuse mutation. |
| Required volume/legacy record unavailable or driver/DB cannot be read safely | Unknown/unavailable; no inferred retirement/deletion, publication or restore. Profile-only unrelated work uses its independent local boundary even with namespace publication pending, provided its local witness is consistent. |

An instance may have an owned marker whose SQL registration is still `publication_pending`; that is an interrupted publication, not usable exclusive ownership. Admission requires registration active, owner marker matching its owner/binding generation/event, namespace lineage consistent, no pending local fence, and no restored legacy active record. These checks also gate rebind, delete and startup. A partially created unmarked instance stays reserved/unknown and is never treated as absent.

Delete writes and verifies the retirement event/tombstone only after confirmed instance/disk absence in the available namespace; its removed instance marker remains represented by the immutable event. A disappeared volume does not retire ownership. An adoption publishes the new owner only after the backed-up legacy record is retired and quiescence confirmed. Incomplete metadata publication never returns control to both managers.

## Backup and restore predicates

A consistent SQLite backup has a private manifest `{schema_version:1,backup_id,registry_id,db_schema_version,recovery_generation,recovery_digest,event_id,event_hash,namespace_id,namespace_epoch,db_sha256,artifact_hashes,created_at}`. Take it while critical publication is quiescent, using the adapter's online-backup API; record the head corresponding to that snapshot, not a later observation. Verify the standalone destination and referenced small artifacts before reporting success. An unresolved operation may be backed up, but its complete intent/claims/artifacts must be included. Neither disk images nor private credentials are copied.

V1 restore is deliberately conservative: the candidate must match the independently retained current local head's registry/event/recovery digest, match current namespace/handoff lineage, and preserve **all** unresolved effects/claims represented there. For a fully retained pending namespace event, each marker may be exactly its recorded old or target digest in a permitted publication prefix; the candidate must retain that event/claims and resume publication before VM dispatch. A third value or an impossible order refuses restore. A backup older than any critical event is refused; do not choose a timestamp or merge invented records to make it current. Missing/contradictory witness or required volume/legacy evidence also refuses automatic restore. Inventory is not an alternate witness. Pure inventory/log acknowledgment lag does not invalidate an otherwise equal critical snapshot.

If the current DB is readable, compare its critical projection and event chain as well. A committed-but-locally-unpublished event must first reconcile using that DB; do not replace it with the older backup/head. Pending namespace publication may be preserved by an exactly matching backup, but cannot be dropped or marked complete. If the DB is unreadable and an intent fence remains, automatic restore cannot prove which generation is safe and refuses with preserved evidence. Loss of every authoritative compatible copy requires manual reconstruction outside v1 automation.

### Swap ordering

1. Under the exclusive registry-open guard, block new opens and close all ordinary handles. Validate a staged candidate, including integrity/FKs, event/projection hashes and manifest; verify required external lineage without changing it. The stage path is derived from a fresh restore operation ID, not arbitrary manifest input.
2. Prepare the candidate as a consistent standalone WAL-mode database using an explicit staging checkpoint in this maintenance action. With a readable old database, attempt its explicit bounded checkpoint and abort on busy/unknown outcome. If the old database is corrupt, retain its complete raw database/WAL/SHM file group without attempting to pair it with the candidate; this is forensic preservation, not a claimed consistent backup.
3. Write/fsync `restore.intent.json`: `{schema_version:1,restore_id,registry_id,expected_head,old_file_set:[{relative_path,sha256}],candidate_file_set:[{relative_path,sha256}],backup_id,phase,finalization}`. Phase is `prepared`, `old_quarantined`, `candidate_installed`, `finalizing` or `verified`. Initially finalization is null. Paths are constrained to known state/staging/quarantine locations. No live handle survives this point.
4. Rename each old-generation file into `restore/<restore-id>/old/` with manifest tracking and directory sync, then install only the validated candidate file set. A crash may split file placement; every opener sees the fence and refuses ordinary access. Never reuse an old WAL/SHM beside the candidate. Before finalization, compare exact physical file hashes and validate the complete installed candidate without writable access; close the validation handle.
5. **Journal the final write before writable opening.** Under the same exclusive guard, prepare the restore event/operation once, fixing its IDs, timestamps, before/after recovery generations/digests, event hash and intended local/namespace publications. Atomically publish phase `finalizing` with non-null `finalization={operation_id,event_id,event_hash,before_generation,before_digest,after_generation,after_digest,expected_local_head,expected_namespace_head,publication_event_payload}`. The full intended SQL projection change is reproducible from this frozen event and accepted candidate. This record is durable before a writable maintenance connection can create WAL/SHM or commit anything. Ordinary opening remains fenced.
6. **Finalize the same event.** Open the completely installed candidate group only through this maintenance path. Validate integrity/FKs and logical lineage, then use the normal critical SQL/local/namespace publication protocol for the preallocated restore event. On resume, reconcile its exact ID and before/after projection instead of allocating another event. Publication-only acknowledgments may change physical DB/WAL bytes without changing the expected critical projection. A candidate group is never accepted by refreshing hashes to observed values.
7. After the exact event, required local/namespace publications and their acknowledgments are verified, atomically mark `verified`, close all maintenance handles, then remove/fsync the swap fence. A crash at either boundary is resumable from the same event. Preserve prior attempts/claims for subsequent explicit reconciliation; restore never replays guest work. Refresh inventory from this authority afterward.

The file-hash and logical-generation phases have separate recovery predicates:

| Restore journal and observed evidence | Explicit maintenance continuation |
| --- | --- |
| `prepared` / `old_quarantined` / `candidate_installed`, no finalization record | Compare exact old/candidate file hashes at original, staged and quarantine paths. Complete only a recognized placement of that file set. Reversal may restore the whole unchanged usable old group before finalization begins. |
| `finalizing`, valid installed candidate group has exact before generation/digest and no committed allocated restore event | Verify the frozen event/intent, then commit that same event through ordinary publication. A matching orphan local intent is reconciled by event ID; do not invent a new restore. Writable-opening side files are candidate-local and must have valid SQLite lineage, never quarantined old side files. |
| `finalizing`, exact allocated restore event and after generation/digest are committed; witnesses pending or complete | Reconcile the event's existing local/namespace publication using its expected old/target evidence, then acknowledge it. Do not repeat the SQL generation change or require pre-write candidate file hashes to match the legitimate new database. |
| `verified`, exact after generation/event and all required publications match; fence still present | Validate that exact final state, close maintenance handles and finish fence removal. Do not append another event. |
| Any unexpected event/generation, invalid or mixed SQLite group, contradictory marker, missing required evidence, or third value | Preserve the fence and all evidence; refuse automatic continuation or reversal. Do not bless observed contents by updating expected hashes/digests. |

Once `finalizing` is durable, recovery proceeds forward through its frozen event; the old quarantined group's physical hashes remain fixed forensic evidence and cannot authorize rolling back newly published authority. Diagnostics report journal/file-group status and inventory without opening a potentially mixed generation. Only the exclusive maintenance opener may validate a fully placed finalizing candidate through SQLite. Old quarantined corrupt bytes remain available for manual investigation.

## Planned implementation evidence

Apply these definitions in the initial SQL migration, state store/guards/maintenance, profile and connection coordinators. Add `internal/state/publication.go`, `publication_test.go`, and `testdata/state/publication-v1.json`. Existing schema/backup/recovery tests gain constraints for the four additional tables, credential/trust bindings and restore fences. The native marker/outbox serializers share one versioned codec; public CLI JSON remains a separate schema.

Required cases: SQL committed but epoch unpublished; target marker published but acknowledgment absent; two publication writers; profile-only work with VM storage absent; stale backup; backup preserving unresolved dispatch; restored legacy ownership; foreign registry; torn/mixed restore file sets; corrupt old DB quarantined without grafting its WAL; missing namespace evidence; post-install restore-event commit, local publication, namespace publication, verified-before-fence-removal crashes, candidate-local WAL creation before commit, cross-profile/profile-free identity transitions with retained history, and read-only diagnosis throughout. A fixture must assert the exact allowed continuation from every table row above. No such runtime test has yet been executed.

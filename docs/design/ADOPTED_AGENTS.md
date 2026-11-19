> Design record: this document predates implementation. For current behavior and validation, see the [user guides](../../README.md) and [test record](../testing.md). Historical status statements remain part of the decision trail.

# Adopted profile agents: capabilities and migration

**Status:** ELD-02 policy accepted by George Uche-Umeh on 2026-11-19 ("Agreed"). This is a normative supplement to [the implementation design](design_draft.md), with internal mechanisms selected under the standing delegation. Independent design re-review is complete; runtime qualification remains pending.

An adopted agent may also serve projects outside easy-lima. Preserve its verified Git authentication/signing use, but refuse Lock and credential changes until an explicit migration establishes an easy-lima-managed binding. Adoption of a VM does not transfer ownership of a shared agent, key file, or credential-store item.

## Ownership and capability records

Record ownership per resource, not as one profile-wide boolean. The committed credential revision records `agent_management` (`owned` or `external`), socket reference, expected public key/fingerprint, supervisor kind/reference, key reference and ownership, credential-store reference and ownership, and the source of the adoption evidence. `profile_runtime` records observed availability separately from owned-agent lock inhibition and any pending migration operation. The migration keeps old and candidate resource bindings until completion.

An imported private-key file or remembered item can remain externally owned after agent migration. The new agent must be exclusively controlled by easy-lima; ownership of the file containing the key is not required. Referencing that file or reading an expressly authorized store item does not grant permission to overwrite or delete it. No credential value enters state, inventory, plans, or logs.

Derive capabilities from the committed binding and verified resource evidence. Do not trust a TOML ownership flag, a process name, or the mere presence of a socket. External does not mean unavailable: a verified external agent with exactly the expected key can be ready. Absence, an unexpected key, extra keys, and uncertain ownership remain separate results.

For an external agent, the displayed lock state is **managed externally**, never an inferred "locked" from an empty socket. Show supported actions and a reason for unavailable actions in profile details, checks, previews, and structured output. Public code/field spelling is completed with ELD-11–ELD-12; this contract fixes the semantics.

## Capability matrix

| Action | Owned agent | Externally managed adopted agent |
| --- | --- | --- |
| List/show/check | Observe without activation or repair. | Same read-only observation; show external ownership and capability limits. |
| Use for Git, including another VM | Reuse or activate the same configured key under the accepted lock/remember policy. | Reuse only the verified, already-available expected key. Preparation of an easy-lima-owned forward is allowed; administration of the external agent is not. |
| Missing agent or unloaded key | Follow the accepted automatic activation/in-flow unlock policy. | Report external-agent unavailable or key-unavailable with the recorded external supervisor reference and migration guidance. Do not run its helper, restart it, load a key, or open an unlock dialog on its behalf. |
| Lock | Inhibit reload durably and remove the key through the owned-agent protocol. | Refuse before effects. Do not set a misleading lock flag, unload a key, stop a service, or silently substitute connection-only revocation for Lock. |
| Unlock, start/restart agent, load/remove keys, change agent binding | Explicit owned-agent operation. | Refuse administration until migration. An already-ready observation is not a successful Unlock operation. |
| Remember/forget | Act only on the selected authorized store reference; deletion requires easy-lima ownership. | Preserve the imported unlock policy/reference. Do not change or delete its external item. A separately identified easy-lima-owned unused item may be forgotten without claiming to change the external agent's policy. |
| Edit profile TOML or rename its label | Desired intent/metadata only. | Allowed; it does not change active ownership, credentials, or capabilities. |
| Apply identity changes | Follow [the profile transition contract](PROFILE_TRANSITIONS.md). | Guest-only settings retaining the exact credential binding can use that contract. A key, account, unlock-policy, socket, or supervisor change is refused before effects; migration is separate. |
| Remove an unbound profile | Reconcile pending work and remove only owned, unchanged resources. Imported keys remain. | Require no VM bindings or unfinished operations, retire only easy-lima records/exports/owned forwards, and preserve the external agent, keys, store item and supervisor. |
| Migrate the agent | Not implicit in adoption or ordinary activation. | Explicitly preview and establish a separate owned agent, preserving external users as described below. |

Both public commands and the private alias proxy enforce these limits before calling an adapter. The external adapter exposes observation and key-list/signing-probe capabilities; it does not expose lifecycle/key-loading methods to normal activation. Diagnostics cannot change ownership or capabilities.

A new VM may use a ready external profile without another key or GitHub registration. If the external agent is unavailable, expose that limitation before new-VM allocation. Existing VM-only lifecycle work remains independent of credentials. Observe readiness again before forwarding; a supervisor restart or socket replacement invalidates earlier readiness evidence until owner/binding and the exact offered-key set are rechecked.

## Explicit migration to an owned agent

V1 migration creates a distinct easy-lima-owned socket and supervisor for the same profile and existing key. It does not take over, stop, rewrite, or delete the shared legacy service. Keep the profile ID, expected account, key fingerprint, VM IDs, disks and aliases. Changing the key/account is a subsequent, separately previewed profile-wide update. Public command spelling belongs to the interface appendix; the explicit migration action itself is accepted.

The preview includes every non-retired VM and pending membership from [ELD-01](PROFILE_TRANSITIONS.md), old/new agent references, preserved external resources, required credential access, all owned forwarding/guest-setting changes, and any required stops. It lists known external users and labels incomplete discovery as unknown; it never claims all users of a private key have been enumerated. A same-key migration needs no repeat GitHub registration. Remembering remains optional and retains its prior explicit choice; copying a legacy secret into a new store item requires explicit consent.

The supported prototype exposes its agent through direct SSH agent forwarding. Existing user SSH/editor sessions may outlive an alias edit. Therefore this migration requires every affected running VM to be gracefully stopped under the previewed stop/reconnect allowance, with the stopped state verified before switching bindings. Already-stopped VMs remain stopped. Profile administration never starts them. No force escalation is automatic; an unobserved stop or an unknown forwarding holder retains the operation and prevents cutover.

1. **Freeze and claim.** Take the profile transition claim plus all member/reservation and connection claims in one admission transaction. Record the complete member generation, unchanged public identity, old external binding, distinct candidate owned binding, expected-old setting hashes and backup references. Block new profile connections/membership while cutover is in progress.
2. **Prepare the candidate.** Install only the new owned supervisor/socket, using the host lifecycle adapter. Load the existing host key via supported credential access and verify exactly its fingerprint. Do not invoke legacy askpass scripts or copy private-key bytes to another file or guest. If supported access cannot be established, preserve the external path and report migration incomplete. The candidate is not yet forwarded.
3. **Quiesce adopted connections.** Gracefully stop affected running VMs as previewed and prove each stopped. Verify old easy-lima/legacy forwarding paths for those registrations are ended; close only proven owned controllers. Preserve the external service for non-adopted projects. Back up and replace only unchanged owned alias/native/guest-binding metadata needed to prevent the next managed start from restoring the legacy path. An unclassifiable forwarding mechanism is a conflict, not permission to guess.
4. **Commit and hand off.** Once the revocation barrier is satisfied, atomically commit the owned credential binding, capability generation and per-member pending identity targets. Clear the transition barrier only after durable child completion operations exist for every deferred VM. Follow ELD-01's claim-release rules; a stopped VM's pending update cannot hold a claim that prevents its next explicit start.
5. **Complete on explicit use.** The next public start/open completes the saved guest target and verifies the owned forwarding path before Git access. A direct alias proxy reports a pending update instead of applying guest changes. The result distinguishes **agent migrated** from each VM's pending/ready identity. No tools or presets change.

After cutover, Lock controls the easy-lima-managed agent and its managed VM paths; the legacy agent can still use the same key for external projects. Lock is not global revocation of every copy of a private key. This scope must be stated in migration details. If any supported old path into a managed VM can still bypass that control, do not publish Lock as available for that binding. Manually introduced out-of-band forwards remain outside cooperative management, like raw external Lima mutations.

## Interruption and publication

| Observed migration boundary | Reconciliation |
| --- | --- |
| Intent only, candidate absent | Verify the original external binding and resume the recorded preparation, or cancel before effects. |
| Candidate exists; active binding still external | Reuse the exact owned candidate after verification. Do not dispatch duplicate services or claim migration complete. Explicit cancellation removes only the unchanged candidate and leaves stopped VMs stopped. |
| Some VMs stopped; revocation/edits incomplete | Retain all old/new bindings and member claims. Resume observation and the remaining previewed stops/edits. External users retain their agent; affected easy-lima use remains blocked. |
| Owned binding committed; VM targets pending | Continue the saved per-member operations. Never restore the external binding from stale TOML or repeat the agent migration. |
| Resource/key/hash/owner evidence disagrees | Stop with a conflict or unknown result and recovery guidance. Do not unload the external agent, seize its socket or silently roll back a credential. |

All resource observations carry the binding and operation generation. A private socket path, a successful service command, or a new database value alone cannot establish the migration postcondition. Publish owned administration capabilities only after the new agent is verified, old managed forwarding is revoked, the binding commit is durable, and deferred guest targets are recorded. If the new agent later fails, report its availability separately; that failure does not restore external ownership or bypass an explicit lock.

## Implementation and acceptance evidence

Add the capability predicate to `internal/identity/profile.go` and enforce it in `internal/app/profiles.go`, activation and `internal/connection/proxy.go`. `internal/host/agent` must distinguish observation-only external bindings from owned lifecycle adapters. Retain migration evidence in profile revisions/runtime and operation steps; ELD-13 supplies the complete SQL schema. Use the existing planned profile, connection, adoption and recovery tests, plus real-host acceptance checks.

- Adopt one of two prototype VMs sharing an agent. Verify Git authentication/signing in both. Lock, Unlock and credential Apply on the external profile refuse with no external service/key/store or forwarding mutation; status never says locked.
- Restart the legacy supervisor externally. Its key reload does not turn a refused Lock into a false success. The non-adopted VM remains usable.
- A new VM using a ready external profile needs no per-VM key setup. An absent/wrong/extra-key agent is reported precisely and never repaired by the alias proxy or doctor.
- Migrate the whole easy-lima profile while a non-adopted VM remains on the legacy agent. Verify unchanged fingerprints/aliases/disks, the previewed stops, durable pending updates, and independent owned Lock after cutover. The old agent and its external users stay usable.
- Inject interruption before/after candidate installation, every stop, forwarding revocation, binding commit and member completion. Retry resumes the same resources. No duplicate agent, credential deletion, hidden VM startup, or implicit key rollback occurs.
- User edits, unsupported legacy forwards, denied credential access, a stale controller and concurrent member creation prevent unsafe publication. Read-only checks do not progress migration.

These are required scenarios, not executed tests. No live resource was inspected or changed for this design resolution.

The complete relational additions and external publication/restore predicates are specified in [STATE_CONTRACT.md](STATE_CONTRACT.md). Exact public field and command spelling is specified in [INTERFACE_V1.md](INTERFACE_V1.md), approved on 2026-11-19; independent design re-review is complete.

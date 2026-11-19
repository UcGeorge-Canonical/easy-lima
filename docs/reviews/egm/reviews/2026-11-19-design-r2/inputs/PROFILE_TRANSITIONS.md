# Profile revisions and credential transitions

**Status:** ELD-01 accepted by George Uche-Umeh on 2026-11-19 and applied to the design. The author approved requiring an SSH-key change to include every VM using the profile; a subset needing a different key uses a separate profile. The internal protocol below implements that decision and the existing no-implicit-VM-start, explicit-lock, owned-settings, and recovery rules. It has not received an independent re-review or runtime validation.

This is a normative supplement to [the design](design_draft.md). It addresses ELD-01 only. [ADOPTED_AGENTS.md](ADOPTED_AGENTS.md) defines the accepted ELD-02 external-agent capability/migration policy. The broader state/API specifications remain open under ELD-11–ELD-13.

## Scope of each change

| Change | Required scope | Effect |
| --- | --- | --- |
| Key fingerprint or key reference; agent socket/supervisor binding; provider/expected-account binding | Every non-retired registration using the profile, including stopped VMs; check pending assignments and attempts too. | One profile-wide credential transition. These shared properties cannot have different active versions behind one agent. A partial selection is refused before effects; never silently expand a confirmed selection. |
| Git author name/email and other owned guest settings that retain the same credential revision | The explicitly selected registrations. | Each selected VM receives its own recorded target identity revision. Unselected VMs retain their applied guest settings. |
| Label | Profile metadata only. | Existing registrations keep their immutable profile ID. A rename changes no key, guest setting or applied revision. |
| Remember, forget, explicit lock/unlock | The existing explicit host-profile workflow. | These runtime controls are not per-VM identity revisions and do not start VMs. A credential transition cannot silently clear a lock or change the remember preference. |
| Reassign selected VMs to another profile | The explicit registration set, plus both profiles' applicable claims. | Leave both profiles' credentials unchanged. Preserve the existing parent/child overlap rules; a new profile does not bypass them. |

The profile-apply preview lists every affected VM, old/new public fingerprints, guest-setting differences, stopped/deferred members, and any connection interruption. New-key provider registration is still a one-time user action for that key; easy-lima does not upload it. Local readiness and provider verification retain their separate statuses.

## Revision model

TOML remains desired configuration. Editing it does not change an active key, an existing VM's settings, or the default used for a newly created VM. Only an explicit successful apply advances the relevant committed references.

The following fields refine T6; they are design schema requirements, not created database tables:

| Record | Required identity fields | Meaning |
| --- | --- | --- |
| `profiles` | `active_credential_revision_id`, `default_identity_revision_id`, `membership_generation` | One committed credential for the profile; an explicitly applied default for future registrations; an integer generation incremented whenever membership/reservations change. An uninitialized profile may have null active references and is not identity-ready. |
| `profile_credential_revisions` | Immutable `id`, `profile_id`, public fingerprint/key, key and credential-store references, owned/external agent binding, provider/expected-account binding, content digest | Shared credential/runtime configuration. No private key or passphrase is stored. Foreign keys always include the owning profile ID. |
| `profile_revisions` | Immutable `id`, `profile_id`, `credential_revision_id`, explicit guest Git metadata/settings, input digest | A complete public guest-identity snapshot tied to one credential revision. Multiple registrations may share an identical snapshot. This replaces the ambiguous single profile-wide “active identity revision.” |
| `registrations` | Nullable `applied_profile_revision_id`, `pending_profile_revision_id`, `identity_operation_id` | Applied records verified guest effects; pending records an already-authorized target. No-profile registrations have neither identity reference. Missing applied evidence is not inferred from a profile default. |
| `profile_runtime` | Nullable `transition_operation_id`, existing explicit-lock inhibition, observed agent generation/fingerprint | A durable transition blocks ordinary activation and connection preparation until the recorded cutover is safe. Observations do not replace committed credential references. |
| `operations` | Nullable `parent_operation_id`, immutable member IDs/revisions, old/new credential IDs, per-member old/target identity IDs | A credential transition and any per-VM completion operations retain one traceable plan and frozen targets. There is at most one unfinished credential transition per profile. |
| `connection_bindings` | Registration ID, profile/credential revision, observed identity revision and connection generation | A tunnel may be reused only when its binding still matches committed and verified identity state. |

Profile revision references must belong to the registration's selected profile. A target identity must reference the transition's target credential. Only confirmed guest effects advance `applied_profile_revision_id`; queuing work never does. Pending targets cannot be overwritten by a later apply: reconcile, finish, or explicitly supersede them through a new recorded plan first.

A metadata-only apply advances the profile's default for future VMs and the selected existing VMs' targets. Other existing VMs keep their own applied revisions. Start/connect reads those registration references, not the latest TOML or profile default.

For a credential change, derive each VM's target from its own last applied or already-authorized identity snapshot and replace the shared credential reference. Preserve guest fields that the new apply did not explicitly change. If the same apply explicitly changes guest fields too, show those changes for every included member. Retain a separate complete target snapshot per distinct result. An unresolved earlier target blocks admission until reconciled; it is never silently merged away.

## Admission and concurrency

Resolve all live members and pending bindings under the authoritative registry. Unknown or uninitialized ownership/profile membership blocks the transition; it cannot be treated as an empty set. Reservations and unfinished creates/reassignments using the profile must be reconciled before cutover. Frozen attempts targeting the old credential cannot resume with a new key implicitly; any superseding identity authority must be explicitly linked to the approved transition while retaining their VM and environment inputs.

The preview records membership generation and every affected registration revision. At apply, acquire the exclusive profile claim and all member/configuration claims in one admission transaction, checking that the complete set still matches. A newly added, removed or reassigned VM invalidates the preview. Do not hold a subset of claims while waiting for the remainder. No database transaction spans an agent call, guest call or prompt.

Validate key metadata and obtain any necessary interactive credential input before long-lived VM claims. Recheck references and public fingerprint before use. An existing explicit profile lock remains effective; necessary unlocking uses the already-approved explicit in-flow Unlock action. Noninteractive execution returns needs-unlock instead of opening a dialog. A file edit alone remains possible while locked.

Shared profile claims protect finite preparation/mutation operations, not the entire lifetime of every detached SSH tunnel or VM. Persistent connections are tracked separately in `connection_bindings`. Credential transitions therefore require both exclusive admission and a verified connection-revocation barrier. Claims alone do not revoke already-open agent channels.

## Ordered credential transition

1. **Prepare durable intent.** Freeze old/new credential IDs, member generation, member IDs, per-VM old/target identity revisions, owned-file hashes, and previous VM states. Record all pending targets and the profile transition marker before external effects. A profile with zero VM members still uses the agent/credential part of this protocol.
2. **Close profile access.** Prevent new activation/forwarding through the transition marker. Close every owned profile-forwarding connection and verify it has stopped. Record each outcome. A foreign, untracked or uncertain forwarding path blocks cutover; do not unlink it or claim revocation. This affects agent access, not arbitrary already-authenticated Git sessions or other guest workloads.
3. **Prepare guest updates.** On already-running, reachable VMs, stage only owned public identity files and verify expected old contents. A user edit blocks that member. On verified stopped VMs, retain the immutable target and a blocked connection binding without booting the VM. A running but unreachable member blocks credential cutover until it can be inspected or is explicitly stopped; do not automatically stop it. Unknown VM state is not a stopped/deferred result. No VM is started merely to administer the profile.
4. **Switch the owned agent.** After all old forwarding paths are revoked, unload/stop the old owned agent as required by its lifecycle adapter and establish exactly the target key at the recorded endpoint. Verify the public fingerprint and offered-key set. Keep all forwarding blocked. A crash here can leave the external agent ahead of SQLite; the transition marker prevents an ordinary connection from reloading the old key. Externally shared agents cannot be switched by this step; [ADOPTED_AGENTS.md](ADOPTED_AGENTS.md) requires its separate same-key migration before credential administration.
5. **Commit credential ownership and continuations.** Once the agent result is known, atomically advance the profile's active credential and future-VM default. Retain each member's pending target until verified. Record per-VM completion operations, linked to the parent, with the necessary claims before releasing the parent's profile-wide exclusion. An unfinished guest effect transfers to its recorded completion operation; it is never dropped merely to release a claim.
6. **Apply and verify each guest.** Using Lima's transport without profile forwarding, install the recorded owned bundle, verify its effective settings and hashes, then record its applied revision. Establish the target-bound tunnel and verify local authentication/signing capability before marking that VM ready. User edits, unknown effects or failed checks keep that VM's profile access blocked. Successfully completed members need not wait for a stopped member once the credential commit and all continuations are durable.
7. **Report exact outcomes.** Report the committed credential separately from ready, pending and blocked VM identities. The parent credential operation can finish after its defined commit/handoff postconditions; the overall `profile apply` result stays partial while any requested VM application is pending or failed. Do not label all VMs updated because the key changed. Future profile changes must respect unfinished member operations and targets.

The profile-wide gate may reopen only after credential commit and durable assignment of every unfinished member to a safe continuation. Each VM still has its own gate. Opening that gate requires the committed credential, that VM's verified applied identity, no pending target, and the matching connection binding. The implementation must not expose the new agent at an old guest binding simply because its socket pathname is unchanged.

A verified stopped member with no dispatched guest effect has a prepared continuation and a persisted connection gate, not an indefinitely held VM execution claim. Release its mutation claims explicitly after recording that safe state. On an explicit start/open, acquire the VM claim and hand off the saved identity continuation within that recorded workflow; do not run two conflicting executors. A member with an uncertain dispatched effect retains the claims needed to reconcile it. This distinction prevents a deferred identity update from deadlocking the VM start needed to finish it.

## Stopped VMs and later connections

A stopped VM remains stopped, with an explicit pending target. On its next explicit start/open flow, resume only that authorized identity continuation before exposing the profile to the guest. Apply the frozen pending bundle rather than current TOML. This is completion of the user's profile apply, not an environment/tool upgrade or a new implicit profile choice.

The private alias proxy never edits guest identity files or starts a VM. If it encounters a pending target, it returns a specific pending-update result with the existing `easy-lima code`/`ssh` completion path. An ordinary healthy VM without a pending transition retains the accepted automatic profile reuse and editor reconnect experience.

If an already-running VM cannot finish its update after credential commit, retain its own completion operation/claims and keep its profile access blocked. Other members can use the committed key only after their own target is verified. VM-only operations need no Git credential; conflicting mutations still follow T7's claim/reconciliation rules. Diagnostics and unrelated profiles remain independent. An explicit profile Lock request coordinates with an active transition through those same rules; a continuation cannot clear its inhibition. An externally managed agent refuses Lock before effects under [ELD-02](ADOPTED_AGENTS.md).

## Recovery decisions

Reconcile the existing parent and per-VM operations before redispatch. Keep the same profile, VM IDs, key fingerprints and frozen target bundles. Recover forward toward the approved target; returning to the old credential is a separate explicit, full-scope plan, not automatic rollback.

Repeating an identical apply locates and reconciles its existing member operations. It does not create replacement targets or repeat the committed shared-key switch. A still-stopped member remains deferred until an explicit start/open. Explicitly superseding a pending target is allowed only after its old effects are reconciled to a known safe boundary.

| Durable state and observed evidence | Permitted continuation |
| --- | --- |
| Intent exists; no external effect confirmed | Revalidate membership, ownership, lock state and old bindings before proceeding. Cancellation before dispatch leaves the old setup intact. |
| Transition is fenced; some forward closures are unknown | Keep the gate closed and claims retained; observe the same connections. Do not switch the key. |
| Some guest bundles are staged or installed; credential not committed | Compare each owned file with its old/target hash. Continue only known states; third-party values are conflicts. Keep profile forwarding closed. |
| Target key is present but SQLite still names the old credential | Under the recorded transition, verify the exact target key and revocation evidence, then complete the intended commit/handoff. Ordinary activation cannot choose either revision on its own. |
| Agent is empty after a crash/restart | Resume the recorded transition and its required unlock flow. An empty socket is not permission to abandon the target or clear explicit lock inhibition. |
| A foreign key, extra key, owner change or unproved effect appears | Preserve evidence and stop with conflict/recovery instructions. Do not infer success, reset ownership, load an alternative identity or automatically roll back. |
| Credential committed; one member ready and another pending | Reuse the committed key for the verified member. Resume the pending member's linked operation without replaying completed members or changing its target. |
| Stopped member has a pending target | Keep it stopped until an explicit start/open; apply and verify the authorized target before forwarding. |

## Required implementation scenarios

These extend the planned `internal/identity/profile_test.go`, `internal/connection/connection_test.go`, `internal/app/recovery_test.go`, and acceptance suite; they are not tests executed during drafting.

- Two VMs share K1. Selecting only one for K2 fails before a key, bundle or tunnel changes. A separate-profile reassignment is allowed only after existing overlap checks pass.
- A metadata-only change applies to one VM while the other retains its prior Git name/email and credential. A subsequent key-only change preserves those distinct metadata revisions while updating both credential references.
- Membership changes after preview, including a reserved new VM or unfinished reassignment, invalidate the plan or block admission; no silent scope expansion occurs.
- Interrupt after each forward closes, after one guest bundle changes, after the agent changes but before SQLite commit, and after only one VM becomes ready. Recovery follows the table and never offers a wrong key or loses a pending target.
- Rotate with one running and one stopped VM. The stopped VM does not boot; the command reports its pending status. Its next explicit open applies the saved target before Git access. A direct proxy cannot bypass that gate.
- Denied unlock, explicit profile lock, agent restart, a user-edited guest file and an unreachable member preserve truthful status and do not fall back to another profile. Unrelated and profile-free VMs remain usable.
- A completed parent with durable per-VM handoffs does not lose their claims or pending targets. A later apply cannot silently overwrite them.

See [ELD-01 in the live review report](consolidated_review_report.md#eld-01-make-shared-credential-changes-profile-wide) for acceptance and review status. The original review inputs and reports remain unchanged.

The complete relational additions and external publication/restore predicates are specified in [STATE_CONTRACT.md](STATE_CONTRACT.md). Exact public field and command spelling is specified in [INTERFACE_V1.md](INTERFACE_V1.md), approved on 2026-11-19; independent re-review pending.

**Pass — R2-READY-01 is resolved at the document-contract level.** I found no concrete remaining inconsistency within this focused correction.

The revised model now distinguishes selected membership, verified applied identity, and an explicit pending target. Independent profile/revision foreign keys permit A’s applied identity to remain recorded while B is pending. The `none` target retains a completion operation for deferred profile-free cleanup; `preparing` and `committed` distinguish the two sides of membership cutover. See current `STATE_CONTRACT.md` line 21 and constraints at lines 24–30.

Historical tunnels reference immutable registration/profile/revision identities, rather than mutable `registrations.profile_id`. Retiring them preserves those references. Current connection authority separately requires matching membership and applied generations, cleared pending state, and successful probes. The owning completion operation can probe behind the gate without enabling ordinary proxy use. See current `STATE_CONTRACT.md` line 57 and line 66.

The assign/unassign protocol now specifies admission, reservations, verified revocation, atomic membership cutover, durable completion operations, guest cleanup/application, and readiness publication. It explicitly fences a pending `none` target, preventing profile-free selection from bypassing cleanup. See current `PROFILE_TRANSITIONS.md` lines 92–104.

The existing same-profile credential protocol uses `preparing` before effects and `committed` after credential cutover. Membership remains unchanged in that case, so the revised equality constraints are compatible with the protocol. Its stopped-member claim handoff remains explicit at current `PROFILE_TRANSITIONS.md` lines 53, 57 and 63. Assignment applies the same safe deferral rule at line 98: stopped VMs without dispatched guest effects release execution claims while retaining their pending operation and connection gate.

Required implementation evidence remains:

- Stopped A→B, A→none, and profile-free→B, followed by explicit start/open.
- Retained retired tunnel rows and valid foreign keys throughout cutover.
- Interruptions around revocation, membership commit, guest effects, and atomic readiness publication.
- Same-profile credential rotation with running/stopped members under the new phases.
- Concurrent assignment/rotation, locked targets, user-edited cleanup, and refusal of proxy access for either pending kind.
- Safe stopped continuations releasing VM claims; uncertain dispatched effects retaining them.

These scenarios are specified in current `PROFILE_TRANSITIONS.md` lines 106 and 112–118, and mapped into `design_draft.md` lines 313–315.

Read the current root `PROFILE_TRANSITIONS.md`, `STATE_CONTRACT.md`, and relevant manifest entries. This was a read-only correction verification; no tests or live operations were performed, and no runtime validation or broader design approval is claimed. This is focused verification by the original reviewer, not a new fresh-context review round.

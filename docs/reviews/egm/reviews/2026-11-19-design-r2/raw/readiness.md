Implementation readiness does **not yet meet the zero-decision standard**. I found **1 Major**, **0 Blockers**, and **0 Nits**. The file manifest, incremental build order, and verification gates are substantial; the remaining finding is a concrete mismatch between approved profile commands and the persistence/transition contracts.

**R2-READY-01 — Major — Profile reassignment and removal lack a representable membership transition**

**Affected implementation scope:** schema/admission and profile implementation in [design_draft.md:265](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/inputs/design_draft.md:265), [design_draft.md:286](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/inputs/design_draft.md:286), [design_draft.md:292](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/inputs/design_draft.md:292), and [design_draft.md:299](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/inputs/design_draft.md:299).

**Evidence:**

- [INTERFACE_V1.md:172–173](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/inputs/INTERFACE_V1.md:172) approves reassignment between profiles and explicit transition to profile-free, with pending guest changes and no VM start from profile administration.
- [PROFILE_TRANSITIONS.md:15](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/inputs/PROFILE_TRANSITIONS.md:15) identifies reassignment scope but does not define its ordered transition. The detailed protocol at lines 51–69 changes a credential within one profile.
- [STATE_CONTRACT.md:21](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/inputs/STATE_CONTRACT.md:21) binds both applied and pending identity revisions to the registration’s selected profile. A pending revision and its completion operation must occur together; profile-free registrations have null identity references.
- [STATE_CONTRACT.md:49](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/inputs/STATE_CONTRACT.md:49) also binds connection records to the registration’s profile through composite foreign keys. Connection records explicitly retain retired state and retirement time at lines 54–56, while referenced history has restricted deletion under line 9.

**Concrete failure scenario:** A stopped VM belongs to profile A, has a verified applied A identity and an existing A connection record. The user assigns it to B.

Keeping `registrations.profile_id=A` prevents storing a B pending identity under the specified composite foreign key. Changing it to B invalidates the old applied A reference and the existing A connection record. Marking the connection retired does not resolve its foreign key: the constraint is not conditional on connection state. Consequently, even a completed reassignment cannot retain that historical connection row under the specified mutable parent tuple.

Unassigning the stopped VM has another representation gap: guest cleanup must remain pending without starting the VM, but a profile-free registration cannot retain identity references, and the completion-operation invariant requires a non-null pending revision. The documents do not select an alternative representation or specify when membership changes, old forwarding becomes unusable, and deferred cleanup completes.

An implementer would have to invent a membership protocol, change the schema, discard/rewrite history, or alter the accepted stopped-VM behavior.

**Actionable correction:** Specify an ordered assign/unassign protocol and its persistence representation before implementing these commands. It must distinguish verified old guest identity from the pending target membership, including an explicit profile-free target. Either introduce immutable membership/history references or explicitly decouple historical connection rows from the registration’s mutable current-profile tuple. If existing binding reservations are intended to carry this transition, define their exact relationship to pending identity fields, connection gates, membership generations, claim handoff, and completion.

Add scenarios to the already planned profile/connection/recovery tests covering:

- Stopped A→B assignment and A→profile-free removal, with no implicit VM start.
- Completion on the next explicit start/open.
- Reassignment while retained retired connection rows exist.
- Interruption around revocation, membership commit, and guest changes.
- Foreign-key validity, preserved old evidence, and no access through the wrong profile at every boundary.

**Material read**

Reviewer instructions and standard:

- `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/implementation_readiness.md`
- `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/implementation-readiness-standard.md`

All document references above and below refer to the frozen directory:

`/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/inputs/`

Read in full:

| File | Lines |
|---|---:|
| `design_draft.md` | 1–374; implementation audit scope 256–374 |
| `TECHNICAL_DETAILS.md` | 1–851 |
| `STATE_CONTRACT.md` | 1–141 |
| `PROVISIONING.md` | 1–130 |
| `ADOPTION.md` | 1–76 |
| `INTERFACE_V1.md` | 1–297 |
| `PROFILE_TRANSITIONS.md` | 1–104 |
| `ADOPTED_AGENTS.md` | 1–77 |
| `LIMA_BOUNDARIES.md` | 1–110 |
| `FEATURES.md` | 1–214 |
| `UX_PROPOSAL.md` | 1–113 |

The finding follows directly from the frozen contracts and requires no external factual assumption. I did not read earlier review reports, inspect live resources or credentials, execute application/prototype/VM commands, write files, or run tests. Missing implementation files, publisher version selections, capacity measurements, and pending runtime qualification were not treated as findings. This report does not authorize implementation.

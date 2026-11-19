Review result: **2 Major findings; 0 Blockers; 0 Nits.**

Scope: the complete frozen `design_draft.md`, lines 1–374, including architecture and implementation planning, cross-checked against the supplied baselines and normative supplements. This is a document audit, not runtime qualification or implementation approval.

**R2-DESIGN-01 — Major: Historical tunnel foreign keys prevent profile reassignment**

Affected references:

- [STATE_CONTRACT.md:21](../inputs/STATE_CONTRACT.md:21): `(registrations.id, registrations.profile_id)` is the parent tuple for connection membership.
- [STATE_CONTRACT.md:46](../inputs/STATE_CONTRACT.md:46), lines 46–58: connection rows retain profile, credential and identity references, including rows in `retired` state.
- [INTERFACE_V1.md:172](../inputs/INTERFACE_V1.md:172), lines 172–173: approved assign/unassign commands change a registration’s selected profile.
- [design_draft.md:222](../inputs/design_draft.md:222): delegates these exact relational constraints to the state supplement.

Concrete trigger: VM V has used profile A and therefore has a `connection_bindings` row referencing `(V,A)`. The user assigns V to profile B, or unassigns it. Revoking the tunnel and marking its row `retired` does not disable its foreign key. Updating the registration to `(V,B)` or `(V,NULL)` removes the referenced parent tuple, so the transition fails referential integrity. Deferring the constraint only postpones failure until commit. Cascading the new profile into the historical row would instead falsify its old binding and conflict with its A-owned credential/identity references.

The schema couples historical connection evidence to mutable current membership, although the public contract requires reassignment while preserving recovery evidence.

Actionable correction: reference the registration by immutable ID independently of the connection’s historical profile. Keep composite references from the connection to its immutable profile/credential/identity revisions. Enforce agreement with the registration’s current profile only for states allowed to expose a live tunnel, using the existing readiness/admission predicate. Specify how old and target membership are represented during deferred assign/unassign operations. Add FK-enabled scenarios for A→B and A→no-profile after a tunnel has been retired, including interruption and a stopped VM.

**R2-DESIGN-02 — Major: Restore recovery does not recognize the database after its own final write**

Affected references:

- [STATE_CONTRACT.md:129](../inputs/STATE_CONTRACT.md:129), lines 129–135: swap manifest, candidate installation, restore-event commit and hash-based interruption recovery.
- [STATE_CONTRACT.md:87](../inputs/STATE_CONTRACT.md:87): the restore fence blocks ordinary registry opening.
- [TECHNICAL_DETAILS.md:805](../inputs/TECHNICAL_DETAILS.md:805): interrupted recovery uses recorded files/hashes and stops on unknown contents.
- [design_draft.md:246](../inputs/design_draft.md:246): promises a journaled, recoverable restore.

Concrete trigger: restore installs the candidate file set, reopens it, commits the new restore event/generation, then crashes before marking the swap verified and removing `restore.intent.json`. This is an explicitly required write after the candidate hashes were recorded. The installed database/WAL group can therefore legitimately differ from the manifest’s original candidate group. Publication may also have advanced the external head or namespace epoch.

The interruption rule identifies files only as exact old or candidate hashes. It defines no permitted continuation for this legitimate post-install generation. Ordinary opening remains fenced, while the prescribed file comparison cannot recognize the restore’s own changes. Repeating finalization without a recorded event identity also risks appending another restore generation. The general publication protocol does not settle how this changed candidate is admitted through the outstanding swap fence.

Actionable correction: extend the restore journal with a preallocated restore-event identity and expected logical before/after generations or digests. Distinguish an untouched installed candidate from a candidate whose recorded restore event committed and whose publication is pending or complete. Under exclusive maintenance, validate the candidate’s complete file group and exact event lineage before completing the existing publication/fence cleanup. Do not obtain acceptance merely by refreshing hashes from whatever files are present. Add crash cases immediately after the restore-event SQL commit, after local/namespace publication, and after marking verified but before fence removal.

**Read-completion evidence.** All files below were read in full from `/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/inputs`:

- `design_draft.md` — 1–374.
- `FEATURES.md` — 1–214.
- `UX_PROPOSAL.md` — 1–113.
- `TECHNICAL_DETAILS.md` — 1–851.
- `INTERFACE_V1.md` — 1–297.
- `PROFILE_TRANSITIONS.md` — 1–104.
- `ADOPTED_AGENTS.md` — 1–77.
- `LIMA_BOUNDARIES.md` — 1–110.
- `ADOPTION.md` — 1–76.
- `PROVISIONING.md` — 1–130.
- `STATE_CONTRACT.md` — 1–141.
- `README.md` — 1–133.
- `concepts_brief.md` — 1–89.
- `DECISIONS.md` — 1–102.

Also read in full: [Design Critic persona](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/design_critic.md) and [Feature Specification Standard](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/feature-spec-standard.md).

The assembled specification addresses the standard’s problem, goals/non-goals, architecture, alternatives and risk requirements. The findings concern concrete transition/recovery gaps. No earlier review reports, credentials or live state were inspected; no files were changed, commands executed against the application/prototype/VMs, or external sources consulted.

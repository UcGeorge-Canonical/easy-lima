# easy-lima: final targeted design re-review

**Date:** 2026-11-19. **Status:** Resolved at design level; no open findings.

George Uche-Umeh approved the complete [v1 interface contract](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/INTERFACE_V1.md) and the announced final re-review. Four fresh-context reviewers audited the frozen assembled design. Five raw Major findings consolidated to four corrections; all were accepted under the standing delegation, applied to the documents and checked by the relevant reviewers. No Blockers or Nits were reported. The public commands and accepted user experience are unchanged by these internal corrections.

This was a targeted re-review, followed by focused correction verification using the original reviewers. It was not another full five-stage pipeline, two consecutive clean rounds, or runtime certification. No application code, application tests, dependency builds, VM/credential changes, commits or publication were performed. Documentation parsing, link and integrity checks are reported separately.

## Findings and resolution

| ID | Severity | Raw findings | Resolution and verification |
| --- | --- | --- | --- |
| ELR-01 | Major | R2-DESIGN-01, R2-READY-01 | Applied and verified: selected membership, old applied identity and pending targets are separate; profile-free cleanup is explicit; historical tunnels reference immutable IDs. |
| ELR-02 | Major | R2-DESIGN-02 | Applied and verified: restore finalization journals one event and exact logical before/after generations before writable opening; crashes resume that event and its publication. |
| ELR-03 | Major | R2-CODE-01 | Applied and verified: the legacy fixture binds module ROOT to its sandbox, supplies exact synthetic script data and on-disk SSH-config fixtures, and exercises complete positive controls. |
| ELR-04 | Major | R2-STYLE-01 | Applied and verified: references consistently identify the exact interface as approved, separately from runtime qualification. Final sentence-level copy edits are applied. |

### ELR-01: Preserve identity history through assignment and removal

The old schema tied every applied identity and historical tunnel to mutable current membership. Assigning a stopped VM from A to B would violate foreign keys; profile-free cleanup had no pending-target representation. [PROFILE_TRANSITIONS.md](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/PROFILE_TRANSITIONS.md#assignment-reassignment-and-profile-free-cleanup) now defines admission, reservations, revocation, membership cutover, deferred guest work and readiness. [STATE_CONTRACT.md](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/STATE_CONTRACT.md#existing-table-additions) defines independent immutable references, explicit target kind/phase and generation checks. A stopped VM remains stopped; old applied evidence never grants old-profile access after cutover.

Verification: [design reviewer](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/verification-design.md) and [readiness reviewer](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/verification-readiness.md). Required runtime scenarios cover A→B, A→none, profile-free→B, retained tunnel rows, interruption, locked targets, concurrent rotation and owned-setting conflicts.

### ELR-02: Recognize restore's own committed generation

The old swap protocol compared only pre-write file hashes after its own final database write. [The corrected restore protocol](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/STATE_CONTRACT.md#swap-ordering) uses exact physical placement before finalization, then an event ID and expected logical generations/digests journaled before writable access. It explicitly resumes after SQL commit, local publication, namespace publication and verified-before-fence-removal crashes. It refuses unexpected lineage and never blesses files by refreshing expected hashes.

Verification: [design reviewer](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/verification-design.md). The file/test manifest includes the corresponding fault cases; these are future implementation tests.

### ELR-03: Make the legacy fixture exercise the real paths

`Manager(root=...)` did not change the helper's module ROOT, and real provisioning reads a script before a fakeable subprocess call. [ADOPTION.md](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/ADOPTION.md#executable-legacy-fixture-eld-10) now specifies the sandbox binding, exact synthetic bytes/hash, real digest calculation and fake responses. It also supplies the on-disk instance `ssh.config` actually read by `update_ssh`, with include-preservation assertions. The fixture contract distinguishes an already-stopped stop as a successful no-op from ownership refusal. No guest script or live helper is executed during review.

Verification: [codebase reviewer](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/verification-codebase.md). The first focused check found and retained one residual fake-command mismatch; its correction then passed.

### ELR-04: Record the interface approval consistently

The exact interface was approved, but older passages still called its flags/results proposed or deferred. The architecture, technical references and manifest now consistently use the approved contract. This records an existing author decision and needs no renewed approval. Verification: [style reviewer](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/verification-style.md).

## Review evidence

| Reviewer | Initial review | Raw result | Focused closure |
| --- | --- | --- | --- |
| Design critic | Completed; persona, standard and supplied contracts read | [2 Major](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/raw/design.md) | Passed both corrections |
| Implementation readiness | Completed; persona, standard and required contracts read | [1 Major](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/raw/readiness.md) | Passed membership correction |
| Codebase details | Completed; required documents and allowed prototype source read; six source hashes matched | [1 Major](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/raw/codebase.md) | Passed after recorded fixture residual was fixed |
| Style | Completed; persona and all supplied contracts read | [1 Major](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/raw/style.md) | Approval authority passed; requested copy edits applied |

The [frozen manifest](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/manifest.json) records the target and input hashes; architecture range was 1–255 and implementation range 256–374. [Dispatch](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/dispatch.json), [triage](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/triage.json), [annotated corrections](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/goldfish_review_report.md), and [document verification](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/verification.json) preserve completion and validation evidence. Fresh conversation context used `fork_turns=none`; filesystem and instruction-file isolation are not asserted.

The first review's nineteen corrections, including [interface approval](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/resolutions/ELD-11-12/decision.md), remain recorded. Its [pre-approval live report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r2/prior-live-report.md), frozen inputs, raw reports and the [closed concept review](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/goldfish_review_report.md) are preserved unchanged. Historical pending statuses describe their snapshot date; this report is the current disposition.

## Implementation boundary

The approved design is ready for implementation against [the file and verification plan](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/design_draft.md#detailed-implementation-plan). Exact host/Lima compatibility, SQLite behavior, credentials, actual VM adoption, preset workloads and interruption recovery still require the planned executable qualification. The review closes documented design findings; it does not advertise runtime support.

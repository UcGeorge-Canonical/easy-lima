# easy-lima

- **Author:** George Uche-Umeh
- **Updated:** 2026-11-19
- **Status:** V1 scope approved on 2026-11-18. All six concept Goldfish reviews completed; all 11 findings are accepted and resolved. The complete [implementation design draft](design_draft.md) is ready for review, with [public experience details](UX_PROPOSAL.md) approved on 2026-11-19. A fresh implementation-design review is in progress; runtime qualification remains pending. The application has not been implemented.

## Overview

easy-lima is a standalone, installable binary for project-oriented Lima VMs. Running it from any working directory opens a project browser rooted there; an explicit path can select a different starting directory. A developer selects a folder, starts its persistent Ubuntu VM, and connects through SSH or VS Code. Registrations and user settings persist across working directories. A grouping folder creates one VM for that folder, not separate VMs for every child.

The existing `make local` helper is a separate prototype. easy-lima requires neither that Makefile nor a source checkout or prescribed workspace layout, and has no compatibility entry points for the helper. Lima remains a runtime prerequisite. The author selected Go for the standalone binary, with reuse of suitable scripts and test scenarios.

The author prefers a straightforward Lima-derived name and has proposed `easy-lima`. The earlier independent-product naming direction is superseded. Lima is the runtime; a generic layer over competing VM providers is outside the chosen direction.

## Analysis

The existing helper establishes a single-Mac workflow with persistent VMs, nested selection, development tooling, SSH/editor access, storage safeguards, and one signing identity. Public reuse requires explicit configuration and tested behavior.

Public reuse adds installation, ownership/rebinding, profiles, configuration boundaries, diagnosis/recovery, and adoption. Storage safety and identity separation remain essential.

Reviewers supported reusing Lima's mechanisms and avoiding a second environment platform. The main prototype risks are path-derived instance identities, provisioning tied to current wrapper code, and global identity administration.

## Approved V1 Scope

The approved source of truth is [FEATURES.md](FEATURES.md). It defines F1–F9, release criteria, limits, approved platforms, later feature groups, and acceptance checks. It includes the author's standalone-application clarification and accepted identity, profile-management, storage, environment-update, rebinding, adoption-ownership, recovery, diagnostics, and logging rules, plus the selective-reuse direction (GF-01, GF-02, GF-03, GF-04, GF-05, GF-06, GF-07, GF-08, GF-09, GF-10, and GF-11).

V1 covers installation/setup; nested project selection and CLI parity; persistent lifecycle and stable project associations; native Lima configuration and limited Ubuntu presets; storage/resource visibility; SSH, VS Code, shared files, and explicit ports; optional named Git identities; diagnosis/recovery; and a public release with prototype adoption.

macOS and Linux are the approved first-release targets. The design proposes named host/architecture/Lima combinations for qualification; advertised support requires passing the release matrix. Windows/WSL is deferred.

## Goals vs. Non-Goals

**Goals:** Other developers can install the tool without machine-specific edits, use a complete project VM workflow, choose storage deliberately, keep identities separate, and understand failures. Existing instances and credentials survive adoption.

**Later proposals:** Automatic storage relocation, snapshot/clone controls, multiple environments per project, branch/worktree workflows, more editors, Workshop integration, and Windows support.

**Outside the chosen direction:** An independent VM engine, competing-provider abstraction, SDK marketplace, and hosted cloud platform.

## Alternatives Weighed

The author narrowed the earlier broad environment-manager idea to a Lima wrapper and rejected clever independent naming. A direct name such as easy-lima reflects that boundary.

**Accepted reuse direction (GF-11):** Plain Lima templates and saved SSH settings provide VM mechanisms but do not by themselves supply the agreed project browser, persistent project associations, deliberate storage workflow, and isolated identity selection. The standalone application adds those workflows while delegating VM operations to Lima. Use selective reuse of portable prototype behavior and suitable code as the starting point for implementation design, with fresh implementation where it offers a clear advantage. Evaluate distribution, portability, maintainability, and testing trade-offs before selecting the language and dependencies. The prototype’s Makefile, machine-specific assumptions, and path-derived identities do not define the application. Existing behavior and tests are evidence to assess, not proof that the v1 guarantees already pass.

| Alternative | Mechanism and trade-off | Decision |
| --- | --- | --- |
| Plain Lima templates and saved SSH settings | Use Lima directly with manually maintained project, storage, and identity conventions. This minimizes application code but leaves the agreed workflow and its safeguards to each user. | Does not meet the approved application scope on its own; Lima remains the underlying VM mechanism. |
| Selective reuse | Adapt suitable prototype code, tests, and provisioning knowledge while replacing assumptions that conflict with the public application. This can preserve useful behavior, but extracting machine-specific or tightly coupled logic may cost more than replacing it. | Starting point for the design comparison, subject to the standalone application requirements and v1 validation. |
| Fresh implementation | Implement components anew where distribution, portability, maintainability, or testing materially improves. This allows different boundaries and language choices, but requires reproducing and validating established behavior. | Use where it offers a clear advantage; a full rewrite is not selected in advance. |

Workshop remains a possible later guest integration. Lima's formats and operations remain the foundation. The subsequent implementation-design decision T1 selects Go for the standalone binary while retaining suitable scripts and test scenarios. The design selects release mechanics and records the installation experience and license in UX_PROPOSAL.md, approved on 2026-11-19.

## Review Synthesis — Resolved Decisions

The [annotated Goldfish report](goldfish_review_report.md) contains all findings, frozen-document/code references, six reviewer statuses, and resolution decisions. All eleven findings are accepted:

- **GF-01 — Accepted:** At most one profile applies throughout a VM; profiles remain optional. Reject registered parent/child overlaps with different profiles, including conflicts introduced by adoption, rebinding, or profile changes. Descendant registrations do not switch a parent VM's identity. Checks state which repositories were actually examined.
- **GF-02 — Accepted:** One configured VM storage root covers all new and adopted VMs, independent of the working directory. No automatic moves or additional-location exceptions. Initial setup may explicitly select the existing VM location as the root; conflicting adoption otherwise stops before changing ownership, registrations, or the selected root.
- **GF-10 — Accepted:** Keep operational logs by default, excluding known secrets. Display raw command/provisioning output normally, but save it through easy-lima only with explicit opt-in to user-only files. Do not automatically share captures or claim raw output is universally sanitized. Identify separately any potentially sensitive output already retained by Lima or the guest.
- **GF-03 — Accepted:** Updating the binary or starting a VM never automatically upgrades its tools or presets. Environment updates require an explicit action with a preview. Each VM retains its resolved inputs and provisioning revision; interrupted setup uses its recorded operation, and adoption diagnoses unknown history instead of applying the latest preset.
- **GF-04 — Accepted:** A stable registration identifies the actual VM and SSH alias independently of paths or CWD. Explicit rebinding preserves the VM, disk, and alias when folders move. Reject conflicting registrations and identity overlaps; interrupted rebinding remains diagnosable and recoverable. Adoption retains original names and aliases.

- **GF-05 — Accepted:** Adoption transfers exclusive management to easy-lima after backing up affected metadata and retiring the old helper’s ownership. Handoffs are diagnosable and recoverable. Reject adoption, downgrades, or metadata restoration that would create conflicting owners; no compatibility launcher is introduced.

- **GF-06 — Accepted:** Profile administration is independent of VM startup and provisioning. Preserve the complete usable key/unlock/agent/SSH/Git setup during adoption. Replacement or removal changes only easy-lima-owned settings, preserving user settings and reporting conflicts. Unavailable profiles do not disrupt unrelated or profile-free projects.

- **GF-07 — Accepted:** Retain operation inputs, instance association, last confirmed stage, and outcome. Reconcile previous work before retrying and reuse its VM; do not overlap active work. If the state remains unknown, stop with recovery instructions. Operational evidence excludes known secrets and does not require raw capture. Arbitrary script rollback is outside v1.

- **GF-08 — Accepted:** Diagnostics are read-only and return useful partial results when dependencies or individual records are unavailable. Separate VM, agent, forwarding, signing, transport/authentication, and expected-account results; unavailable GitHub verification remains unverified. Distinguish recorded information from observed state and unknown/unreachable from not created. Repairs are separate, explicit actions.

- **GF-09 — Accepted:** Check conservative creation requirements for shipped presets across all download, temporary, cache, and VM destinations, combining requirements where they share capacity. Reject known insufficient space before download or allocation. Report current allocation and logical disk maximum separately, with future VM growth explicitly unreserved.

- **GF-11 — Accepted:** Start implementation design with selective reuse of suitable prototype behavior and code. Choose fresh implementation where it offers a clear advantage, comparing distribution, portability, maintainability, and testing before selecting language or dependencies.

All ten Major findings and the advisory finding are accepted and applied to the specification and concept brief. No review findings remain unresolved. No reviewer found a concept blocker or proposed removing approved v1 scope. Existing browsing, ownership safeguards, SSH integration, and identity/workload checks are reference and reuse candidates; static review does not establish runtime compatibility or adoption success.

## Key Decisions and Next Steps

Approved v1 scope: F1–F9, a standalone binary usable from any working directory, macOS plus Linux as initial hosts, Ubuntu as the tested guest, one VM per project, one optional identity profile per VM, rejection of parent/child overlaps with different profiles, and one VM storage root for new and adopted VMs with no automatic moves. Operational logs are enabled by default with known secrets excluded; raw output capture is opt-in. Updating existing tools or presets requires an explicit environment-update action with a preview. Explicit project rebinding preserves the VM, disk, and SSH alias, rejects conflicts, and is recoverable if interrupted. Adoption requires exclusive ownership transfer, metadata backups, and a recoverable handoff; conflicting adoption or downgrades are rejected. Profile management is independent of VM startup; adoption preserves usable identities, and changes affect only owned settings with user conflicts reported. Retries reconcile previous work and reuse its VM; unknown state stops progress with recovery instructions. Arbitrary script rollback is outside v1. Diagnostics remain read-only, return useful partial results with separate check statuses, and leave repairs to explicit actions. Creation checks cover all actual destinations and shared capacity; future VM growth remains explicitly unreserved. Selective reuse is the starting point, with fresh implementation where it offers a clear advantage. Workshop integration remains deferred. Go is selected for the host application. V1 operations run in foreground commands, with VMs and profile SSH agents remaining available after command exit. The installed limactl CLI is the v1 integration, with Lima kept as an external dependency and compatibility checked explicitly. SQLite is authoritative for managed state, with a separate diagnostic-only recovery inventory. TOML is selected for easy-lima configuration while Lima retains native YAML; the proposed per-user locations, selected-folder discovery, and field-specific precedence are approved. WAL is selected with explicit checkpointing and preserved read-only diagnostics. T7 selects `github.com/ncruces/go-sqlite3` through `database/sql` and the internal operation state model. T8 selects isolated OpenSSH agents and host lifecycle adapters. The author accepts optional remembering and requires a new VM with an existing profile to have Git authentication and signing configured automatically. Reuse a loaded key or restore it from an accessible remembered credential without a per-VM setup prompt or separate unlock command; respect explicit locks and handle necessary OS interaction in the current flow. Exact versions and runtime compatibility still require validation.

On 2026-11-18 the author delegated decisions that do not directly affect the user interface and experience. The assistant should choose internal mechanisms, document the reasoning, and continue without backend-choice approval gates. Commands, prompts, defaults, installation experience, credential unlocking, and visible recovery behavior remain for author review. Existing accepted policies and the design-only phase remain in force.

The author closed the concept review and approved the opening Overview, Problem, Goals, and Non-Goals sections of [design_draft.md](design_draft.md). T1–T8 record accepted language, execution, Lima, persistence, configuration, and credential-experience decisions. T9–T14 complete connection, storage, recovery/adoption, state-maintenance, interface-library, and release design under the internal-choice delegation. The complete draft includes alternatives, a file-by-file implementation plan, requirement traceability, and validation gates. The author approved the consolidated [v1 experience](UX_PROPOSAL.md) on 2026-11-19 and authorized the fresh implementation-design Goldfish review. The existing helper remains separate; implementation has not begun.

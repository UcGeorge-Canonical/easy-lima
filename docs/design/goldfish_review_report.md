> Design record: this document predates implementation. For current behavior and validation, see the [user guides](../../README.md) and [test record](../testing.md). Historical status statements remain part of the decision trail.

# Goldfish Annotated Review Report

**Status:** resolved — 2026-11-18

**Product:** easy-lima · **Author:** George Uche-Umeh · **Round:** 2026-11-18-v1 · **Mode:** BRAINSTORM

**Reviewed targets:** frozen [FEATURES.md](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md), lines 1–158, and frozen [concepts_brief.md](../reviews/egm/reviews/2026-11-18-v1/inputs/concepts_brief.md), lines 1–49. References below point to these frozen inputs or the whitelisted prototype source.

All six reviewers completed. The review found **0 Blockers, 10 Major findings, and 1 Nit** after consolidating 21 raw findings. The original review identified three policy decisions, seven implementation-design requirements, and one advisory finding. No reviewer recommended removing approved v1 scope or adding another readiness gate. The author has resolved all findings and requested implementation design.

The F1–F9 boundaries, macOS/Linux scope, one persistent Ubuntu VM per registered project, Lima ownership of VM mechanisms, and deferred features remain intact. Since review, the author has accepted GF-01, GF-02, GF-03, GF-04, GF-05, GF-06, GF-07, GF-08, GF-09, GF-10, and GF-11 and clarified that easy-lima is a standalone installable binary, usable from any working directory, with no `make local` compatibility entry points. All findings are accepted and applied to the specification and concept brief. “Major” means a consequential ambiguity or design-entry risk to address; it does not mean an implementation design must already exist before concept review can finish.

## Resolution outcome

**Outcome:** 11 findings accepted and applied to the specification and concept brief; 0 contested, 0 escalated, 0 declined, and 0 unaddressed. Original severities are unchanged. A resolved specification finding is not a claim that the application implements or passes it.

The author resolved each finding through the interactive discussion. The final decision accepts selective reuse as the starting point, with fresh implementation where it offers a clear advantage. All three policy clarifications, seven implementation-design requirements, and the advisory finding are now recorded as accepted requirements or direction. The author authorized the transition to implementation design on 2026-11-18.

The standalone-binary boundary is an explicit author clarification, not a new Goldfish finding. It supersedes the former compatibility-shim option and is reflected in F1/F9 and the accepted GF-05 ownership rule. The existing `make local` tool remains separate.

| Finding | Category | Current decision |
| --- | --- | --- |
| GF-01 | PROPOSE FIX | accepted and applied to specification |
| GF-02 | PROPOSE FIX | accepted and applied to specification |
| GF-03 | PROPOSE FIX | accepted and applied to specification |
| GF-04 | PROPOSE FIX | accepted and applied to specification |
| GF-05 | PROPOSE FIX | accepted and applied to specification |
| GF-06 | PROPOSE FIX | accepted and applied to specification |
| GF-07 | PROPOSE FIX | accepted and applied to specification |
| GF-08 | PROPOSE FIX | accepted and applied to specification |
| GF-09 | PROPOSE FIX | accepted and applied to specification |
| GF-10 | PROPOSE FIX | accepted and applied to specification |
| GF-11 | PROPOSE FIX | accepted and applied to specification |

The [original review](../reviews/egm/reviews/2026-11-18-v1/original_report.md), frozen inputs, raw reports, and original triage remain historical evidence. [Relocation metadata](../reviews/egm/reviews/2026-11-18-v1/relocation.json) maps their former paths into this repository. These resolution annotations are authored in the primary conversation; no new Goldfish review has been run.

## Reviewer status and evidence

| Reviewer | Native agent identifier | Wave | Status | Required reads | Raw findings |
| --- | --- | --- | --- | --- | --- |
| egm-design | `/root/goldfish_synthesis/design_brainstormer` | 1 | completed | Self-reported complete | 4 Major |
| egm-codebase-details | `/root/goldfish_synthesis/codebase_brainstormer` | 1 | completed | Self-reported complete | 4 Major |
| egm-zoom-out | `/root/goldfish_synthesis/zoom_out_brainstormer` | 2 | completed | Self-reported complete | 3 Major, 1 Nit |
| egm-comprehension | `/root/goldfish_synthesis/comprehension_brainstormer` | 2 | completed | Self-reported complete | 2 Major, 1 Nit |
| egm-implementation-readiness | `/root/goldfish_synthesis/implementation_readiness_brainstormer` | 3 | completed | Self-reported complete | 3 Major |
| egm-structural | `/root/goldfish_synthesis/structural_brainstormer` | 3 | completed | Self-reported complete | 3 Major |

The native tool returned canonical task names, recorded above as agent identifiers. The synthesizer observed each native dispatch and final completion, with no failure, timeout, or retry. Every final report acknowledges its persona, both complete frozen targets, and both complete standards. Child read-tool traces are not exposed to this parent, so these acknowledgments are **self-reported evidence**, not independently verified native read traces.

Each general-purpose reviewer was launched with `fork_turns: "none"`, received identical frozen inputs and standards, and was not supplied parent transcripts or earlier findings. Persona mappings were confirmed from bundled frontmatter before dispatch. Four active-agent slots were available: root and synthesizer occupied two, leaving two reviewers at a time. The six reviewers ran as three pairs; raw-report persistence overlapped later reviewer work. The host’s exclusion of project/user instruction files from fresh contexts is unverified.

The [dispatch record](../reviews/egm/reviews/2026-11-18-v1/dispatch-details.json) contains actual persona, input, standard, agent-ID, scope, and status details. Raw reports were preserved before deduplication: [design](../reviews/egm/reviews/2026-11-18-v1/raw/egm-design.md), [codebase](../reviews/egm/reviews/2026-11-18-v1/raw/egm-codebase-details.md), [zoom-out](../reviews/egm/reviews/2026-11-18-v1/raw/egm-zoom-out.md), [comprehension](../reviews/egm/reviews/2026-11-18-v1/raw/egm-comprehension.md), [implementation readiness](../reviews/egm/reviews/2026-11-18-v1/raw/egm-implementation-readiness.md), and [structural](../reviews/egm/reviews/2026-11-18-v1/raw/egm-structural.md).

## Original review severity and disposition

| Disposition | Blocker | Major | Nit | Consolidated IDs |
| --- | ---: | ---: | ---: | --- |
| Author decision needed | 0 | 3 | 0 | GF-01, GF-02, GF-10 |
| Implementation-design requirement | 0 | 7 | 0 | GF-03–GF-09 |
| Advisory | 0 | 0 | 1 | GF-11 |
| **Total** | **0** | **10** | **1** | **11 findings** |

Raw totals were 0 Blockers, 19 Major findings, and 2 Nits. Repeated observations are consolidated, not counted as additional independent risks.

## Prioritized findings

### GF-01 — Grouping folders need an explicit identity and shared-source boundary — Major

**Original disposition:** author decision needed. **Resolution:** accepted and applied to the specification. **Lenses:** design, comprehension, zoom-out.

The concept permits nested and grouping selections while assigning profiles to projects ([F2/F3](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L24), [F6/F7](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L68)). A work-profile grouping VM can therefore mount the same checkout as a personal-profile child VM. Separate disks and agents do not separate the shared repository or make its child registration’s identity apply inside the parent VM. The prototype sets a guest-global identity ([git-guest.py:63](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-guest.py:63)); its override check examines `/workspace` only ([git-check.py:23](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-check.py:23)).

**Original recommendation:** Declare at most one active profile per VM, applying across its mounted tree; descendant registrations do not silently switch credentials. Choose whether known conflicting-profile overlaps are refused or require explicit acknowledgment. Show known overlaps and the selected profile before creation/rebinding, and state which repositories an identity check actually validated. This does not require recursive repository discovery or credential multiplexing.

**Acceptance example:** A grouping folder and a registered child use different profiles. The behavior is explained before creation, and no unexamined child is reported as identity-validated.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author chose one profile per VM and rejection of parent/child overlaps with different profiles. This resolves the ambiguity without adding descendant credential routing. The original warning/acknowledgment alternative is rejected.

**Applied requirement — F7 and release acceptance check 3 in FEATURES.md:**

> A VM uses at most one selected profile throughout its mounted project tree; Git integration remains optional. Descendant registrations do not switch that VM’s identity. Reject known parent/child source overlaps between registered projects with different profiles during creation, adoption, rebinding, and profile changes. Compare resolved source paths, report the conflicting projects/profiles, and identify which repositories an identity check actually examined.

### GF-02 — Reconcile the single storage root with adoption in place — Major

**Original disposition:** author decision needed. **Resolution:** accepted and applied to the specification. **Lens:** comprehension.

Setup selects storage, F5 limits v1 to one managed root, and F9 preserves existing storage destinations ([workflow](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L18), [F5](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L58), [F9](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L94)). The case “choose a new root, then adopt the prototype at its old destination” needs a visible policy.

**Original recommendation:** Prefer the smallest interpretation consistent with the approved one-root boundary: adoption establishes the existing root, and a conflicting setup choice requires an explicit correction showing both paths. An alternative is a documented exception permitting adopted locations while the selected root governs new creation; that exception requires author approval. Neither option moves disks or introduces storage pools.

**Acceptance example:** A user selects a destination different from the prototype’s. Onboarding explains the conflict and applies the chosen policy before allocation or metadata cutover, without silent relocation or silent root changes.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author accepted one VM storage root for v1, including adopted VMs, and no automatic moves. This resolves the adoption/storage conflict without grandfathered locations. The configured root remains independent of the current working directory.

**Applied requirement — F5 storage boundary and F9 adoption acceptance in FEATURES.md:**

> All new and adopted VMs use one configured managed VM storage root, independent of the current working directory. Adoption keeps disks in place and requires their existing storage root to match the configured root. During initial setup, before other VMs are managed, the user may explicitly select the existing location as that root. Otherwise, a conflicting adoption stops before ownership or metadata changes and shows both paths. It cannot silently switch roots, orphan existing registrations, or add an exception for another location. No automatic moves.

### GF-03 — Preserve environment provenance across wrapper updates and adoption — Major

**Original disposition:** implementation-design requirement. **Resolution:** accepted and applied to the specification. **Lenses:** design, zoom-out, implementation readiness, structural.

F4 forbids silent upgrades on ordinary startup and F9 preserves adopted environments ([F4](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L50), [F9](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L94)). The prototype fingerprints the currently installed provisioning script and current options, then runs it when the digest changes ([local.py:369](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:369), [local.py:378](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:378)). The script installs packages, changes Snap channels, and rewrites configuration ([provision.sh:14](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:14)). Reusing this policy could turn a wrapper upgrade or adoption into an unsolicited guest update.

**Original recommendation:** Record the environment’s resolved inputs and applied provisioning revision independently of the installed wrapper. Distinguish completed legacy setup, incomplete setup with known inputs, unknown provenance, and explicitly requested changes. Resume the recorded operation; do not substitute new defaults or treat an unrecognized marker as permission to execute the latest preset. Use Lima provisioning facilities and a limited set of wrapper-owned fields; preserve or explicitly reject incompatible custom settings.

**Acceptance examples:** Updating easy-lima and starting a completed VM causes no unsolicited package/channel transition. Adoption preserves a completed legacy environment; an interrupted legacy setup remains diagnosable and resumable using its recorded inputs.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author accepted an explicit environment-update action with a preview and prohibited automatic tool or preset upgrades when updating easy-lima or starting a VM. Recording the environment inputs and provisioning revision makes that boundary survive retries and adoption without substituting the current binary’s defaults.

**Applied requirement — F4 environment-update boundary, F9 adoption acceptance, and release acceptance check 11 in FEATURES.md:**

> Updating easy-lima or starting a VM never automatically upgrades its tools or presets. Each VM retains its resolved template/creation inputs and applied provisioning revision independently of the installed binary. Interrupted setup resumes its recorded operation instead of substituting current defaults. Applying newer tools or presets requires an explicit environment-update action with a preview before changes run. The preview identifies requested changes and known restart or recreation requirements. Adoption preserves completed legacy setup and distinguishes it from incomplete or unknown setup; unknown provisioning history requires diagnosis instead of automatically applying the latest preset.

### GF-04 — Make registration authoritative rather than deriving VM identity from paths — Major

**Original disposition:** implementation-design requirement. **Resolution:** accepted and applied to the specification. **Lenses:** zoom-out, structural.

Stable rebinding and preserved adoption aliases are explicit promises ([F3](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L42), [F9](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L94)). Today the workspace and relative path derive the name and alias ([local.py:77](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:77)); record lookup and ownership depend on that name ([local.py:302](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:302)), and the TUI repeats the derivation ([tui.py:146](/Volumes/Nebula/dev/Canonical/local/.local-launcher/tui.py:146)). The current remount operation changes a source under an existing logical path, so it is not complete rebinding support.

**Original recommendation:** Persist a stable registration with its actual Lima instance locator and SSH alias, preserving legacy values. Treat paths as rebindable associations, and use one lookup across CLI, TUI, lifecycle, and adoption. Define collision and interrupted-rebind behavior without choosing an unnecessary database or abstraction.

**Acceptance example:** Adopt, move the project or workspace, rebind, and reconnect through the preserved alias to the same guest disk. An interruption between mount and association updates retains enough evidence to reconcile the same instance.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author accepted an explicit rebind workflow that preserves the VM, guest disk, and SSH alias when a project moves, rejects conflicts, and remains recoverable if interrupted. Stable registration separates the VM association from project paths, CWD, and application location, including bindings retained during adoption.

**Applied requirement — F3 registration/rebinding, F9 preserved bindings, and release acceptance check 7 in FEATURES.md:**

> A persistent registration identifies the actual Lima instance and SSH alias independently of the project path, current working directory, and application installation location. CLI and TUI operations use that registration. Moving or renaming a project uses an explicit rebind workflow that updates its source association while preserving the VM, guest disk, and SSH alias. Reject conflicting registrations and identity overlaps before changing the association. Interrupted rebinding retains ownership and enough information about the old/new source and mount state to diagnose and recover the same instance without creating a duplicate. Adoption preserves original instance names and aliases as registered bindings.

### GF-05 — Adoption must transfer authority to one manager — Major

**Original disposition:** implementation-design requirement. **Resolution:** accepted and applied to the specification. **Lenses:** design, implementation readiness.

The prototype locks workspace-local state, while `make local` directly invokes it ([local.py:239](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:239), [local.py:285](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:285), [Makefile:5](/Volumes/Nebula/dev/Canonical/local/Makefile:5)). Importing records into an installed CLI could leave both launchers managing the same disk through different locks and settings, contrary to [F3/F9](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L94). Metadata backups alone cannot undo later mount, SSH, or identity changes.

**Original recommendation:** Design a recoverable ownership handoff with one coordination domain. Supported legacy entry points should delegate to v1 or refuse mutation after cutover. Preserve imported names and aliases. Define which interrupted phases can resume and when a downgrade must be refused pending reconciliation; a full guest rollback guarantee is unnecessary.

**Acceptance examples:** Interrupt adoption around cutover, invoke the legacy entry point afterward, and retry after a rebind. Each sequence retains one authoritative association and prevents competing managers from mutating the instance.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author accepted exclusive ownership transfer to easy-lima, metadata backups, and recoverable handoffs, and requires rejection of adoption or downgrades that would create conflicting owners. This closes the two-manager risk without a make local compatibility entry point. Metadata backups support recovery but do not establish safe rollback of later changes.

**Applied requirement — F9 adoption ownership and release acceptance check 9 in FEATURES.md:**

> Adoption explicitly transfers exclusive management ownership of an instance to easy-lima. Back up affected metadata and retire the old helper’s ownership of that instance before easy-lima can mutate it. Record handoff progress and the last established ownership state so interruptions can be diagnosed and recovered against the same VM. Refuse adoption if exclusive ownership cannot be established. Reject downgrades or metadata-restoration operations that would create conflicting owners; a backup does not by itself make a downgrade safe. Preserve the existing instance, disk, and connection identifiers throughout the handoff. This transfer introduces no compatibility launcher.

### GF-06 — Isolate profile administration and migrate the complete identity binding — Major

**Original disposition:** implementation-design requirement. **Resolution:** accepted and applied to the specification. **Lenses:** implementation readiness, structural.

F7 makes identities optional and selected per project; F9 preserves credentials ([F7](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L76), [F9](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L94)). The prototype has singleton credential/service/socket identifiers ([git_identity.py:14](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git_identity.py:14)), forwards one socket for every alias, and runs full startup during global Git setup ([local.py:317](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:317), [local.py:406](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:406)). Guest identity settings persist beyond a connection ([git-guest.py:29](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-guest.py:29)). A preserved fingerprint alone proves neither usability nor correct profile selection.

**Original recommendation:** Separate profile creation/import and platform-specific agent availability from project binding and guest application. Scope services and sockets to profiles. Adoption must preserve or deliberately migrate unlock references, managed SSH directives, and managed guest settings without generating a replacement key. Profile changes, including removal, should reconcile only wrapper-owned settings and report user conflicts. Use the approved stop/reconnect model.

**Acceptance examples:** Creating a personal profile does not start or alter work VMs. An unavailable profile does not block unrelated or identity-free projects. Adoption, profile replacement, and profile removal preserve the intended signing behavior and fail clearly after an interrupted transition.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author accepted profile management independent of VM startup, preservation of the complete identity setup during adoption, and modifications restricted to easy-lima-owned settings with user-edit conflicts reported. Preserving a fingerprint alone does not establish usable authentication/signing; the credential, agent, SSH, and guest configuration must remain coherent without affecting unrelated profiles.

**Applied requirement — F7 profile administration/migration, F9 usable identity preservation, and release acceptance check 3 in FEATURES.md:**

> Profile creation, import, repair, and agent administration are independent of VM startup and do not automatically start or provision VMs. Scope each profile’s agent and credential handling independently; an unavailable profile must not disrupt unrelated profiles or profile-free projects. Adoption preserves the complete usable identity setup: the existing key and fingerprint, its unlock mechanism or reference, agent binding, managed SSH settings, and managed guest Git identity and signing settings. Private keys remain on the host. Profile replacement or removal changes only settings owned by easy-lima, preserves user-owned settings, and reports conflicts with user edits instead of overwriting them. Apply changes only to explicitly affected projects; the existing stop/reconnect allowance remains sufficient for profile changes.

### GF-07 — Recovery needs attempt evidence and an indeterminate state — Major

**Original disposition:** implementation-design requirement. **Resolution:** accepted and applied to the specification. **Lens:** codebase details.

F3/F8 require recoverable setup and distinguishable completed/incomplete stages ([F3](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L42), [F8](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L86)). Ownership-before-creation is a useful foundation, but the prototype has one final provisioning marker and an append-only log ([local.py:353](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:353), [provision.sh:12](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:12), [provision.sh:97](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:97)). A host timeout and released lock do not establish that guest execution stopped.

**Original recommendation:** Retain minimal local operation/attempt identity, intended inputs, stage, last proven completion, outcome, and log locations. Represent unobserved work as indeterminate until reconciled; check whether an earlier attempt remains active before retrying against its instance. Correlation need not introduce telemetry services or a general workflow engine. Keep arbitrary-script rollback outside the guarantee.

**Acceptance examples:** Interrupt creation, provisioning, identity application, and final connection validation separately. Diagnosis names the known stage and uncertainty, and retry does not create another VM or overlap unreconciled work.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author requires reconciliation before retrying, reuse of the existing VM, and stopping with recovery instructions if the earlier attempt’s state remains unknown. Durable stage evidence supports that decision without assuming a disconnected guest process stopped. Arbitrary script rollback remains outside v1.

**Applied requirement — F3 retry rule, F8 recovery boundary, and release acceptance check 6 in FEATURES.md:**

> Persist each operation attempt’s instance association, intended inputs or configuration references, last confirmed completed stage, outcome, and relevant log locations. Keep this operational evidence available without opt-in raw capture and exclude known secrets. A timeout, disconnection, or unobserved result leaves the outcome unknown until reconciled. Before retrying, determine whether the previous attempt is still active or has completed, and reuse its recorded VM and inputs. Do not overlap an active attempt or allocate a replacement VM merely because observation failed. If the prior state remains unknown, stop with recovery instructions instead of retrying. Automatic rollback of arbitrary guest scripts is outside v1.

### GF-08 — Diagnostics must survive unavailable prerequisites and distinguish failure layers — Major

**Original disposition:** implementation-design requirement. **Resolution:** accepted and applied to the specification. **Lenses:** codebase details, comprehension.

F8 promises useful paths and distinguishable failures ([F8](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L86)). Prototype inspection first requires live Lima/storage, registry loading is all-or-nothing, and the TUI can label unavailable state “Not created” ([local.py:281](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:281), [local.py:299](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:299), [local.py:490](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:490), [tui.py:165](/Volumes/Nebula/dev/Canonical/local/.local-launcher/tui.py:165)). Identity checks also collapse distinct failures into an empty agent or key-registration advice ([git_identity.py:50](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git_identity.py:50), [git-check.py:48](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-check.py:48)).

**Original recommendation:** Separate locally known associations/paths from observed runtime state, isolate malformed records, and report failed, unavailable, skipped, and unknown checks truthfully. Keep inspection distinct from repair/provisioning. For identity, separate profile/agent readiness, forwarding, local signing, external transport/authentication, and expected-account comparison. Preserve offline checks and define when each check blocks an operation.

**Acceptance examples:** Missing storage, missing Lima, or one corrupt record still yields useful nonmutating diagnosis. GitHub unavailability is neither “wrong account” nor “verified”; a usable VM and its optional online verification have separate statuses.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author accepted read-only diagnostics with useful partial results and separate statuses for each check, and requires repairs to be explicit actions. Keeping locally recorded information available and distinguishing VM readiness, local identity checks, and online verification prevents unavailable prerequisites from producing misleading absence, mismatch, or success reports.

**Applied requirement — F8 diagnostic boundary and release acceptance check 12 in FEATURES.md:**

> Inspection and doctor are read-only and return useful partial results when prerequisites are unavailable. Report readable local registrations, expected paths, and log locations even when Lima, storage, SSH, or a profile cannot be reached, distinguishing recorded information from observed runtime state. One malformed record must not hide other environments. Give each check its own result, distinguishing success, failure, unavailable, skipped, and unknown as appropriate; inaccessible or unknown state must not be reported as not created. Separate VM readiness, profile/host-agent readiness, guest forwarding, local signing, external transport/authentication, and expected-account verification. GitHub unavailability leaves account verification unverified, not mismatched or successful. Diagnosis must not implicitly start VMs, provision environments, or repair settings; repairs are separate, explicit actions. Partial diagnostic results continue to exclude known secrets and do not enable raw output capture.

### GF-09 — Capacity checks must reflect actual creation destinations and footprint — Major

**Original disposition:** implementation-design requirement. **Resolution:** accepted and applied to the specification. **Lens:** design.

F5 requires capacity checks, cache visibility, and correct sparse-disk reporting ([F5](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L58)). The prototype uses a fixed 10 GiB creation threshold for one destination ([local.py:361](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:361)). That threshold is not evidence that each shipped preset’s downloads, temporary files, cache, and VM allocation will fit. Apparent destinations may share the same filesystem capacity.

**Original recommendation:** Establish conservative creation requirements for the shipped presets and evaluate the actual affected filesystems, accounting for shared capacity. Distinguish current allocation, logical maximum, and unreserved future growth. Keep the promised CPU/RAM visibility, but do not infer a resource scheduler or guarantee arbitrary workload growth.

**Acceptance examples:** A known insufficient-space condition fails before download/allocation on either distinct or shared cache/VM filesystems. Missing external storage never triggers fallback, and sparse capacity is not labeled as current usage.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author accepted checks across all creation destinations, accounting for shared filesystem capacity, with future VM growth explicitly unreserved. Conservative requirements for shipped presets replace the fixed-threshold assumption while keeping current allocation, logical disk limits, and future workload growth distinct. This strengthens creation checks without introducing capacity reservation or a resource scheduler.

**Applied requirement — F5 creation-capacity boundary and release acceptance check 5 in FEATURES.md:**

> Check conservative creation requirements for each shipped preset against the actual filesystems used for downloads, temporary files, cache, and VM allocation. Account for destinations sharing the same free-space budget by evaluating their combined requirements. Reject known insufficient-space conditions before download or allocation. Report current allocation and logical disk maximum separately, and explicitly identify future VM growth as unreserved. Passing the creation check does not reserve capacity for arbitrary later workloads.

### GF-10 — Bound the promise that logs avoid secrets — Major

**Original disposition:** author decision needed. **Resolution:** accepted and applied to the specification. **Lens:** codebase details.

F4 accepts user YAML and F6 accepts guest commands, while F8 says logs avoid secrets ([F4](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L50), [F6](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L68), [F8](../reviews/egm/reviews/2026-11-18-v1/inputs/FEATURES.md#L86)). The prototype’s generic errors include arguments/output and provisioning stores raw output ([local.py:37](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:37), [provision.sh:12](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:12)); its sensitive-command wrapper already avoids known secret-bearing details ([git_identity.py:84](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git_identity.py:84)). Arbitrary custom output can itself contain credentials.

**Original recommendation:** Clarify that wrapper-generated diagnostics omit known secrets and unnecessary credential-bearing arguments/environment values; raw custom-command or provisioning output is treated as potentially sensitive. Decide its persistence, access/retention, and disclosure behavior without promising universal sanitization. A support-bundle exporter remains deferred.

**Acceptance examples:** Known secret-handling failures retain stage/status/next-action context without private values. Custom output is not implicitly presented as sanitized or safe to share.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author chose operational logs enabled by default with known secrets excluded, and explicit opt-in for raw output capture. This preserves ordinary diagnostic context without treating arbitrary command output as safe to retain or share. Existing Lima/guest logs remain a separately identified boundary.

**Applied requirement — F8 logging boundary and release acceptance check 10 in FEATURES.md:**

> Operational logs record steps, results, and useful failure details by default, excluding known secrets and unnecessary credential-bearing arguments or environment values. Raw command and provisioning output is displayed normally, but easy-lima saves it only when the user explicitly enables capture. Captured files are accessible only to the current user and are not automatically included in shareable diagnostics. Raw output may contain sensitive values; there is no guarantee of universal sanitization. Identify separately any output already retained by Lima or the guest and treat it as potentially sensitive. Access and retention mechanisms remain implementation-design work.

### GF-11 — Record the incremental reuse alternative — Nit

**Original disposition:** advisory. **Resolution:** accepted and applied to the concept brief and specification. **Lens:** zoom-out.

The brief’s [Alternatives Weighed](../reviews/egm/reviews/2026-11-18-v1/inputs/concepts_brief.md#L37) records product direction and naming but provides little technical comparison. The feature standard asks for alternatives and concrete trade-offs. Existing code already supplies lazy browsing, atomic writes, ownership checks, SSH include preservation, isolated-agent checks, signing verification, and provisioning/workload knowledge.

**Original recommendation:** Record why plain Lima templates plus saved SSH settings are insufficient for the requested project associations, storage safeguards, and identity workflow. Treat incremental extraction of the prototype as the baseline when comparing a rewrite or new dependencies; preserve useful behavior while replacing the assumptions identified above. No implementation language or packaging choice is implied.

**AI suggestion:** PROPOSE FIX. **Resolution status:** accepted and applied to specification.

**Justification:** The author accepted selective reuse as the starting point, with fresh implementation where it offers a clear advantage, and requested closure of the review. Recording the comparison preserves useful prototype knowledge while allowing the standalone application’s distribution, portability, maintainability, and testing needs to determine its implementation. This does not select a language, dependency, or full rewrite in advance.

**Applied direction — Alternatives Weighed in concepts_brief.md and Implementation-design starting point in FEATURES.md:**

> Plain Lima templates and saved SSH settings provide VM mechanisms but do not by themselves supply the agreed project browser, persistent project associations, deliberate storage workflow, and isolated identity selection. The standalone application adds those workflows while delegating VM operations to Lima. Use selective reuse of portable prototype behavior and suitable code as the starting point for implementation design, with fresh implementation where it offers a clear advantage. Evaluate distribution, portability, maintainability, and testing trade-offs before selecting the language and dependencies. The prototype’s Makefile, machine-specific assumptions, and path-derived identities do not define the application. Existing behavior and tests are evidence to assess, not proof that the v1 guarantees already pass.

## Reuse and boundaries worth preserving

Zoom-out inspected all ten whitelisted source/test files. Concrete candidates are the [lazy tree](/Volumes/Nebula/dev/Canonical/local/.local-launcher/tui.py:11), [atomic writes](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:52), [ownership and locking](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:285), [SSH Include preservation](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:220), [public-only identity payloads](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git_identity.py:77), [disposable signing checks](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-check.py:29), [retry-aware LXD steps](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:61), and the [thin Make adapter](/Volumes/Nebula/dev/Canonical/local/Makefile:9). These are reuse candidates, not evidence that expanded v1 guarantees already pass.

Reviewers consistently supported the Lima boundary, optional Git, truthful agent/mount trust model, explicit storage and ports, no unrelated-instance management, and deferral of moves, snapshots, variants, Workshop, and Windows. Linux support remains approved and must earn its compatibility claims through the later release checks.

## Raw finding mapping and triage decisions

“Retained” and “Merged” describe the synthesizer's treatment of a review finding. They do not mean the author has accepted a recommendation or scope amendment.

| Raw finding | Raw severity | Consolidated | Treatment |
| --- | --- | --- | --- |
| DESIGN-01 | Major | GF-01 | Retained; shared-source/identity ambiguity |
| DESIGN-02 | Major | GF-03 | Merged; wrapper-triggered guest changes |
| DESIGN-03 | Major | GF-05 | Merged; adoption authority |
| DESIGN-04 | Major | GF-09 | Retained; creation capacity, without a scheduler |
| CODEBASE-01 | Major | GF-08 | Merged; degraded diagnostic availability |
| CODEBASE-02 | Major | GF-07 | Retained; attempt evidence and reconciliation |
| CODEBASE-03 | Major | GF-10 | Retained; pending logging boundary clarification |
| CODEBASE-04 | Major | GF-08 | Merged; identity failure classification |
| ZOOM-01 | Major | GF-04 | Merged; registration authority |
| ZOOM-02 | Major | GF-03 | Merged; provisioning provenance and Lima boundary |
| ZOOM-03 | Major | GF-01 | Merged; VM-wide identity model |
| ZOOM-04 | Nit | GF-11 | Retained as advisory |
| COMPREHENSION-01 | Major | GF-01 | Merged; user expectations for child assignments |
| COMPREHENSION-02 | Major | GF-02 | Retained; pending storage/adoption policy |
| COMPREHENSION-03 | Nit | GF-08 | Merged as UX detail of a Major diagnostic risk |
| READINESS-01 | Major | GF-05 | Merged; cutover and safe downgrade refusal |
| READINESS-02 | Major | GF-03 | Merged; completed/incomplete legacy provisioning |
| READINESS-03 | Major | GF-06 | Merged; usable credential binding during migration |
| STRUCTURAL-01 | Major | GF-04 | Merged; path/registration separation |
| STRUCTURAL-02 | Major | GF-03 | Merged; recorded revisions and custom configuration boundary |
| STRUCTURAL-03 | Major | GF-06 | Merged; profile administration without global startup |

No whole raw finding was rejected or downgraded. COMPREHENSION-03 retains its raw Nit rating; its incorporation into GF-08 does not independently promote a cosmetic issue to Major. GF-08’s severity follows the broader operational failure described by CODEBASE-01/04. The original synthesis retained acceptance examples as recommendations. The resolution annotations identify the author-approved GF-01, GF-02, GF-03, GF-04, GF-05, GF-06, GF-07, GF-08, GF-09, GF-10, and GF-11 changes; no proposed resolution remains undecided.

The synthesis rejects broader interpretations that the findings do not justify: automatic per-descendant identity routing, arbitrary guest rollback, a general provisioning/telemetry platform, or storage scheduling. The author rejected grandfathered adoption locations in GF-02: one VM storage root covers both new and adopted VMs, with no automatic moves. Exact versions, packaging, Linux credential mechanisms, and schema filenames remain legitimate design decisions; no concrete contradiction makes their current TBD status a concept blocker.

## Scope and method limitations

This was a static BRAINSTORM review of concept quality and design-entry risks. It was not a release certification, runtime compatibility check, security audit, migration trial, or completeness review of an implementation document. The evidence whitelist was `README.md`, `Makefile`, `local-vms.toml`, and the eight source/script plus two test files under `.local-launcher/`. Tests were read, not run. No installations, VM commands, migrations, or source changes were performed. Reviewers report no inspection of live `.local-state`, credentials, Keychain, host SSH settings, agent state, unrelated repositories, or conversation/memory services. No external sources were used.

The root’s initial integrity check reported that both frozen inputs and all 13 source-evidence files matched the manifest. No claim is made that prototype hazards have occurred in the current live environment, or that the suggested mitigations have been implemented. The raw reports, [dispatch record](../reviews/egm/reviews/2026-11-18-v1/dispatch-details.json), and [machine-readable triage](../reviews/egm/reviews/2026-11-18-v1/triage.json) preserve the review trail. The updated [concept brief](concepts_brief.md) records all accepted decisions. Implementation design has begun in [design_draft.md](design_draft.md); no production implementation has been written.

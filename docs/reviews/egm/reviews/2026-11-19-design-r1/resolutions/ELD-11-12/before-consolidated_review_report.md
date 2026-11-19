# easy-lima: consolidated implementation-design review

**Date:** 2026-11-19
**Review:** All five stages completed; all nine fresh-context reviewer runs completed.
**Readiness:** Revisions required before production implementation.
**Findings:** 0 Blockers, 14 Major findings, 5 Nits after consolidation. Raw reports contain 22 Major and 6 Nit entries, including explicitly carried findings.
**Corrections:** Seventeen findings have applied design/document corrections: 12 Major and 5 Nit items. They have not received a new independent review. **Open: 2 Major findings, ELD-11/ELD-12**, drafted together for author review.

The author approved the v1 experience on 2026-11-19. That approval remains recorded; this review identifies enforcement gaps and missing contracts, and does not silently replace accepted user-facing behavior. The comprehension and structural reviewers found no issues within their scopes. The implementation-readiness reviewer found that a new implementer would still need to design public schemas, state publication and provisioning evidence.

Reviewed target: [frozen design, lines 1–1239](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md). FeatureRange was 1–1129; DetailedRange was 1130–1239. Findings below cite this frozen copy. [The working draft](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/design_draft.md) contains the seventeen applied findings, with linked profile, adopted-agent, Lima, adoption, provisioning and state contracts; the frozen input and original reviewer reports are preserved.

This was one consolidated review round. It is not a converged re-review or runtime certification. No production implementation, builds, VM operations, credential changes, commits or publishing were performed.

## Priorities and authority

The author accepted ELD-01 on 2026-11-19: credential changes cover every VM using the profile; a subset needing a different key uses a separate profile. [The transition contract](PROFILE_TRANSITIONS.md) now specifies its revisions, effects and recovery. The author accepted ELD-02 on 2026-11-19: preserve verified use of external agents and refuse Lock/credential administration until explicit migration. [Its capability contract](ADOPTED_AGENTS.md) is applied. Internal ELD-03–ELD-05 corrections are applied in [the Lima boundary contract](LIMA_BOUNDARIES.md). The author accepted ELD-08 with “Yep”; [ADOPTION.md](ADOPTION.md) now specifies the previewed stop, proven quiescence, exclusive handoff and eligible prior-state restoration.

Internal corrections need no new backend-choice approval under the standing delegation. The remaining internal findings are specified and applied; an applied design is not runtime evidence. [INTERFACE_V1.md](INTERFACE_V1.md) is the complete proposed command/configuration/result appendix for the remaining author decision. Independent re-review follows that decision.

| ID | Severity | Action | State |
| --- | --- | --- | --- |
| ELD-01 | Major | Make shared credential changes profile-wide | Applied — author approved; re-review pending |
| ELD-02 | Major | Define administration of externally managed adopted agents | Applied — author approved; re-review pending |
| ELD-03 | Major | Include namespace-wide Lima configuration in frozen inputs | Applied — re-review pending |
| ELD-04 | Major | Specify VM host-key enrollment and verification | Applied — re-review pending |
| ELD-05 | Major | Suppress implicit application-port forwarding | Applied — re-review pending |
| ELD-06 | Major | Separate design approval, implementation qualification and release gates | Applied — not independently re-reviewed |
| ELD-07 | Major | State the approved GitHub HTTPS-to-SSH mapping normatively | Applied — not independently re-reviewed |
| ELD-08 | Major | Prove legacy guest work has stopped before adoption cutover | Applied — re-review pending |
| ELD-09 | Major | Define development and Docker preset composition | Applied — re-review pending |
| ELD-10 | Major | Specify an executable legacy-ownership contract fixture | Applied — re-review pending |
| ELD-11 | Major | Finish normative TOML schemas and CLI input grammar | Drafted — author review pending |
| ELD-12 | Major | Finish command-result schemas, stable codes and exit precedence | Drafted — author review pending |
| ELD-13 | Major | Complete state schemas and ownership-publication predicates | Applied — re-review pending |
| ELD-14 | Major | Specify provisioning stages, receipts and replay decisions | Applied — re-review pending |
| ELD-15 | Nit | Separate architecture from implementation recipes | Applied — re-review pending |
| ELD-16 | Nit | Calibrate the Ubuntu support claim | Applied — not independently re-reviewed |
| ELD-17 | Nit | Describe SQLite as a selected trade-off | Applied — not independently re-reviewed |
| ELD-18 | Nit | Remove repeated drafting-only execution disclaimers | Applied — not independently re-reviewed |
| ELD-19 | Nit | Keep logging claims bounded to controlled fields and known secrets | Applied — not independently re-reviewed |

## Findings and proposed corrections

### ELD-01: Make shared credential changes profile-wide

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Author approved and applied to design; independent re-review pending.

**Scope:** T6–T8 profile revisions and claims; T11 profile change; implementation identity work. [Frozen design:548](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:548).

One shared agent offers one key, while the plan allows a changed profile to be applied to only some VMs. Replacing that key would affect VMs outside the selected set. Per-VM applied revisions and interrupted multi-VM transitions are also underspecified.

**Correction applied:** [PROFILE_TRANSITIONS.md](PROFILE_TRANSITIONS.md) defines full-member credential changes; subset changes use a separate profile. It separates the active credential from each VM's applied/pending identity, specifies the forwarding barrier and commit/handoff order, and keeps stopped VMs stopped until their authorized pending update is completed. The design, UX and feature scope are synchronized.

**Required acceptance evidence (not executed):** Two VMs share K1; a request to move only one to K2 must refuse without changing either, or explicitly use a different profile. Interrupt a full-scope transition after one guest changes; retained revisions must prevent either VM silently using the wrong identity.

**Authority:** Explicit author approval on 2026-11-19 (“Yes”); internal transition details follow the standing delegation. [Decision record](.egm/reviews/2026-11-19-design-r1/resolutions/ELD-01/decision.md).

**Raw findings:** [S5-ZOOM-01](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage5-zoom-out.md), [S8-ZOOM-01](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md), [S9-READY-05](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md).

### ELD-02: Define administration of externally managed adopted agents

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Author approved and applied to design; independent re-review pending.

**Scope:** T8 lock/activation; T9 persistent forwarding; T11 adoption. [Frozen design:1037](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:1037).

The adopted legacy agent may still serve non-adopted projects. Its supervisor reloads the key after restart. Unloading it disrupts those projects; an easy-lima inhibition flag alone cannot prevent reload or revoke existing forwards.

**Correction applied:** Applied: [ADOPTED_AGENTS.md](ADOPTED_AGENTS.md) defines resource-level ownership, the use/administration capability matrix, refusal of Lock and credential changes, same-key migration to a distinct owned agent, whole-profile forwarding quiescence, deferred VM completion and interruption recovery. External services, keys, credential items and non-adopted projects remain intact. The UX, feature scope and profile-transition contract are synchronized.

**Required acceptance evidence:** Planned, not executed: adopt one of two legacy projects, refuse unsupported Lock without effects, restart its external supervisor, then migrate to an independent owned binding and verify both projects retain their intended Git use. Interrupt migration and verify truthful capabilities/pending updates.

**Authority:** Explicit author approval on 2026-11-19 (“Agreed”); internal mechanisms follow the standing delegation. [Decision record](.egm/reviews/2026-11-19-design-r1/resolutions/ELD-02/decision.md).

**Raw findings:** [S5-ZOOM-02](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage5-zoom-out.md), [S7-DESIGN-03](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-design.md), [S8-ZOOM-02](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md), [S9-CODE-02](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-codebase.md).

### ELD-03: Include namespace-wide Lima configuration in frozen inputs

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Applied under internal-choice delegation; independent re-review pending.

**Scope:** T3 adapter; T4 retained inputs; T10 assets; T11 lifecycle/adoption. [Frozen design:267](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:267).

Lima reads _config/default.yaml and override.yaml on later starts, plus base.yaml during creation. Unchanged instance YAML therefore does not prove unchanged effective mounts, scripts, or resources.

**Correction applied:** Applied: [LIMA_BOUNDARIES.md](LIMA_BOUNDARIES.md#frozen-namespace-configuration-eld-03) specifies a concrete v1 boundary: absent/empty namespace globals only, no silent edits to shared files, retained presence/digests and consumed dependencies, separate creation-base provenance, pre/post-dispatch drift checks and conservative refusal. Native instance YAML remains supported within its recorded boundaries.

**Required acceptance evidence:** Planned, not executed: change only a namespace default/override and prove start/create/resume/adoption refuse before dispatch. Cover empty/absent files, symlinks, comment drift, creation-base-only changes and preservation of unrelated instances.

**Authority:** Internal design correction under the standing delegation.

**Raw findings:** [S7-DESIGN-01](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-design.md), [S8-ZOOM-03](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md).

**Primary sources used by reviewers:** [Lima global configuration](https://github.com/lima-vm/lima/blob/v2.2.1/templates/default.yaml#L625), [Lima loader](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/limayaml/load.go#L31).

### ELD-04: Specify VM host-key enrollment and verification

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Applied under internal-choice delegation; independent re-review pending.

**Scope:** T9 tunnel, proxy, aliases and diagnostics. [Frozen design:904](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:904).

The design promises verified VM SSH transport but specifies only GitHub key enrollment. Lima's generated settings disable strict host-key checking and use /dev/null for known hosts, so retaining them cannot establish the promised trust.

**Correction applied:** Applied: [LIMA_BOUNDARIES.md](LIMA_BOUNDARIES.md#vm-ssh-host-key-trust-eld-04) defines local endpoint trust-on-first-use with an explicit local-host assumption, imported pins, deferred stopped-VM enrollment, authoritative trust generations, strict owned SSH configuration/exports, mismatch refusal and explicit replacement repair. Diagnostics never enroll; Lima internal bootstrap is not misrepresented as pinned.

**Required acceptance evidence:** Planned, not executed: inspect controlled effective SSH configurations, enroll new/adopted VMs, substitute an endpoint/key after enrollment, preserve trust across port/rebind changes, and interrupt pin/export publication. Fail before profile exposure and leave diagnostic paths read-only.

**Authority:** Internal trust protocol; any added user prompt requires UX review.

**Raw findings:** [S7-DESIGN-02](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-design.md), [S8-ZOOM-04](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md).

**Primary sources used by reviewers:** [Lima SSH options](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/sshutil/sshutil.go#L375), [OpenSSH host-key checking](https://man.openbsd.org/ssh_config#StrictHostKeyChecking).

### ELD-05: Suppress implicit application-port forwarding

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Applied under internal-choice delegation; independent re-review pending.

**Scope:** T5 ports; T9 transport; native configuration generation/adoption. [Frozen design:513](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:513).

An empty requested-port list does not disable Lima's automatic localhost forwarding. Unlisted guest listeners could appear on host ports and conflict across projects.

**Correction applied:** Applied: [LIMA_BOUNDARIES.md](LIMA_BOUNDARIES.md#explicit-application-port-forwarding-eld-05) defines approved mappings followed by the exact catch-all ignore rule, explicit GRPC backend for persistent TCP/UDP mappings, localhost address/port/protocol accounting, the separate SSH transport, and pre-retirement refusal of incompatible adopted rules.

**Required acceptance evidence:** Planned, not executed: start unlisted IPv4/IPv6 TCP/UDP listeners in two VMs after creation/restart. Observe no unrequested host listeners; verify approved mappings, collisions, the SSH exception and adoption validation.

**Authority:** Internal enforcement of already approved explicit port exposure.

**Raw findings:** [S7-DESIGN-04](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-design.md), [S8-ZOOM-03](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md).

**Primary sources used by reviewers:** [Lima port forwarding](https://lima-vm.io/docs/config/port/#disabling-port-forwarding).

### ELD-06: Separate design approval, implementation qualification and release gates

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Applied — not independently re-reviewed.

**Scope:** T5/T7 verification wording; implementation build order. [Frozen design:529](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:529).

Some tests were required before implementation approval although they need implemented components; the final build step also combined design review with release review.

**Correction:** Applied: make design-findings resolution a prerequisite to production implementation, run driver/Lima qualification early during implementation, and require implementation review plus release gates before publication. Preserve the live-resource boundary in the completion section.

**Acceptance evidence:** Documentation consistency checked; the correction has not received a new independent review.

**Authority:** Internal documentation correction.

**Raw findings:** [S7-STYLE-01](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md), [S9-STYLE-01](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-style.md).

### ELD-07: State the approved GitHub HTTPS-to-SSH mapping normatively

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Applied — not independently re-reviewed.

**Scope:** T9 guest Git transport. [Frozen design:937](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:937).

The technical paragraph still described the approved mapping as optional/proposed, allowing an implementation to omit it.

**Correction:** Applied: new profile-enabled guests use the owned Git-only mapping; show it in the setup preview and reject user-configuration conflicts. Preserve repository remotes and host Git settings.

**Acceptance evidence:** Text checked against the approved UX. Runtime transport behavior remains an implementation acceptance test.

**Authority:** Alignment with the already approved UX contract.

**Raw findings:** [S7-STYLE-02](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md).

### ELD-08: Prove legacy guest work has stopped before adoption cutover

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Applied to design; independent re-review pending.

**Scope:** T11 handoff admission/recovery; adoption scenario fixtures. [Frozen design:1022](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:1022).

The legacy host lock can be released after a controller failure while a dispatched guest provision command continues. The final provisioning digest does not identify active jobs. Retiring the record prevents new old-helper calls but does not quiesce existing work.

**Correction applied:** [ADOPTION.md](ADOPTION.md) applies the accepted policy: Adoption previews a graceful stop of a running legacy VM, proves the same VM and legacy work are stopped, and restores its prior running state only after verified handoff and supported recorded startup. Already-stopped VMs stay stopped; unknown state blocks cutover and no force escalation is automatic.

**Required acceptance evidence (planned, not executed):** Terminate or time out the legacy controller after guest dispatch while its job survives. Ownership retirement alone must not admit a conflicting easy-lima mutation.

**Authority:** Explicit author acceptance on 2026-11-19 (“Yep”); internal mechanisms use the standing delegation. [Decision record](.egm/reviews/2026-11-19-design-r1/resolutions/ELD-08/decision.md).

**Raw findings:** [S9-CODE-01](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-codebase.md).

### ELD-09: Define development and Docker preset composition

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Applied to design; independent re-review pending.

**Scope:** T10 presets; preset scripts and workload release gates. [Frozen design:983](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:983).

The prototype includes a Docker/LXD forwarding workaround and combined workload checks. The new separate presets do not define installation/backend policy, order, owned network changes or restart behavior, and the release gate omits Docker-only/combined cases.

**Correction applied:** [PROVISIONING.md](PROVISIONING.md) defines official exact-version Docker packages, the iptables backend, owned pre-start gating and forwarding configuration, deterministic LXD/Docker address selection, per-tool holds, and Docker-only/combined restart/reboot workload gates. No global FORWARD-ACCEPT workaround is installed.

**Required acceptance evidence (planned, not executed):** Restart Docker, then verify Docker networking, LXD container networking and the Juju workload for the combined supported preset.

**Authority:** Internal implementation of the approved optional Docker combination.

**Raw findings:** [S9-CODE-03](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-codebase.md).

**Primary sources used by reviewers:** [Docker packet filtering](https://docs.docker.com/engine/network/packet-filtering-firewalls/#docker-on-a-router), [LXD firewall guidance](https://canonical.com/lxd/docs/latest/howto/network_bridge_firewalld/).

### ELD-10: Specify an executable legacy-ownership contract fixture

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Applied to design; independent re-review pending.

**Scope:** Prototype manifest, contract tests and test infrastructure. [Frozen design:1206](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:1206).

A JSON prototype manifest plus generic Go fakes cannot independently prove that the supported old helper refuses start/stop/delete after record retirement. Exact source provenance, executable fixture paths and the isolated subprocess harness are missing.

**Correction applied:** [ADOPTION.md](ADOPTION.md) defines a digest-pinned unaltered legacy source fixture, Python 3.11+ test-only driver, bounded fake Lima/process protocol, positive controls and zero-mutation refusal cases. All source/manifest/scenario/harness paths are in the implementation manifest; no fixture has been executed.

**Required acceptance evidence (planned, not executed):** Run the actual fixture against retired metadata with no live Lima namespace or credentials accessible. Verify refusal and an empty mutation log.

**Authority:** Internal test design and provenance work.

**Raw findings:** [S9-CODE-04](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-codebase.md).

### ELD-11: Finish normative TOML schemas and CLI input grammar

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Drafted — consolidated public appendix awaiting author review.

**Scope:** T5/T12 schemas; T13 CLI; model/config/examples/tests. [Frozen design:1045](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:1045).

The selected tables and illustrative example omit exact field shapes, validation, defaults and immutability for editor/UI, mounts/ports and profiles. Noninteractive profile inputs, secret file-descriptor flags and update/creation mappings are not fully specified.

**Proposal drafted:** [INTERFACE_V1.md](INTERFACE_V1.md) now proposes all field/type/default/validation rules, exact command inputs, mount/port input files, noninteractive profile setup and secret file-descriptor handling. Not approved or applied as a public contract yet.

**Acceptance evidence:** Use the normative examples as configuration and CLI contract fixtures, including missing values, duplicate/unknown fields, invalid quantities and noninteractive secret input.

**Authority:** Draft one consolidated public-interface appendix for author review.

**Raw findings:** [S9-READY-01](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md).

### ELD-12: Finish command-result schemas, stable codes and exit precedence

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Drafted — consolidated public appendix awaiting author review.

**Scope:** T12 diagnostics; T13 JSON; renderers/model/CLI tests. [Frozen design:1081](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:1081).

Only the JSON envelope and coarse numeric exits are defined. Command-specific data, outcome values, stable diagnostic codes, malformed-record representation and mixed-failure aggregation remain choices for the implementer.

**Proposal drafted:** [INTERFACE_V1.md](INTERFACE_V1.md) now proposes the complete result envelope, typed command payloads, stable check/error codes, malformed-row handling and mixed-result precedence, with representative JSON fixtures. Not approved or applied as a public contract yet.

**Acceptance evidence:** Golden contract fixtures must cover list with a malformed row and doctor with both a failed ownership check and unavailable provider verification.

**Authority:** Draft alongside the consolidated public-interface appendix.

**Raw findings:** [S9-READY-02](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md).

### ELD-13: Complete state schemas and ownership-publication predicates

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Applied to design; independent re-review pending.

**Scope:** T6/T12 SQLite additions; namespace markers; restore/handoff recovery. [Frozen design:1041](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:1041).

The four later tables and external ownership/epoch markers lack complete versioned representations, constraints, locations and recovery predicates. Phrases such as compare evidence or reject lost effects do not determine whether a backup or half-published marker is safe.

**Correction applied:** [STATE_CONTRACT.md](STATE_CONTRACT.md) defines complete added records/constraints, canonical marker payloads, critical generations, local then namespace publication, deterministic interruption predicates and exact-head backup/restore admission. Missing VM storage does not block unrelated local-only profile administration; inventory cannot authorize mutation.

**Required acceptance evidence (planned, not executed):** Fixtures must cover committed-but-unpublished epochs, stale backups, interrupted handoffs, contradictory markers and unavailable volume evidence without inventory-authorized mutation.

**Authority:** Internal protocol specification under the standing delegation.

**Raw findings:** [S9-READY-03](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md).

### ELD-14: Specify provisioning stages, receipts and replay decisions

**Severity:** Major. **Disposition:** PROPOSE FIX. **State:** Applied to design; independent re-review pending.

**Scope:** T7 evidence contracts; T10 provisioning; stage runner and scenario fixtures. [Frozen design:987](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:987).

The plan selects jobs, receipts and conservative reconciliation but leaves the shipped stage catalogue, receipt format, owned LXD configuration and stage-specific completion/replay predicates undefined.

**Correction applied:** [PROVISIONING.md](PROVISIONING.md) defines named stage prerequisites/effects, frozen package inputs, durable bounded receipts, a controlled transient-service runner, and stage-specific continuation or refusal. Native one-time scripts keep frozen semantics; interrupted arbitrary scripts never receive automatic replay/rollback.

**Required acceptance evidence (planned, not executed):** Interrupt between package installation and LXD pool/network setup. Reconciliation must identify the actual completed stage and permitted continuation without replaying unrelated scripts.

**Authority:** Internal protocol specification; preserve the accepted explicit-update policy.

**Raw findings:** [S9-READY-04](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md).

### ELD-15: Separate architecture from implementation recipes

**Severity:** Nit. **Disposition:** PROPOSE FIX. **State:** Applied to design; independent re-review pending.

**Scope:** Schema/connection/publication details and repeated alternatives before DetailedRange. [Frozen design:543](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:543).

Low-level recipes occupy the architecture narrative while the detailed plan mostly lists files. This makes selected ownership/runtime contracts harder to locate.

**Correction applied:** The main [design](design_draft.md) now retains architecture, component ownership, selected invariants and the file/test plan. [TECHNICAL_DETAILS.md](TECHNICAL_DETAILS.md) contains detailed contracts, focused supplements contain exact protocols, and [DECISIONS.md](DECISIONS.md) centralizes alternatives. Frozen review inputs remain unchanged.

**Required acceptance evidence (planned, not executed):** Check links, heading hierarchy and traceability after moving sections; retain every safety invariant and acceptance requirement.

**Authority:** Internal document organization; address alongside the missing contract appendices.

**Raw findings:** [S5-ZOOM-03](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage5-zoom-out.md), [S8-ZOOM-05](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md).

### ELD-16: Calibrate the Ubuntu support claim

**Severity:** Nit. **Disposition:** PROPOSE FIX. **State:** Applied — not independently re-reviewed.

**Scope:** Overview guest-support wording. [Frozen design:21](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:21).

Calling Ubuntu the tested guest implied validation that has not occurred.

**Correction:** Applied: identify Ubuntu as the selected guest for initial qualification.

**Acceptance evidence:** Document wording now matches the unexecuted runtime qualification status.

**Authority:** Factual documentation correction.

**Raw findings:** [S7-STYLE-03](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md).

### ELD-17: Describe SQLite as a selected trade-off

**Severity:** Nit. **Disposition:** PROPOSE FIX. **State:** Applied — not independently re-reviewed.

**Scope:** Alternatives summary. [Frozen design:1120](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:1120).

The summary implied cross-record consistency intrinsically requires SQLite, although the earlier comparison describes a file/journal alternative.

**Correction:** Applied: SQLite supplies the selected commit mechanism; authoritative JSON would need an application-owned commit/recovery protocol.

**Acceptance evidence:** The rationale is consistent with the retained T4 alternatives.

**Authority:** Factual documentation correction.

**Raw findings:** [S7-STYLE-04](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md).

### ELD-18: Remove repeated drafting-only execution disclaimers

**Severity:** Nit. **Disposition:** PROPOSE FIX. **State:** Applied — not independently re-reviewed.

**Scope:** Repeated section-end no-execution statements. [Frozen design:349](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:349).

Repeated global no-test/no-change statements obscured section-specific qualification requirements without adding an operator decision.

**Correction:** Applied: remove the repeated global statements while retaining the document-wide evidence boundary and concrete local validation requirements.

**Acceptance evidence:** No operational invariant or qualification gate was removed.

**Authority:** Internal editorial correction.

**Raw findings:** [S7-STYLE-05](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md), [S8-ZOOM-05](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md).

### ELD-19: Keep logging claims bounded to controlled fields and known secrets

**Severity:** Nit. **Disposition:** PROPOSE FIX. **State:** Applied — not independently re-reviewed.

**Scope:** Process-runner manifest and logging traceability row. [Frozen design:1165](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:1165).

Secret-safe errors and secret exclusion could imply universal sanitization beyond the accepted policy.

**Correction:** Applied: specify allowlisted classified errors excluding known credential values/raw subprocess details, and known-secret exclusion. Raw capture remains opt-in.

**Acceptance evidence:** The summary wording matches T12 and the approved logging policy.

**Authority:** Alignment with accepted GF-10 logging policy.

**Raw findings:** [S9-STYLE-02](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-style.md).

## Reviewer completion and limits

| Stage | Reviewer | Result | Original report |
| --- | --- | --- | --- |
| 5 | egm-zoom-out | Completed; 0 Blocker, 2 Major, 1 Nit | [Full report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage5-zoom-out.md) |
| 6 | egm-comprehension | Completed; 0 Blocker, 0 Major, 0 Nit | [Full report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage6-comprehension.md) |
| 7 | egm-design | Completed; 0 Blocker, 4 Major, 0 Nit | [Full report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-design.md) |
| 7 | egm-structural | Completed; 0 Blocker, 0 Major, 0 Nit | [Full report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-structural.md) |
| 7 | egm-style | Completed; 0 Blocker, 2 Major, 3 Nit | [Full report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md) |
| 8 | egm-zoom-out | Completed; 0 Blocker, 4 Major, 1 Nit; all carried, no new independent findings | [Full report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md) |
| 9 | egm-implementation-readiness | Completed; 0 Blocker, 5 Major, 0 Nit | [Full report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md) |
| 9 | egm-codebase-details | Completed; 0 Blocker, 4 Major, 0 Nit | [Full report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-codebase.md) |
| 9 | egm-style | Completed; 0 Blocker, 1 Major, 1 Nit | [Full report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-style.md) |

Every run used a new agent with `fork_turns=none` and acknowledged reading its required persona, standards, scope and context. Stage 6 read the target alone. Stage 8 received the earlier stage reports as required by the workflow. No reviewer was failed, timed out, retried or counted as a zero-finding success without a report.

Fresh conversation context does not prove isolation from filesystem or host instruction files; no stronger isolation claim is made. Code evidence was restricted to the permitted prototype source/tests. Selected Lima/OpenSSH/Docker/LXD primary sources were checked where needed; the entire linked bibliography was not independently verified. Runtime qualification remains future implementation work.

## Traceability matrix

Every raw finding maps to an action below. S8-ZOOM-03 covers two distinct effective-configuration corrections; S8-ZOOM-05 covers organization and repeated disclaimers. No raw severity was downgraded, and carried findings are not counted as additional independent issues.

| Raw finding | Severity | Consolidated action(s) | Original evidence |
| --- | --- | --- | --- |
| S5-ZOOM-01 | Major | ELD-01 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage5-zoom-out.md) |
| S5-ZOOM-02 | Major | ELD-02 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage5-zoom-out.md) |
| S5-ZOOM-03 | Nit | ELD-15 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage5-zoom-out.md) |
| S7-DESIGN-01 | Major | ELD-03 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-design.md) |
| S7-DESIGN-02 | Major | ELD-04 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-design.md) |
| S7-DESIGN-03 | Major | ELD-02 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-design.md) |
| S7-DESIGN-04 | Major | ELD-05 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-design.md) |
| S7-STYLE-01 | Major | ELD-06 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md) |
| S7-STYLE-02 | Major | ELD-07 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md) |
| S7-STYLE-03 | Nit | ELD-16 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md) |
| S7-STYLE-04 | Nit | ELD-17 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md) |
| S7-STYLE-05 | Nit | ELD-18 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage7-style.md) |
| S8-ZOOM-01 | Major | ELD-01 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md) |
| S8-ZOOM-02 | Major | ELD-02 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md) |
| S8-ZOOM-03 | Major | ELD-03, ELD-05 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md) |
| S8-ZOOM-04 | Major | ELD-04 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md) |
| S8-ZOOM-05 | Nit | ELD-15, ELD-18 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage8-zoom-out.md) |
| S9-CODE-01 | Major | ELD-08 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-codebase.md) |
| S9-CODE-02 | Major | ELD-02 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-codebase.md) |
| S9-CODE-03 | Major | ELD-09 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-codebase.md) |
| S9-CODE-04 | Major | ELD-10 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-codebase.md) |
| S9-READY-01 | Major | ELD-11 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md) |
| S9-READY-02 | Major | ELD-12 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md) |
| S9-READY-03 | Major | ELD-13 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md) |
| S9-READY-04 | Major | ELD-14 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md) |
| S9-READY-05 | Major | ELD-01 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-readiness.md) |
| S9-STYLE-01 | Major | ELD-06 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-style.md) |
| S9-STYLE-02 | Nit | ELD-19 | [Report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/raw/stage9-style.md) |

## Follow-through

Review [INTERFACE_V1.md](INTERFACE_V1.md) for ELD-11/ELD-12, then run a targeted independent re-review before production implementation. Seventeen findings have applied corrections; runtime qualification remains pending.

Evidence: [frozen-input manifest](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/manifest.json), [dispatch/completion record](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/dispatch.json), [machine-readable triage](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/triage.json), [applied corrections](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/corrections.json), [verification record](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/verification.json). The earlier concept [Goldfish report](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/goldfish_review_report.md) and its review records remain unchanged.

Previous resolution: [ELD-01 approval and applied protocol](.egm/reviews/2026-11-19-design-r1/resolutions/ELD-01/decision.md); [resolution verification](.egm/reviews/2026-11-19-design-r1/resolutions/ELD-01/verification.json).

Latest resolutions: [ELD-08 acceptance](.egm/reviews/2026-11-19-design-r1/resolutions/ELD-08/decision.md), [delegated ELD-09/10/13/14/15 corrections](.egm/reviews/2026-11-19-design-r1/resolutions/ELD-09-15/decision.md), and [document verification](.egm/reviews/2026-11-19-design-r1/resolutions/ELD-08/verification.json). [ELD-11/ELD-12](INTERFACE_V1.md) remain proposed for author review.

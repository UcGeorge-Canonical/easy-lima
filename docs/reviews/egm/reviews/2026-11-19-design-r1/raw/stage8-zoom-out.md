Stage 8 — Intermediate Zoom Out, Mode REVIEW

**Outcome: no new findings.** This pass carries forward four Major architectural items and one Nit, consolidating earlier findings as identified below. These are not additional independent risks for the final deduplicated count. No draft fixes have been applied; earlier findings remain unresolved.

The objective and motivation are sufficient. The proposed Lima wrapper remains aligned with the approved product boundary. Durable operation evidence, selective prototype reuse, and separate diagnostic results address real requirements. The strongest simplification opportunity is to make shared-resource ownership and effective runtime configuration explicit, before adding further recovery mechanisms.

All target references below concern the frozen [design_draft.md](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md).

**S8-ZOOM-01 — Major — Make the shared agent determine credential-change scope**

**Carried from:** S5-ZOOM-01.
**Affected scope:** Lines 548–549, 783, 812–814, 866–872, and 1003; approved `UX_PROPOSAL.md`, line 71.

The profile architecture combines one shared, exactly-one-key agent with independently applied VM identity revisions. These are incompatible when a credential change targets only some consumers. Changing the shared agent from K1 to K2 changes access for every connected VM, regardless of which guest configuration revisions were committed.

The high-level problem is the mismatch between resource ownership and mutation scope. Additional locks, revision rows, or recovery states cannot make one socket simultaneously preserve incompatible credential revisions.

**Actionable direction:** Separate profile-wide credential/runtime properties from per-VM guest settings. A credential, socket, or supervisor change must enumerate all dependent registrations and either obtain the necessary complete scope or refuse before effects. A subset requiring a different credential can use a separate profile and explicit reassignment. Record each VM’s applied guest identity revision and pending transition clearly. Avoid introducing versioned agents or implicit key switching merely to preserve the conflicting scope model.

Qualification should include two VMs sharing a profile while a credential change selects only one. The plan must include the necessary dependency scope or refuse without changing the other VM’s identity.

**S8-ZOOM-02 — Major — External profile resources need an explicit capability boundary**

**Carried from:** S5-ZOOM-02 and S7-DESIGN-03.
**Affected scope:** Lines 818, 826–840, 850–872, 906–918, and 1037; approved `UX_PROPOSAL.md`, line 46.

Adoption preserves a legacy agent shared with non-adopted projects and prohibits taking control of it. The general Lock contract nevertheless promises profile-wide key unloading and persistent reload inhibition. Those guarantees depend on controlling the agent and its supervisor.

The permitted prototype source confirms the distinction: [git_identity.py:124](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git_identity.py:124) installs a persistent supervisor, and [git-agent.py:43](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-agent.py:43) loads the key when that supervisor starts. An easy-lima inhibition record cannot control that external restart behavior. Unloading the shared key also affects consumers outside the adopted scope.

**Simplicity rationale:** Owned and external resources should not appear interchangeable to activation, lock, apply, forget, and removal workflows. Otherwise each workflow accumulates exceptions while still presenting an inaccurate common guarantee.

**Actionable direction:** Define supported administrative capabilities for external profiles before ownership retirement. The smallest safe contract refuses unsupported administration until an explicit ownership migration. If Lock must remain available, specify its narrower access-revocation mechanism and account for existing forwarding paths; blocking new connections alone is insufficient. Any reduced locking behavior must be surfaced against the approved UX.

This is a boundary clarification, not justification for a generic resource-management framework.

**S8-ZOOM-03 — Major — Enforce one effective Lima configuration contract across lifecycle operations**

**Carried synthesis of:** S7-DESIGN-01 and S7-DESIGN-04.
**Affected scope:** Lines 267–269, 373–375, 513–521, 904, 957–965, 987–1005, and 1125.

Freezing instance YAML does not freeze all configuration Lima consumes. The tagged template confirms that namespace defaults and overrides participate in subsequent starts, including combined lists. Separately, Lima’s native automatic forwarding requires an explicit suppression rule; an empty requested-port list does not establish the approved exposure boundary. [Lima v2.2.1 global configuration semantics](https://github.com/lima-vm/lima/blob/v2.2.1/templates/default.yaml#L625), [Lima port-forwarding controls](https://lima-vm.io/docs/config/port/#disabling-port-forwarding).

These findings share an architectural cause: the preview and frozen-input model currently describe intended instance settings without a complete boundary around effective provider behavior. Fixing each symptom independently in creation, startup, adoption, and diagnostics would duplicate policy and leave inconsistent paths.

**Actionable direction:** Put the supported effective-configuration contract in the existing version-specific Lima adapter. It must cover namespace configuration, retained inputs, approved mounts/provisioning, and suppression of unrequested application-port forwarding. Use the same contract for preview, admission, and drift checks.

For unsupported global configuration, a precise refusal is simpler than implementing a general replacement for Lima’s configuration engine. Do not rewrite shared namespace files to make an individual VM acceptable. Adoption must perform this check before retiring old ownership.

There is direct reuse evidence for the port safeguard: [local.py:200](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:200) already implements the catch-all ignore rule, and [test_local.py:64](/Volumes/Nebula/dev/Canonical/local/.local-launcher/tests/test_local.py:64) protects its configuration. Port that behavior into the supported effective-config invariant.

**S8-ZOOM-04 — Major — Give all managed connections one VM trust contract**

**Carried from:** S7-DESIGN-02.
**Affected scope:** Lines 155–157, 904–914, and 951–953.

The tunnel, alias proxy, ordinary SSH client, and diagnostic transport all rely on the VM endpoint being the recorded VM. The design promises host-key verification without defining enrollment and retained trust. Lima’s common SSH options explicitly disable strict host-key checking and use `/dev/null` for user known hosts, so retaining those options does not establish the promised trust. [Lima v2.2.1 SSH options](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/sshutil/sshutil.go#L427).

**Simplicity rationale:** Separate connection paths should not invent separate enrollment or acceptance rules. That would multiply policy and recovery states around the same registration.

**Actionable direction:** Define one registration-to-VM-host-key binding, its initial trust source, and its behavior through adoption, endpoint changes, rebind, and unexpected replacement. Reuse OpenSSH’s verification mechanisms with effective settings that implement that binding. State any trust-on-first-use policy explicitly. Establish trust before exposing the profile socket, and have diagnostics consume existing trust without enrolling or repairing it.

The prototype’s provider known-host configuration is useful for GitHub handling; it does not supply VM host-key enrollment. A changed-key or wrong-endpoint case must fail before profile forwarding.

**S8-ZOOM-05 — Nit — Separate architectural decisions from implementation recipes**

**Carried from:** S5-ZOOM-03; reinforced by S7-STYLE-05.
**Affected scope:** Lines 543–557, 607–619, 644–650, 706–708, 1061–1063, and repeated alternative comparisons; DetailedRange begins at line 1130.

The architecture contains extensive schema columns, connection settings, publication recipes, and database-swap mechanics. The detailed plan then concentrates on filenames and validation work. This distribution makes the selected ownership and runtime contracts harder to locate, despite the document being comprehensible.

**Actionable direction:** Keep responsibilities, invariants, authority boundaries, lifecycle flow, and essential failure behavior in the architectural narrative. Move detailed schemas and publication/restore procedures into the detailed plan or linked decision records. Consolidate repeated alternatives and document-wide execution disclaimers without removing concrete qualification requirements.

The other supplied reports do not justify removing approved v1 scope or replacing selected Go, SQLite/WAL, or foreground execution choices. The comprehension and structural passes found no issues within their lenses. S7-STYLE-01 still needs the approval/qualification timing corrected to match the dependency-first implementation order at lines 1136–1142. S7-STYLE-02 still needs the approved GitHub transport mapping stated normatively; the prototype already demonstrates that behavior at [git-guest.py:68](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-guest.py:68). These are carried observations, not new Stage 8 findings.

**Reuse assessment**

The required searches support selective behavior reuse:

| Source evidence | Reuse conclusion |
| --- | --- |
| `tui.py:11–67` | Lazy tree enumeration, expansion state, and cycle avoidance remain suitable behavior-port candidates. |
| `local.py:52–74, 220–233` | Atomic replacement, path handling, and SSH include preservation supply useful patterns and scenarios. |
| `local.py:285–423, 459–544` | Ownership refusal, lifecycle sequencing, retained association, and temporary forwarding inform adapters and adoption fixtures. |
| `git_identity.py:21–151`; `git-agent.py:12–56` | Exact-key/public-payload behavior is reusable; legacy supervisor semantics must remain distinguished from owned-agent policy. |
| `git-guest.py:29–71`; `git-check.py:15–56` | Git configuration, provider trust, HTTPS mapping, and signing scenarios are useful. Mutating checks do not become read-only diagnostics. |
| `provision.sh:14–98`; `verify.sh:5–20` | Package and LXD workload knowledge can be adapted into explicitly invoked stages and acceptance checks. |
| Both Python test files | Ownership, missing-storage, SSH preservation, key isolation, source preservation, and interruption scenarios remain relevant. |

No additional wheel-reinvention finding is supported. The inspected prototype does not provide the proposed transactional claims, recovery inventory, or complete durable reconciliation machinery. Its useful pieces do not eliminate the accepted need for new portable state and recovery behavior.

**Read acknowledgment and limitations**

I read completely:

- [Zoom Out persona](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/zoom_out.md).
- [Zoom-Out Standard](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/zoom-out-standard.md).
- Frozen `inputs/design_draft.md`, all **1–1239**, including FeatureRange **1–1129** and DetailedRange **1130–1239**.
- Every supplied frozen sibling: `FEATURES.md` **1–208**, `concepts_brief.md` **1–83**, `UX_PROPOSAL.md` **1–103**, and `goldfish_review_report.md` **1–305**.
- Every supplied stage report: `stages/zoom_out_report.md` **1–80**, `comprehension_report.md` **1–19**, and `critic_report.md` **1–163**.

The historical Goldfish report was treated as closed concept-review context. The supplied new stage reports were treated as unresolved findings against this unchanged snapshot.

I performed `rg` searches across all eight permitted source/scripts and both Python test files for component definitions, ownership, locking, SSH/agents, provisioning, profiles, transactions, inventory, reconciliation, receipts, assets, checkpointing, backups, and capacity. I then read the implementations and selected test bodies identified above. Searches excluded state and caches. I did not inspect other project source.

Primary upstream checks were limited to relevant sections of Lima v2.2.1’s `templates/default.yaml` and `pkg/sshutil/sshutil.go`, plus the official port-forwarding documentation. These corroborate specific carried findings; they do not qualify the assembled application or its host matrix.

No required evidence was unavailable. No files were written, source/scripts or tests executed, builds or VM commands run, live resources/state inspected, keys or credential stores accessed, host SSH configuration read, conversation/history/memory services consulted, or subagents spawned. All conclusions are static architectural assessments.

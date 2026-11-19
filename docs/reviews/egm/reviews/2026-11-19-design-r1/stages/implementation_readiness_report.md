# Stage 9 implementation readiness review

All three reviewers completed. Raw total: 0 Blockers, 10 Major findings, 1 Nit. The plan does not yet pass implementation readiness. These reports concern the frozen target, with no fixes applied.


## Implementation readiness

Stage 9 Implementation Readiness — REVIEW

Result: Does not yet pass the zero-design-choice standard. Five Major findings remain. No Blockers or Nits.

Frozen target: `/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md`

Active scope: lines 1130–1239, beginning `## Detailed Implementation Plan`. References below distinguish that worklist from supporting specifications elsewhere in the same frozen target.

The manifest supplies exact proposed files, the work is logically ordered, and substantial architecture, recovery policy, dependency selection, and verification scenarios are already specified. Pending runtime qualification is correctly identified as future implementation work; its unexecuted status is not a finding.

**S9-READY-01 — Major — Configuration and command inputs still require interface design**

Affected active scope: lines 1154–1155, 1176, and 1194: `internal/model/model.go`, configuration schemas, CLI grammar, and examples. Supporting evidence: T5 lines 447–465 and 493–527; T12 lines 1043–1047; T13 lines 1073–1081.

T5 fixes discovery, path anchoring, ownership, and precedence. T12 selects parsers and lists accepted tables, but does not finish the field schemas within those tables. The implementer must still choose names, types, required fields, defaults, and validation rules for structures such as `editor`, `ui`, `mounts`, `ports`, and profile `git`, `github`, `key`, and `unlock`. The illustrative project example covers only a subset.

The approved command table likewise requires noninteractive profile fields and secret input through an inherited file descriptor without specifying their flags or argument grammar. Explicit creation choices and environment-update inputs also lack a complete CLI mapping. These are durable user interfaces, not private helper implementation details.

Actionable direction: Add a normative schema and command-input appendix: exact TOML fields and list element shapes; required/default/immutable fields; quantity and value validation; and the complete CLI flag/type/requiredness mapping. Include valid and rejected examples for creation, profile import/create, additional mounts/ports, and an explicit environment update. Map those examples to the manifest’s configuration and CLI tests.

**S9-READY-02 — Major — Script-facing result contracts stop at the JSON envelope**

Affected active scope: lines 1154, 1174–1176, 1186, and 1207: model/error codes, diagnostics, renderers, and exact script-output verification. Supporting evidence: lines 1069 and 1081; approved UX metadata exit-code rules.

The envelope and numeric exit codes are selected. However, `data` has no command-specific schema, `outcome` has no defined value set, and the promised stable diagnostic/error codes are not enumerated. The document also does not settle result aggregation when a command contains both established failures and incomplete checks.

For example, two implementations could both obey the described `list --json` envelope while returning incompatible registration fields, nesting, optional values, and recorded-versus-observed state representations. Similarly, `doctor` with a failed ownership check and unavailable provider check has no stated exit-precedence rule. The implementer would be designing the scripting API while writing its tests.

Actionable direction: Define version-1 payloads and stable codes for each metadata command family, including partial results and malformed records. Specify aggregation and exit precedence, and provide representative success, failure, unavailable, and mixed-result JSON examples as the expected test outputs.

**S9-READY-03 — Major — Authoritative state and ownership-publication contracts remain incomplete**

Affected active scope: lines 1157–1161, 1164, and 1183–1184: initial migration, guards, maintenance, reconciliation, and recovery tests. Supporting evidence: T6 lines 541–565; T11 lines 999 and 1024–1035; T12 lines 1041–1043 and 1053–1063.

T6 provides useful principal columns for the initial model. T12 then adds `profile_runtime`, `connection_bindings`, `capacity_claims`, and `namespace_events` primarily by responsibility, without their complete columns, constraints, state values, or relationships. The external namespace/instance ownership markers and mirrored epochs also lack exact locations, versioned formats, and comparison/publication rules.

Those omissions affect authoritative decisions. The restore implementation must distinguish an acceptable backup from one that loses a later ownership transition, while migration must distinguish a committed schema change awaiting epoch publication from contradictory ownership evidence. “Compare evidence,” “publish a monotonic epoch,” and “reject lost unresolved effects” do not specify the representations and predicates needed to implement those cases consistently.

Actionable direction: Complete the version-1 state specification, including the four added tables and external marker formats. Define transaction-versus-filesystem publication order and recovery decisions for every intermediate marker/epoch state. Supply concrete fixtures covering committed-but-unpublished epochs, stale backups, interrupted handoffs, and contradictory markers. This asks for the selected protocol, not completed crash testing.

**S9-READY-04 — Major — Provisioning stages lack the concrete evidence contracts required by the coordinator**

Affected active scope: lines 1169, 1178–1180, 1187–1189, and 1205–1206: environment orchestration, preset scripts, stage runner, and recovery/workload tests. Supporting evidence: T7 lines 714–774; T10 lines 981–991.

The design chooses explicit stages, frozen scripts, named transient jobs, receipts, and conservative reconciliation. It does not define the actual stage catalogue, receipt schema, or stage-specific completion and replay predicates. The package/preset description is insufficient to derive these: it leaves such decisions as stage boundaries, exact owned LXD configuration, Docker installation policy, and the postconditions that distinguish a completed stage from partially applied package/configuration work.

For example, after installation succeeds but LXD pool/network configuration is interrupted, the implementer must decide which stage completed, what evidence survives, and which continuation is permitted. The generic instruction that each adapter “needs an evidence contract” delegates that design work to implementation.

Actionable direction: Specify each shipped create/update stage’s immutable ID, inputs, prerequisites, ordered effects, owned configuration, receipt fields, and satisfied/active/not-applied/partial/unknown predicates. Define the supported mapping of native-template provisioning into those stages. Add expected reconciliation outcomes to the planned scenario fixtures. Publisher image values and measured capacity bounds can remain scheduled release inputs; the stage protocol itself must be selected.

**S9-READY-05 — Major — Applying profile revisions across several VMs lacks a complete transition policy**

Affected active scope: lines 1139, 1164, 1170–1173, and 1185: profile operations, runtime policy, connection handling, and profile-isolation tests. Supporting evidence: lines 548–550, 783, 866–876, 1003, and 1041–1045; approved UX `profile edit`/`profile apply`.

The design selects one agent and one offered key per profile, immutable identity revisions, and application to an explicitly listed set of VMs. It does not finish the relationship between a profile’s global active revision and each VM’s applied identity revision.

If two VMs use a profile and an edit is applied to only one, the plan must say exactly which revision the other VM uses on its next start/connect. For changes to a key or supervisor reference, it must also decide whether the field is immutable in v1, whether all consumers must join the transition, or how the shared agent remains compatible. No such field-specific policy is stated. The one-row “Profile change” workflow does not settle multi-VM partial completion.

Actionable direction: Define mutable profile fields, per-VM applied-revision references, and which changes require a profile-wide operation. Explicitly prohibit unsupported shared-key/supervisor changes or enumerate every affected consumer before allowing them. Add ordered effects and recovery outcomes for interruption after only some VM bundles have changed, while preserving the unaffected VMs’ recorded identity.

Read acknowledgment: I read the complete persona `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/implementation_readiness.md` and complete standard `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/implementation-readiness-standard.md`. I read the full frozen target, lines 1–1239: the active range 1130–1239, opening sections and journeys, responsibilities/reuse/upstream evidence, T1–T14, and Alternatives Considered. I also read all four supplied frozen siblings: `FEATURES.md`, `concepts_brief.md`, `UX_PROPOSAL.md`, and `goldfish_review_report.md`; the latter was treated only as historical concept-review context.

These documents are the complete evidence set used. I inspected no prototype source, other new reviewer reports, conversation/history/memory, live state, credentials, or unrelated projects. I made no writes and ran no builds, tests, VM operations, or runtime qualification. Proposed creation paths were not treated as missing-reference defects.


## Codebase details

Stage 9 Codebase Details Review found **4 Major findings, 0 Blockers, and 0 Nits**. These are implementation-design gaps; none treats a proposed file as missing current code or a qualification target as certified support.

References below use:

- **D:** `/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md`
- **P:** `/Volumes/Nebula/dev/Canonical/local/.local-launcher`

**S9-CODE-01 — Major: Adoption needs a legacy-work quiescence check before transferring ownership.**

Affected scope: D:1163, D:1184, D:1206 and D:1228; governing adoption contract at D:1022–1037. Source: `P/local.py:37–49` (`run`), `P/local.py:285–297` (`Manager.lock`), `P/local.py:378–389` (`Manager.provision`), and `P/provision.sh:97–99`.

The proposed handoff acquires `launcher.lock`, retires the old ownership record, and permits unknown provisioning history. The source supports the narrower claim that an existing instance without its record is rejected by `Manager.check_ownership`; that claim is accurate. It does not establish that previously dispatched guest work has stopped. Provisioning runs through a guest shell, the lock releases when the host workflow exits, and the prototype retains only a final provisioning digest—not a durable job identity, dispatch receipt, or cancellation result.

Consequently, “old record retired” and “provisioning history unknown” cannot establish that a surviving legacy provision command will not overlap subsequent easy-lima work. The adoption recovery table currently tracks ownership publication but supplies no admission predicate or retained claim for this case.

Actionable direction: Specify how adoption establishes that legacy work is quiescent, distinguishing historical unknown provenance from potentially active work. Define stopped-VM evidence or a positive running-work check, the refusal/resume behavior when evidence is unavailable, and the persisted evidence required before releasing handoff claims. Add an adoption scenario where the legacy controller exits after guest dispatch but its work remains active; retirement alone must not admit a conflicting mutation.

**S9-CODE-02 — Major: Preserved shared legacy agents cannot implement the stated profile-lock behavior.**

Affected scope: D:1164, D:1170–1172, D:1185, D:1204 and D:1228. Relevant contracts: D:828, D:872 and D:1037. Source: `P/git_identity.py:115–127` (`setup` and its launchd job), and `P/git-agent.py:20–21,30–48` (`main`).

The design requires a lock to unload the profile key and inhibit automatic reload across agent restarts. It also imports an agent shared with non-adopted projects as an external resource whose existing supervisor/socket must remain usable and must not be stopped, reconfigured, or exclusively controlled.

The actual legacy job has `RunAtLoad` and `KeepAlive` enabled. Its helper starts an agent and unconditionally loads the key through its existing askpass mechanism on each start. It does not consult easy-lima’s `profile_runtime` inhibition. Unloading its shared key disrupts remaining prototype users; preserving its restart behavior can reload a supposedly locked key.

Actionable direction: Define explicit capabilities and command behavior for an externally managed profile. Specify whether locking is unavailable until a separately approved ownership migration, or how an independent managed binding will provide the lock guarantee while preserving legacy users. Any user-visible exception requires reconciliation with the approved lock experience. Test partial adoption, lock attempts, supervisor restart, and continued operation of a non-adopted project.

**S9-CODE-03 — Major: The development/Docker split omits an existing networking dependency and its acceptance coverage.**

Affected scope: D:1179, D:1189 and D:1205; preset specification at D:983. Source: `P/provision.sh:21–23,43–79` and `P/verify.sh:8–19`.

The prototype does more than install Docker and LXD: it installs a service ordered after Docker that changes forwarding policy, then configures LXD networking and both groups. Its verification script checks Docker while exercising a real LXD container. The new worklist separates `development.sh` and `docker.sh`, but does not select their composition/order, the replacement networking policy, or handling after Docker restarts. The explicit preset release gate lists Snap/LXD/Juju without Docker or the combined selection.

This is a concrete reuse dependency: Docker’s forwarding policy can prevent LXD traffic, and the appropriate remedy depends on the selected Docker version/backend. Official [Docker networking documentation](https://docs.docker.com/engine/network/packet-filtering-firewalls/#docker-on-a-router) and [LXD firewall guidance](https://canonical.com/lxd/docs/latest/howto/network_bridge_firewalld/) describe the interaction; neither establishes compatibility of this proposed application.

Actionable direction: Specify Docker’s installation source/version policy, firewall backend, preset ordering, owned networking changes, and restart postconditions. Explicitly retain or replace the prototype workaround rather than copying its global policy change without review. Add Docker-only and development-plus-Docker acceptance cases, including a Docker restart followed by LXD networking and Juju workload checks.

**S9-CODE-04 — Major: The legacy ownership contract test lacks a specified executable fixture and harness.**

Affected scope: D:1184, D:1187–1189 and D:1206; the required proof appears at D:1024. Source: `P/tests/test_local.py:16–25,108–122,138–149`; `P/tests/test_work_git.py:18–27,87–114`.

The plan requires a fixture to prove that retirement makes the old helper’s start/stop/delete paths refuse the adopted instance. The manifest provides only `testdata/prototype/manifest.json` for prototype evidence plus generic fakes/faults. It does not specify how the actual legacy behavior will be executed, which source revision identifies the supported importer, or how legacy subprocess dependencies will be isolated.

The existing infrastructure creates temporary roots and patches `instances`/`run`; its ownership test directly invokes the Python manager. It is useful evidence, but it is not a portable executable contract fixture in the new repository. D:134 also correctly says prototype source references are not runtime dependencies. A Go fake that merely reproduces the intended refusal would not independently prove the legacy handoff assumption.

Actionable direction: Define the legacy contract harness before implementation: exact provenance/version identification, fixture files or an explicitly supplied fixture source, test-only interpreter requirements if applicable, fake Lima/process protocol, isolated configuration/home/state paths, and permitted effects. Add exact manifest paths for any fixture/helper code. Require execution of the supported legacy refusal paths against retired metadata, with assertions that no VM mutation was dispatched, rather than only comparing an expected JSON result.

Read acknowledgment: I read the complete `codebase_details.md` persona, complete implementation-readiness standard, all 1,239 target lines, and all frozen `FEATURES.md`, `concepts_brief.md`, `UX_PROPOSAL.md`, and `goldfish_review_report.md` lines. The latter was treated solely as historical concept-review evidence.

Actual checks: Read all eight permitted prototype source/scripts and both test files, focusing on their setup, mocks, subprocess boundaries, ownership checks, provisioning evidence, agent supervision, and guest identity configuration. Checked primary OpenSSH/Git documentation for forwarding/signing semantics and primary Docker/LXD documentation for the networking interaction.

Limitations: Static review only. No files were changed; no builds, tests, scripts, VM operations, or credential interactions were executed. No live registry/VM state, keys, credential stores, host SSH configuration, unrelated repositories, conversation/history/memory, or other new reviewer reports were inspected. No subagents were used.


## Style review

2 findings: 1 Major, 1 Nit. No Blockers.

**S9-STYLE-01 — Major — Separate the design-review prerequisite from release review**

Affected scope: `design_draft.md:1142`, with the prerequisite stated at `1237`.

“Build in this order” places “fresh implementation-design/implementation review” in the final step, after implementation and alongside publication. The slash combines two distinct reviews without identifying their sequence. Line 1237 separately requires resolving design findings before implementation.

A reader following the worklist could defer the design gate until release preparation or infer an additional, unexplained design-review cycle.

State completion of the current design review and resolution of its findings as a prerequisite to step 1. Make step 7 explicitly require implementation review and passing release gates before publication. **RELOCATE** the drafting-only limitation in line 1142 to “Completion and review boundary,” preserving the restriction on publishing, registering keys, and changing live resources.

**S9-STYLE-02 — Nit — Preserve the bounded logging guarantee in the worklist**

Affected scope: `design_draft.md:1165` and `1227`.

“Secret-safe errors” and “secret exclusion” omit the qualification that the accepted guarantee concerns known secrets and controlled operational fields. F8/GF-10 explicitly avoids a universal sanitization guarantee; T12 at line 1067 gives the concrete mechanism: allowlisted fields and errors that do not stringify complete arguments, environments, or raw stderr.

An implementer using these summary rows as acceptance criteria could interpret them as requiring or promising universal sanitization.

**CALIBRATE** the summaries to the specified mechanism, for example “allowlisted classified errors excluding known credential values and raw subprocess details” and “known-secret exclusion.” Retain the separate opt-in raw-capture policy.

Read acknowledgment: I read the complete Style Goldfish persona at `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/style.md`, the entire active range 1130–1239, and all supplied frozen context: `FEATURES.md`, `concepts_brief.md`, `UX_PROPOSAL.md`, and `goldfish_review_report.md`. The historical report was used only as closed concept context. Earlier target passages consulted were lines 1–173, 245–256, 300–350, 941–954, and 1039–1129, plus heading and targeted-term searches. No additional standard was supplied.

Limitations: This was a static wording review. I made no writes, ran no builds/tests/VM commands, inspected no live state or credentials, accessed no conversation/history/memory or unrelated repositories, read no other new reviewer/stage reports, and spawned no subagents. Runtime behavior and external dependency claims were not independently verified.

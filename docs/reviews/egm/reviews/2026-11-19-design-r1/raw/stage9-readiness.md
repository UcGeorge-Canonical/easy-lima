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

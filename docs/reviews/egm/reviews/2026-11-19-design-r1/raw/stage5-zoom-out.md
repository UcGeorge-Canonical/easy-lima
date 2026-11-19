Stage 5 — Initial Zoom Out, Mode REVIEW

Outcome: **2 Major findings and 1 Nit; no Blockers.** The objective and motivation are clear. The design largely stays aligned with the approved Lima-wrapper boundary, and its recovery machinery generally follows accepted requirements. The material issues below concern incompatible ownership scopes within the identity architecture, rather than complexity by itself.

All target line references refer to the frozen [design_draft.md](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md).

**S5-ZOOM-01 — A shared profile agent cannot support independently applied credential revisions — Major**

**Affected scope:** T6 profile revision model, lines 548–549; T7 profile claims, line 783; T8 shared agent and frozen identity, lines 812–814 and 866–868; T11 profile-change workflow, line 1003. Supporting approved UX: `UX_PROPOSAL.md`, line 71.

The design gives each profile one agent/socket containing exactly its selected key, shared by every VM assigned to that profile. It also allows editing a profile and applying the changed profile to an explicitly selected set of VMs, while other VMs retain their applied identity. The configuration includes credential references and fingerprints, and the claim model explicitly permits credential changes.

These contracts conflict when an agent-level change applies to only some bindings. Suppose VMs A and B share profile P and key K1. Applying P’s new key K2 to A changes the same agent endpoint B already uses. B therefore loses its old authentication/signing capability or encounters the new identity despite being outside the selected application scope. Keeping K1 instead prevents A from completing the requested change. Serializing the transition with claims does not resolve the incompatible steady state after those claims are released.

**Simplicity rationale:** The ownership boundary of a shared agent must determine the scope of mutations to that agent. Trying to preserve independent credential revisions behind one exactly-one-key socket invites implicit switching or additional versioned agents, neither of which is part of the selected architecture.

**Actionable direction:** Distinguish profile-wide runtime/credential properties from per-VM applied guest settings. For key, socket, or supervisor changes, enumerate all dependent registrations and refuse a partial application that leaves incompatible consumers attached. A change intended for only some VMs can use a separate profile and explicit reassignment. Define where each VM’s applied identity revision is recorded and how stopped or partially updated consumers are represented.

A focused acceptance scenario should apply a credential change to A while B shares the profile: the plan must either include the required complete dependency scope or refuse without changing B’s usable identity.

**S5-ZOOM-02 — External legacy agents cannot inherit the managed profile-lock contract — Major**

**Affected scope:** T8 explicit-lock behavior, lines 828, 857 and 872; T9 persistent forwarding, lines 906–918; T11 externally managed adopted identity resources, line 1037. Supporting approved UX: `UX_PROPOSAL.md`, line 46.

Adoption correctly preserves a shared legacy agent as externally managed so that remaining prototype projects are not disrupted. The design prohibits stopping, reconfiguring, forgetting, or taking exclusive control of that resource. However, the general profile contract says Lock unloads the key, applies across the profile’s VMs, and continues to inhibit reload after agent restarts.

The inspected prototype does not provide that lock contract:

- [git_identity.py:124](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git_identity.py:124) configures `RunAtLoad` and `KeepAlive`.
- [git-agent.py:43](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-agent.py:43) loads the key whenever its supervisor starts the agent.

Unloading the shared external agent affects non-adopted consumers. Merely recording an easy-lima lock does not stop the external supervisor from loading the key again, and an existing forwarded endpoint can remain usable. Preserving the legacy supervisor is therefore materially different from supervising an owned agent that starts empty.

**Simplicity rationale:** Ownership and supported administrative capabilities need one explicit boundary. Treating owned and externally managed agents as interchangeable pushes conflicting policy into activation, locking, forwarding and recovery.

**Actionable direction:** Define the capability contract for externally managed profiles. The smallest safe option is to refuse unsupported agent administration with a specific explanation until an explicit migration establishes ownership of all affected consumers. If the required experience instead promises a usable Lock for these profiles, specify a mechanism that revokes access through all easy-lima-managed connections without mutating the external agent, and explain its scope to the user. Do not report the existing global lock guarantee as satisfied by an inhibition flag alone.

Any reduced user-visible locking capability needs to be surfaced against the approved UX. Qualification should cover adoption while another prototype project still shares the agent, followed by Lock and a legacy-agent restart.

**S5-ZOOM-03 — Implementation mechanics obscure the high-level architecture — Nit**

**Affected scope:** FeatureRange, especially the column-level schema at lines 543–557; checkpoint/connection settings at lines 644–650 and 706–708; restore file-swap procedure at lines 1061–1063. The designated Detailed Implementation Plan begins at line 1130.

The high-level section contains substantial column, driver-hook, pool-size and file-publication detail, while the detailed section primarily contains the file manifest and validation worklist. Several unselected-option comparisons also remain in the main flow and are summarized again under Alternatives Considered.

These details can be useful, particularly where they establish durability constraints, but their placement makes it harder to identify component ownership and the selected end-to-end behavior. This is an organizational issue, not evidence that SQLite, durable claims or recovery should be removed.

**Actionable direction:** Keep authority boundaries, invariants, component contracts and the operation lifecycle in the high-level section. Move concrete schema columns, connection settings and filesystem publication recipes into the detailed plan or linked decision records. Keep one concise disposition of alternatives in the architectural narrative.

**Reuse assessment**

The required source searches found relevant existing behavior already recognized by the draft:

| Existing source evidence | Reuse conclusion |
| --- | --- |
| `tui.py:11–67` | Lazy enumeration, expanded-node state and cycle avoidance are suitable behavior-port candidates. |
| `local.py:52–74, 220–233` | Atomic replacement and SSH include/symlink preservation provide useful scenarios. They do not supply the proposed multi-resource recovery guarantees. |
| `local.py:285–315, 335–422, 459–479` | Existing locking, ownership refusal, creation association and lifecycle behavior are concrete references for the adapter and adoption fixture. |
| `git_identity.py:21–91, 94–151`; `git-agent.py:12–56` | Exact-key checks and public-only payloads are reusable patterns. The shared legacy supervisor’s restart behavior supports S5-ZOOM-02 and must not be treated as equivalent to the new owned-agent policy. |
| `git-guest.py:29–71`; `git-check.py:15–56` | Existing Git/SSH/signing configuration and real commit/tag checks supply useful implementation knowledge and scenarios. The draft appropriately distinguishes their mutating probes from read-only diagnosis. |
| `provision.sh:14–98`; `verify.sh:5–20` | Package, LXD storage/network and workload knowledge is reusable. The draft correctly avoids copying the current-script upgrade trigger and persistent raw-output policy unchanged. |
| `tests/test_local.py` and `tests/test_work_git.py` | Searches and selected test-body reads confirm existing scenarios for ownership refusal, interruption, source preservation, SSH configuration, key isolation and public-only payloads. These remain scenario sources, not proof of the new design. |

No additional wheel-reinvention finding is warranted from the permitted prototype search. Directly reusing its host modules would retain rejected workspace, singleton-identity and path-derived-name assumptions; the accepted Go implementation reasonably requires behavior ports.

**Read acknowledgment, evidence and limitations**

I read completely:

- Persona: `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/zoom_out.md`.
- Standard: `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/zoom-out-standard.md`.
- Frozen target: `design_draft.md`, all lines 1–1239, including FeatureRange 1–1129 and DetailedRange 1130–1239.
- Frozen sibling context: `FEATURES.md` lines 1–208, `concepts_brief.md` lines 1–83, `UX_PROPOSAL.md` lines 1–103, and `goldfish_review_report.md` lines 1–305.
- Frozen `README.md`, lines 1–110, for project alignment.

The historical Goldfish report was treated as closed concept-review context, not findings about this design.

I searched the eight permitted prototype source/script files and both Python test files using `rg` for component definitions and ownership, locking, storage, SSH, agent, identity, provisioning and recovery-related behavior, then inspected the relevant implementations identified above. Source reads were limited to the permitted launcher files and tests; caches and state were excluded.

No required review evidence was unavailable. No external sources were consulted; the target’s linked upstream bibliography was not independently verified in this pass. No files were written, tests/builds/scripts or VM commands executed, live resources inspected, credentials accessed, or other agents spawned. All validation gates remain planned work. These findings are static architectural conclusions, not claims of reproduced runtime failures.

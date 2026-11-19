# Stage 7 critic review

All three reviewers completed. Raw total: 0 Blockers, 6 Major findings, 3 Nits. No edits were applied to the frozen target.


## Design critic

Stage 7 Design Critic — REVIEW

Result: **0 Blockers, 4 Major findings, 0 Nits.** The draft satisfies the standard’s broad coverage of motivation, architecture, alternatives, and constraints. The following integration and ownership gaps need concrete design corrections. These are static findings; no qualification tests were executed.

All scopes below refer to the frozen [design_draft.md](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md).

**S7-DESIGN-01 — Major — Freeze Lima’s namespace-wide configuration as an input to every managed start**

Affected scope: [lines 267–269](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:267), 373–375, 957–965, and 987–1005.

The design freezes each environment’s template and provisioning inputs, controls the subprocess environment, and checks the recorded instance configuration before starting. It does not define how `$LIMA_HOME/_config/default.yaml`, `override.yaml`, and creation-time `base.yaml` participate in that frozen input boundary.

This is a concrete integration issue. Lima’s tagged template documents that `default.yaml` and `override.yaml` are reapplied on every start; override values can supersede instance configuration, and lists such as mounts and provisioning are combined. The tagged loader reads these files independently of the instance YAML. Controlling environment variables or materializing an instance template therefore does not neutralize them. [Lima v2.2.1 global configuration semantics](https://github.com/lima-vm/lima/blob/v2.2.1/templates/default.yaml#L625), [configuration loader](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/limayaml/load.go#L31).

Failure scenario: after successful creation or adoption, a user changes a shared namespace override between operations. The next ordinary start finds unchanged owned instance YAML but boots with an additional mount or provisioning script. That can violate the no-unsolicited-upgrade, approved-mount, and frozen-retry contracts before a post-start comparison notices the difference. This scenario does not require a concurrent external mutation.

Recommendation: define an explicit policy for namespace configuration. Either reject unsupported global configuration without modifying it, or include the effective global files, their absence, relevant dependencies, and merge semantics in the retained input revision. Check them before create/start/update and refuse unreviewed drift. Explain how this works when the root also contains unrelated or remaining prototype instances. Add qualification cases that change only global configuration while leaving the instance YAML intact. The general instruction to prevent ambient settings from changing the target, and the later configuration-merging test gate, do not yet specify this enforcement boundary.

**S7-DESIGN-02 — Major — Specify VM host-key enrollment and effective SSH verification**

Affected scope: [lines 155–157](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:155), 904–914, and 951–953.

T9 requires the dedicated tunnel to use a verified VM host key and says the ordinary SSH/VS Code client performs host-key verification. The design supplies an enrollment mechanism for GitHub host keys, but none for the VM’s host key.

Lima 2.2.1’s common SSH options contain `StrictHostKeyChecking=no`, `UserKnownHostsFile=/dev/null`, and `NoHostAuthenticationForLocalhost=yes`. Its host agent uses those options when generating the instance’s SSH configuration. Retaining that configuration does not supply the verification asserted by T9. [Lima SSH options](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/sshutil/sshutil.go#L375), [generated configuration path](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/hostagent/hostagent.go#L198).

Failure scenario: an implementation preserves Lima’s generated settings and establishes the profile-agent reverse tunnel to a reachable loopback endpoint without authenticating that endpoint as the recorded VM. Alternatively, merely enabling strict checking leaves a fresh VM without an enrolled key and breaks the noninteractive connection path. OpenSSH requires an established trust entry for that strict behavior. [OpenSSH host-key checking](https://man.openbsd.org/ssh_config#StrictHostKeyChecking).

Recommendation: specify the initial trust source, when enrollment occurs, the per-registration host-key record/known-hosts location, and behavior for adoption, port changes, rebind, and unexpected key replacement. Explicitly override the three Lima defaults on every managed transport and alias where verification is promised. If initial trust relies on trust-on-first-use, state that policy rather than calling it independently verified. Add effective-configuration checks and a changed-key/wrong-endpoint test that fails before exposing the profile socket. Provider known-hosts handling does not resolve VM transport trust.

**S7-DESIGN-03 — Major — Define lock behavior for externally managed adopted agents**

Affected scope: [lines 812–818](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:812), 857–872, 906–918, and 1037. Related approved experience: `UX_PROPOSAL.md`, lines 44–46.

The explicit lock contract unloads the profile key and inhibits automatic reload across all its VMs. Adoption introduces another resource class: an agent shared with non-adopted projects that easy-lima may use but must not stop, reconfigure, or take exclusive control of. The design does not reconcile these rules.

Failure scenario: adopt one VM while another prototype VM still uses the same legacy agent. The user locks the imported profile. Removing the key from that shared agent disrupts the remaining prototype VM, contrary to the preservation contract. Recording inhibition without removing the key leaves an existing forwarding connection able to use it. Blocking future alias preparation alone does not revoke an already established forwarding path.

This follows from the draft’s own shared-agent and persistent-tunnel architecture; it is not a claim about the current live environment. The prototype source confirms that its managed aliases share the configured agent socket in `local.py`, lines 317–328.

Recommendation: add a capability and mutation policy for externally managed profiles covering lock, unlock/activation, apply, forget, and removal. Choose a concrete, truthful outcome for lock: for example, refuse it before effects until an explicit ownership migration, or define and implement a narrower revocation mechanism covering every relevant forwarding path. Do not report a full profile lock while access remains available, and do not unload a key used by resources outside the approved scope. Any user-visible limitation or altered lock meaning must be surfaced against the already approved UX. Validate with one adopted and one non-adopted VM sharing the legacy identity.

**S7-DESIGN-04 — Major — Disable Lima’s implicit port-forwarding fallback**

Affected scope: [lines 45–46](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:45), 513–521, 904, and 1125. Related approved experience: `UX_PROPOSAL.md`, lines 36 and 64.

The design promises explicit localhost exposure, describes wrapper port-list precedence, and retains native Lima port forwarding. It does not establish the native rule that prevents unrequested ports from being forwarded.

Lima automatically forwards guest localhost ports. Its documented disabling mechanism is a catch-all ignore rule; since Lima 2.0, the wildcard guest-address rule also needs `guestIPMustBeZero=false` to cover loopback listeners. An empty wrapper port list alone does not disable the native fallback. [Lima port-forwarding documentation](https://lima-vm.io/docs/config/port/#disabling-port-forwarding). The prototype already handles this explicitly in `local.py`, lines 200–202.

Failure scenario: two newly created VMs have no requested mappings and each later starts a development server on guest port 8080. Native automatic forwarding can expose one at host port 8080, while the other encounters a collision. The preview advertised no such mapping, and the host endpoint can serve the wrong project from the user’s perspective.

Recommendation: make suppression of implicit forwarding an owned native-configuration invariant. Define ordering for approved mappings followed by a catch-all ignore rule, including IPv4/IPv6, TCP/UDP, and the unavoidable VM SSH transport exception. Apply the check to effective custom/adopted configuration, rejecting or explicitly resolving incompatible behavior. Add tests that start previously unlisted guest listeners after creation and verify that no new host application port appears. This is a known default requiring a design rule, rather than merely an unexecuted compatibility claim.

**Read acknowledgment and limitations**

I read completely:

- The supplied `agents/design_critic.md` persona and `feature-spec-standard.md`.
- Frozen `design_draft.md`, active lines **1–1129**, plus **1130–1239** for implementation details, mitigations, and traceability.
- Frozen `FEATURES.md` (208 lines), `concepts_brief.md` (83), `UX_PROPOSAL.md` (103), and `goldfish_review_report.md` (305). The last was treated only as closed historical concept-review context.

Actual local code evidence consulted: `/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py`, lines **1–170 and 200–550**, covering configuration, ownership, locking, SSH publication, lifecycle, and forwarding. No test files were consulted or executed.

Primary upstream evidence consulted: Lima v2.2.1 `templates/default.yaml`, `pkg/limayaml/load.go`, relevant portions of `pkg/sshutil/sshutil.go` and `pkg/hostagent/hostagent.go`; Lima port-forwarding documentation; and the OpenSSH `ssh_config` manual. Source behavior is distinguished above from inferred application failure scenarios.

No files were written. No builds, tests, VM commands, installations, live VM/state inspection, credentials, keys, credential stores, host SSH configuration, conversation/history/memory services, unrelated repositories, current-stage reviewer reports, or subagents were accessed. The selected versions and host matrix remain qualification targets; this review does not certify or disprove runtime compatibility.


## Structural review

Stage 7 Structural Review — Mode REVIEW

**No findings: 0 Blockers, 0 Major findings, 0 Nits.** No S7-STRUCT finding IDs assigned.

The active feature scope satisfies the standard’s structural requirements:

- **Problem:** “The Problem,” lines 25–31.
- **Goals and non-goals:** Separate sections, lines 33–65.
- **Proposed architecture:** “Technical Plan,” beginning at line 110, includes component responsibilities, flows, integration contracts, and diagrams.
- **Alternatives:** Detailed comparison tables throughout T1–T7 and a consolidated “Alternatives Considered” section at lines 1110–1128 document mechanisms and trade-offs.
- **Risks and limits:** Distributed sections explicitly address failure modes, security boundaries, scalability constraints, and mitigations—for example, lines 300–315, 621–666, 832–840, 967–977, and 1077. The standard does not mandate a separate section literally titled “Risks & Mitigations.”

Concrete checks:

- Extracted **86 headings**: one H1, eight H2s, 22 H3s, and 55 H4s. No skipped heading levels; T1–T14 are consistently nested beneath “Technical Plan.”
- Checked **28 Markdown tables**. All have valid delimiter rows and consistent column counts.
- Checked **five fenced blocks**—four Mermaid and one TOML. All are closed.
- Reviewed list structure and inline formatting; no malformed structure identified.
- Checked **99 inline links**: all 18 local references resolve to existing files, including both referenced feature-section anchors; all 81 external links have valid HTTPS URL structure.
- Treated the detailed file manifest as proposed future work, not as evidence of missing current files.

Read acknowledgment: Read the complete `structural.md` persona and `feature-spec-standard.md`; the complete frozen `design_draft.md` FeatureRange, lines 1–1129; DetailedRange, lines 1130–1239, as context; and all four frozen sibling context files: `FEATURES.md` (208 lines), `concepts_brief.md` (83), `UX_PROPOSAL.md` (103), and `goldfish_review_report.md` (305). The last was treated solely as closed historical concept-review context.

Limitations: This was a static structural and formatting audit. External URLs were checked syntactically, not fetched; Mermaid blocks were not rendered. Prototype link targets were checked for file existence without inspecting their contents. No other new reviewer reports, live state, credentials, conversation/history/memory, or unrelated repositories were inspected. No files were written, and no builds, tests, VM commands, or subagents were run.


## Style review

No Blockers. Two Major and three Nit findings in [design_draft.md](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md).

**S7-STYLE-01 — Major — Verification timing creates a circular approval gate**

Affected scope: line 529 and lines 798–802.

“Before implementation is approved” introduces tests that require implemented configuration, diagnostics, operation transitions, and release builds. The heading at line 798 repeats that timing. Elsewhere, line 75 places these checks during implementation; the build order at lines 1136–1141 creates the components needed to perform them, and line 1237 explicitly leaves validation as implementation/release work.

A fresh implementer cannot tell whether the document requires these results before beginning implementation or before accepting the resulting implementation. This ambiguity affects permission and work sequencing.

Suggested direction: distinguish design approval, early implementation qualification, and release acceptance. Rename the heading and opening sentence accordingly. If driver/Lima experiments must precede dependent feature work, state that specific dependency rather than placing all tests before “implementation approval.”

**S7-STYLE-02 — Major — Approved GitHub transport behavior remains optional in the technical contract**

Affected scope: line 937.

“New profile-enabled guests may use” the HTTPS-to-SSH mapping, followed by “this visible convenience is proposed,” conflicts with the supplied approved UX contract, which says to use the scoped mapping and show it in the preview. Line 1073 also identifies that experience as approved.

An implementer following T9 could omit the mapping as an optional proposal, leaving existing HTTPS remotes outside the accepted automatic profile-authentication experience.

Suggested direction: state the approved behavior normatively and identify its scope, preview, and conflict rules. Replace the stale proposal wording with a reference to the approved UX contract.

**S7-STYLE-03 — Nit — “Tested guest” exceeds the document’s evidence**

Affected scope: line 21.

Editorial action: **CALIBRATE**.

“Ubuntu is the tested guest” reads as an existing validation result. The document expressly says runtime qualification has not begun, and lines 981–983 and 1102 describe Ubuntu as a qualification target.

Suggested direction: use “Ubuntu is the selected guest for initial qualification,” or equivalent target language. This preserves the approved scope without implying completed testing.

**S7-STYLE-04 — Nit — The alternatives summary overstates SQLite’s necessity**

Affected scope: line 1120.

Editorial action: **CALIBRATE**.

“Cross-record ownership/claims need SQLite transactions” implies that the required consistency cannot be implemented without SQLite. T4’s comparison at lines 405–408 explicitly describes a viable file/journal approach that would require an application-owned consistency protocol.

This changes a documented trade-off into a technical necessity and gives future maintainers an inaccurate selection rationale.

Suggested direction: say that SQLite supplies the selected multi-record transaction mechanism, while the file-based alternative would require a custom commit/recovery protocol.

**S7-STYLE-05 — Nit — Repeated global execution disclaimers obscure local qualification requirements**

Affected scope: line 349; line 432’s second sentence; line 666’s second sentence; line 804; line 896.

Editorial action: **DROP** the redundant no-execution statements.

These passages repeatedly say that drafting performed no builds, runtime tests, or live-resource changes. That document-wide status is already established at lines 4 and 11. Repeating it after individual technical sections adds audit narration without changing an implementation or release decision, and makes specific unresolved qualification requirements harder to distinguish from general drafting status.

Suggested direction: keep one document-wide evidence boundary and retain section-specific evidence limitations and acceptance gates. Remove only the repeated “nothing was executed/changed” statements; preserve operational invariants and concrete limitations such as non-atomic external effects, unsupported read-only behavior, and ownership uncertainty.

Read acknowledgment: I read the complete Style Goldfish persona at `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/style.md`, the full active target range 1–1129, and all supplied frozen context: `FEATURES.md` (208 lines), `concepts_brief.md` (83 lines), `UX_PROPOSAL.md` (103 lines), and `goldfish_review_report.md` (305 lines). I also consulted the target’s later build-order and completion/validation sections to interpret approval timing. No additional style standard was supplied. The historical concept review was treated only as baseline context.

Limitations: this was a static, read-only editorial review. I did not independently verify linked upstream documentation or prototype source, inspect other new reviewer/stage reports, access conversation/history/memory or live state/credentials, run builds/tests/VM commands, write files, or spawn subagents.

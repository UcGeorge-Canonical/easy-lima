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

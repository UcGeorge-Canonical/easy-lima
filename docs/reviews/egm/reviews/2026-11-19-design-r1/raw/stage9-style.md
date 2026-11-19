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

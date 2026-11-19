What this proposes

easy-lima would become an installable Go application that associates one persistent Ubuntu development VM with a selected project folder, offering both a terminal browser and direct commands. It would use installed Lima for VM operations while adding deliberate storage placement, stable registrations and SSH aliases, optional identity profiles, and explicit environment updates. Each foreground command would coordinate through authoritative SQLite records, durable claims, and evidence-based recovery; a separate recovery inventory would provide diagnostic information without granting permission to mutate resources. Dedicated profile agents and SSH tunnels would keep credentials on the host and support later shell/editor connections after the launcher exits. The document also defines rebinding, prototype adoption, implementation order, and qualification gates, while clearly distinguishing selected design decisions from untested runtime behavior.

How the existing system works, per this document

The existing system is a Python-based local launcher used on one Mac, with workspace-specific assumptions, path-derived instance identities, directory browsing, and Lima lifecycle helpers. It uses project ownership records and `launcher.lock` to coordinate operations, and refuses to mutate an existing instance when its project ownership record is absent. Its identity helpers assume a single hard-coded profile and agent with macOS-specific administration, while provisioning updates depend on the current script hash. Existing code and tests supply useful behavior and failure scenarios, but the proposed application must replace those assumptions and add portable configuration, durable recovery, and explicit ownership transfer.

Findings

No Blocker, Major, or Nit comprehension findings.

The proposal and existing-system behavior can both be paraphrased from the target document. Across FeatureRange lines 1–1129 and DetailedRange lines 1130–1239, the document connects component responsibilities to persistence, external effects, recovery, and user-visible outcomes. Its project-specific concepts are explained inline or accompanied by documentation links; the implementation manifest and verification gates remain understandable without opening those links.

Read acknowledgment

Read the complete persona, [comprehension.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/comprehension.md), and the complete target, [design_draft.md](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md), lines 1–1239, including its headings and both specified audit ranges.

Limitations: This was a document-only comprehension audit. Linked specifications, source files, historical reports, and external documentation were not opened. No code correctness, upstream claims, or runtime behavior was independently verified; no files, builds, tests, VMs, credentials, or live state were changed.

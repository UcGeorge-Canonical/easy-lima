Read acknowledgment: I read the persona and all four required documents completely:

- [implementation_readiness.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/implementation_readiness.md). Its frontmatter is `name: egm-implementation-readiness`; I followed its BRAINSTORM migration, compatibility, and rollout focus.
- Frozen [FEATURES.md](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md).
- Frozen [concepts_brief.md](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/concepts_brief.md).
- [feature-spec-standard.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/feature-spec-standard.md).
- [zoom-out-standard.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/zoom-out-standard.md).

I found no concept blocker and recommend no scope reduction. The three Major findings below concern how the approved adoption and identity guarantees must survive transition from the prototype. They are implementation-design requirements, not demands for implementation mechanisms to be settled before design begins.

**READINESS-01 — Major — Adoption needs an explicit ownership cutover and safe downgrade policy.**

**Dimension:** Migration & Backward Compatibility.  
**Disposition:** implementation-design requirement.

**Grounding:** Frozen [FEATURES.md:42](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:42) requires stable associations and operation locking; [FEATURES.md:94](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:94)–96 promises adoption, preserved aliases, backed-up metadata, safe version failures, and potentially a compatibility `make local` entry point. The prototype derives instance names from the workspace root and relative path in [local.py:77](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:77)–95, reads unversioned records in [local.py:299](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:299)–315, and locks only its workspace state in [local.py:285](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:285)–297. [Makefile:5](/Volumes/Nebula/dev/Canonical/local/Makefile:5)–16 still invokes that implementation directly.

**Compatibility hazard and why it matters:** A new registry can preserve the disk and alias during import yet leave two launchers able to operate on the same instance using different association rules and locks. After a v1 rebind, invoking the old entry point can address a stale association or derive a different instance name. Restoring a metadata backup also does not reverse changes made after adoption to mounts, SSH configuration, or identity assignments. “Backed up” is therefore insufficient as a rollback contract.

**Concrete mitigation:** In design, establish a single authoritative manager through adoption. Preserve each existing instance name and alias as imported identifiers instead of recalculating them from the new registry path. Make supported legacy entry points delegate to v1 or refuse operation after cutover. Define the recoverable phases of import and the point beyond which downgrade requires explicit reconciliation; safely refusing an unsupported downgrade is acceptable and does not require transactional guest rollback.

Extend adoption verification to cover an interruption around cutover, an old supported entry point invoked afterward, and a project rebind followed by a retry. These should retain one owned instance and one authoritative association.

**Proposed concept amendment:** None required. A short F9 acceptance clarification about preventing concurrent legacy/new management would make the existing preservation guarantee easier to assess.

**READINESS-02 — Major — A completed prototype VM must not be mistaken for an unprovisioned v1 preset.**

**Dimension:** Migration & Backward Compatibility.  
**Disposition:** implementation-design requirement.

**Grounding:** Frozen [FEATURES.md:50](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:50)–54 prohibits silent environment upgrades on ordinary startup and requires completed unchanged setup to be skipped. [FEATURES.md:94](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:94)–96 preserves existing environments through adoption. The prototype calls provisioning on each start in [local.py:369](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:369), and its completion identity is a digest of the entire provisioning script plus selected settings in [local.py:378](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:378)–389. That script installs packages, can change Snap channels, rewrites system configuration, and finally records completion: [provision.sh:14](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:14)–39, [provision.sh:43](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:43)–59, and [provision.sh:97](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:97)–98.

**Compatibility hazard and why it matters:** Moving to Lima provisioning snippets, changing the script, or introducing a new completion format can make a healthy legacy VM appear unfinished. The first post-adoption start could then replay the new development preset against the preserved disk. Package/channel and system configuration changes can alter workloads without recreating the disk, so checking disk preservation alone will not establish safe adoption. The inverse problem also matters: a genuinely interrupted prototype installation must not be marked complete merely because its disk exists.

**Concrete mitigation:** Design an explicit translation of known legacy provisioning provenance into the new state model. Preserve the legacy environment’s recorded inputs and actual Lima configuration, distinguish known-completed from incomplete or unknown provisioning, and keep adoption separate from an environment upgrade. A changed or unrecognized marker should lead to diagnosis and an explicit reconciliation decision, not automatic execution of the current preset. This does not require fully locked package builds.

Add migration fixtures for completed and interrupted prototype provisioning. Verify that adopting and starting the completed fixture performs no unsolicited package/channel transition, while the incomplete fixture stays diagnosable and resumable.

**Proposed concept amendment:** None required; this follows F4 and F9. Adding “the first start after adoption does not implicitly convert the guest to the current preset” to adoption acceptance would expose the important interaction.

**READINESS-03 — Major — Preserving a key fingerprint is insufficient for migrating the prototype’s single identity into per-project profiles.**

**Dimension:** Migration & Backward Compatibility.  
**Disposition:** implementation-design requirement.

**Grounding:** Frozen [FEATURES.md:76](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:76)–80 requires selected-profile forwarding, optional identities, and no unrelated-agent fallback. [FEATURES.md:94](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:94)–96 promises preserved aliases and key fingerprints; [concepts_brief.md:31](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/concepts_brief.md:31) states that credentials survive adoption.

The prototype has fixed credential-service and agent identifiers in [git_identity.py:14](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git_identity.py:14)–22 and installs an unlock helper and service around those identifiers in [git_identity.py:115](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git_identity.py:115)–138. Its generated SSH configuration binds every registered alias to the same work socket in [local.py:317](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:317)–333. Its guest setup persists author identity, signing settings, SSH command, URL rewriting, and public-key files in [git-guest.py:29](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-guest.py:29)–71.

**Compatibility hazard and why it matters:** Importing the key path and fingerprint alone can leave the key unusable if the new profile cannot access its existing unlock mechanism. Independently, adding new profile configuration while retaining active legacy SSH directives and guest Git settings can leave an adopted project using stale work configuration. Switching that project to another profile—or to no profile—must reconcile these persisted settings. Otherwise the implementation can satisfy fingerprint preservation while failing the selected-identity guarantee.

**Concrete mitigation:** Treat the identity transition as a recoverable migration across the credential reference, agent service/socket, managed host SSH configuration, and managed guest Git configuration. Import the explicit legacy settings as an explicitly bound profile; preserve or deliberately migrate the existing unlock reference without generating a replacement key. Reconcile wrapper-owned legacy SSH directives when enabling the new assignment. For profile changes, including removal, remove or replace only settings the wrapper owns and report conflicts with user changes; do not restore an old whole-file backup over later edits.

Verify adoption through an actual signing operation and account check, then exercise another profile and no-profile selection. Interrupt the configuration transition and confirm that retry either establishes the requested identity consistently or fails clearly without forwarding an unrelated key.

**Proposed concept amendment:** None required. This supplies the migration meaning of the existing F7/F9 guarantees.

The concept already bounds the difficult areas well: existing disks remain in place; automatic storage moves and cloning are deferred; unrelated instances are excluded; arbitrary guest scripts have no transactional rollback promise; custom-template support is limited; Linux-specific credential and lifecycle work is acknowledged; and host/version combinations must earn support through workload checks. Those boundaries should remain. Selecting exact versions, packaging, schema filenames, and implementation language belongs in design and is not a concept defect.

This was a read-only migration review, not a runtime audit. I inspected the whitelisted README, Makefile, configuration, relevant portions of `local.py`, `git_identity.py`, `git-agent.py`, `git-guest.py`, `provision.sh`, and the migration/identity-related portions of `tests/test_local.py` and `tests/test_work_git.py`. I did not run tests, execute VM operations, inspect live state, keys, host SSH configuration, Keychain, or other repositories. No public sources were needed. The findings identify hazards evident from the prototype’s implementation; they do not claim those failures have occurred in the current environment.

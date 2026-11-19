Read acknowledgment: I read the complete [design critic persona](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/design_critic.md), whose frontmatter declares `name: egm-design`, and all four required documents:

- Frozen [FEATURES.md](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md).
- Frozen [concepts_brief.md](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/concepts_brief.md).
- [Feature Specification Standard](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/feature-spec-standard.md).
- [Zoom Out Standard](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/zoom-out-standard.md).

The concept is understandable and appropriately centered on Lima. I found no concept-level Blocker. One interaction needs an author decision; three substantial risks should become explicit implementation-design requirements. These findings do not require withdrawing the approved macOS/Linux scope or adding another readiness gate.

**DESIGN-01 — Major — Nested selections need an explicit identity and shared-source rule**

**Disposition:** author decision needed.  
**Dimension:** Architecture, identity boundaries, and duplicated environment cost.

**Grounding:** Frozen [FEATURES.md:24](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:24) permits grouping folders; lines 36–46 permit arbitrary nested selections with one VM per registered project. [Lines 68–82](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:68) combine writable source mounts with selected identity profiles. The prototype mounts the selected tree writable at [local.py:151](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:151), applies identity through guest-global Git settings at [git-guest.py:63](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-guest.py:63), and checks repository overrides only at `/workspace` at [git-check.py:23](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-check.py:23).

**Risk and why it matters:** A user can register a grouping folder with `work`, then register a child checkout with `personal`. Both VMs can access the same checkout, including its Git metadata and build outputs. The dedicated-agent promise can hold while the work-profile VM still signs changes to that personal checkout. A grouping folder that is not itself a repository also bypasses the prototype’s repository-specific identity check. Separate guest disks do not establish separate source or repository-identity boundaries.

**Concrete mitigation:** State that a profile applies to the entire selected VM and its mounted tree, with no automatic per-descendant identity switching. Show overlaps with registered projects and their profiles before creation or rebinding. The author should choose whether conflicting-profile overlaps require an explicit warning acknowledgment or are refused in v1. Add an acceptance example covering a grouping folder and a separately registered child with different profiles. This preserves nested selection while making its consequences explicit.

**DESIGN-02 — Major — Wrapper updates must not become implicit guest upgrades**

**Disposition:** implementation-design requirement.  
**Dimension:** Environment reproducibility, recovery, and repeated provisioning cost.

**Grounding:** Frozen [FEATURES.md:50](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:50) correctly requires recorded resolved inputs and forbids silent environment upgrades on ordinary startup; lines 86–90 require distinguishable completed and incomplete stages. The prototype calls provisioning on every start at [local.py:369](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:369). Its completion fingerprint includes the currently installed provisioning script and current settings at [local.py:378](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:378).

**Risk and why it matters:** Reusing that mechanism unchanged means updating the wrapper’s shipped script changes the fingerprint. The next ordinary start can rerun package installation against an existing persistent VM. Adoption is particularly exposed because the new release’s preset may differ from the prototype that created the disk. This can violate the already-approved startup contract without the user changing project configuration.

**Concrete mitigation:** Give each VM an applied provisioning revision and resolved input record independent of the currently installed wrapper release. Distinguish resuming its recorded incomplete operation from applying a new preset revision. Make an update’s proposed guest changes inspectable and explicitly applied. Acceptance should cover updating the wrapper and starting an existing VM without guest package changes, alongside resuming interrupted provisioning using its recorded inputs.

**DESIGN-03 — Major — Adoption needs one authoritative manager after the handoff**

**Disposition:** implementation-design requirement.  
**Dimension:** State ownership, coordination, and persistent-data safety.

**Grounding:** Frozen [FEATURES.md:42](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:42) requires conflicting-operation locks and retained ownership; [lines 94–98](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:94) require adoption and allow `make local` as a compatibility entry point. Today [Makefile:12](/Volumes/Nebula/dev/Canonical/local/Makefile:12) dispatches directly to the prototype. Prototype metadata and its lock are workspace-local at [local.py:239](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:239) and [local.py:286](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:286); ownership checking relies on local records and names at [local.py:309](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:309).

**Risk and why it matters:** Preserving disks, aliases, and fingerprints does not by itself transfer authority. If the installed CLI adopts an instance into new state while the old entry point still operates from the old registry, both managers can mutate the same instance while holding different locks. They can also regenerate competing SSH configuration or apply different provisioning and identity settings.

**Concrete mitigation:** Design adoption as an explicit ownership handoff. Quiesce prototype mutations; preserve the verified instance mapping; redirect the compatibility entry point to the installed CLI; and make interrupted handoffs recognizable and resumable. All supported mutation entry points should use the same authority and coordination domain. A simple compatibility shim is sufficient; there is no need for two coequal managers. Acceptance should include interruption during adoption and invoking the old entry point afterward.

**DESIGN-04 — Major — Storage admission must account for creation footprint and cumulative growth**

**Disposition:** implementation-design requirement.  
**Dimension:** Cost, quota, and resource economics.

**Grounding:** Frozen [FEATURES.md:58](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:58) promises disk/cache visibility and capacity checks; lines 60–62 require known insufficient-space failures and exclude a scheduler or automatic mover. The prototype defaults to 8 GiB RAM and an 80 GiB sparse disk per project at [local-vms.toml:17](/Volumes/Nebula/dev/Canonical/local/local-vms.toml:17), while its creation check is a fixed 10 GiB threshold against one destination at [local.py:361](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:361).

**Risk and why it matters:** A persistent VM per arbitrary selected folder multiplies guest tool installations, caches, and guest data. A fixed free-space threshold cannot establish enough space for the selected base and development preset. Separately displayed cache and VM paths may share one underlying capacity budget. Sparse maximum sizes also leave future growth unreserved, including growth while several VMs run.

**Concrete mitigation:** Base creation checks on the actual filesystems used for downloads, temporary files, cache, and VM allocation, with a conservative requirement for each shipped preset. Account for shared destinations as one capacity budget. Show projected running RAM and clearly distinguish current allocation, logical maximum, and unreserved future growth. Document that preflight protects measurable creation requirements, not unlimited workload growth. Carry this into release checks instead of inheriting the prototype’s 10 GiB constant.

The strongest existing boundaries are native Lima configuration and mechanisms, one VM per registered project, explicit storage destinations, preservation of guest data, no unrelated-instance management, optional identity setup, and the truthful forwarded-agent trust model. Deferring automatic moves, snapshots, multiple environments, Workshop, and Windows also limits the number of persistent-data and compatibility problems v1 must solve. Exact versions, packaging, and Linux credential mechanisms are legitimate design work, not findings by themselves.

Method and limitations: this was a read-only BRAINSTORM review, not an implementation audit or runtime verification. I consulted the whitelisted `README.md`, `Makefile`, `local-vms.toml`, relevant portions of `.local-launcher/local.py`, `git_identity.py`, `git-guest.py`, `git-check.py`, `provision.sh`, and the two test source files. I inspected test code but executed no tests, VM operations, installation, or mutations. I did not inspect live state, host SSH configuration, keys, Keychain, other repositories, or conversation services. No external official sources were needed. All required reads succeeded.

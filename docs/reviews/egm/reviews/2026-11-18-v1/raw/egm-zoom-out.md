Read acknowledgment: I fully read the persona and all four required documents:

- [zoom_out.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/zoom_out.md); its frontmatter confirms `name: egm-zoom-out`. I followed BRAINSTORM mode.
- [Frozen FEATURES.md](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md).
- [Frozen concepts_brief.md](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/concepts_brief.md).
- [feature-spec-standard.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/feature-spec-standard.md).
- [zoom-out-standard.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/zoom-out-standard.md).

The concept has a clear purpose and sensible boundaries. I found **no Blocker**. The strongest risks concern which prototype assumptions survive into implementation design. I recommend one concept clarification and two explicit design requirements; none requires removing approved v1 scope.

**ZOOM-01 — Major — Persisted registration must become the authority for VM identity.**  
**Dimension:** Contrarian / YAGNI & Code Minimization.  
**Disposition:** implementation-design requirement.

F3 promises stable associations and rebinding without another disk, while F9 preserves existing instance connections during adoption ([FEATURES.md:42–46](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:42), [94–96](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:94)). The prototype derives both VM name and SSH alias from workspace and relative path, then uses that derived name to locate its ownership record ([local.py:77–95](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:77), [299–315](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:299)). The TUI independently derives the same identity ([tui.py:146](/Volumes/Nebula/dev/Canonical/local/.local-launcher/tui.py:146)). Its existing remount operation updates a source target under the old logical identity; it does not solve registration moves ([local.py:441–457](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:441)).

This matters because adding rebinding only around the existing path-derived model leaves other operations capable of finding or allocating the wrong instance. Adoption also becomes harder if names are regenerated when the wrapper moves out of this workspace.

**Mitigation:** Make persisted registration authoritative for the actual instance name and SSH alias, with the project path as rebindable metadata. Preserve adopted names and aliases. Reuse the ownership checks and stop/edit/update sequencing, but route browsing, lifecycle commands, and rebinding through the same registration lookup. Avoid separate legacy and new lifecycle models.

**ZOOM-02 — Major — Wrapper updates must not implicitly become preset updates.**  
**Dimension:** Contrarian / YAGNI & Code Minimization.  
**Disposition:** implementation-design requirement.

F4 explicitly delegates provisioning to Lima, requires unchanged completed setup to be skipped, and prohibits silent environment upgrades during ordinary startup ([FEATURES.md:50–54](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:50)). Today every `up` calls the wrapper’s provisioning runner. That runner fingerprints the currently installed shell script and settings, and reruns provisioning whenever they differ ([local.py:369–389](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:369)). The script installs packages and refreshes changed Snap channels ([provision.sh:14–41](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:14)). Even an implementation refactor can therefore change the fingerprint of an existing environment.

The risk is carrying this policy into an installed product: updating easy-lima, splitting the preset, or adopting the prototype could trigger guest changes on the next ordinary start. Expanding this runner to understand arbitrary user YAML would also create the second provisioning platform the concept explicitly rejects.

**Mitigation:** Separate wrapper release changes from each VM’s recorded preset inputs and completion state. Adoption should retain the existing guest environment until a specific change is requested. Reuse the package operations and workload checks as maintained Lima provisioning snippets; keep automatic retry scoped to the recorded setup being completed. Let Lima remain the provisioning mechanism instead of generalizing the current runner into a workflow engine.

**ZOOM-03 — Major — Clarify the identity boundary of grouping-folder VMs.**  
**Dimension:** Contrarian / YAGNI & Code Minimization.  
**Disposition:** author decision needed.

The concept permits selecting a grouping folder and gives that selection one VM ([FEATURES.md:24](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:24), [36–46](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:36)). Named profiles then promise intended metadata and clear failures for conflicting repository overrides ([FEATURES.md:76–80](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:76)).

A grouping folder can contain separately registered work and personal projects. The documents do not state whether their child assignments have any effect inside the grouping VM. The prototype applies one identity through guest-global Git settings ([git-guest.py:63–70](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-guest.py:63)), and its repository override check examines `/workspace` only. It skips descendant repositories when `/workspace` itself is a grouping directory ([git-check.py:23–28](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-check.py:23)).

This ambiguity can produce incorrect attribution or tempt implementation into automatic repository discovery and credential routing.

**Proposed concept amendment:** State that one selected registration has at most one active profile, applying throughout that VM; descendant registrations do not silently introduce other credentials. Decide how the UI handles a grouping selection containing known child registrations with different profiles—an explicit warning or refusal is sufficient. Also state which repository an identity check validates. This preserves grouping folders and named profiles without introducing per-repository agent multiplexing.

**ZOOM-04 — Nit — Document the incremental implementation alternative.**  
**Dimension:** Contrarian / YAGNI & Code Minimization.  
**Disposition:** advisory.

The alternatives section establishes the Lima-wrapper direction and defers Workshop, but primarily discusses product direction and naming ([concepts_brief.md:37–41](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/concepts_brief.md:37)). It does not record a concrete comparison between extending the existing helper and starting a new implementation, despite the feature-spec standard’s alternatives requirement.

The existing launcher already implements much of the distinguishing workflow using the standard library. A rewrite could discard useful behavior while spending effort on capabilities Lima or the prototype already supplies.

**Mitigation:** Add a short alternatives record: plain Lima templates and saved SSH settings provide the VM substrate; this wrapper remains justified by project registration, storage safeguards, and selected identity handling. Treat incremental extraction of the existing helper as the baseline against which any rewrite or new dependency is justified. This is documentation and design guidance, not another approval gate.

**What is already well bounded**

- Lima owns VM mechanisms, while wrapper configuration has explicitly limited responsibilities ([FEATURES.md:11](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:11), [100–109](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:100)).
- One VM per registration, one managed storage root, optional Git, and limited presets prevent several forms of platform expansion.
- Deferring automatic moves, snapshots, variants, Workshop, and Windows is justified by their additional data and compatibility contracts.
- The agent-forwarding and writable-mount trust boundaries are stated accurately; arbitrary provisioning rollback is explicitly excluded.
- Linux portability work and unresolved cache behavior are acknowledged. Exact versions, packaging, and credential mechanisms are legitimate implementation-design decisions, not concept blockers.

**Concrete existing-code reuse candidates**

- **Lazy project browser:** [tui.py:11–67](/Volumes/Nebula/dev/Canonical/local/.local-launcher/tui.py:11) already supplies lazy directory reads, cycle avoidance, hidden-folder filtering, and iterative traversal. Preserve these behaviors while replacing path-derived VM lookup. Existing coverage is at [test_local.py:169–183](/Volumes/Nebula/dev/Canonical/local/.local-launcher/tests/test_local.py:169).
- **State and lifecycle safeguards:** atomic writes, operation locking, ownership-before-creation, and confirmed deletion are useful building blocks ([local.py:52–64](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:52), [285–297](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:285), [353–363](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:353), [459–479](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:459)).
- **SSH integration:** the preserved, backed-up Include installation and delegation to Lima-generated connection settings are directly relevant ([local.py:220–233](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:220), [317–333](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:317)). Profile selection must become per registration.
- **Identity validation:** exact agent-key comparison, public-only guest payloads, and disposable signing/account checks provide useful foundations ([git_identity.py:50–81](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git_identity.py:50), [git-check.py:15–56](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git-check.py:15)). Fixed work-profile names, sockets, and macOS service handling require deliberate adaptation.
- **Provisioning and workload knowledge:** retry-aware LXD setup and the real container/DNS probe can inform the optional development preset ([provision.sh:61–79](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:61), [verify.sh:14–20](/Volumes/Nebula/dev/Canonical/local/.local-launcher/verify.sh:14)). Their current all-tools bundle should be split according to the already-approved optional choices.
- **Compatibility entry point:** [Makefile:9–16](/Volumes/Nebula/dev/Canonical/local/Makefile:9) is already thin and can remain a small adapter.

These are inspected reuse candidates, not claims that they already satisfy the new release requirements.

**Scope and method limitations**

This was a read-only concept and reuse review. I consulted the complete whitelisted `README.md`, `Makefile`, `local-vms.toml`, all eight launcher source/script files, and both test files. I did not execute tests, VM commands, installation, or source changes. I did not inspect `.local-state`, host SSH configuration, credentials, Keychain, live agent state, unrelated repositories, or conversation/history services. No official external sources were needed. Runtime compatibility, adoption feasibility against actual state, and test results remain unverified.

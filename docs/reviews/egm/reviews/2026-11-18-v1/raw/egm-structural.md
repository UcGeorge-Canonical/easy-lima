Read acknowledgment: I read the complete persona and all four required documents:

- [structural.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/structural.md); its frontmatter identifies `name: egm-structural`. I applied its **BRAINSTORM** instructions.
- Frozen [FEATURES.md](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md).
- Frozen [concepts_brief.md](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/concepts_brief.md).
- [feature-spec-standard.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/feature-spec-standard.md).
- [zoom-out-standard.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/zoom-out-standard.md).

The concept is sufficiently clear to enter implementation design. I found three significant maintainability risks in adapting the prototype, but no concept-level blocker. **No scope amendment or additional readiness gate is proposed.** These findings identify boundaries the implementation design should preserve; they do not treat acknowledged implementation TBDs as defects.

**STRUCTURAL-01 — Major: Separate registered project identity from paths, instance names, and SSH aliases.**

**Dimension:** Long-Term Tech Debt & Maintainability.  
**Disposition:** implementation-design requirement.

**Grounding:** [FEATURES.md:42–46](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:42) requires stable associations and explicit rebinding; [FEATURES.md:94–96](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:94) requires adoption to preserve disks and aliases. The prototype derives the instance name from the workspace root and relative project path, then derives the SSH alias from that name in [local.py:77–95](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:77). Its record location and ownership checks depend on those derived values in [local.py:302–314](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:302).

**Why it matters:** The prototype’s path is simultaneously a lookup key, naming input, and ownership boundary. Retaining that coupling would make moving the installed wrapper, changing workspace roots, or rebinding a checkout affect VM discovery. Adoption would then accumulate exceptions to recreate old naming inputs, making preservation guarantees progressively harder to maintain.

**Concrete mitigation:** Give a registered project a stable identity independent of its current folder path. Persist its actual Lima instance locator and SSH alias as explicit bindings, including legacy values imported during adoption. Rebinding should change the project/source association without recomputing the instance name or alias. Define collision handling and recovery when the association changes but the mount update does not complete. The design need not choose a particular database or identifier format to establish this boundary.

A useful acceptance scenario combines adoption and rebinding: adopt a legacy VM, change the project or workspace location, and reconnect through the preserved alias to the same guest disk.

**STRUCTURAL-02 — Major: Keep environment revisions independent of the currently installed wrapper and current defaults.**

**Dimension:** Long-Term Tech Debt & Maintainability.  
**Disposition:** implementation-design requirement.

**Grounding:** [FEATURES.md:50–54](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:50) requires recorded inputs, no silent startup upgrades, preservation or explicit rejection of custom Lima settings, and provisioning through Lima’s facilities. Recovery must distinguish stages without promising arbitrary rollback in [FEATURES.md:86–90](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:86). The prototype loads current options during every start and invokes provisioning in [local.py:335–369](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:335). It computes completion from the currently installed script plus those options in [local.py:378–389](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:378). That script performs package installation and channel changes in [provision.sh:14–41](/Volumes/Nebula/dev/Canonical/local/.local-launcher/provision.sh:14).

**Why it matters:** Reusing this mechanism would let a wrapper upgrade change a provisioning hash and turn an ordinary start into environment modification. An interrupted operation could also resume against different defaults or snippets from those originally selected. Once arbitrary Lima YAML is accepted, a parallel wrapper-owned reconciliation mechanism would create two competing interpretations of the intended environment.

**Concrete mitigation:** Associate the resolved creation inputs and maintained provisioning revision with each environment. Distinguish completed configuration, an incomplete operation’s intended inputs, and newly requested changes. Ordinary startup should reuse the established environment; recovery should resume its recorded operation rather than silently substitute current defaults. Changes requiring provisioning or recreation should be surfaced as explicit operations.

Keep Lima authoritative for its native configuration and execution mechanisms. Define the small set of wrapper-controlled fields and reject incompatible custom settings explicitly, as the concept already permits. Limit automatic retry guarantees to understood, retry-safe maintained steps; expose the recovery expectations of arbitrary custom provisioning without inventing a general transaction engine.

**STRUCTURAL-03 — Major: Decouple profile administration from VM lifecycle and scope credential services to individual profiles.**

**Dimension:** Long-Term Tech Debt & Maintainability.  
**Disposition:** implementation-design requirement.

**Grounding:** [FEATURES.md:76–80](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:76) introduces optional named profiles, selected-agent forwarding, and reconnect-based profile changes. [FEATURES.md:113](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:113) explicitly recognizes Linux agent lifecycle and credential-storage work. The prototype holds one manager-wide Git configuration in [local.py:237–245](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:237), assigns the same forwarding socket to every recorded alias in [local.py:317–328](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:317), and applies Git setup by calling the full startup path for every registered VM in [local.py:406–422](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:406). Its service label, Keychain identifiers, and socket are singletons in [git_identity.py:14–22](/Volumes/Nebula/dev/Canonical/local/.local-launcher/git_identity.py:14).

**Why it matters:** Adding a profile selector to this global model would leave hidden coupling between otherwise independent projects. Creating or repairing one identity could start other VMs, invoke provisioning, or disturb existing agent connections. Adding macOS/Linux conditionals throughout that orchestration would multiply lifecycle combinations and recovery paths.

**Concrete mitigation:** Separate host profile creation/import and agent availability from project binding and guest identity application. Identify each profile’s service/socket independently, and place platform-specific service and credential handling behind a narrow boundary. Apply identity changes only to explicitly affected projects; profile setup should not invoke the general startup/provisioning path for every registered VM. Use the already-approved stop/reconnect allowance to avoid implementing live identity switching.

Carry forward three observable invariants: creating a personal profile does not start or reconfigure work VMs; an unavailable profile does not prevent unrelated profiles or identity-free projects from operating; adopting the existing work profile preserves its key fingerprint without recreating its credential.

The following aspects are already well bounded:

- Lima owns VM mechanisms; a competing-provider abstraction and custom provisioning platform are expressly excluded ([FEATURES.md:11–13](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:11), [FEATURES.md:134–136](/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md:134)).
- One VM per registered project, one managed storage root, no arbitrary provisioning rollback, and deferred storage moves/snapshots materially constrain state complexity.
- Git identity is optional, its trust boundary is stated, and live switching is unnecessary.
- The configuration responsibility table provides an appropriate starting point; filenames and precedence can remain implementation-design decisions.
- The acceptance checks already cover ownership, interrupted creation, rebinding, identity separation, and preservation during adoption.

Scope and method limitations: this was a read-only concept and maintainability review supported by static source inspection. I did not run tests, VM operations, installation, or migration; inspect private state, keys, Keychain, host SSH configuration, other repositories, or conversation services; or modify files. The findings concern implementation hazards, not verified failures of an implemented v1.

Additional evidence consulted: complete `README.md`, `Makefile`, and `local-vms.toml`; relevant portions of `.local-launcher/local.py`; complete `.local-launcher/git_identity.py`, `.local-launcher/git-guest.py`, and `.local-launcher/provision.sh`; selected search results from `.local-launcher/git-agent.py`; and relevant excerpts of `.local-launcher/tests/test_local.py` and `.local-launcher/tests/test_work_git.py`. No external official sources were needed or consulted.

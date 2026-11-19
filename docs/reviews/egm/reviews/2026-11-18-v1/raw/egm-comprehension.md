Read acknowledgment: I read the persona and all four required documents completely. The persona frontmatter is `name: egm-comprehension`.

- Persona: `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/comprehension.md`
- Frozen features: `/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/FEATURES.md`
- Frozen concept brief: `/Volumes/Nebula/dev/Canonical/local/.egm/reviews/2026-11-18-v1/inputs/concepts_brief.md`
- Feature standard: `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/feature-spec-standard.md`
- Zoom-out standard: `/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/zoom-out-standard.md`

The concept is understandable: a selected folder becomes a registered project with one persistent Lima VM, while easy-lima adds browsing, storage safeguards, identity selection, and recovery. Lima remains the VM configuration and runtime mechanism. I found no comprehension blocker that should prevent implementation design, and I do not recommend narrowing the approved platform or feature scope.

**COMPREHENSION-01 — Major — Grouping-folder VMs need an explicit identity boundary.**

Dimension: UX & Developer Ergonomics.  
Disposition: **author decision needed**, as a proposed clarification of the approved concept.

Grounding: Frozen `FEATURES.md:24`, `:36–38`, `:68–82`, and `:109`; frozen `concepts_brief.md:31`. Relevant prototype behavior appears in `.local-launcher/git-guest.py:63–70`, which installs a guest-global identity, and `.local-launcher/git-check.py:23–28`, which checks repository overrides at `/workspace` only.

The feature set deliberately permits selecting a grouping folder and separately assigning identities to projects. Consider registered children `clients/work-app` and `clients/personal-app` with different profiles, then selecting `clients` as another project. A fresh reader cannot tell whether the parent VM uses one identity throughout, honors the children’s assignments, or refuses this combination. “Keep identities separate” does not explain what separation means when the source trees overlap.

This is consequential UX ambiguity rather than a request for implementation detail. A user may enter a child repository from the grouping VM and reasonably assume its registered profile applies. The prototype’s root-only repository check also provides no coverage of nested repositories when the selected folder is not itself a repository.

Concrete mitigation / ergonomic requirement: state that an identity is VM-wide if that is the intended model, and explicitly state whether child project assignments have any effect inside a grouping VM. Choose between rejecting known conflicting group selections and allowing them with an explicit acknowledgement. The creation/details view should show the chosen identity and known overlapping registered projects. Separately define the scope of repository-override validation; a check must not imply that unexamined descendant repositories were validated. This clarification preserves grouping-folder selection and named profiles rather than silently removing either.

**COMPREHENSION-02 — Major — Storage selection and adoption need one visible rule when their destinations differ.**

Dimension: UX & Developer Ergonomics.  
Disposition: **author decision needed**, as a proposed clarification of the storage/adoption boundary.

Grounding: Frozen `FEATURES.md:18`, `:58–64`, and `:94–96`; frozen `concepts_brief.md:31` and `:47`.

First-run setup lets a user select a managed storage root. F5 then limits v1 to one managed root while saying existing environments stay in place during adoption. F9 promises to preserve their storage destinations. The ordinary case “choose a new storage root, then adopt the prototype at its old destination” therefore has two plausible outcomes that the concept does not distinguish.

This matters during onboarding: refusing adoption after setup, silently changing the selected root, or accepting two managed locations would each communicate a different product contract. Storage is deliberately central to this product, so the user should not discover that policy through an implementation error.

Concrete mitigation / ergonomic requirement: select and document one of these policies:

- Adoption establishes the single managed root from the existing environment; a conflicting selection produces an explanation showing both paths and an explicit correction.
- The single-root limit governs new VM creation, while adopted VMs retain grandfathered locations that remain visible in listings and diagnostics.

The second is a small explicit scope exception, not a requirement for per-VM storage pools or automatic moves. Neither choice requires moving existing disks.

**COMPREHENSION-03 — Nit — Separate VM readiness, local identity readiness, and online account verification in the UX.**

Dimension: UX & Developer Ergonomics.  
Disposition: **implementation-design requirement**, not a proposed scope amendment.

Grounding: Frozen `FEATURES.md:76–80` and `:86–88`. The prototype already distinguishes these concerns: `README.md:109–135` describes manual GitHub registration and an offline signing check; `.local-launcher/git-check.py:29–56` performs local signing checks before its optional GitHub authentication check.

F7 requires an account check and clear failures, but leaves unclear when it runs and which workflow it blocks. During travel or an outage, “GitHub unavailable” must not be presented as “wrong account,” and a VM that is already usable must not be described simply as a failed VM because an optional online check could not finish. Conversely, an unchecked account must not be reported as verified.

Concrete mitigation / ergonomic requirement: carry the prototype’s offline/online distinction into the CLI and TUI design. Name the validation stages and report their separate results. Specify which operations require a ready selected key and when account verification occurs. Preserve the manual registration guidance, and make authentication failure, account mismatch, and inability to contact GitHub distinct outcomes. This requires no additional provider integration.

What is already well bounded: the documents clearly distinguish VM persistence from source sharing, forbid silent identity inheritance and unrelated-agent fallback, expose the forwarded-agent trust boundary, require explicit extra mounts and ports, and defer storage moves and broader environment models. The preview-before-creation workflow, noninteractive prompt rule, ownership protections, and recoverable incomplete-operation states provide useful ergonomic constraints without prescribing unnecessary machinery. Exact compatibility versions, packaging, and configuration filenames are appropriately left to design.

Scope and method limitations: this was a read-only BRAINSTORM review for usability and cognitive friction, not a release-readiness audit or validation of Lima behavior. I executed no tests, installations, VM operations, or modifications and consulted no other reviews, state directories, credentials, or host SSH settings. No public sources were needed.

Additional evidence consulted: complete `README.md` and `local-vms.toml`; relevant ranges of `.local-launcher/local.py`; complete `.local-launcher/git-check.py` and `.local-launcher/git-guest.py`; targeted search results from `.local-launcher/git_identity.py`, `.local-launcher/tui.py`, `.local-launcher/tests/test_local.py`, and `.local-launcher/tests/test_work_git.py`. These establish existing workflow examples only; they do not establish that the proposed v1 guarantees currently pass.

Read-only REVIEW completed. **0 Blockers, 1 Major, 0 Nits.** The remaining finding concerns the executable legacy fixture, not missing planned application files.

**R2-CODE-01 — Major: Define the legacy fixture’s module-level script dependency**

Affected frozen scope:

- [design_draft.md:321](../inputs/design_draft.md:321), lines 321–322: vendor `local.py` and execute its real ownership/start/stop/delete paths.
- [ADOPTION.md:58](../inputs/ADOPTION.md:58), lines 58–68: vendor the unaltered source, construct `Manager` with sandbox paths, patch host boundaries, and require a matching-record positive control to reach a fake mutation.

Source evidence:

- [local.py:22](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:22) derives module-level `ROOT` from the imported file’s location.
- [local.py:237](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:237), lines 237–246: supplying the sandbox `root` changes `self.root`, not module-level `ROOT`.
- [local.py:369](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:369) unconditionally calls `self.provision(...)` after the create/start decision.
- [local.py:379](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:379), lines 379–389: provisioning reads `ROOT/.local-launcher/provision.sh` before issuing its first fakeable command.
- [test_local.py:124](/Volumes/Nebula/dev/Canonical/local/.local-launcher/tests/test_local.py:124), lines 124–129: the existing provisioning fixture also depends on that actual script read.

With the proposed vendored location, the unaltered module resolves this dependency to `testdata/prototype/.local-launcher/provision.sh`. That file is not supplied by the fixture manifest. Passing a disposable root to `Manager` and faking `local.run` does not redirect the read.

Consequently, a valid running-instance `up` positive control encounters a missing-script error before reaching its provisioning mutation; a stopped-instance control that successfully fakes the start subsequently encounters the same failure. Retired-record refusal cases can still pass because ownership checking exits earlier. This leaves the implementer to invent an additional fixture boundary or prematurely terminate the positive control.

**Correction:** Specify the exact treatment of module-level `ROOT` and provisioning-script bytes. For example, explicitly authorize rebinding `local.ROOT` to a disposable root, populate its expected script path with fixed synthetic bytes supplied by the driver, and define the fake digest-read/provision responses. Preserve the real `up`, `provision`, `check_ownership`, and JSON-reading logic; never execute the synthetic or original guest script. Require a matching-record positive control that reaches the intended fake provisioning dispatch and distinguishes missing fixture inputs from ownership refusal. State the expected no-op for an already-stopped `stop` control separately.

This is an implementation-readiness gap under the standard’s zero-decision and test-infrastructure requirements. It is not a claim that an implemented harness has failed.

The checked ownership and reuse claims otherwise agree with the allowed source:

- Missing active records cause real `up`, `stop`, and `delete` refusal before mutation for existing instances: [local.py:309](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:309), [local.py:335](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:335), [local.py:430](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:430), and [local.py:459](/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py:459).
- The new design explicitly replaces the prototype’s path-derived authority, broad forwarding-policy change, current-script-driven provisioning, and temporary-repository diagnostic probe. It does not falsely present those behaviors as suitable unchanged production implementations.
- All six source hashes listed in the permitted provenance manifest match the current allowed source files, including the exact `local.py` digest recorded by ADOPTION.md.

Primary-source verification found no additional defect in the checked Lima claims. The tagged loader reads namespace defaults/overrides, while the tagged template distinguishes creation-time base embedding; the design’s restrictions reflect those mechanisms. [Lima v2.2.1 loader](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/limayaml/load.go), [tagged template](https://github.com/lima-vm/lima/blob/v2.2.1/templates/default.yaml). The catch-all suppression and explicit GRPC selection agree with upstream guidance and the tagged matching paths. [Port-forwarding documentation](https://lima-vm.io/docs/config/port/), [GRPC matcher](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/portfwd/forward.go), [host-agent rules](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/hostagent/port.go). The documented start/create hazard, permissive generated SSH settings, skipped underscore directories, and local-file cache behavior also have tagged-source support. [Start implementation](https://github.com/lima-vm/lima/blob/v2.2.1/cmd/limactl/start.go), [SSH implementation](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/sshutil/sshutil.go), [instance store](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/store/store.go), [downloader](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/downloader/downloader.go).

Files actually read:

- Required persona: [codebase_details.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/agents/codebase_details.md).
- Required standard: [implementation-readiness-standard.md](/Users/ucgeorge/.codex/plugins/cache/openai-curated-remote/egm/0.1.1/resources/document_standards/implementation-readiness-standard.md).
- Frozen target and required context: `design_draft.md`, `TECHNICAL_DETAILS.md`, `ADOPTION.md`, `PROVISIONING.md`, `STATE_CONTRACT.md`, `ADOPTED_AGENTS.md`, `PROFILE_TRANSITIONS.md`, `LIMA_BOUNDARIES.md`, `INTERFACE_V1.md`, `FEATURES.md`, and `UX_PROPOSAL.md`, all in the frozen `inputs` directory.
- Allowed prototype source: `local.py`, `tui.py`, `git_identity.py`, `git-agent.py`, `git-guest.py`, `git-check.py`, `provision.sh`, `verify.sh`, `tests/test_local.py`, and `tests/test_work_git.py`, all under `/Volumes/Nebula/dev/Canonical/local/.local-launcher/`.
- Hash evidence only: [prototype-source-manifest.json](../../2026-11-19-design-r1/resolutions/ELD-08/prototype-source-manifest.json).

No optional documents, earlier reviewer reports, conversation memory, live metadata, credentials, or host configuration were read. No files were written; no prototype modules, tests, scripts, VMs, or SSH commands were executed. This report does not authorize implementation.

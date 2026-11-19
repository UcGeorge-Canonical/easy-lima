# Approved-design drift report

The [requirement matrix](2026-11-19_REQUIREMENTS.md) traces the approved user experience to implementation and evidence. The target is the complete standalone application, not a published-release diff. Local source manifests and test logs are retained under ignored `test-results/`. No commit, push or release publication was performed.

The author deferred release distribution, broader host qualification and advanced work outside everyday use. Setup, documentation, ordinary profile/lifecycle use and normal recovery remain in scope. Historical design approval is not runtime verification.

## Review and dispositions

Baseline reviews covered UX/CLI/configuration, identity/SSH/credentials and lifecycle/storage/state/adoption. The coordinator made the application changes. Fresh integrated reviews used separate GPT-6.2 Sol and GPT-6.2 Astra reviewers with read-only snapshots; neither implemented the fixes. Focused rechecks closed their findings. Actual runtime evidence is recorded separately in [testing](../testing.md).

| Document | Difference and disposition |
| --- | --- |
| UX_PROPOSAL | Replaced bare storage input with drives/free space/folder navigation, writable home starting location, complete preview, optional profile and explicit retry. Retained browser state and exact selection across async loading; separate VM/identity observations, wrapped approval and task progress. Tests and real PTY cover setup. |
| INTERFACE_V1 | Corrected path `--`, canonical errors, exec status, secret FDs, browser selectors, command help, dry-run consent and log-follow errors. Generated command docs and parsed examples now match implementation. |
| PROFILE_TRANSITIONS | Metadata-only rename, accurate transition previews, usable pending completion, selective field preservation, duplicate/retired profile rejection, committed result key and transactional projected overlap checks. VM-only actions report known identity unavailability separately. |
| ADOPTED_AGENTS | Preserve external agent/store/key references and recovery guidance. Same-key migration leaves members stopped; external resources are not deleted or silently replaced. |
| FEATURES | Public usage/TOML/YAML docs, accurate storage/resource details, explicit exposures and valid non-Git workspace diagnostics. Workload/shared-profile evidence is separate from source coverage. |
| TECHNICAL_DETAILS | Closed user-visible progress/configuration/state gaps. Kept tested parser instead of replacing it solely for an internal library choice. Internal layout, release and advanced qualification exceptions remain deferred. |
| STATE_CONTRACT | Reconcile deleted-instance absence, reject retained disk paths, revalidate root/capacity on resume, reject tampered namespace witnesses, and avoid dispatch after concurrent completion. Advanced restore finalization remains deferred. |
| PROVISIONING | Retain frozen retries, validate native inputs before ownership and after setup, preserve history, add only missing preset stages, require explicit refresh for unknown history and display actual stages. Richer substep/provenance qualification deferred. |
| LIMA_BOUNDARIES | Check namespace drift at mutations and doctor; explicitly accept supported changes. Validate trust exports, recover original enrollment publication, verify exact host agent/key/signing before forwarding, revoke failed new forwarding and reserve explicit ports. |
| ADOPTION | Check overlaps/profile reuse before handoff, recover profile TOML publication, preserve identity/history, recheck namespace/alias immediately before retirement. Live fixture expanded to a disposable external Git agent. Exhaustive historical-launcher and driver-fault matrix deferred. |
| design_draft | Completion is now scoped through a requirement matrix and actual evidence. The earlier broad completion claim was inadequate; full release/advanced restore completeness is not claimed. |
| DECISIONS | Go, foreground commands, installed external Lima, SQLite/WAL and TOML plus native YAML reflected in implementation. |
| concepts_brief | Standalone arbitrary-CWD application, explicit storage and isolated reusable Git profiles retained. |
| README.original | Historical prototype material; current installation/usage live in public guides. |
| goldfish_review_report | Accepted GF-01–GF-11 policies remain binding and are reflected in the matrix. |
| consolidated_review_report | Retained as design rationale, not runtime proof. Qualification exceptions explicit. |
| design/README | Separate historical design index; public README leads to usage docs. |

## Integrated findings

| Finding | Final correction |
| --- | --- |
| IU-01 | Clamp selection after hidden-folder reload. |
| IU-02 / CORE-01 | Complete usable pending identity on explicit start; report known identity unavailability separately for VM-only commands. |
| IU-03 | Wrap each plan line without dropping paths/ports or original field boundaries. |
| IU-04 | Preserve exact selected project through asynchronous child insertion. |
| IU-05 | Separate first-creation flags from returning-project commands in examples. |
| IU-06 | System-drive choice starts at home; nonwritable new/existing root fails before config publication and permits explicit retry. |
| IU-07 | Missing Lima names its required version and install/selection guidance. |
| CORE-02 | Exact host key set and signing verified before forwarding; failed new forwarding is revoked. |
| CORE-03 | Resume trust publication under original enrollment/repair without reenrollment. |
| CORE-04 | Projected membership and claims admitted atomically; pending targets fence overlap; cutover rechecked. |
| CORE-05 | Namespace and legacy alias rechecked after stopping/backing up, immediately before retirement. |
| CORE-06 | Actual native inputs compared with frozen creation inputs on resume and before baseline commit. |
| CORE-07 | Unused profile checked on removal; new/adopted references to retired/changed profiles rejected. |
| CORE-08 | Removed managed SSH include requires explicit alias repair. |
| CORE-09 | Doctor reports namespace drift separately from instance YAML. |

The live rerun also exposed a transport classification bug: a stalled guest login service was reported as a mismatched workspace. Workspace verification now preserves transport failures, and cancellation closes the SSH transport even while channel creation is blocked. A real local SSH server fixture covers blocked channel and exec requests. The affected disposable VM was explicitly stopped and recovered through its original saved setup, preserving its instance and disk; runtime outcomes are recorded in the testing guide.

## Remaining differences

The [explicit deferrals](2026-11-19_REQUIREMENTS.md#explicitly-deferred-or-outside-this-completion-claim) are the outstanding work register. Release tooling, additional native qualification, exact advanced restore finalization and exhaustive stage/handoff fault evidence are not marked complete. Future claims must update that register and the actual validation record.

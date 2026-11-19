# Approved-design implementation audit

Scope: the approved everyday experience, including setup, documentation, profiles, lifecycle, diagnostics, updates, rebind and supported prototype adoption. Release distribution, additional native host qualification and advanced maintenance qualification are deferred at the author's request. This does not claim every internal design detail or release acceptance criterion is complete.

Requirements: [FEATURES](../design/FEATURES.md), [approved experience](../design/UX_PROPOSAL.md), [technical contracts](../design/TECHNICAL_DETAILS.md) and their normative supplements. See the [drift report](2026-11-19_DRIFT_REPORT.md) for dispositions and [testing](../testing.md) for reproducible commands and actual run outcomes. Historical design approval is not runtime verification.

Evidence terminology: **tests** are automated behavioral/contract checks; **source** is implementation and caller review; **terminal** is the binary exercised through a real PTY; **VM** is a disposable Ubuntu guest using installed Lima. Broader host qualification cannot be inferred from any one of these.

## Product requirements

| Requirement | Implementation and verification | Disposition |
| --- | --- | --- |
| F1: standalone installation and arbitrary CWD | Go binary, persistent user paths/registry, explicit browsing root; Make install and CLI tests. No `make local` compatibility entry point. | Implemented; distribution deferred. |
| F1: first setup and dependency preflight | Drive/free-space and folder picker, manual path, Lima 2.2.1 probe, complete disk/cache/temp/state preview, socket path limit. Init regressions and real terminal cancellation/approval. | Implemented. |
| F1: repeat/interrupted setup | Existing root/config preserved; missing config recovery; nonwritable root rejected before saving it; explicit retry. Init permission/recovery tests. | Implemented. |
| F1: optional identity and opening preference | First-run profile choice, SSH/editor preference; uninstall preserves VMs. Source and terminal. | Implemented. |
| F2: exact folder selection at any depth | Lazy tree, symlink-aware associations, cycle protection, grouping folders and paths with spaces. Project/TUI tests and VM workspace check. | Implemented. |
| F2: navigation/filter/toggles/session | Bounded workers; retained browser state; exact selection preserved across async insertion; global VM view and explicit pagination. TUI tests and source. | Implemented. |
| F2: useful selected-project state | Async cancellable VM/details observation; independent runtime and identity states, resources and next action. Source and CLI contracts. | Implemented. |
| F2: scriptable equivalent CLI | Shared coordinator, JSON metadata, canonical grammar/errors, path `--`, secret FD validation, no unexpected noninteractive prompts. CLI contracts. | Implemented. |
| F3: ordinary lifecycle | Immutable VM/alias association; stop/restart preserve disk; delete requires confirmation, verifies disk/path absence and removes only unchanged owned alias. Tests and VM. | Implemented. |
| F3: interrupted setup reuses same VM | Frozen attempt, controller token/guard, bounded reconciliation, disk presence, artifact and actual native-input checks before/after provisioning. Tests and VM recovery. | Implemented. |
| F3: explicit project rebind | Old/target source and hashes; atomic overlap reservation; workspace-only native patch; exact mounted-source probe. Tests and VM. | Implemented. |
| F4: Ubuntu and optional presets | Ubuntu 24.04 base, separate development and Docker presets; Docker build/Compose, LXD containers and Juju bootstrap/deploy/cleanup workload checks. | Implemented; VM evidence. |
| F4: native YAML | Strict bounded YAML/local graph resolution through installed Lima; supported fields preserved, incompatible ones rejected; semantic creation comparison. Native CLI contracts. | Implemented. |
| F4: readable setup preview | Template, instance paths, workspace, mounts, ports, profile, package roots, Snap/Docker versions and ordered stages; wrapped scrollable approval. TUI tests/source. | Implemented. |
| F4: explicit environment updates only | Start uses frozen revisions; update previews then applies explicit differences; added presets install only new stages; no automatic tool upgrade. Stage tests and VM resource update. | Implemented. |
| F4: legacy setup history | Completed/incomplete/unknown receipt evidence; rebind/resource updates preserve history; unknown history needs explicit refresh. Receipt and stage tests; adoption VM. | Implemented. |
| F5: one explicit root for new/adopted VMs | Volume identity, no fallback/moves; adoption requires existing matching root. Source and VM. | Implemented. |
| F5: creation capacity | All image/temp/disk/state destinations grouped by shared capacity; durable pending claims and resume admission; future growth explicitly unreserved. Source/recovery tests. | Implemented. |
| F5: resource visibility | Root/cache/temp paths, allocated blocks versus logical maximum, free capacity and recorded running CPU/RAM totals with measurement time. Source/VM inspection. | Implemented. |
| F6: SSH/editor/guest commands | Stable alias, pinned host keys, installed proxy, `/workspace`, exact guest arguments/stdout/exit; editor dependency preflight and specific SSH fallback. OpenSSH contracts and VM SSH/exec. | Implemented; editor GUI not automated. |
| F6: deliberate mounts and ports | No ambient home/key exposure; exact loopback TCP/UDP mappings, durable cross-VM reservations and occupied endpoint checks. Tests and IPv4/IPv6 VM port scenarios. | Implemented. |
| F7: explicit reusable profiles | No host identity inference; generate/import, independent host agents, public key display, SSH commit/tag signing. Agent tests and shared-profile VM signing. | Implemented. |
| F7: optional remember and explicit lock | Reuse loaded/remembered keys without per-VM enrollment; lock inhibits reload, verifies unload and revokes forwarding. Agent/credential tests and VM reuse. | Implemented. |
| F7: one profile per VM; overlap rejection | Transactional create/rebind/adopt and projected batch profile membership; pending targets fence competing overlapping paths. SQLite regressions. | Implemented. |
| F7: selective changes and owned settings | Metadata-only rename; per-member Git fields; all-member key/agent changes; duplicate labels/user edits refused; returned key reflects committed revision. Source/identity tests. | Implemented. |
| F7: stopped members and VM-only starts | Ready pending identity finishes on explicit start/open; known unavailable identity is separate from VM-only success; identity connections stay gated. Tests and VM. | Implemented. |
| F7: adopted agents/credentials | External key/agent/store references preserved; explicit same-key migration leaves members stopped; external resources are not deleted. External-profile tests and disposable adopted-agent signing. | Implemented. |
| F7: signing versus account verification | Exact host key-set/signing challenge, guest effective Git/repository provenance and opt-in expected-account provider checks. Tests and VM local signing. | Implemented; real account enrollment remains user-owned. |
| F8: read-only partial diagnostics | Read-only SQLite snapshot/inventory fallback; independent storage/runtime/trust/identity/repository statuses; explicit unavailable guest checks. Tests and live durable-file comparison. | Implemented. |
| F8: operational logs and opt-in raw capture | Safe event/stage/result progress; completed exports bounded at 10 MiB/file and 100 MiB total; active evidence and SQLite history retained. Output/retention tests. | Implemented. |
| F8: reconcile before retry, explicit repairs | Durable attempts, claims/receipts, operation-bound trust publication, alias/trust/connection/identity repair; unknown effects retain recovery state. Recovery/owned-file tests. | Implemented; no arbitrary rollback. |
| F9: exclusive supported adoption | Pinned source, metadata backups, legacy owner retirement, final namespace/alias checks, active reused profile check, same disk/name/alias/root. Tests and VM adoption. | Implemented; exhaustive handoff matrix deferred. |
| F9: docs/schema/license/examples | Public getting-started/usage/profile/recovery guides, full TOML/native-YAML and generated command references, parsed examples, link checks, license/notices. | Implemented; release qualification deferred. |

## Technical contracts

| Contract | Trace and disposition |
| --- | --- |
| T1: Go | `cmd/easy-lima`, Go module and native build. Other signed release artifacts deferred. |
| T2: foreground/shared components | CLI/TUI issue typed requests to `internal/app`; VMs and agents survive command exit. Tests/VM. |
| T3: installed Lima boundary | `internal/host/lima`: selected executable, exact version, controlled environment, complete observations, bounded output and explicit mutations. Contracts/VM; extra drivers deferred. |
| T4: intent versus authority | TOML desired settings, SQLite associations/revisions/outcomes, diagnostic inventory never promoted to authority. Read-only/publication tests. |
| T5: config syntax/locations/precedence | Strict user/project/profile TOML plus native YAML, canonical declaring-directory paths, exact project lookup, creation/update precedence. Public references and parser tests. |
| T6: schema/WAL/inventory | Foreign keys, guarded opens, ordered publication, read-only snapshots, explicit checkpoint. Advanced restore exceptions below. |
| T7: driver/operations | ncruces SQLite, prepare/dispatch/reconcile/finish, tokens/claims, terminal recheck after executor acquisition. Fault/claims tests. |
| T8: agents/unlocking | launchd/detached agent abstractions, native credential stores, explicit inhibition, verified key unload/signing. Linux desktop integration qualification deferred. |
| T9: connections/Git | Separate VM login and Git agent credentials, pinned proxy, owned guest bundle, HTTPS rewrite and SSH signing. OpenSSH/guest tests plus VM. |
| T10: storage/revisions | Volume/capacity witnesses, bounded hashed local assets, frozen namespace inputs and immutable environments. Tests/source/VM. |
| T11: lifecycle/rebind/adoption | Same-instance recovery, disk guard, recoverable rebind/handoff and final retirement-boundary validation. Tests/VM. |
| T12: evidence/diagnostics/state | Typed metadata and partial independent checks; safe logs. Advanced restore finalization remains deferred. |
| T13: UI/API | Shared plans/requests, bounded lazy tree, readable approval, separate progress/stdout and raw exec contracts. Existing tested parser retained instead of changing libraries solely to match the suggested Cobra choice. |
| T14: release/dependencies | Exact dependency contract/local build retained; publishing, Homebrew, notarization and broader qualification deferred. |

## Normative supplements

| Supplement | Verified boundaries and limits |
| --- | --- |
| INTERFACE_V1 | Command/flag/TOML field contracts, selection exclusivity, preview/apply, secret FDs, pagination, result envelope, operation IDs, diagnostic statuses and raw exec status; parser/output fixtures. |
| PROFILE_TRANSITIONS | Frozen targets, admission/cutover fences, selective versus shared changes, same-target continuation, pending profile-free cleanup, active membership and removed-profile rejection. Tests/source and live stopped-member continuation. |
| ADOPTED_AGENTS | External capabilities/key/agent/store references, explicit same-key migration, no external resource deletion, stopped member preservation. Source/contracts and external-agent VM fixture. |
| LIMA_BOUNDARIES | Frozen namespace inputs and explicit drift acceptance; pinned export/enrollment recovery; exact agent verification before forwarding; loopback port reservations and observation. Tests/VM. |
| ADOPTION | Single root, pinned supported source, backup/retire/register ordering, stable disk/alias, preserved identity and receipt history. Full historical executable/driver-fault matrix deferred. |
| PROVISIONING | Frozen stage graph/receipts, publisher resolution, explicit composition, immutable revisions and conservative arbitrary-script replay. Richer substep/provenance qualification deferred. |
| STATE_CONTRACT | Guarded SQLite, head/event/namespace/owner witnesses, diagnostic inventory, stale restore refusal and retirement evidence. Exact advanced restore finalization deferred. |
| DECISIONS/concept/design draft | Go, foreground commands, external Lima, TOML/YAML, WAL/SQLite decisions implemented. Illustrative file/package layout is distinguished from required behavior. |

## Explicitly deferred or outside this completion claim

1. Release distribution: publishing, Homebrew channel, signing/notarization, hosted CI execution and release automation qualification.
2. Native host qualification beyond the recorded Mac run: Intel Mac, Linux QEMU/KVM, Linux desktop Secret Service/systemd agents and filesystem/driver combinations. Linux guest-userspace tests are not host acceptance.
3. Advanced restore: complete namespace-epoch finalization, readable pre-swap checkpoint and every interrupted verified-fence path in the exact contract. Current backup/admission/publication tests do not qualify those paths; `state restore` is outside everyday completion.
4. Exhaustive provisioning/adoption evidence: every crash boundary, richer substep/cgroup/snap-change provenance, executable historical-launcher positive/negative controls, all driver-specific quiescence evidence and exact archival representation. Ordinary recovery/adoption remain in scope and are exercised separately.
5. Actual GitHub registration/authentication using the user's work/private keys; fixtures use unregistered disposable identities. VS Code GUI rendering is not independently automated; launcher prerequisites and SSH alias behavior are checked.

Windows/WSL, multiple roots, automatic moves, generic third-party adoption, arbitrary rollback, extra editors and global scheduling remain later features as agreed.

## Evidence record

Ignored `test-results/` retains source manifests, check logs, terminal runs and VM workflow/adoption evidence. Final run outcomes are recorded in [testing](../testing.md). No user VM, registered private credential, Git push or release publication is part of this audit.

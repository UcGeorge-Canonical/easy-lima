Stage 9 Codebase Details Review found **4 Major findings, 0 Blockers, and 0 Nits**. These are implementation-design gaps; none treats a proposed file as missing current code or a qualification target as certified support.

References below use:

- **D:** `/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md`
- **P:** `/Volumes/Nebula/dev/Canonical/local/.local-launcher`

**S9-CODE-01 — Major: Adoption needs a legacy-work quiescence check before transferring ownership.**

Affected scope: D:1163, D:1184, D:1206 and D:1228; governing adoption contract at D:1022–1037. Source: `P/local.py:37–49` (`run`), `P/local.py:285–297` (`Manager.lock`), `P/local.py:378–389` (`Manager.provision`), and `P/provision.sh:97–99`.

The proposed handoff acquires `launcher.lock`, retires the old ownership record, and permits unknown provisioning history. The source supports the narrower claim that an existing instance without its record is rejected by `Manager.check_ownership`; that claim is accurate. It does not establish that previously dispatched guest work has stopped. Provisioning runs through a guest shell, the lock releases when the host workflow exits, and the prototype retains only a final provisioning digest—not a durable job identity, dispatch receipt, or cancellation result.

Consequently, “old record retired” and “provisioning history unknown” cannot establish that a surviving legacy provision command will not overlap subsequent easy-lima work. The adoption recovery table currently tracks ownership publication but supplies no admission predicate or retained claim for this case.

Actionable direction: Specify how adoption establishes that legacy work is quiescent, distinguishing historical unknown provenance from potentially active work. Define stopped-VM evidence or a positive running-work check, the refusal/resume behavior when evidence is unavailable, and the persisted evidence required before releasing handoff claims. Add an adoption scenario where the legacy controller exits after guest dispatch but its work remains active; retirement alone must not admit a conflicting mutation.

**S9-CODE-02 — Major: Preserved shared legacy agents cannot implement the stated profile-lock behavior.**

Affected scope: D:1164, D:1170–1172, D:1185, D:1204 and D:1228. Relevant contracts: D:828, D:872 and D:1037. Source: `P/git_identity.py:115–127` (`setup` and its launchd job), and `P/git-agent.py:20–21,30–48` (`main`).

The design requires a lock to unload the profile key and inhibit automatic reload across agent restarts. It also imports an agent shared with non-adopted projects as an external resource whose existing supervisor/socket must remain usable and must not be stopped, reconfigured, or exclusively controlled.

The actual legacy job has `RunAtLoad` and `KeepAlive` enabled. Its helper starts an agent and unconditionally loads the key through its existing askpass mechanism on each start. It does not consult easy-lima’s `profile_runtime` inhibition. Unloading its shared key disrupts remaining prototype users; preserving its restart behavior can reload a supposedly locked key.

Actionable direction: Define explicit capabilities and command behavior for an externally managed profile. Specify whether locking is unavailable until a separately approved ownership migration, or how an independent managed binding will provide the lock guarantee while preserving legacy users. Any user-visible exception requires reconciliation with the approved lock experience. Test partial adoption, lock attempts, supervisor restart, and continued operation of a non-adopted project.

**S9-CODE-03 — Major: The development/Docker split omits an existing networking dependency and its acceptance coverage.**

Affected scope: D:1179, D:1189 and D:1205; preset specification at D:983. Source: `P/provision.sh:21–23,43–79` and `P/verify.sh:8–19`.

The prototype does more than install Docker and LXD: it installs a service ordered after Docker that changes forwarding policy, then configures LXD networking and both groups. Its verification script checks Docker while exercising a real LXD container. The new worklist separates `development.sh` and `docker.sh`, but does not select their composition/order, the replacement networking policy, or handling after Docker restarts. The explicit preset release gate lists Snap/LXD/Juju without Docker or the combined selection.

This is a concrete reuse dependency: Docker’s forwarding policy can prevent LXD traffic, and the appropriate remedy depends on the selected Docker version/backend. Official [Docker networking documentation](https://docs.docker.com/engine/network/packet-filtering-firewalls/#docker-on-a-router) and [LXD firewall guidance](https://canonical.com/lxd/docs/latest/howto/network_bridge_firewalld/) describe the interaction; neither establishes compatibility of this proposed application.

Actionable direction: Specify Docker’s installation source/version policy, firewall backend, preset ordering, owned networking changes, and restart postconditions. Explicitly retain or replace the prototype workaround rather than copying its global policy change without review. Add Docker-only and development-plus-Docker acceptance cases, including a Docker restart followed by LXD networking and Juju workload checks.

**S9-CODE-04 — Major: The legacy ownership contract test lacks a specified executable fixture and harness.**

Affected scope: D:1184, D:1187–1189 and D:1206; the required proof appears at D:1024. Source: `P/tests/test_local.py:16–25,108–122,138–149`; `P/tests/test_work_git.py:18–27,87–114`.

The plan requires a fixture to prove that retirement makes the old helper’s start/stop/delete paths refuse the adopted instance. The manifest provides only `testdata/prototype/manifest.json` for prototype evidence plus generic fakes/faults. It does not specify how the actual legacy behavior will be executed, which source revision identifies the supported importer, or how legacy subprocess dependencies will be isolated.

The existing infrastructure creates temporary roots and patches `instances`/`run`; its ownership test directly invokes the Python manager. It is useful evidence, but it is not a portable executable contract fixture in the new repository. D:134 also correctly says prototype source references are not runtime dependencies. A Go fake that merely reproduces the intended refusal would not independently prove the legacy handoff assumption.

Actionable direction: Define the legacy contract harness before implementation: exact provenance/version identification, fixture files or an explicitly supplied fixture source, test-only interpreter requirements if applicable, fake Lima/process protocol, isolated configuration/home/state paths, and permitted effects. Add exact manifest paths for any fixture/helper code. Require execution of the supported legacy refusal paths against retired metadata, with assertions that no VM mutation was dispatched, rather than only comparing an expected JSON result.

Read acknowledgment: I read the complete `codebase_details.md` persona, complete implementation-readiness standard, all 1,239 target lines, and all frozen `FEATURES.md`, `concepts_brief.md`, `UX_PROPOSAL.md`, and `goldfish_review_report.md` lines. The latter was treated solely as historical concept-review evidence.

Actual checks: Read all eight permitted prototype source/scripts and both test files, focusing on their setup, mocks, subprocess boundaries, ownership checks, provisioning evidence, agent supervision, and guest identity configuration. Checked primary OpenSSH/Git documentation for forwarding/signing semantics and primary Docker/LXD documentation for the networking interaction.

Limitations: Static review only. No files were changed; no builds, tests, scripts, VM operations, or credential interactions were executed. No live registry/VM state, keys, credential stores, host SSH configuration, unrelated repositories, conversation/history/memory, or other new reviewer reports were inspected. No subagents were used.

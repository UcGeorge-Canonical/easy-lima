# Delegated internal design corrections

Selected on 2026-11-19 under the author's standing delegation for internal choices. These are design specifications, not executed tests. ELD-11/ELD-12 remain proposals for author review.

## ELD-09

[PROVISIONING.md](../../../../../PROVISIONING.md) defines official exact-version Docker packages, the iptables backend, owned pre-start gating and forwarding configuration, deterministic LXD/Docker address selection, per-tool holds, and Docker-only/combined restart/reboot workload gates. No global FORWARD-ACCEPT workaround is installed.

## ELD-10

[ADOPTION.md](../../../../../ADOPTION.md) defines a digest-pinned unaltered legacy source fixture, Python 3.11+ test-only driver, bounded fake Lima/process protocol, positive controls and zero-mutation refusal cases. All source/manifest/scenario/harness paths are in the implementation manifest; no fixture has been executed.

## ELD-13

[STATE_CONTRACT.md](../../../../../STATE_CONTRACT.md) defines complete added records/constraints, canonical marker payloads, critical generations, local then namespace publication, deterministic interruption predicates and exact-head backup/restore admission. Missing VM storage does not block unrelated local-only profile administration; inventory cannot authorize mutation.

## ELD-14

[PROVISIONING.md](../../../../../PROVISIONING.md) defines named stage prerequisites/effects, frozen package inputs, durable bounded receipts, a controlled transient-service runner, and stage-specific continuation or refusal. Native one-time scripts keep frozen semantics; interrupted arbitrary scripts never receive automatic replay/rollback.

## ELD-15

The main [design](../../../../../design_draft.md) now retains architecture, component ownership, selected invariants and the file/test plan. [TECHNICAL_DETAILS.md](../../../../../TECHNICAL_DETAILS.md) contains detailed contracts, focused supplements contain exact protocols, and [DECISIONS.md](../../../../../DECISIONS.md) centralizes alternatives. Frozen review inputs remain unchanged.

All new implementation paths and acceptance cases are listed in [the implementation plan](../../../../../design_draft.md#file-manifest). Independent re-review and runtime qualification remain pending.

# Legacy fixture correction verification

## First focused check — residual

**Residual — R2-CODE-01 is not fully closed.**

The correction covers module-level `ROOT`, the synthetic script, real digest calculation, provisioning responses, and complete positive controls. The stated script hash is correct for the exact bytes `#!/bin/sh\nexit 97\n`. `ADOPTION.md` lines 65–72 and `design_draft.md` line 321.

One fixture response contradicts the pinned source: ADOPTION.md line 72 specifies fake `show-ssh` output, but the helper never calls `show-ssh`. Its real `update_ssh` checks for `<lima_home>/<record.name>/ssh.config` on disk and adds the include only when that file exists. Without an explicitly populated synthetic file, that branch silently skips the alias; fake successful SSH probes could then let the positive control pass without exercising the intended publication. Prototype `local.py` lines 317–333 and 374–375.

Replace the fake `show-ssh` instruction with explicit creation of a bounded synthetic `ssh.config` at that sandbox instance path, and require assertions that the actual generated include references it while preserving unrelated entries. Keep successful fake control-exit/workspace probes.

The other requested checks agree with prototype `local.py` lines 378–393 and 430–479, including the already-stopped `stop` no-op.

Only documentation/source inspection and static-byte hashing were performed. No prototype import, execution, tests, writes, or live-resource access occurred.

## Follow-up after that correction — pass

**Pass — the R2-CODE-01 residual is closed at the documentation level.**

Current `ADOPTION.md` line 72 now supplies the exact sandbox instance `ssh.config`, excludes `show-ssh`, and requires assertions for both generated includes and preservation of unrelated entries. This matches prototype `local.py` lines 317–333 and 220–233.

No remaining mismatch in this scoped correction. Source inspection only; no execution, tests, writes, or runtime-validation claim.

These are focused checks by the original reviewer, not new fresh-context review rounds. The unsuccessful first check is retained rather than represented as a clean first pass.

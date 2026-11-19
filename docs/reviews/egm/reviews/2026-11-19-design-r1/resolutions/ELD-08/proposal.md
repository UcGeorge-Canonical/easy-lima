# ELD-08: confirm the interruption required for prototype adoption

**Status:** Proposed, awaiting the author's decision on the stop/restoration experience. This is not an applied resolution. Severity remains Major. No VM operation has been performed.

## Concrete proposal

Require verified stopped-VM evidence before transferring ownership from the supported prototype. If the VM is running, the adoption preview includes a graceful stop, disconnection of its active SSH/editor sessions, and restoration of its previous running state after a successful handoff when the recorded environment can safely start. A VM already stopped stays stopped. No automatic force-stop escalation and no stop merely to inspect or preview adoption.

The user can decline the interruption and leave the VM with the prototype. Noninteractive adoption must explicitly accept the plan under the existing `--yes` rules. This requirement is specific to the supported prototype's missing active-job evidence, not a general claim that every importer must stop a VM.

The stop is needed because a free host lock and a completed-provisioning digest cannot prove that a previously dispatched guest job has exited. A terminated or timed-out legacy controller can leave guest work behind. The review's [source evidence](../../raw/stage9-codebase.md) identifies the legacy dispatch and lock paths. Historical provisioning provenance can remain unknown after adoption, but potentially active work must be resolved before ownership cutover.

## Proposed execution and recovery contract

1. Preview the exact instance/source/disk/alias, current running state, preserved identity, stop/restoration effects, backups and ownership transition. Validate storage, namespace inputs, supported native hooks/forwarding and profile preservation before requesting the interruption.
2. Under the prototype's `launcher.lock` and easy-lima's durable namespace/registration/handoff claims, recheck those inputs and persist the handoff intent. The single preview-authorized pre-transfer VM effect is graceful stop of this proven legacy-owned target. No provisioning, config repair, start or unrelated operation is admitted under that exception.
3. If running, request that stop and observe the same instance fully stopped using the qualified driver/host-agent evidence, not just command exit or a missing PID file. Retain namespace/instance identity, prior boot/runtime generation when available, stop dispatch/outcome, observation source and time. Ambiguous or still-active host control work prevents proceeding; do not treat a single stopped observation as proof against an unresolved controller that could restart it.
4. Once stopped/quiescent is established and rechecked, back up/retire the old ownership record, publish the new ownership and alias/identity metadata using the shared handoff ID, and verify exclusive ownership. Preserve the external profile agent and non-adopted VMs. Holding the legacy lock prevents a new cooperating legacy workflow during the transfer; it does not manufacture evidence about earlier dispatched work.
5. Release the legacy lock only after its active ownership record is retired and the new ownership evidence is consistent. If the accepted plan restores a previously running VM, start the same recorded instance after cutover through the existing guarded-start protocol. Reuse its retained configuration; do not apply a new preset or resume an unknown legacy script. Unsupported or unknown automatic startup hooks prevent restoration and produce a specific pending/blocked result requiring diagnosis.
6. Report ownership transfer and running/connection/identity readiness separately. Handoff can be complete while restoration is pending. A failed restoration never restores the prototype ownership record or allocates a replacement VM.

| Interruption | Required next action |
| --- | --- |
| Stop intent exists but stop outcome is unknown | Retain handoff claims and old ownership. Inspect/reconcile the same instance; no retirement or duplicate stop until evidence permits it. |
| VM confirmed stopped; ownership still legacy | Resume the existing handoff after rechecking quiescence and hashes. Cancellation may leave it stopped; report that effect and do not silently restart under another owner. |
| Legacy record retired; new publication incomplete | Finish the recorded transfer under the existing fenced recovery rules. Neither manager receives general mutation authority from incomplete evidence. |
| New owner committed; restoration unfinished | Reconcile/resume restoration against the same VM and frozen inputs. The old manager remains retired. |
| A controller, owner, native hook or runtime state remains unknown | Keep the affected operation blocked/unknown with inspect/resume guidance. No timeout-based claim release, force-stop or automatic rollback. |

The internal stopped-state adapter evidence and executable legacy fixture are completed alongside ELD-10/ELD-13. This proposal does not count those contracts as resolved. Unsupported evidence leads to refusal, not a best-effort handoff.

## Required acceptance scenarios

Terminate the old host controller after guest dispatch while a recognizable guest job survives. Acquiring the old lock and retiring metadata alone must not permit handoff. Accept the previewed graceful stop and prove the old guest job is gone before publication; exercise unavailable stop evidence and a still-active host controller. Repeat for an initially stopped VM, declined interruption, stop timeout, crash at every handoff boundary, and failed restoration. Verify stable disk/alias/key, no preset upgrade and no effect on another legacy VM or shared profile agent.

**Decision requested:** Adopt the previewed graceful-stop requirement, restoring an originally running VM after a verified handoff when its recorded startup is supported.

# ELD-08 accepted adoption policy

George Uche-Umeh accepted the stop/restoration proposal on 2026-11-19 with **"Yep"**.

Adoption previews a graceful stop of a running legacy VM, proves the same VM and legacy work are stopped, and restores its prior running state only after verified handoff and supported recorded startup. Already-stopped VMs stay stopped; unknown state blocks cutover and no force escalation is automatic.

The applied [adoption contract](../../../../../ADOPTION.md) defines quiescence, the complete handoff and every interruption boundary. [The state contract](../../../../../STATE_CONTRACT.md) defines ownership publication. This approval concerns the design; no VM was stopped or changed.

The original proposal, pre-change snapshots and earlier review evidence are preserved. Independent re-review and executable qualification remain pending. ELD-11/ELD-12 remain open for review of the [public interface proposal](../../../../../INTERFACE_V1.md).

# ELD-01: accepted profile-wide key changes

George Uche-Umeh answered “Yes” on 2026-11-19 to requiring an SSH-key change to include every VM using the profile, with a separate profile when only some VMs need a different key.

The decision is applied in [PROFILE_TRANSITIONS.md](../../../../../PROFILE_TRANSITIONS.md) and the working design, UX, feature scope and summaries. The protocol records all member targets, revokes old forwarding before changing the shared key, retains stopped VMs as pending without booting them, and preserves per-VM completion/recovery evidence. These internal mechanisms follow the standing delegation and existing accepted policies.

ELD-02 external-agent administration is not approved by this reply and remains open. The broader state and public-interface specifications also remain open. This is a substantive design correction awaiting independent re-review; it is not implementation or runtime validation. Original review snapshots and raw reports are unchanged.

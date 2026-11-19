# Goldfish annotated re-review

**Status:** Resolved at design level. All four corrections are applied and their scoped reviewer checks are retained.

All four reviewer reports are complete. Five raw Major findings consolidate to four corrections, applied under the standing delegation; focused correction verification is complete, with no open findings. No accepted public behavior changes.

## ELR-01 — Major — PROPOSE FIX

R2-DESIGN-01 and R2-READY-01 identify mutable membership foreign keys and no representation of deferred profile removal. Separate verified old identity from pending target membership, retain historical connections against immutable registration/profile references, and specify ordered assign/unassign with no implicit starts. Add explicit profile-free cleanup and interruption scenarios.

## ELR-02 — Major — PROPOSE FIX

R2-DESIGN-02 identifies legitimate finalization writes rejected by original candidate file hashes. Journal a fixed restore event and logical before/after generations before writable finalization; recover that event and publication exactly once while the swap fence remains. Keep exact physical-file checks for pre-write installation and quarantine.

## ELR-03 — Major — PROPOSE FIX

R2-CODE-01 identifies the unbound module ROOT and missing script fixture. Specify an explicit sandbox ROOT binding, fixed synthetic script bytes and exact fake digest/provision responses. Preserve actual ownership/provision logic; test the already-stopped stop no-op separately.

## ELR-04 — Major — PROPOSE FIX

R2-STYLE-01 identifies contradictory pending-approval language after explicit approval. Synchronize the architecture, exact interface, technical references and file manifest with the already-granted decision. No renewed UI decision is necessary.

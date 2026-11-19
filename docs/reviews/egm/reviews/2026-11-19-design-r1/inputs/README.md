# easy-lima

A standalone application for project-oriented Ubuntu VMs, built on Lima.

The intended interface is an installable `easy-lima` binary. Run it from any
directory to browse that directory and its children, select a project, and manage
its VM. An explicit path can select another starting directory. VM registrations,
storage settings, and identity profiles belong to the user and persist across
working directories.

## Project status

V1 scope is approved. The six-lens EGM concept review is closed: all 11 findings
are accepted and resolved. The opening design sections are approved, and Go is
selected for the standalone binary. V1 operations run in foreground commands,
while running VMs and profile SSH agents remain available after command exit.
V1 uses the installed `limactl` CLI as an external dependency, with explicit
compatibility checks. SQLite holds authoritative managed state; a separate
recovery inventory supports read-only diagnostics and cannot authorize changes.
TOML is selected for easy-lima configuration, with native Lima YAML and approved
location/precedence rules. SQLite uses WAL with explicit checkpointing;
diagnostics remain read-only. The internal design selects `ncruces/go-sqlite3`
through `database/sql` and a recorded operation/reconciliation model. Profiles
support optional remembering and automatic reuse: a new VM uses its existing
profile for Git authentication and signing without another credential-setup step
when the loaded key or remembered credential is available.

The complete implementation-design draft is ready for review. It specifies
connections and guest Git configuration, storage and cache placement, recovery
and adoption, schema maintenance, dependencies, and the implementation/test plan.
The remaining commands, interactions, defaults, and installation choices are
collected in the v1 experience document, approved on 2026-11-19. The fresh
implementation-design Goldfish review is in progress; runtime qualification
remains pending.
This repository contains design documents and review records; the binary has
not been implemented.

- [Feature scope](FEATURES.md)
- [Concept brief and accepted decisions](concepts_brief.md)
- [Resolved Goldfish review](goldfish_review_report.md)
- [Implementation design draft](design_draft.md)
- [Approved v1 experience](UX_PROPOSAL.md)

The existing `make local` helper is a separate prototype and a source of workflow
experience. easy-lima does not require that Makefile, a source checkout, or a
particular workspace layout. It will have no `make local` compatibility entry
points. Lima remains a runtime prerequisite.

## Confirmed v1 rules

Each VM has at most one selected Git identity profile; using no profile remains
supported. Registered parent/child projects with different profiles cannot have
overlapping source trees. Descendant registrations do not switch a parent VM's
identity.

All new and adopted VMs share one configured VM storage root, independent of the
working directory. Adoption preserves disks in place and stops if their location
conflicts with that root. There are no automatic moves or additional-location
exceptions.

Creation checks cover downloads, temporary files, cache, and VM allocation,
combining requirements where destinations share capacity. Known insufficient
space stops creation before download or allocation. Current usage and virtual
disk maximum are shown separately; future VM growth remains unreserved.

Operational logs are enabled by default with known secrets excluded. Saving raw
command or provisioning output through easy-lima requires explicit opt-in;
captured files are user-only and are not automatically shared.

Updating easy-lima or starting a VM does not automatically upgrade its tools or
presets. An explicit environment-update action previews proposed changes before
applying them.

Moving or renaming a project uses explicit rebinding, preserving its VM, disk,
and SSH alias. Conflicts are rejected, and interrupted rebinding is recoverable.

Adoption transfers exclusive management to easy-lima with metadata backups and
a recoverable handoff. Adoption or downgrades that would create conflicting
owners are rejected.

Profile administration is independent of VM startup. Adoption preserves the
complete usable identity setup; profile changes affect only easy-lima-owned
settings and report conflicts with user edits. Remembering is an explicit choice
per profile. VM creation/connection automatically activates that existing profile
when its saved credential is usable. Necessary OS unlocking stays within the
interactive flow, explicit profile locks remain effective, and noninteractive
commands never display hidden credential dialogs.

Retries reconcile the previous attempt and reuse its VM. If its state remains
unknown, easy-lima stops with recovery instructions. Automatic rollback of
arbitrary guest scripts is outside v1.

Diagnostics are read-only and return useful partial results when dependencies
are unavailable. Each check has a separate status; unavailable GitHub verification
remains unverified. Repairs are separate, explicit actions.

Implementation design starts with selective reuse of suitable prototype behavior
and code, choosing fresh implementation where it offers a clear advantage.
Distribution, portability, maintainability, and testing guide that comparison.

The host application will be written in Go, reusing suitable scripts and test
scenarios. Commands share the same operation logic and run in the foreground;
VMs and profile SSH agents have independent lifetimes. The installed `limactl`
CLI is the selected Lima integration, and SQLite is the selected state backend.
TOML, location/precedence rules, and WAL with explicit checkpointing are selected.
The driver, schema, recovery protocols, dependencies, and release mechanics are
selected in the completed draft. The implementation plan defines the checks
required before advertised support; no listed host/Lima combination is certified
by the design alone. Internal choices follow the author's delegation. The
consolidated public experience was approved on 2026-11-19.

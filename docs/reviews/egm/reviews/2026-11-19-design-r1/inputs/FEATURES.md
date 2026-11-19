# easy-lima: v1 scope and later features

- **Author:** George Uche-Umeh
- **Updated:** 2026-11-19
- **Status:** The author approved the v1 scope on 2026-11-18. EGM Goldfish review round 1 is complete: all six reviewers completed, with 0 Blockers, 10 Major findings, and 1 Nit after deduplication. The author has accepted GF-01, GF-02, GF-03, GF-04, GF-05, GF-06, GF-07, GF-08, GF-09, GF-10, and GF-11 and clarified that easy-lima is a standalone, installable binary. The [annotated review](goldfish_review_report.md) is resolved: all 11 findings are accepted and applied to the specification and concept brief. The complete [implementation design draft](design_draft.md) is ready for review; the [consolidated public experience](UX_PROPOSAL.md) was approved on 2026-11-19. Runtime qualification remains pending; the fresh implementation-design review is in progress.

## Product boundary

easy-lima is a standalone, installable binary that helps a developer select a project folder, start its Lima VM, and work through SSH or VS Code. Running `easy-lima` in any working directory opens a project browser rooted there; an explicit path can select another starting directory. It adds deliberate storage choices and separate Git identities around Lima's existing capabilities.

The application does not require a Makefile, its own source checkout, a folder named `local`, or a prescribed project layout. Registrations and user settings persist across working directories. The existing `make local` helper is a separate prototype; easy-lima has no compatibility entry points for it. Its existing behavior can inform the application without determining the public interface.

Lima remains responsible for VM creation, drivers, guest images, provisioning mechanisms, mounts, networking, and connection details. Use ordinary [Lima templates](https://lima-vm.io/docs/templates/) and Lima YAML for VM settings. easy-lima should store only the additional project associations, user preferences, and identity settings it needs.

The instance name and configuration path must remain visible, and ordinary `limactl` inspection must work. Unrelated Lima instances are outside easy-lima's management unless explicitly adopted. There is no easy-lima online account requirement.

## V1 workflow

1. Install the binary, run it from any working directory, and check host/Lima prerequisites.
2. Use the current directory or choose another browsing root. Inspect the actual VM disk and image-cache destinations before any download or VM creation.
3. Optionally create/import a named Git identity and assign it to a project.
4. Select any folder in the TUI, or pass a path through the CLI. Inspect the template, mounts, resources, and identity before first creation.
5. Start one persistent VM for that folder; open SSH or VS Code at `/workspace`.
6. Stop and resume the same environment later. Diagnose failures with logs and `doctor`. Explicitly delete the VM when its guest data is no longer needed.

A project need not be a Git repository. Selecting a grouping folder creates one VM for that folder; it does not recursively create VMs for its children.

## V1 requirements

### F1. Installation and first-run setup

Include an installable `easy-lima` binary, a documented installation path for every supported host, dependency/version checks, browsing-root selection, storage selection, and optional identity setup. The current working directory is the default browsing root; an explicit path overrides it. Application configuration and registrations are independent of the current directory. Explain required host changes before making them. Repeating setup must preserve configuration and key fingerprints. Uninstalling the application preserves VMs by default.

**Done when:** A new user can install and run the binary from an arbitrary directory without a source checkout, Makefile, or the original author's paths. Starting it elsewhere changes the browsing root without losing existing registrations or settings. An explicit project path works from another directory. Missing dependencies and unsupported versions fail with useful instructions before VM allocation.

### F2. Project tree and scriptable CLI

Include keyboard navigation, expand/collapse at any depth, hidden/dependency-folder toggles, VM status, a details pane, and filtering of the loaded tree and registered projects. Read directories lazily. Provide direct path-based commands for the same operations, useful exit codes, and structured VM listings for scripts.

**Done when:** Repositories, subfolders, and grouping folders are selectable; paths with spaces work; symlinks cannot cause traversal loops; browsing does not execute provisioning. Noninteractive commands never unexpectedly open a picker or hang on an unanswered prompt. Full-drive indexing, favorites, and recent-project features can follow later.

### F3. Lifecycle and stable project associations

Include create/start, stop, restart, list, inspect, logs, and confirmed deletion. Re-running start reuses the same instance. Maintain a stable project-to-VM association and an explicit operation to rebind a moved folder. Lock conflicting operations and retain enough state to resume or diagnose incomplete setup. Reconcile the previous attempt before retrying, reuse its recorded VM, and stop with recovery instructions if its state remains unknown.

**Accepted registration and rebind boundary (GF-04):** A persistent registration identifies the actual Lima instance and SSH alias independently of the project path, current working directory, and application installation location. CLI and TUI operations use that registration. Moving or renaming a project uses an explicit rebind workflow that updates its source association while preserving the VM, guest disk, and SSH alias. Reject conflicting registrations and identity overlaps before changing the association. Interrupted rebinding retains ownership and enough information about the old/new source and mount state to diagnose and recover the same instance without creating a duplicate. Adoption preserves original instance names and aliases as registered bindings.

**Done when:** Rebinding a moved project, including an adopted one, preserves its VM, disk, and SSH alias. Conflicting rebinds leave existing associations intact. Interrupted rebinding is diagnosable and recoverable without a duplicate instance or loss of ownership. Interrupted startup neither reports success nor loses ownership of a partial instance. Guest files survive stop/start. Deletion identifies guest-only data that will be lost, preserves the host checkout, and cannot target an unowned VM.

**V1 limit:** One managed VM per registered project. Multiple environments for the same project are later work.

### F4. Native Lima configuration and limited presets

Include one documented Ubuntu LTS base template, resource choices, and user-provided Lima YAML. Add a small optional Ubuntu development preset supporting the existing Snap/LXD/Juju workflow. Docker and language-specific extras are explicit choices. Show what provisioning will run. Use pinned image/template revisions where supported and record the resolved inputs. Distinguish creation-time settings from changes that require a restart or recreation; ordinary startup must not silently upgrade the environment.

**Accepted environment-update boundary (GF-03):** Updating easy-lima or starting a VM never automatically upgrades its tools or presets. Each VM retains its resolved template/creation inputs and applied provisioning revision independently of the installed binary. Interrupted setup resumes its recorded operation instead of substituting current defaults. Applying newer tools or presets requires an explicit environment-update action with a preview before changes run. The preview identifies requested changes and known restart or recreation requirements. Adoption preserves completed legacy setup and distinguishes it from incomplete or unknown setup; unknown provisioning history requires diagnosis instead of automatically applying the latest preset.

**Done when:** The base and advertised development preset pass their workload checks on supported hosts. Repeated starts skip unchanged completed setup. Updating the binary and ordinary startup cause no unsolicited tool or preset upgrade. An environment-update preview makes no guest changes; application requires an explicit user action. Interrupted setup uses its recorded inputs. Unsupported configuration changes give a precise explanation rather than resetting a disk. Custom Lima settings are preserved or explicitly rejected; they are never silently discarded.

**V1 limit:** Template selection and maintained provisioning snippets using Lima's facilities. No SDK registry, package manager, or new provisioning language. Custom templates do not inherit the support guarantee of the shipped Ubuntu preset. The design proposes preset versions for qualification; those versions still require testing, and fully locked package builds are outside v1.

### F5. Storage choices and resource visibility

Include selection of the managed VM storage root, including an external volume; separate visibility of image-cache placement; mount, permission, path, and capacity checks; disk maximum versus allocated usage; and configured CPU/RAM totals for running managed VMs. Refuse an unavailable selected volume instead of falling back to another drive.

**Done when:** Users can locate large disk/image files from the CLI. A missing external drive causes no fallback allocation or download. Sparse capacity is not presented as already-used space. Known insufficient-space conditions fail before download or allocation across all creation destinations, accounting for shared capacity. Future VM growth is explicitly unreserved. Ordinary setup does not silently relocate another tool's Lima state or shared cache.

**Accepted storage boundary (GF-02):** All new and adopted VMs use one configured managed VM storage root, independent of the current working directory. Adoption keeps disks in place and requires their existing storage root to match the configured root. During initial setup, before other VMs are managed, the user may explicitly select the existing location as that root. Otherwise, a conflicting adoption stops before ownership or metadata changes and shows both paths. It cannot silently switch roots, orphan existing registrations, or add an exception for another location. No automatic moves.

**Accepted creation-capacity boundary (GF-09):** Check conservative creation requirements for each shipped preset against the actual filesystems used for downloads, temporary files, cache, and VM allocation. Account for destinations sharing the same free-space budget by evaluating their combined requirements. Reject known insufficient-space conditions before download or allocation. Report current allocation and logical disk maximum separately, and explicitly identify future VM growth as unreserved. Passing the creation check does not reserve capacity for arbitrary later workloads.

**V1 limit:** One managed VM storage root, including adopted VMs. No global resource scheduler, automatic storage mover, per-VM storage pools, disk shrinking, or live migration. Image-cache placement remains separately visible and does not create an exception for VM storage.

**Design issue to resolve:** Lima documents [`LIMA_HOME`](https://lima-vm.io/docs/config/environment-variables/) for instance state, but its environment-variable reference does not document `LIMA_CACHE_HOME`. Cache placement must be verified against the supported Lima versions and platforms. The prototype's global cache symlink must not become an installer action that silently affects other users of Lima. If moving a shared cache is necessary, expose its scope and require an explicit user choice.

### F6. SSH, VS Code, shared files, and explicit ports

Include a stable SSH alias, interactive shell, VS Code launcher, noninteractive guest command execution, the selected folder mounted at `/workspace`, and explicit localhost port tunnels. Preserve existing SSH entries when adding the managed configuration. Additional host paths must be deliberate; do not implicitly mount the host home.

**Done when:** SSH/editor connections work after a restart, source changes are visible on both sides, and two VMs can use the same guest application port without taking each other's host port. Guest commands return their exit status. Connection and host-key handling are tested against the supported Lima release. The user can see exactly which paths and ports are exposed.

**V1 limit:** VS Code is the first automated editor integration. Other SSH-capable tools can use the alias. Guest-native checkout/synchronization modes, extra editor launchers, and advanced networking follow later.

### F7. Named Git identity profiles

Include explicitly configured name, email, key, and signing settings for profiles such as `work`, `personal`, and `client`. Import a supported OpenSSH key or generate one once per profile. Reuse it across that profile's projects. Forward only the selected identity through a dedicated agent and keep private keys on the host. Support SSH commit/tag signing, public-key display, and a GitHub authentication check against an explicitly configured expected account.

**Accepted identity boundary (GF-01):** A VM uses at most one selected profile throughout its mounted project tree; Git integration remains optional. Descendant registrations do not switch that VM's identity. Reject known parent/child source overlaps between registered projects with different profiles, rather than allowing an acknowledgment to override the conflict. Apply the rule during creation, adoption, rebinding, and profile changes, comparing resolved source paths. Show the conflicting projects and profiles before changing their assignments. Identity checks must state which repositories they examined and must not imply that unexamined descendants were validated.

**Accepted profile administration and migration boundary (GF-06):** Profile creation, import, repair, and agent administration are independent of VM startup and do not automatically start or provision VMs. Scope each profile’s agent and credential handling independently; an unavailable profile must not disrupt unrelated profiles or profile-free projects. Adoption preserves the complete usable identity setup: the existing key and fingerprint, its unlock mechanism or reference, agent binding, managed SSH settings, and managed guest Git identity and signing settings. Private keys remain on the host. Profile replacement or removal changes only settings owned by easy-lima, preserves user-owned settings, and reports conflicts with user edits instead of overwriting them. Apply changes only to explicitly affected projects; the existing stop/reconnect allowance remains sufficient for profile changes.

**Accepted profile reuse experience (T8):** Offer optional per-profile remembering through the supported OS credential store, off until explicitly enabled. Creating a VM with an established profile automatically reuses its loaded key or starts the owned agent and restores that same key from an accessible remembered credential. Configure guest Git authentication, identity, and commit/tag signing before reporting the environment ready. No per-VM key creation, repeat key registration, repeated setup prompt, or separate unlock command is required on the normal path. Necessary OS consent/unlocking stays within the interactive creation/connection flow; explicit profile locks are respected, and noninteractive commands never trigger hidden dialogs. Remote account verification remains a separate result when the provider is unreachable.

**Done when:** Work and personal projects offer different selected keys and use the intended author/committer metadata. Registering a work-profile parent and personal-profile child is rejected in either order, and rebinding or changing profiles cannot introduce that conflict. An unrelated agent is never a fallback. Missing keys, a wrong account, or conflicting repository identity overrides fail clearly. Setup reruns preserve the fingerprint, and the host's global Git identity remains unchanged. Creating or repairing a profile does not start or provision VMs or reconfigure unrelated projects. Adoption retains usable authentication and commit/tag signing with the existing key. Profile replacement/removal preserves user-owned settings and reports conflicts with user edits; an interrupted identity transition fails clearly without using an unrelated profile.

**V1 limits:** Git integration is optional; a public project can start without GitHub setup. No name, email, or credential is silently inferred from the host. A profile change may require stopping the VM and reconnecting; live switching is unnecessary. Automatic account registration, other providers' account checks, hardware-token integrations, team credential distribution, and rotation management come later.

**Trust model:** A forwarded agent gives the connected guest use of that identity even though the key stays on the host. Writable source mounts allow guest changes to those files. V1 supports trusted development work and must describe these boundaries accurately.

### F8. Diagnostics and recovery

Include `doctor`, operational logs enabled by default, actual instance/configuration/storage paths, and checks for storage, Lima compatibility, SSH, selected identity, and failed provisioning. Report which step failed and an appropriate next action. Cancellation and retries retain recoverable state.

**Accepted logging boundary (GF-10):** Operational logs record steps, results, and useful failure details by default, excluding known secrets and unnecessary credential-bearing arguments or environment values. Raw command and provisioning output is displayed normally, but easy-lima saves it only when the user explicitly enables capture. Captured files are accessible only to the current user and are not automatically included in shareable diagnostics. Raw output may contain sensitive values; there is no guarantee of universal sanitization. Identify separately any output already retained by Lima or the guest and treat it as potentially sensitive. Access and retention mechanisms remain implementation-design work.

**Accepted retry and recovery boundary (GF-07):** Persist each operation attempt’s instance association, intended inputs or configuration references, last confirmed completed stage, outcome, and relevant log locations. Keep this operational evidence available without opt-in raw capture and exclude known secrets. A timeout, disconnection, or unobserved result leaves the outcome unknown until reconciled. Before retrying, determine whether the previous attempt is still active or has completed, and reuse its recorded VM and inputs. Do not overlap an active attempt or allocate a replacement VM merely because observation failed. If the prior state remains unknown, stop with recovery instructions instead of retrying. Automatic rollback of arbitrary guest scripts is outside v1.

**Accepted diagnostic boundary (GF-08):** Inspection and doctor are read-only and return useful partial results when prerequisites are unavailable. Report readable local registrations, expected paths, and log locations even when Lima, storage, SSH, or a profile cannot be reached, distinguishing recorded information from observed runtime state. One malformed record must not hide other environments. Give each check its own result, distinguishing success, failure, unavailable, skipped, and unknown as appropriate; inaccessible or unknown state must not be reported as not created. Separate VM readiness, profile/host-agent readiness, guest forwarding, local signing, external transport/authentication, and expected-account verification. GitHub unavailability leaves account verification unverified, not mismatched or successful. Diagnosis must not implicitly start VMs, provision environments, or repair settings; repairs are separate, explicit actions. Partial diagnostic results continue to exclude known secrets and do not enable raw output capture.

**Done when:** A missing drive, occupied port, unavailable key, interrupted provision, and failed SSH connection are distinguishable without reading source. Read-only diagnosis still returns useful local information and separate check results when dependencies or individual records are unavailable. Repairs require a separate explicit action. A retry reconciles the previous attempt and reuses its recorded VM. Completed, incomplete, active, and unknown outcomes are distinguishable. An unknown prior state prevents automatic retry and produces recovery instructions; interruption does not create duplicate VMs or overlapping work. Default operational logs retain useful failure context without known secrets or raw output. Raw output capture requires explicit opt-in and produces user-only files; existing Lima/guest logs are not represented as sanitized.

**V1 limit:** Automatic rollback of arbitrary guest scripts is outside v1. A polished support-bundle exporter or metrics dashboard is later work.

### F9. Release baseline and adoption of the prototype

Include versioned configuration, installation/update/removal/recovery documentation, a chosen license, examples without personal data, release notes, and a compatibility matrix. Provide an explicit adoption path from this prototype that preserves existing disks, SSH aliases, key fingerprints, and storage destinations. Adoption is an explicit operation of the standalone application; it does not introduce a `make local` compatibility entry point.

**Accepted adoption ownership boundary (GF-05):** Adoption explicitly transfers exclusive management ownership of an instance to easy-lima. Back up affected metadata and retire the old helper’s ownership of that instance before easy-lima can mutate it. Record handoff progress and the last established ownership state so interruptions can be diagnosed and recovered against the same VM. Refuse adoption if exclusive ownership cannot be established. Reject downgrades or metadata-restoration operations that would create conflicting owners; a backup does not by itself make a downgrade safe. Preserve the existing instance, disk, and connection identifiers throughout the handoff. This transfer introduces no compatibility launcher.

**Done when:** The current environment is adopted without recreating its disk or re-registering its key. The complete credential, unlock, agent, SSH, and guest Git setup remains usable for authentication and signing after adoption. Its actual instance name and SSH alias are imported as stable bindings and survive subsequent project rebinding. Its first start after adoption does not convert its tools or preset to the current application defaults. Completed, incomplete, and unknown provisioning histories are distinguished; unknown history leads to diagnosis, not implicit provisioning. New and adopted VMs share the configured storage root. A conflicting adoption leaves disks, existing registrations, and the selected root unchanged. Metadata changes are backed up and reported. Unsupported configuration versions fail safely. Fresh-install and adoption workflows both pass release checks. Interrupted handoffs are diagnosable and recoverable with one owner; adoption and downgrade attempts that would create competing owners are rejected. The public interface is the installed CLI, with no machine-specific edits required.

**V1 limit:** Generic adoption of unrelated third-party Lima instances comes later.

## Configuration ownership

| Configuration | Responsibility |
| --- | --- |
| Shareable Lima YAML/template | Image, VM resources, mounts, provisioning, and other Lima settings. |
| Small project configuration | Template reference and project-level wrapper choices that are safe to share. |
| Local user configuration | Browsing preferences, actual storage paths, identities, editor preference, and local overrides, independent of the current working directory. |
| Managed local state | Stable project/VM associations, resolved settings, operation progress, and migration records. |

Define concrete filenames, precedence, and the fields the wrapper owns in the design. Distributed defaults must contain no personal identity, credentials, or machine-specific paths. A project may refer to a profile label; the user explicitly binds that label to a local identity. Shared definitions should avoid host-specific choices wherever possible.

## Platform scope

Approved scope: **macOS and Linux in v1**, native CPU architecture, Ubuntu as the tested guest. Publish the exact tested OS, CPU, and Lima-version combinations. Linux needs actual work on agent lifecycle, credential storage, mounts, and storage behavior; removing the current macOS check is insufficient.

Lima documents macOS and Linux hosts, with VZ normally used on macOS and QEMU on Linux. Its WSL2 driver is currently described as experimental and has option differences. This supports proposing Windows/WSL for a later easy-lima release. Upstream support alone does not establish wrapper compatibility. Sources: [installation](https://lima-vm.io/docs/installation/), [VM types](https://lima-vm.io/docs/config/vmtype/), and [WSL2 caveats](https://lima-vm.io/docs/config/vmtype/wsl2/).

Nested hardware VMs, GPU passthrough, cross-architecture emulation, every guest distribution, and every Lima driver are not v1 guarantees. The advertised Snap/LXD/Juju behavior must be tested on the shipped preset for each supported host combination.

## What comes later

These are proposed groups, not scheduled commitments.

| Stage | Candidate features | Why defer |
| --- | --- | --- |
| **V1.x: convenience** | Favorites/recent projects, broader search, more editor launchers, additional maintained templates, and richer shell completions. | Improve an established workflow without broadening the initial operating model. |
| **V1.x: maintenance** | Guided offline disk/cache moves, supported resizing UI, Lima-backed snapshots and cloning, and explicit adoption of unrelated instances. | Persistent-data operations need partial-failure and version-compatibility tests. Snapshot restore is not a substitute for tested backup/restore. |
| **Later: project variants** | Several environments per project, branch/worktree environments, disposable task VMs, and optional idle shutdown. | Requires rules for shared source, guest data, identity reuse, and activity detection. |
| **Later: portability** | Windows/WSL, additional tested host/guest architectures, and guest-native checkouts with synchronization. | Requires specific filesystem, networking, and credential behavior. |
| **Later: integrations** | Workshop preset/handoff, useful devcontainer interoperability, more Git-provider account checks, hardware keys, and rotation management. | Add integrations for demonstrated workflows while reusing existing capabilities. |
| **Only with demand** | Remote Lima hosts over SSH, offline image bundles, team template distribution, and desktop UI. | Adds transport, distribution, update, and support responsibilities beyond the initial local wrapper. |

Canonical Workshop is an optional later guest integration, not a requirement for starting a v1 VM. Its currently documented LXD requirement differs from the prototype's 5.21 channel, so any integration needs a tested preset and upgrade plan. See [Workshop](https://github.com/canonical/workshop).

## Outside the current direction

The author selected a Lima wrapper. A new hypervisor, an abstraction over competing VM products, a custom package/SDK marketplace, and a hosted cloud platform are outside that direction. VM features should delegate to Lima wherever it supplies the mechanism.

## Release acceptance checks

Run these on every host combination claimed as supported:

1. Install the binary → launch from an arbitrary directory → select folder → start Ubuntu → edit shared source through VS Code → execute a guest command → stop → restart. Launching from a second directory or supplying an explicit path preserves existing registrations and user settings.
2. Two projects retain separate VM state and use deliberate host port mappings.
3. Work and personal profiles use their intended keys; signed commits/tags have the intended identity; wrong-account and wrong-key cases fail clearly. A new VM with an established profile is ready for Git authentication and signing without another setup prompt when its key is loaded or its remembered credential is accessible. Repeat with an empty/stopped agent, after restart, during concurrent creation, and after closing the launcher/reconnecting VS Code. Explicit locks stay locked, necessary interactive OS consent is handled in the active flow, and noninteractive requests produce no hidden dialog. Parent/child overlaps with different profiles are rejected during registration, adoption, rebinding, and profile changes. Profile administration does not start/provision VMs or alter unrelated projects. Adoption, profile replacement, and removal preserve the intended authentication/signing setup, change only easy-lima-owned settings, and report user-edit conflicts. An unavailable profile does not block unrelated or profile-free projects.
4. A project without an identity profile starts without GitHub setup or host-identity inheritance.
5. A missing selected volume or known insufficient space causes no fallback allocation or download. Check conservative requirements for the selected shipped preset across download, temporary, cache, and VM destinations on both distinct filesystems and shared capacity. Destinations sharing a free-space budget are assessed together. Current allocation, logical disk maximum, and unreserved future VM growth are reported separately.
6. Interrupt creation, provisioning, identity configuration, and final connection checks separately. Retain the intended inputs and last confirmed stage; reconcile previous work before retrying and reuse the recorded VM. Active work is not duplicated. If its state remains unknown, stop with recovery instructions. No duplicate allocation, lost ownership, or automatic rollback of arbitrary scripts occurs.
7. Explicitly rebinding a moved or renamed project preserves its VM, guest disk/files, and SSH alias, including after adoption. Conflicting registrations or identity overlaps are rejected without changing existing associations. An interruption between source/mount updates is diagnosable and recoverable against the same instance without duplicate allocation.
8. Deletion targets only an owned VM; uninstall preserves VMs by default; unrelated instances, project files, SSH entries, and host Git settings survive.
9. Prototype adoption preserves disks, connection aliases, and key fingerprints. New and adopted VMs share one storage root; conflicting adoption stops before ownership changes and performs no automatic move or root switch. Affected metadata is backed up, old ownership is retired before new management begins, and interrupted handoffs recover against the same instance. Adoption and downgrades that would create conflicting owners are rejected.
10. Operational logs are present by default and exclude known secrets. Raw output is displayed but not saved by easy-lima without opt-in; explicitly captured files are user-only and are not automatically shared. Existing Lima/guest logs are identified separately as potentially sensitive.
11. Updating the binary or starting an existing VM performs no automatic tool or preset upgrade. An explicit environment-update action previews changes before applying them, and the preview itself does not mutate the guest. Interrupted setup resumes recorded inputs; adoption does not substitute the latest preset for completed or unknown legacy setup.

12. With missing storage, unavailable Lima/SSH, or one malformed registration, inspection and doctor return useful partial results without starting, provisioning, or repairing a VM. Other readable registrations remain visible, and unknown/unreachable state is not labeled as not created. VM, agent, forwarding, signing, transport/authentication, and expected-account checks have separate results. Unavailable GitHub verification is reported as unverified, never as an account mismatch or successful verification. Repairs require a separate explicit action.

These are approved release requirements, not a claim that the prototype already passes them. Choose responsiveness and startup targets against the eventual supported hardware; do not promise unmeasured timings.

## Implementation-design starting point

**Accepted reuse direction (GF-11):** Use selective reuse of portable prototype behavior and suitable code as the starting point. Choose fresh implementation where it offers a clear advantage, comparing distribution, portability, maintainability, and testing before selecting the language and dependencies. See [Alternatives Weighed](concepts_brief.md#alternatives-weighed). The standalone-binary boundary remains the basis for these choices.

**Accepted implementation direction (T1):** Build the standalone host application in Go. Reuse suitable scripts and test scenarios, adapting their behavior to the approved v1 requirements. The [design draft](design_draft.md) records the language comparison and the proposed component design.

**Accepted execution direction (T2):** V1 uses foreground commands with shared CLI/TUI operation logic. Running VMs and profile SSH agents remain available after command exit. Interrupted work retains its recorded attempt and is reconciled before retry. A persistent easy-lima management service is outside the selected v1 design.

**Accepted Lima integration (T3):** Use the installed `limactl` CLI for v1, keeping Lima as an external dependency. Check runtime compatibility explicitly and reject unsupported mutations without hiding readable local metadata. The tested numerical version range remains to be established; updating easy-lima or starting a VM does not upgrade Lima or its guest tools automatically.

**Accepted state persistence (T4):** Keep managed registrations, ownership, bindings, and operation state authoritative in SQLite. Maintain a separate read-only recovery inventory for partial diagnostics. The inventory may be stale and must never authorize mutations or automatically reconstruct ownership. T7 selects the driver and T12 completes schema/maintenance contracts. Migration, export, and recovery validation remain implementation gates.

**Accepted configuration contract (T5):** Use TOML for easy-lima settings and native YAML for Lima templates. Use the per-user configuration/state/cache locations and selected-folder discovery defined in the design. Apply field-specific precedence: creation choices may override creation defaults; storage, identity, ownership, and existing environment revisions remain controlled by their explicit workflows. File-relative paths resolve from their declaring file, independently of browsing CWD.

**Accepted SQLite journal direction (T6):** Use WAL with full commit synchronization, explicit checkpointing, and preserved read-only diagnostics. Ordinary operations and diagnostics must not checkpoint implicitly, including on connection close. The selected driver must demonstrate this contract; pending inventory exports remain diagnostic-only.

**Selected internal state design (T7):** Under the author's delegation, use `github.com/ncruces/go-sqlite3` through `database/sql` with narrow native hooks, and the recorded operation state/evidence/claim model. Retain the same VM and inputs during reconciliation and linked retry. The driver version, filesystem behavior, checkpoint suppression, and read-only paths require validation before release.

**Accepted profile experience (T8):** Use an isolated OpenSSH agent per profile with a lifetime independent of the CLI/TUI. Optional remembering and automatic activation of existing profiles are accepted. Profile-aware VM creation/connection handles routine activation as part of the workflow, so a usable existing identity needs no per-VM credential setup. Adoption keeps its existing usable policy. T9 specifies the owned forwarding tunnel and guest Git/SSH settings; the planned release checks must validate reconnect behavior and automatic reuse.

## Design review and validation remaining

The author has delegated internal implementation choices to the assistant. The complete [implementation design draft](design_draft.md) now specifies components, dependencies, configuration/schema details, recovery/adoption, storage/cache placement, identity connections, qualification targets, and the file-by-file implementation/test plan. Decisions that directly affect the user experience are collected in [UX_PROPOSAL.md](UX_PROPOSAL.md), approved on 2026-11-19: command syntax, TUI behavior, prompts, defaults, installation/support targets, and proposed license. These details now form the approved v1 experience contract.

The qualification matrix and version pins are design targets; real host, driver, workload, interruption, and installation checks remain required before release. Namespace availability for `easy-lima` remains unchecked and is a publishing check, not an unresolved internal design choice.

The concept Goldfish review is complete: all eleven findings are accepted and applied, with none unresolved. The author approved the public experience and authorized a fresh Goldfish review of the completed implementation design on 2026-11-19. No implementation or live-resource changes have begun.

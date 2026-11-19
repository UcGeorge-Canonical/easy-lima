# easy-lima: v1 scope and later features

**Author:** George Uche-Umeh  
**Updated:** 2026-11-18  
**Status:** The author approved this v1 scope on 2026-11-18 and requested an EGM Goldfish review before implementation design. Review round 1 is in progress. Exact compatibility versions and implementation details remain to be designed.

## Product boundary

easy-lima helps a developer select a project folder, start its Lima VM, and work through SSH or VS Code. It adds a project browser, deliberate storage choices, and separate Git identities around Lima's existing capabilities.

Lima remains responsible for VM creation, drivers, guest images, provisioning mechanisms, mounts, networking, and connection details. Use ordinary [Lima templates](https://lima-vm.io/docs/templates/) and Lima YAML for VM settings. easy-lima should store only the additional project associations, user preferences, and identity settings it needs.

The instance name and configuration path must remain visible, and ordinary `limactl` inspection must work. Unrelated Lima instances are outside easy-lima's management unless explicitly adopted. There is no easy-lima online account requirement.

## V1 workflow

1. Install the CLI and check host/Lima prerequisites.
2. Choose workspace roots and inspect the actual VM disk and image-cache destinations before any download or VM creation.
3. Optionally create/import a named Git identity and assign it to a project.
4. Select any folder in the TUI, or pass a path through the CLI. Inspect the template, mounts, resources, and identity before first creation.
5. Start one persistent VM for that folder; open SSH or VS Code at `/workspace`.
6. Stop and resume the same environment later. Diagnose failures with logs and `doctor`. Explicitly delete the VM when its guest data is no longer needed.

A project need not be a Git repository. Selecting a grouping folder creates one VM for that folder; it does not recursively create VMs for its children.

## V1 requirements

### F1. Installation and first-run setup

Include an installed `easy-lima` command, a documented installation path for every supported host, dependency/version checks, workspace-root selection, storage selection, and optional identity setup. Explain required host changes before making them. Repeating setup must preserve configuration and key fingerprints. Uninstalling the wrapper preserves VMs by default.

**Done when:** A new user can follow the quick start on a clean supported host without editing source code or supplying the original author's paths. Missing dependencies and unsupported versions fail with useful instructions before VM allocation.

### F2. Project tree and scriptable CLI

Include keyboard navigation, expand/collapse at any depth, hidden/dependency-folder toggles, VM status, a details pane, and filtering of the loaded tree and registered projects. Read directories lazily. Provide direct path-based commands for the same operations, useful exit codes, and structured VM listings for scripts.

**Done when:** Repositories, subfolders, and grouping folders are selectable; paths with spaces work; symlinks cannot cause traversal loops; browsing does not execute provisioning. Noninteractive commands never unexpectedly open a picker or hang on an unanswered prompt. Full-drive indexing, favorites, and recent-project features can follow later.

### F3. Lifecycle and stable project associations

Include create/start, stop, restart, list, inspect, logs, and confirmed deletion. Re-running start reuses the same instance. Maintain a stable project-to-VM association and an explicit operation to rebind a moved folder. Lock conflicting operations and retain enough state to resume or diagnose incomplete setup.

**Done when:** A moved project does not silently create another disk. Interrupted startup neither reports success nor loses ownership of a partial instance. Guest files survive stop/start. Deletion identifies guest-only data that will be lost, preserves the host checkout, and cannot target an unowned VM.

**V1 limit:** One managed VM per registered project. Multiple environments for the same project are later work.

### F4. Native Lima configuration and limited presets

Include one documented Ubuntu LTS base template, resource choices, and user-provided Lima YAML. Add a small optional Ubuntu development preset supporting the existing Snap/LXD/Juju workflow. Docker and language-specific extras are explicit choices. Show what provisioning will run. Use pinned image/template revisions where supported and record the resolved inputs. Distinguish creation-time settings from changes that require a restart or recreation; ordinary startup must not silently upgrade the environment.

**Done when:** The base and advertised development preset pass their workload checks on supported hosts. Repeated starts skip unchanged completed setup. Unsupported configuration changes give a precise explanation rather than resetting a disk. Custom Lima settings are preserved or explicitly rejected; they are never silently discarded.

**V1 limit:** Template selection and maintained provisioning snippets using Lima's facilities. No SDK registry, package manager, or new provisioning language. Custom templates do not inherit the support guarantee of the shipped Ubuntu preset. Exact preset versions are still to be selected and tested; fully locked package builds are outside v1.

### F5. Storage choices and resource visibility

Include selection of the managed VM storage root, including an external volume; separate visibility of image-cache placement; mount, permission, path, and capacity checks; disk maximum versus allocated usage; and configured CPU/RAM totals for running managed VMs. Refuse an unavailable selected volume instead of falling back to another drive.

**Done when:** Users can locate large disk/image files from the CLI. A missing external drive causes no fallback allocation or download. Sparse capacity is not presented as already-used space. Known insufficient-space conditions fail before allocation. Ordinary setup does not silently relocate another tool's Lima state or shared cache.

**V1 limit:** One managed storage root. No global resource scheduler, automatic storage mover, per-VM storage pools, disk shrinking, or live migration. Existing environments stay in place during adoption.

**Design issue to resolve:** Lima documents [`LIMA_HOME`](https://lima-vm.io/docs/config/environment-variables/) for instance state, but its environment-variable reference does not document `LIMA_CACHE_HOME`. Cache placement must be verified against the supported Lima versions and platforms. The prototype's global cache symlink must not become an installer action that silently affects other users of Lima. If moving a shared cache is necessary, expose its scope and require an explicit user choice.

### F6. SSH, VS Code, shared files, and explicit ports

Include a stable SSH alias, interactive shell, VS Code launcher, noninteractive guest command execution, the selected folder mounted at `/workspace`, and explicit localhost port tunnels. Preserve existing SSH entries when adding the managed configuration. Additional host paths must be deliberate; do not implicitly mount the host home.

**Done when:** SSH/editor connections work after a restart, source changes are visible on both sides, and two VMs can use the same guest application port without taking each other's host port. Guest commands return their exit status. Connection and host-key handling are tested against the supported Lima release. The user can see exactly which paths and ports are exposed.

**V1 limit:** VS Code is the first automated editor integration. Other SSH-capable tools can use the alias. Guest-native checkout/synchronization modes, extra editor launchers, and advanced networking follow later.

### F7. Named Git identity profiles

Include explicitly configured name, email, key, and signing settings for profiles such as `work`, `personal`, and `client`. Import a supported OpenSSH key or generate one once per profile. Reuse it across that profile's projects. Forward only the selected identity through a dedicated agent and keep private keys on the host. Support SSH commit/tag signing, public-key display, and a GitHub authentication check against an explicitly configured expected account.

**Done when:** Work and personal projects offer different selected keys and use the intended author/committer metadata. An unrelated agent is never a fallback. Missing keys, a wrong account, or conflicting repository identity overrides fail clearly. Setup reruns preserve the fingerprint, and the host's global Git identity remains unchanged.

**V1 limits:** Git integration is optional; a public project can start without GitHub setup. No name, email, or credential is silently inferred from the host. A profile change may require stopping the VM and reconnecting; live switching is unnecessary. Automatic account registration, other providers' account checks, hardware-token integrations, team credential distribution, and rotation management come later.

**Trust model:** A forwarded agent gives the connected guest use of that identity even though the key stays on the host. Writable source mounts allow guest changes to those files. V1 supports trusted development work and must describe these boundaries accurately.

### F8. Diagnostics and recovery

Include `doctor`, readable logs, actual instance/configuration/storage paths, and checks for storage, Lima compatibility, SSH, selected identity, and failed provisioning. Report which step failed and an appropriate next action. Cancellation and retries retain recoverable state; logs avoid secrets.

**Done when:** A missing drive, occupied port, unavailable key, interrupted provision, and failed SSH connection are distinguishable without reading source. A retry reuses the incomplete operation's instance. Completed and incomplete stages are distinguishable.

**V1 limit:** No claim of transactional rollback for arbitrary guest scripts. A polished support-bundle exporter or metrics dashboard is later work.

### F9. Release baseline and adoption of the prototype

Include versioned configuration, installation/update/removal/recovery documentation, a chosen license, examples without personal data, release notes, and a compatibility matrix. Provide an explicit adoption path from this prototype that preserves existing disks, SSH aliases, key fingerprints, and storage destinations. Keep `make local` as a small compatibility entry point during that transition if useful.

**Done when:** The current environment is adopted without recreating its disk or re-registering its key. Metadata changes are backed up and reported. Unsupported configuration versions fail safely. Fresh-install and adoption workflows both pass release checks. The public interface is the installed CLI, with no machine-specific edits required.

**V1 limit:** Generic adoption of unrelated third-party Lima instances comes later.

## Configuration ownership

| Configuration | Responsibility |
| --- | --- |
| Shareable Lima YAML/template | Image, VM resources, mounts, provisioning, and other Lima settings. |
| Small project configuration | Template reference and project-level wrapper choices that are safe to share. |
| Local user configuration | Workspace roots, actual storage paths, identities, editor preference, and local overrides. |
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

1. Clean install → select folder → start Ubuntu → edit shared source through VS Code → execute a guest command → stop → restart.
2. Two projects retain separate VM state and use deliberate host port mappings.
3. Work and personal profiles use their intended keys; signed commits/tags have the intended identity; wrong-account and wrong-key cases fail clearly.
4. A project without an identity profile starts without GitHub setup or host-identity inheritance.
5. A missing selected volume or insufficient space causes no fallback allocation or download.
6. Interrupted creation/provisioning is diagnosable and resumable without duplicate instances or loss of ownership.
7. Rebinding a moved project preserves its guest-only files.
8. Deletion targets only an owned VM; uninstall preserves VMs by default; unrelated instances, project files, SSH entries, and host Git settings survive.
9. Prototype adoption preserves disks, connection aliases, and key fingerprints.

These are proposed release requirements, not a claim that the prototype already passes them. Choose responsiveness and startup targets against the eventual supported hardware; do not promise unmeasured timings.

## Decisions still needed

Review findings and any resulting scope amendments; exact host versions and architecture coverage; template contents; configuration precedence and ownership; supported Lima range; cache behavior; Linux credential handling; installation/distribution format; and license.

Namespace availability for `easy-lima` remains unchecked. That does not prevent using it as the working name. Implementation design has not started; the author has authorized Goldfish review of this scope.

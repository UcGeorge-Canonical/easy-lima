> Design record: this document predates implementation. For current behavior and validation, see the [user guides](../../README.md) and [test record](../testing.md). Historical status statements remain part of the decision trail.

# easy-lima: approved v1 experience

**Status:** Approved by George Uche-Umeh on 2026-11-19 ("I approve. Proceed"). Command names, interactions, defaults, installation experience, and presentation below are accepted for implementation design. The final targeted Goldfish design review is [complete with no open findings](consolidated_review_report.md); no application has been implemented.

## Everyday workflow

Run `easy-lima` in any folder to browse that folder and its descendants. `easy-lima /another/root` browses an explicit root. The root folder itself is selectable. Selecting a grouping folder creates one VM for that selection, never one per descendant.

The tree loads one expanded directory at a time. It shows registered VM state and the selected profile beside a project; unknown/unavailable is distinct from stopped or absent. Arrow keys navigate, Right expands, Left collapses/goes to the parent, Enter opens actions, `/` filters the loaded tree plus registered projects, and `?` shows help. Hidden and dependency folders have visible toggles; no full-disk scan runs in the background. Paths outside the current tree can be supplied explicitly. A registry view lists all managed VMs independently of CWD.

```text
 easy-lima   ~/dev                           [Projects] [All VMs]

 ▾ github                                   canonical-contracts
   ▾ ucgeorge-canonical                     Ubuntu 24.04 · development
     ▸ canonical-contracts   Running  work  4 CPU · 8 GiB RAM · 80 GiB max disk
     ▸ easy-lima             Stopped  work  Storage: /chosen/volume/vms
   ▸ personal                               Git: ready · account verified

                                            [Open in VS Code]
                                            [SSH] [Stop] [Details] [More…]

 ↑↓ Move   → Expand   ← Collapse   Enter Actions   / Filter   ? Help
```

For an unregistered folder, the primary action is **Create and open**. The first creation has one preview showing the resolved path, Ubuntu template/preset, resources, actual disk/cache/temp destinations, shared mounts, exposed localhost ports, profile, and setup steps. Accepting it starts work and then opens the editor. Existing healthy environments open directly; a stopped registered VM starts first. Closing the TUI after completion leaves the VM, profile agent, and identity connection available.

If VS Code is absent, offer SSH and show the command needed to configure the editor integration. Do not silently install VS Code or its Remote SSH extension. A user can choose SSH as the default opening action during setup. The installed editor command is stored as an executable plus argument array, not an arbitrary shell snippet.

## First setup and defaults

First-run interactive setup checks the host and installed Lima, asks for the VM storage root, shows where disks/images/temp files and small metadata will live, and offers optional profile creation/import. Storage has no silent fallback or preselected internal-drive destination. A project-volume suggestion is only a suggestion; allocation requires the user's selection. A missing dependency produces exact installation/version guidance and leaves existing resources alone.

Accepted creation defaults are Ubuntu 24.04 LTS, 4 CPUs, 8 GiB RAM, and an 80 GiB virtual disk maximum. The preview distinguishes that maximum from current allocation and unreserved future growth. A host that cannot satisfy the selected CPU/RAM request gets an actionable resource error; v1 does not quietly downsize. The base preset is the default. Development adds Snap/LXD/Juju; Docker is a separate optional choice. Existing environments retain their recorded settings.

The selected source mounts at `/workspace`. Additional mounts and localhost port mappings are explicit. Mounts that would expose easy-lima's authoritative state or a configured private-key location are rejected with the exact conflict. An unrepresentable/control-character path or a native Lima socket-path limit produces a specific path error, not an unexplained startup failure.

A project configuration may request a profile label, but a new user explicitly binds it to a local profile. Profile-free use is always available. Nothing is inferred from the host's personal Git configuration.

## Profiles and automatic authentication

Create/import a profile once with explicit Git name/email, expected GitHub account, and key. Offer **Remember on this device**, off until chosen. Show the public key and the one-time GitHub authentication/signing registration instructions. Reuse the same profile across projects.

When creating or opening a VM with an existing profile, automatically reuse its loaded key or load it from an accessible remembered credential. Configure Git authentication and signing before reporting setup ready. There is no repeated key creation, GitHub registration, remember prompt, or separate unlock command. Necessary OS consent/unlocking is handled inside the interactive flow. Noninteractive commands return needs-unlock instead of opening a hidden dialog.

For an easy-lima-managed agent, an explicit lock applies to the whole profile across its VMs and inhibits automatic reload until the user explicitly unlocks it. An in-flow Unlock action is sufficient. Forget removes an easy-lima-owned remembered passphrase and leaves the key and current agent alone; Lock is separately available. Removing a profile requires resolving its VM bindings first and never silently deletes an imported key or externally owned credential item.

**Accepted profile-update rule (ELD-01, 2026-11-19):** Changing a profile's SSH key includes every VM using that profile. The preview lists all affected VMs; a partial selection is rejected before changes. Use a separate profile when only some VMs need a different key. Shared agent/runtime changes follow the same scope. Git-only settings that keep the same credential may still apply to explicitly selected VMs; a plain profile edit changes no applied identity.

Stopped VMs remain stopped with an approved pending identity update. The next explicit open/start completes that update before Git access; no tools or presets are upgraded. The result distinguishes a committed key from ready, pending and blocked VM identities. Interrupted updates reuse their recorded targets and do not silently roll back the key or apply current defaults. [Transition and recovery details](PROFILE_TRANSITIONS.md).

**Accepted adopted-agent rule (ELD-02, 2026-11-19):** An agent still managed externally may keep serving verified Git authentication and signing, including new VMs when its expected key is available. Show **Managed externally** and its supported actions. Lock, key changes and other agent administration refuse until an explicit migration establishes an easy-lima-managed agent. Missing external agents are reported rather than silently restarted or unlocked. Migration covers all easy-lima VMs using the profile, preserves the existing key and other projects' agent, previews required stops/reconnection, and leaves stopped VMs with durable pending updates. Profile removal/forgetting never deletes externally owned keys or credential items. [Capability and migration contract](ADOPTED_AGENTS.md).

For profile-enabled guests, use a scoped GitHub HTTPS-to-SSH mapping so existing `https://github.com/...` remotes can use the profile immediately. The preview shows this behavior. It affects the guest's effective Git transport and does not rewrite repository remotes or host Git settings. Existing conflicting user configuration is reported rather than overwritten.

## Public commands

Square brackets below mean optional arguments, not literal syntax. Paths are exact selections; no command silently climbs to a parent registration. `--vm ID` selects a registration independently of a moved/missing path. Display names are labels; ambiguous names never select a VM automatically.

| Command | Behavior |
| --- | --- |
| `easy-lima [ROOT]` | Interactive browser, rooted at CWD unless specified. Without a terminal, show usage and exit rather than opening a picker. |
| `init --storage-root PATH` | Explicit host setup. Interactive form gathers missing values; noninteractive form requires them. Repeated setup preserves identity/root ownership. |
| `up [PATH]` | Create if unregistered; otherwise start the recorded VM and finish/reconcile its existing setup. CWD is the default exact path. A new environment shows one preview. |
| `start [PATH]`, `stop [PATH]`, `restart [PATH]` | Operate on an existing registration. Start never creates a missing registered instance. Stop/restart are graceful; no automatic force escalation. |
| `code [PATH]`, `ssh [PATH]` | Open a registered environment; start it if stopped and prepare its existing profile. `--no-start` requires it to be running. Unknown paths direct the user to `up`. |
| `exec [PATH] [--start] -- COMMAND ARG…` | Run a guest command at `/workspace`. No implicit VM start unless requested; return the guest's exit status when known. |
| `list`, `inspect [PATH]`, `doctor [PATH]` | Read-only information. `--json` gives structured metadata; `doctor --online` requests provider checks. No repairs or activation. |
| `logs [PATH] [--follow]` | Default operational events; show separate locations for opt-in raw captures and existing Lima/guest logs. |
| `forward [PATH] GUEST_PORT --local HOST_PORT` | Foreground temporary localhost tunnel; exiting it closes that tunnel and leaves the VM running. Persistent mappings belong in the reviewed environment configuration. |
| `environment update [PATH]` | Preview a proposed environment change. `--apply` performs it after confirmation; ordinary start/binary update does not. |
| `rebind --vm ID --to PATH` | Preview and explicitly change the source binding while preserving VM/disk/alias. Show any stop/restart and identity conflicts. |
| `adopt --from-prototype PATH --project PATH` | Preview the supported handoff, graceful stop, backups, preserved identity and retired ownership. After verified transfer restore prior running state when supported; already-stopped VMs stay stopped. Apply after explicit confirmation. |
| `delete [PATH]` | Show the exact VM and guest data loss. Confirm by its displayed ID; preserve the host checkout and profile key. |
| `profile list`, `profile show LABEL`, `profile check LABEL` | Read-only profile/credential observations; checks do not start VMs or unlock profiles. |
| `profile create LABEL`, `profile import LABEL --key PATH` | Configure explicit identity and optional remembering independently of a VM. Noninteractive fields are required; secrets use a terminal or explicit inherited file descriptor, never argument values. |
| `profile edit LABEL`, `profile apply LABEL` | Edit desired configuration, then preview/apply it. Credential changes require every VM using the profile; guest-only settings may target an explicit subset. Show stopped/deferred members and partial results. A file edit alone changes no applied identity. |
| `profile unlock LABEL`, `profile lock LABEL`, `profile forget LABEL`, `profile remove LABEL` | Explicit credential/profile administration with the ownership and binding rules above; external agents refuse unsupported actions and point to migration. |
| `operations list`, `operations inspect ID`, `operations resume ID` | Inspect attempts; an explicit resume reconciles before continuing the same target and frozen inputs. |
| `repair [PATH]` | Show supported repairs for the recorded problem. Applying a chosen repair is explicit; there is no blanket force/unlock-all repair. |
| `state backup`, `state migrate`, `state restore PATH`, `state checkpoint` | Explicit metadata maintenance, each with its own scope/preview. Checkpoint defaults to bounded PASSIVE; stronger modes require an explicit option. |
| `config explain [PATH]`, `storage inspect` | Read-only effective configuration/provenance and actual storage destinations/usage. |
| `version` | Binary, schema, embedded dependency, and compatible Lima contract information. |

Mutating commands support `--dry-run` where they can produce a complete nonmutating plan; unresolved inputs are reported, not downloaded or executed by a preview. `--yes` accepts the displayed resolved plan in scripts, but never bypasses ownership, explicit locks, missing required values, or a destructive target confirmation. Noninteractive deletion additionally requires `--confirm-vm ID`; it cannot infer a target from a fuzzy name. Changes between planning and application trigger revalidation and a new preview where the effect changes.

`--config FILE` selects an alternative user file; `--project-config FILE` selects a project file. Neither changes the registry location or ownership namespace. `--capture-output` opts into saving raw output for that operation, excluding credential interactions. These flags preserve the accepted T5/GF-10 rules.

**Accepted adoption rule (ELD-08, 2026-11-19):** Adoption previews a graceful stop of a running legacy VM, proves the same VM and legacy work are stopped, and restores its prior running state only after verified handoff and supported recorded startup. Already-stopped VMs stay stopped; unknown state blocks cutover and no force escalation is automatic. Stopping disconnects current SSH/editor sessions; this is shown in the handoff preview. Ownership completion and runtime restoration have separate results. [Adoption contract](ADOPTION.md).

**Public detail approved (2026-11-19):** [INTERFACE_V1.md](INTERFACE_V1.md) specifies the accepted fields, flags, automation inputs and result schemas for ELD-11/ELD-12. It completes the experience above; independent re-review and runtime qualification remain separate.

## Progress, failures, and recovery

Progress names the current stage and the same operation ID across retries. VM state, Git readiness, account verification, and durable-state/inventory health are distinct. A ready VM with incomplete identity setup is shown as such; opening Details shows the next exact action. Offline GitHub leaves account verification unverified and does not label the VM absent or the account wrong.

Ctrl-C requests cancellation, stops new dispatches, and performs bounded final observation. It does not promise that a guest job stopped or shut down the VM. The exit message names the operation and a concrete inspect/resume command. No second prompt is added to the successful path. Leaving the TUI during active foreground work uses this same cancellation behavior; it does not pretend an easy-lima background manager will finish the workflow.

Operational logs are on by default, bounded to 100 MiB across completed operations with 10 MiB rotation. Unresolved recovery/handoff evidence is retained and shown separately. Raw capture is opt-in, private, and never uploaded automatically. No telemetry or online easy-lima account is required.

Accepted metadata-command exit codes: 0 completed, 2 usage/configuration, 3 dependency/compatibility, 4 conflict/busy/explicit decision needed, 5 credential unlock needed, 6 partial/unknown/recovery required, 7 confirmed operation failure, 8 target not found, 9 resource/storage failure, and 130 cancellation. `doctor` uses 6 when requested checks are incomplete/unverified and 7 for established failed checks. `exec` instead preserves a known guest exit code; failure before receiving one returns 255 with a classified diagnostic on stderr. JSON is supported for metadata commands, not mixed into a guest command's stdout.

## Installation and support

Publish versioned archives containing the installed binary, notices, completions, and documentation. A Homebrew tap formula can install a prebuilt macOS binary; Linux gets the same archive workflow and an optional formula where supported. Installing/running easy-lima does not install or upgrade Lima automatically. Do not require a source checkout, Go, Python, a Makefile, or a C compiler to run the released app.

Initial qualification targets are macOS 14 and later on arm64/amd64 using VZ, and Ubuntu 22.04/24.04 Linux on arm64/amd64 with QEMU/KVM. The first Lima qualification target is 2.2.1; only exact versions/builds that pass the contract suite enter the release compatibility list. Other local Linux distributions can run the binary but are not advertised as tested until qualified. Windows/WSL and emulated guest architectures remain deferred. This is the accepted qualification target, not an assertion that it already passes.

Binary update is a package-manager/archive replacement. It retains state, VMs, profiles, and applied tools. Uninstall preserves those resources by default; optional cleanup has a separate preview and leaves imported/adopted keys alone. Use Apache-2.0 for the application and retain required third-party notices; no license file or release has been published during drafting.

## Review boundary

The author approved this experience as a whole on 2026-11-19. Internal mechanics are selected in [design_draft.md](design_draft.md). The final targeted implementation-design Goldfish review is complete; its [four corrections are applied and verified](consolidated_review_report.md). Findings that would change accepted user-facing behavior must be surfaced for author decision; internal corrections follow the existing delegation.

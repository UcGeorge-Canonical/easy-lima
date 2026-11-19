# Testing and qualification

## Fast checks

```sh
make check
make test-race
make test-contracts
make test-terminal
```

`check` runs formatting, vet, unit tests, documentation checks, and a native build. Contract tests exercise real local SSH/signing processes and guest helpers in temporary directories. The native template merge contract runs when the supported `limactl` is installed. They do not create VMs. Python 3.11+ is needed for the pinned prototype fixture.

`test-terminal` runs the installed-style binary through a real pseudo-terminal with isolated user paths. It covers the first-run drive/folder picker, free-space display, manual path entry, opening preference, declining a preview without saving setup, accepting setup, optional profile creation, and entry into the project browser. The test never creates a VM.

The optional macOS credential test creates and deletes a random, easy-lima-owned Keychain item without prompting:

```sh
EASY_LIMA_TEST_KEYCHAIN=1 make test-contracts
```

It never reads an existing account's saved passphrase. Keep this opt-in on hosts with an accessible test keychain. `EASY_LIMA_TEST_AGENTS=1 make test-contracts` additionally exercises same-key reload under an interrupted VM operation using a temporary owned agent service, then removes that service.

## Real VM acceptance

Use a dedicated local directory with a short absolute path ending in `.easy-lima-test`. All VM disks and image downloads go under it. The test uses a separate small temporary HOME for config/state/SSH and an unregistered fixture key, leaving the normal user's VM registry and Git credentials alone. On macOS that HOME must be on a filesystem with enforced ownership for launchd.

```sh
make build
EASY_LIMA_ACCEPTANCE_ROOT=/chosen/volume/.easy-lima-test \
EASY_LIMA_BINARY="$PWD/bin/easy-lima" \
EASY_LIMA_TEST_PRESETS=base,development,docker \
EASY_LIMA_TEST_PROFILE=1 \
EASY_LIMA_TEST_SHARED_PROFILE=1 \
EASY_LIMA_TEST_JUJU=1 \
EASY_LIMA_TEST_ADOPTION=1 \
make test-acceptance
```

Allow enough RAM for a 6 GiB primary VM, a second 2 GiB VM, and nested LXD/Juju containers. Nested hardware virtualization is not required for the tested LXD containers. Initial package/image downloads need network access. The workflow cleans up its VMs and profile on success; failures preserve evidence for inspection and resumption.

`EASY_LIMA_ACCEPTANCE_HOME` selects a different isolated metadata home. `EASY_LIMA_TEST_REFRESH=1` adds an explicit preset refresh. `TEST_RUN=TestVMWorkflow` or `TEST_RUN=TestLegacyAdoption` selects a scenario. Adoption uses the previously verified image cache, so run the normal workflow first. Add `EASY_LIMA_TEST_ADOPTION_GIT=1` to test preservation of a disposable external profile key and agent, completion of its pending identity on start, and actual SSH commit signing. It never uses a registered account or the user's agent.

The acceptance workflow covers:

- VM creation and workspace access; exact guest stdout and exit status.
- SSH alias use, signed Git commits and tags, two VMs sharing one profile, lock/unlock, and a deferred identity change on a stopped VM.
- Docker build/run, Compose, DNS/HTTPS, and service restart.
- LXD container creation, networking, and service restart; Juju bootstrap, deployment, command execution, and cleanup.
- Explicit environment/resource updates, stop/start, stable project rebind, and deletion.
- Read-only diagnostics against unchanged durable files.
- Valid non-Git workspaces, detection of a repository signing-key override, and VM-only start reporting identity unavailability separately.
- Legacy adoption preserving disk and alias, exclusive old-owner retirement, stop/start without new provisioning, TCP/UDP forwarding over IPv4/IPv6 loopback and wildcard guest listeners, and a cancellable temporary tunnel.

The manually dispatched GitHub Actions VM workflow requires a dedicated self-hosted machine labeled `easy-lima-acceptance`. It must provide Lima 2.2.1 and `EASY_LIMA_ACCEPTANCE_ROOT`. Do not run unreviewed pull-request code on that machine.

## Audit verification on 2026-11-19

The [requirement audit](reviews/2026-11-19_REQUIREMENTS.md) separates everyday completion from deferred release and advanced maintenance work. The current audit retains these local logs under ignored `test-results/`:

| Check | Result and evidence |
| --- | --- |
| Formatting, vet, Go units, native build, documentation links | Passed; `audit-final-checks.log`. All 19 current guides have valid local links and anchors. Copyable TOML examples are parsed by the config tests. |
| Go race tests, including blocked SSH channel/exec cancellation | Passed; `audit-ssh-regression-final.log`. |
| Contract tests and 16 Python guest tests | Passed; `audit-final-checks.log`. |
| Real first-run terminal flow | Passed; `audit-final-checks.log`, 6.86 seconds. |
| Resumed two-VM workflow with shared profile and Docker/development presets | Passed; `audit-acceptance-recovered.log`, 290.62 seconds. Includes signing, lock/unlock, pending identity on start, independent VM/identity status, read-only diagnostics, explicit resource change, rebind and deletion. |
| Clean two-VM creation with the final build and base preset | Passed; `audit-fresh-creation.log`, 613.44 seconds. Both VMs were created from empty disks, reused the profile, signed commits, completed identity/lifecycle checks and were deleted without manual recovery. |
| Juju bootstrap/deploy/cleanup | Completed in `audit-acceptance.log` before a later second-VM SSH failure. This is workload evidence, not a claim that that entire run passed. |
| Adoption with external Git agent and IPv4/IPv6 TCP/UDP forwarding | Passed; `audit-adoption-verified.log`, 32.10 seconds. Preserved the existing disk, alias and key; completed the pending identity and verified a signed commit. |
| Local installation | Passed; `audit-install.log`. Installed binary matches the build; installed help runs and all 19 installed guides pass the same local-link/anchor check. |

The earlier full run and its first retry failed because the second guest's login service was stuck and workspace verification misclassified the SSH timeout. The classifier and cancellation handling were fixed. Only that disposable guest was explicitly stopped; the subsequent passing workflow resumed the same saved VM and disk. Failed logs are retained as evidence, not hidden or counted as passes.

The first Git-enabled adoption fixture omitted the prototype's guest identity files and was correctly rejected. Its public fixture data was completed using the captured prototype writer, and the same adopted VM then passed. Future fresh fixtures run that writer before the handoff. This fixture correction did not relax the production conflict checks.

Advanced restore finalization is still deferred. The presence of a restore command and passing admission/publication tests does not qualify every restore path; see the [explicit outstanding work](reviews/2026-11-19_REQUIREMENTS.md#explicitly-deferred-or-outside-this-completion-claim).

## Current validation record

Validation performed on 2026-11-19 uses Lima **2.2.1** and native Ubuntu **24.04 ARM64** guests.

| Host/build target | Evidence | Remaining qualification |
| --- | --- | --- |
| macOS 27.0.1 ARM64, VZ, APFS | Native build, race/contracts, actual VM lifecycle and recovery, shared profile signing, launchd agents, Keychain round-trip, Docker/LXD/Juju workloads, adoption, explicit IPv4/IPv6 TCP/UDP forwarding. | Wider macOS version coverage; external GitHub registration remains user-owned. |
| Ubuntu 24.04 ARM64 userspace | Native Linux test executables exercised detached agents, key isolation, dead-process recovery, SQLite WAL/read-only/restore/publication, configuration and application recovery. | Full Linux **host** QEMU/KVM VM acceptance, systemd user-agent and desktop Secret Service integration. Running userspace tests in a guest is not host qualification. |
| macOS AMD64 | Cross-build target. | Native Intel Mac acceptance. |
| Linux AMD64 | Cross-build target and Linux CI definition. | Native host acceptance, credential desktop integration. |

Build availability is not the same as runtime qualification. The binary's `version --json` reports its adapter contract separately. This initial repository is a development release candidate; a broad platform support claim requires the remaining native host runs. Hosted CI has been configured, not asserted to have run before the repository is published.

Fault-oriented tests protect crash boundaries, SQLite publication prefixes, stale restore refusal, agent isolation, partial environment recovery, secret/output bounds, edited managed files, native configuration restrictions, and refusal by the pinned old owner after retirement. Custom arbitrary scripts and provider accounts are not silently trusted by those fixtures.

Raw local test logs and release outputs live in ignored `test-results/` and `dist/`. Do not commit credentials, actual user registry files, VM disks, or unrestricted raw captures as test evidence.

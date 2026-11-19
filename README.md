# easy-lima

Ubuntu development VMs for your projects, powered by [Lima](https://lima-vm.io/).

Run `easy-lima` in any directory, browse its folders, and open a project in an Ubuntu VM over SSH or VS Code. Each selected folder gets a persistent VM. Named Git profiles keep work and personal identities separate and reuse the same host-held key across your VMs.

easy-lima is a standalone Go binary. Lima remains an external dependency. This repository contains the initial implementation; see [validation status](docs/testing.md) before choosing a host platform.

## Get started

Install **Lima 2.2.1**, Go 1.26 or newer, and Make. On macOS, building also requires the Xcode Command Line Tools for Keychain integration. Then, from this checkout:

```sh
make install
export PATH="$HOME/.local/bin:$PATH"
easy-lima version
```

Choose where VM disks and image caches live. Use a short path on your intended storage volume; there is no implicit storage location or fallback to another drive.

```sh
cd /path/to/projects
easy-lima  # First run opens the drive/folder picker and setup preview.
```

Select a folder at any level. The browser loads folders on demand and offers creation, SSH, VS Code, lifecycle actions, profiles, diagnostics, and recovery. The selected folder is mounted read/write at `/workspace`; the rest of your home directory is not mounted.

For direct commands:

```sh
easy-lima up ./service --no-profile
easy-lima ssh ./service
easy-lima code ./service
easy-lima exec ./service -- uname -a
easy-lima stop ./service
easy-lima list
```

VS Code needs the `code` command and its [Remote - SSH extension](https://code.visualstudio.com/docs/remote/ssh). `up` prepares the VM; `code` and `ssh` open it. Running VMs and profile agents survive the easy-lima command exiting.

## Reuse a Git profile

Create an encrypted key once, with your intended identity explicitly supplied:

```sh
easy-lima profile create work \
  --git-name "Example Developer" \
  --git-email "developer@example.com" \
  --github-user ExampleWork \
  --remember
```

The command prints the **public key**. Add it to the intended GitHub account as both an **Authentication Key** and a **Signing Key**, then check the account:

```sh
easy-lima profile check work --online
easy-lima up ./work-service --profile work
easy-lima up ./another-service --profile work
```

No per-VM Git key setup is needed. Commits and tags use SSH signing by default. Private keys remain on the host, and each profile has its own agent. `--remember` stores the passphrase in the host credential store; omit it to unlock when needed. An explicitly locked profile stays locked until you unlock it.

For a returning project, use `easy-lima up ./work-service` without creation flags. To add a profile to an existing profile-free VM, find its ID with `easy-lima list` and use `easy-lima profile assign work --vm VM_ID`.

See [profiles](docs/profiles.md) for importing keys, changing identity, account registration, and automation.

## What is included

| Area              | Behavior                                                                                                                                 |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| Project selection | CWD-based nested browser, exact folder selection, global VM registry, scriptable CLI and JSON metadata.                                  |
| Ubuntu            | Native-architecture Ubuntu 24.04 with systemd, SSH, Git, Python, compilers, and common development utilities.                            |
| Presets           | Optional `development` for Snap, LXD, and Juju; independent `docker` for Docker Engine, Buildx, and Compose.                             |
| Connections       | Stable SSH aliases, pinned VM host keys, VS Code, exact guest argument/exit forwarding, explicit loopback TCP/UDP mappings.              |
| Identity          | One optional profile per VM; separate agents, reusable credentials, SSH commit/tag signing, GitHub account verification.                 |
| Storage           | One explicit root for disks and image caches, volume identity checks, allocation/capacity reporting, no automatic migration.             |
| Maintenance       | Explicit environment updates, project rebind preserving VM/disk/alias, recoverable prototype adoption.                                   |
| Recovery          | SQLite-backed operations, durable guest receipts, read-only diagnostics, separate repairs, metadata backup and explicit WAL checkpoints. |

```sh
easy-lima up ./tooling-service --profile work --presets development,docker
easy-lima environment update ./tooling-service --refresh-tools        # preview
easy-lima environment update ./tooling-service --refresh-tools --apply
```

Starting a VM or updating the easy-lima binary does **not** upgrade its guest environment. The default VM has 4 CPUs, 8 GiB RAM, and an 80 GiB maximum disk; flags or project configuration can override creation defaults. Sparse disk capacity is not reserved in advance.

## Documentation

Start with the [usage documentation](docs/README.md), [first workspace walkthrough](docs/getting-started.md), and [everyday commands](docs/usage.md). Field-by-field references: [TOML](docs/reference/toml.md), [Lima YAML](docs/reference/lima-yaml.md), and [CLI](docs/reference/commands.md). Copyable configurations live in [docs/examples](docs/examples/README.md).

- [Installation and supported build targets](docs/installation.md)
- [Configuration, templates, mounts, and ports](docs/configuration.md)
- [Profiles, SSH authentication, and signing](docs/profiles.md)
- [Operations, adoption, and recovery](docs/recovery.md)
- [Architecture and repository layout](docs/architecture.md)
- [Tests and platform qualification](docs/testing.md)
- [Building release archives](docs/releasing.md)

The approved specifications live in [docs/design](docs/design/README.md); historical reviews live in [docs/reviews](docs/reviews/README.md). They are development records, separate from the usage guides above.

## Development

```sh
make check
make test-race test-contracts
```

Real VM tests require an explicitly selected disposable storage root. Read [the test guide](docs/testing.md) before running them. Contributions follow [CONTRIBUTING](.github/CONTRIBUTING.md); security reports follow [SECURITY](.github/SECURITY.md).

Licensed under [Apache-2.0](LICENSE). Dependencies and their licenses are listed in [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt). easy-lima is an independent project and is not a Canonical or Lima product.

# Building a release

No release is published automatically. This process builds local archives and checksums; uploading, tagging, signing, notarization, and enabling a package tap are separate maintainer actions.

## Checks

```sh
make check test-race test-contracts
make licenses
```

Review any changes in `go.mod`, `go.sum`, and `THIRD_PARTY_NOTICES.txt`. The notice generator inventories linked dependencies for both host platforms plus the Go toolchain license. It should produce the same file across supported build hosts.

Run the real VM scenarios for each host combination you intend to advertise. Keep the [qualification table](testing.md) honest: a successful cross-build does not qualify that platform's virtualization or credential lifecycle. Catalog updates need fresh workload and recovery tests; simply starting an existing VM must not apply them.

## Archives

```sh
make release VERSION=0.1.0-dev TARGET=darwin/arm64
make release VERSION=0.1.0-dev TARGET=darwin/amd64
make release VERSION=0.1.0-dev TARGET=linux/arm64
make release VERSION=0.1.0-dev TARGET=linux/amd64
```

macOS targets require a macOS SDK and CGO. Linux targets are built without CGO. Each archive in `dist/` includes the binary, README, usage documentation, shell completions, LICENSE, NOTICE, and third-party notices, plus an adjacent SHA-256 checksum. Archives normalize timestamps; `SOURCE_DATE_EPOCH` may set the tar entry timestamp. This is not a promise of identical binaries across different Go/SDK versions.

Extract and test outside the checkout. Confirm `version`, `--help`, completions, missing-dependency errors, explicit init, and a disposable VM workflow. Check that the binary does not depend on source-tree assets. Keep the installed executable path stable because managed SSH aliases reference it.

## Distribution checklist

Before publishing, choose the version, record actual host qualification, provide release notes, configure private vulnerability reporting, and verify the destination repository and tag. No credentials or local machine paths should be copied into public usage examples. Historical review evidence is clearly separated under `docs/reviews`.

The source installation supports `PREFIX` and `DESTDIR`. Completions install into conventional Bash, Zsh, and Fish data directories. No automatic Homebrew installation, auto-update daemon, host reconfiguration, or compatibility launcher is included.

## Initial release notes

The initial implementation adds CWD-based project browsing, persistent Ubuntu VMs, explicit storage, native Lima YAML, recorded presets, SSH/VS Code, reusable isolated Git identities, signing, read-only diagnostics, durable operation recovery, rebind, supported prototype adoption, and metadata maintenance.

Known release boundaries are documented in [architecture](architecture.md) and [testing](testing.md). Existing VMs and tools are not automatically migrated when a new binary is installed.

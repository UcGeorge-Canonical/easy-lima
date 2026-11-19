# Contributing

Start with the [architecture guide](../docs/architecture.md) and [tests](../docs/testing.md). The approved contracts under `docs/design` explain ownership, identity isolation, recovery, and the v1 public interface.

Use Go packages under `internal/` for host/application logic and `assets/guest` for embedded guest effects. Keep frontends separate from effect orchestration. Changes to on-disk ownership, operation state, native Lima inputs, or credentials need tests for interrupted and conflicting states, not only successful execution.

Run `make check test-race test-contracts` for code changes. Real VM tests are opt-in and use disposable storage. Use a native host for changes to service managers, credential stores, virtualization, mounts, or port matching; cross-compilation alone is insufficient.

Update the usage guides when behavior changes. Planning and review artifacts belong under `docs/design` and `docs/reviews`, not at the repository root. Keep unrelated changes out of a patch. Do not commit build caches, VM disks, actual state databases, private keys, passphrases, or raw user logs.

Pull requests should describe the concrete behavior change, validation performed, and any remaining platform limitations. Changes to an approved user-facing contract should make the new behavior explicit rather than silently changing existing workflows.

Contributions are provided under the repository's Apache-2.0 license.

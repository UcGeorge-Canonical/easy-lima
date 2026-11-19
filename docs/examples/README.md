# Project examples

Copy one file to **`easy-lima.toml`** in the project folder:

- [Base Ubuntu](base.toml)
- [Snap, LXD and Juju](development.toml)
- [Docker, an extra directory and a persistent port](docker.toml)

Replace paths and the requested profile label with your own values. Profiles must already exist locally and be explicitly selected. Run `easy-lima config explain .`, then `easy-lima up . --profile work --dry-run` to inspect the resolved request. For an existing VM, use `environment update --from-project` to preview the changes.

See the [TOML reference](../reference/toml.md) for every field and list precedence.

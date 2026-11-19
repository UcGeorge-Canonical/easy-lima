# Using easy-lima

Start with [your first workspace](getting-started.md), then use the [everyday workflow guide](usage.md). You do not need to read the implementation design to use the app.

| Task | Guide |
| --- | --- |
| Install the binary and Lima | [Installation](installation.md) |
| Choose a storage drive and create a VM | [Getting started](getting-started.md) |
| Browse, open VS Code, SSH, run commands, stop or delete | [Everyday use](usage.md) |
| Separate work and personal GitHub identities; sign commits | [Profiles and Git](profiles.md) |
| Find configuration files and understand precedence | [Configuration](configuration.md) |
| Look up every TOML field | [TOML reference](reference/toml.md) |
| Supply a native Lima template | [YAML reference](reference/lima-yaml.md) |
| Look up CLI commands and flags | [Command reference](reference/commands.md) |
| Resume interrupted setup, move a checkout, repair SSH, adopt a VM | [Recovery and maintenance](recovery.md) |

Copyable project configurations: [base](examples/base.toml), [Snap/LXD/Juju](examples/development.toml), [Docker with ports and an extra mount](examples/docker.toml). Save one as **`easy-lima.toml`** in the exact folder you select. Confirm its effective values with `easy-lima config explain .` after initialization.

Developer material is separate: [architecture](architecture.md), [testing](testing.md), [approved design](design/README.md), and [historical reviews](reviews/README.md). Release engineering and broader platform qualification are deferred while everyday behavior is audited.

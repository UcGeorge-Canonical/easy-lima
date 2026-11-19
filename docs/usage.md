# Everyday use

[Documentation home](README.md) · [First workspace](getting-started.md) · [Commands](reference/commands.md)

## Browse folders or address a VM directly

`easy-lima` opens the folder browser at your current working directory. `easy-lima /path/to/projects` chooses another browsing root. The selected folder, not its nearest Git parent, identifies the workspace. Symlink spellings resolve to the same folder. **Tab** switches to all registered VMs, including those outside the current tree. **/** filters loaded folders and registered projects; it does not recursively crawl the disk. **h** toggles hidden folders; **d** toggles dependency folders. **p** opens profiles, **o** saved operations, **?** keyboard help, and **q** exits.

Commands accept a project path or the full immutable ID from `easy-lima list`:

```sh
cd /path/to/service
easy-lima up --profile work          # first creation with an existing profile
easy-lima up                         # subsequent use: reconcile/start this VM
easy-lima inspect
easy-lima code
# From anywhere:
easy-lima ssh /path/to/service
easy-lima inspect --vm FULL_VM_ID
```

Do not pass both a path and `--vm`. To select a directory beginning with a dash, use `easy-lima inspect -- ./-service`. Quote paths containing spaces. Omitted paths mean the original CWD.

## Shell, VS Code and guest commands

```sh
easy-lima ssh ./service                   # starts the registered VM if needed
easy-lima code ./service                  # opens /workspace over Remote - SSH
easy-lima code ./service --no-start       # require an already running VM
easy-lima exec ./service -- go test ./...
easy-lima exec ./service --start -- bash -lc 'pwd; uname -a'
```

`up` creates a VM if absent; `ssh`, `code` and `exec --start` start only an existing registration. The guest command starts in `/workspace`. Arguments after `--` belong to the guest command. `exec` returns its exit status; if no guest exit status can be established, it returns 255. SSH aliases from `inspect` can also be used by compatible SSH clients.

## Lifecycle and storage

```sh
easy-lima start ./service
easy-lima stop ./service
easy-lima restart ./service
easy-lima storage inspect
easy-lima delete ./service --dry-run
```

Stopping preserves the disk and checkout. Delete removes the VM and disk, never the host checkout. Interactive deletion requires typing its full VM ID. Automation also needs `--confirm-vm FULL_VM_ID --yes`. The configured sparse maximum is different from allocated disk usage. Keep the selected external volume attached; v1 does not automatically move VMs.

## Ports and extra directories

```sh
easy-lima forward ./service 8080 --local 18080
```

Open `http://127.0.0.1:18080` on the host. This foreground TCP tunnel requires a running VM and ends on Ctrl-C. For persistent TCP/UDP mappings, use `[[ports]]` in `easy-lima.toml`. Extra host directories use `[[mounts]]`. See the [complete TOML reference](reference/toml.md#mounts-and-ports) and [Docker example](examples/docker.toml). Only explicit loopback endpoints are exposed.

## Change an existing environment deliberately

Edit `easy-lima.toml`, then preview and apply:

```sh
easy-lima environment update ./service --from-project
easy-lima environment update ./service --from-project --apply
# Or request a specific change:
easy-lima environment update ./service --cpus 6 --memory 12GiB
easy-lima environment update ./service --cpus 6 --memory 12GiB --apply
# Refresh pinned preset tools explicitly:
easy-lima environment update ./service --refresh-tools --apply
```

Preview shows the fields and stages that change, and whether the VM needs a restart. Existing settings omitted from a project update remain recorded values. CPU/RAM updates are supported; disk resizing and uninstalling a previously applied preset are outside v1. Do not combine `--from-project` with per-field overrides.

## Diagnose and resume

```sh
easy-lima doctor ./service
easy-lima doctor ./service --online
easy-lima logs ./service
easy-lima operations list
easy-lima operations inspect OPERATION_ID
easy-lima operations resume OPERATION_ID
```

Doctor is read-only and reports each check independently. An unavailable drive or provider produces partial results; it is not proof that a VM or account is missing. Repairs are explicit and previewable. See [recovery](recovery.md) for moved projects, edited settings, host keys, adoption, backups and interrupted commands.

Operational evidence is saved by default without raw guest output. Use `--capture-output` for an operation when you explicitly want its raw output retained; that output can contain application data. Metadata commands support `--json` for one structured result on stdout; progress goes to stderr. `--yes` accepts an already concrete plan for scripts, and `--dry-run` previews without applying it.

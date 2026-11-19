# Installation

## Requirements

- macOS or Linux, native ARM64 or AMD64. See the [qualification table](testing.md) for what has actually been tested.
- The external **Lima 2.2.1** CLI, including its guest-agent assets. Other versions are rejected for VM mutations rather than treated as compatible automatically.
- OpenSSH client tools. macOS uses Apple's VZ driver; Linux uses QEMU and requires working host virtualization. Consult the [upstream installation guide](https://lima-vm.io/docs/installation/) for Lima and its driver dependencies.
- A local filesystem for VM storage. Remote/network filesystems are not supported. The selected volume must remain mounted at the recorded root.
- Optional: VS Code's `code` launcher and Remote - SSH extension; an unlocked macOS Keychain or Linux Secret Service for remembered passphrases.

Building from source requires Go 1.26+, Make, and Python 3.11+ for contract tests and release tooling. macOS builds require the Xcode Command Line Tools and CGO for native Keychain access. Linux binaries build with `CGO_ENABLED=0`.

## Source installation

From a checkout:

```sh
make check
make install PREFIX="$HOME/.local"
```

Add `$HOME/.local/bin` to your shell's PATH. `PREFIX` and `DESTDIR` are supported for packaging. Installation includes the binary, usage guides, notices, and Bash/Zsh/Fish completion files. The binary embeds its VM templates, preset catalog, and guest helpers; it does not need the checkout at runtime.

The installed documentation starts at `$PREFIX/share/doc/easy-lima/README.md` (by default, `~/.local/share/doc/easy-lima/README.md`). Its relative links work offline, including the getting-started guide and TOML/YAML references.

`make uninstall PREFIX="$HOME/.local"` removes installed application files. It preserves VM data, profiles, imported keys, and user state. Remove unused VMs and profiles explicitly before uninstalling if you want those resources retired too.

## Release archives

Release tooling produces archives for `darwin/arm64`, `darwin/amd64`, `linux/arm64`, and `linux/amd64`, with adjacent SHA-256 checksum files. No hosted release or Homebrew tap is assumed by these instructions. See [releasing](releasing.md) to build an archive.

Verify its checksum, extract it, and install `easy-lima` into a stable executable location:

```sh
shasum -a 256 -c easy-lima_0.1.0_darwin_arm64.tar.gz.sha256
tar -xzf easy-lima_0.1.0_darwin_arm64.tar.gz
install -d "$HOME/.local/bin"
install -m 755 easy-lima "$HOME/.local/bin/easy-lima"
```

Substitute the actual version and target. Locally produced archives are not signed or notarized by this build process.

## First run

```sh
easy-lima init --storage-root /chosen/volume/vms
```

The storage choice is explicit. VM instances, verified image caches, and large operation work files stay under it. Small application configuration, SQLite state, SSH trust, and public profile metadata remain in the user's application directories. Generated private profile keys also stay in the private application configuration directory.

Use a short storage-root path: Unix socket limits also constrain Lima's temporary SSH socket names. easy-lima checks this before creation. Moving the root is outside v1; editing `storage.root` does not move or reassign existing VMs.

If the external disk is unplugged, reconnect it. The application does not create a replacement root on the internal disk.

The default open action is VS Code. Use `--open-action ssh` during init or set `ui.open_action = "ssh"` in user configuration. Missing editor tools do not prevent SSH use.

## Updating the binary

Install a new binary at the same path. This does not start VMs, migrate guest tools, refresh presets, or update Lima. Database compatibility is checked explicitly; a newer schema cannot be opened by an incompatible older binary.

SSH aliases retain the installed binary's absolute path for their connection guard. If you move the executable, use `easy-lima repair --vm VM_ID --kind alias --apply` to preview and publish the new owned alias. Read [recovery](recovery.md) before changing state or ownership.

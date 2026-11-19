# Your first workspace

[Documentation home](README.md) · [Installation](installation.md) · [Configuration reference](reference/toml.md)

Install the supported Lima CLI and easy-lima using the [installation guide](installation.md). `limactl --version` must report the version supported by `easy-lima version` (currently 2.2.1). On macOS the VM uses native Apple virtualization; on Linux it uses QEMU. VS Code is optional.

## 1. Choose where the VMs live

Open a terminal in a folder containing your projects and run:

```sh
cd /path/to/projects
easy-lima
```

On first run, choose a drive or starting location with **↑/↓** and **Enter**. The picker shows free space and the full path. Browse directories or choose **New storage folder here…**, for example `easy-lima-vms` on an external drive. You can also enter a path. Nothing is created until you accept the setup preview. **Esc** goes back; **Ctrl-C** cancels.

Choose whether opening a project should launch VS Code or an SSH terminal. Review the storage directory, image/cache and temporary directories, and small host configuration/state paths. Type `yes` to accept. VM disks and large downloads stay under the selected root. An unavailable external drive blocks operations; the app does not choose another disk.

For an explicit command instead of the picker:

```sh
easy-lima init --storage-root /Volumes/Work/easy-lima-vms --open-action code
```

Use your actual directory; on Linux it might be `/mnt/work/easy-lima-vms`. Use a short dedicated path because Lima's Unix sockets have platform path-length limits. All VMs, including adopted VMs, use this one root in v1. There is no automatic storage migration.

## 2. Set up a reusable Git identity

Skip this step if your VM does not need authenticated Git access. Otherwise, create a work profile with your own name, email and GitHub username:

```sh
easy-lima profile create work \
  --git-name "Example Developer" \
  --git-email "developer@example.com" \
  --github-user ExampleWork \
  --remember
```

Enter a new passphrase when prompted. `--remember` is optional: it stores the passphrase in macOS Keychain or Linux Secret Service so existing profiles can be reused without another unlock prompt. The private key stays on the host. Do not paste it into GitHub or copy it into a VM.

The result prints a **public key** and its fingerprint. In the intended GitHub account's **Settings → SSH and GPG keys**, add that public key twice: once as an **Authentication Key**, once as a **Signing Key**. Make sure the profile's author email is associated with that account. Then:

```sh
easy-lima profile check work --online
```

The observed GitHub account must match the expected username. Account registration is a separate check from local SSH signing. See [profiles](profiles.md) for imports, locked agents and account troubleshooting.

## 3. Select a project and create the VM

Run `easy-lima` again. **→** expands a folder; **←** collapses it or selects its parent. **Enter** shows the selected folder's actions. You can select any level, including a grouping folder. Choose **Create and open**, choose your profile or **No Git profile**, and select the configured environment or a preset. Review the plan and confirm it.

First creation downloads Ubuntu and installs the selected tools, so allow several minutes. Later opens reuse the existing VM and profile. If setup is interrupted, use the saved operation ID with `easy-lima operations resume OPERATION_ID`; see [recovery](recovery.md).

The default Ubuntu 24.04 VM has 4 CPUs, 8 GiB RAM, and an 80 GiB maximum sparse disk. Creation checks capacity before downloading. The full future growth of the disk is not reserved. The selected folder appears at `/workspace` and writes there affect the host checkout.

Direct equivalent:

```sh
easy-lima up ./service --profile work
easy-lima code ./service
# Or open a terminal:
easy-lima ssh ./service
```

For Snap, LXD and Juju, pass `--presets development`. For Docker Engine, Buildx and Compose, pass `--presets docker`. Combine them with `--presets development,docker`. The creation preview describes installed stages and privileged group access. The VM has its own Linux kernel and systemd; it is not a container. Architecture-specific software still needs to support the guest architecture.

VS Code requires its `code` launcher on PATH and the Remote - SSH extension. If the launcher is unavailable, use `easy-lima ssh ./service` or select SSH from the browser. Your VM and profile agent remain available after easy-lima exits.

## 4. Reuse it

```sh
easy-lima list
easy-lima code ./service
easy-lima exec ./service -- uname -a
easy-lima stop ./service
```

An existing profile can be selected for another new VM without generating another key. Existing VMs retain their recorded environment. Merely starting one or updating the binary does not upgrade its tools.

Continue with [everyday use](usage.md), [TOML settings](reference/toml.md), or [recovery](recovery.md).

#!/usr/bin/env python3
"""Persistent Ubuntu development VMs for any folder in this workspace."""
from __future__ import annotations

import argparse
from contextlib import contextmanager
from dataclasses import dataclass
import fcntl
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import tomllib

ROOT = Path(__file__).resolve().parents[1]
INTERNAL = {".local-launcher", ".local-state"}
NOISY = {".git", "node_modules", "vendor", ".venv", "venv", "__pycache__", ".cache", "target", "dist", "build"}
DEFAULTS = dict(cpus=4, memory="8GiB", disk="80GiB", lxd_channel="5.21/stable",
                juju_channel="3.6/stable", go_channel="latest/stable",
                rockcraft=False, charmcraft=False, forward_agent=False)
# Read the entire script before executing it: LXC and package tools otherwise
# consume subsequent bash -s source lines as their own noninteractive stdin.
GUEST_SCRIPT = 'local_guest_script=$(cat); exec bash -c "$local_guest_script" -- "$@" </dev/null'


class LocalError(Exception):
    pass


def run(args, *, capture=False, input=None, timeout=60, check=True, env=None):
    try:
        result = subprocess.run([str(x) for x in args], text=True, input=input,
                                stdout=subprocess.PIPE if capture else None,
                                stderr=subprocess.PIPE if capture else None, timeout=timeout, env=env)
    except FileNotFoundError as exc:
        raise LocalError(f"Missing command: {args[0]}") from exc
    except subprocess.TimeoutExpired as exc:
        raise LocalError(f"Command timed out: {shlex.join(map(str, args))}. Check local-info before retrying.") from exc
    if check and result.returncode:
        detail = (result.stderr or result.stdout or "").strip()
        raise LocalError(f"Command failed ({result.returncode}): {shlex.join(map(str, args))}\n{detail}")
    return result


def atomic_write(path: Path, content: str, mode=0o600):
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, name = tempfile.mkstemp(prefix=".tmp-", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as stream:
            os.fchmod(stream.fileno(), mode)
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def clean_path(path: Path):
    if any(ord(c) < 32 or ord(c) == 127 for c in str(path)) or "{{" in str(path):
        raise LocalError("Directory names must not contain control characters or Lima template expressions.")


def ssh_quote(path: Path):
    clean_path(path)
    return '"' + str(path).replace("\\", "\\\\").replace('"', '\\"') + '"'


def instance_name(root: Path, relative: str):
    slug = re.sub(r"[^a-z0-9-]+", "-", Path(relative).name.lower()).strip("-")[:20] or "project"
    digest = hashlib.sha256(f"{root}\0{relative}".encode()).hexdigest()[:10]
    return f"cl-{slug}-{digest}"


@dataclass(frozen=True)
class Project:
    root: Path
    relative: str
    source: Path

    @property
    def name(self):
        return instance_name(self.root, self.relative)

    @property
    def host(self):
        return f"lima-{self.name}"


def project_at(root: Path, value: str, *, must_exist=True):
    root = root.resolve()
    # Keep the logical path as the identity, including intentionally linked checkouts.
    logical = Path(os.path.abspath(root / value))
    try:
        relative = logical.relative_to(root)
    except ValueError as exc:
        raise LocalError("Select a folder inside local/ (symlinked checkouts are supported).") from exc
    if not relative.parts or relative.parts[0] in INTERNAL:
        raise LocalError("Select a project or grouping folder below local/, not the launcher/state directory.")
    try:
        source = logical.resolve(strict=must_exist)
    except (OSError, RuntimeError) as exc:
        raise LocalError(f"Cannot resolve project: {logical}: {exc}") from exc
    clean_path(source)
    if must_exist and not source.is_dir():
        raise LocalError(f"Project folder does not exist: {logical}")
    if source == root or source in root.parents or any(source == root / p or root / p in source.parents for p in INTERNAL):
        raise LocalError("A project must not expose the launcher state or an ancestor of local/.")
    return Project(root, relative.as_posix(), source)


def load_options(root: Path, relative: str, overrides=None):
    path = root / "local-vms.toml"
    data = tomllib.loads(path.read_text()) if path.exists() else {}
    if set(data) - {"defaults", "projects", "storage", "git"}:
        raise LocalError("Unknown section in local-vms.toml; use defaults, projects, storage, or git.")
    values = DEFAULTS | data.get("defaults", {})
    # A group profile is inherited by its descendants; the most specific wins.
    for part in reversed([Path(relative), *Path(relative).parents]):
        values.update(data.get("projects", {}).get(part.as_posix(), {}))
    values.update({k: v for k, v in (overrides or {}).items() if v not in (None, "")})
    if set(values) - DEFAULTS.keys():
        raise LocalError(f"Unknown VM settings: {', '.join(sorted(set(values) - DEFAULTS.keys()))}")
    try:
        values["cpus"] = int(values["cpus"])
    except (ValueError, TypeError) as exc:
        raise LocalError("cpus must be an integer.") from exc
    if not 1 <= values["cpus"] <= 128:
        raise LocalError("cpus must be between 1 and 128.")
    for key in ("memory", "disk"):
        value = str(values[key])
        if not re.fullmatch(r"[1-9][0-9]*(?:MiB|GiB|TiB)", value):
            raise LocalError(f"{key} must have units, for example 8GiB.")
    for key in ("lxd_channel", "juju_channel", "go_channel"):
        if not isinstance(values[key], str) or not re.fullmatch(r"[A-Za-z0-9._-]+/(stable|candidate|beta|edge)", values[key]):
            raise LocalError(f"Invalid snap channel for {key}.")
    for key in ("rockcraft", "charmcraft", "forward_agent"):
        if not isinstance(values[key], bool):
            raise LocalError(f"{key} must be true or false.")
    return values


def project_mounts(project: Project):
    mounts = [{"location": str(project.source), "mountPoint": "/workspace", "writable": True}]
    # A grouping folder may contain absolute checkout links (as local/github does).
    # Preserve those targets at their absolute guest paths. Stop at repository roots
    # and never walk through links, dependency trees, or launcher state.
    targets = set()
    for directory, folders, _ in os.walk(project.source, followlinks=False):
        parent = Path(directory)
        if (parent / ".git").exists():
            folders[:] = []
            continue
        descend = []
        for folder in folders:
            if folder in INTERNAL | NOISY or folder.startswith("."):
                continue
            child = parent / folder
            if child.is_symlink():
                if Path(os.readlink(child)).is_absolute() and child.is_dir():
                    target = child.resolve()
                    clean_path(target)
                    if target == project.root or target in project.root.parents or any(target == project.root / p or project.root / p in target.parents for p in INTERNAL):
                        continue
                    # Lima rejects overlapping host mounts.
                    if target != project.source and target not in project.source.parents and project.source not in target.parents:
                        targets.add(target)
            else:
                descend.append(folder)
        folders[:] = descend
    for target in sorted(targets):
        if not any(other in target.parents for other in targets):
            mounts.append({"location": str(target), "mountPoint": str(target), "writable": True})
    return mounts


def render_config(project: Project, options: dict, *, work_git=False):
    # Inherit only Ubuntu's image list: the stock template also shares the user's home.
    config = {
        "minimumLimaVersion": "2.0.0",
        "base": ["template:_images/ubuntu-24.04"],
        "vmType": "vz", "arch": "default", "cpus": options["cpus"],
        "memory": options["memory"], "disk": options["disk"],
        "mountType": "virtiofs",
        "mounts": project_mounts(project),
        "containerd": {"system": False, "user": False},
        # Work forwarding is scoped to a specific socket in our SSH Include.
        # Lima's boolean forwarding would forward the caller's personal agent
        # when someone invokes raw `limactl shell` outside this launcher.
        "ssh": {"localPort": 0, "loadDotSSHPubKeys": False,
                "forwardAgent": options["forward_agent"] and not work_git},
        # Multiple projects may all run a server on 8080. Forward explicitly via SSH/VS Code.
        "portForwards": [{"guestIP": "0.0.0.0", "guestIPMustBeZero": False,
                          "proto": "any", "guestPortRange": [1, 65535], "ignore": True}],
        "message": "Ubuntu is booted. The local launcher will now provision development tools.",
    }
    # JSON is valid YAML, and handles spaces, quotes, and colons in folder names safely.
    return json.dumps(config, indent=2) + "\n"


def parse_instances(output: str):
    if not output.strip():
        return {}
    try:
        parsed = json.loads(output)
        rows = parsed if isinstance(parsed, list) else [parsed]
    except json.JSONDecodeError:
        rows = [json.loads(line) for line in output.splitlines() if line.strip()]
    return {row["name"]: row for row in rows}


def install_ssh_include(config: Path, include: Path):
    """Prepend one Include without changing any existing user configuration."""
    # Respect symlink-managed dotfiles by updating the target instead of replacing the link.
    target = config.resolve() if config.is_symlink() else config
    original = target.read_text() if target.exists() else ""
    line = f"Include {ssh_quote(include)}"
    if line in original.splitlines():
        return False
    config.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    backup = target.with_name(target.name + ".before-canonical-local")
    if target.exists() and not backup.exists():
        atomic_write(backup, original)
    atomic_write(target, f"# Canonical local project VMs\n{line}\n\n" + original)
    return True


class Manager:
    def __init__(self, root=ROOT, *, lima_home=None, ssh_config=None):
        self.root = root.resolve()
        self.state = self.root / ".local-state"
        config_path = self.root / "local-vms.toml"
        config = tomllib.loads(config_path.read_text()) if config_path.exists() else {}
        self.storage = config.get("storage", {}) if lima_home is None else {}
        self.git = config.get("git", {})
        self.lima_home = Path(lima_home or self.storage.get("lima_home") or os.environ.get("LIMA_HOME", Path.home() / ".lima")).expanduser().resolve()
        self.ssh_config = Path(ssh_config or Path.home() / ".ssh/config")
        self.overrides = {k: os.environ.get(k.upper()) for k in ("cpus", "memory", "disk")}

    def require_storage(self):
        if not self.storage:
            return
        volume = Path(self.storage["volume"])
        if not volume.is_mount():
            raise LocalError(f"{volume} is not mounted. Connect Nebula before starting a VM; internal storage is not used as a fallback.")
        cache = Path(self.storage["cache_home"]).resolve()
        for path in (self.lima_home, cache):
            if not path.is_relative_to(volume.resolve()):
                raise LocalError(f"VM storage must remain on {volume}: {path}")
        default_cache = Path.home() / "Library/Caches/lima"
        if default_cache.resolve() != cache:
            raise LocalError(f"Lima's image cache must point to {cache}; refusing to download onto the internal drive.")

    def lima(self, args, **kwargs):
        self.require_storage()
        environment = os.environ.copy()
        environment["LIMA_HOME"] = str(self.lima_home)
        if self.git.get("enabled"):
            from git_identity import agent_socket
            environment["SSH_AUTH_SOCK"] = str(agent_socket())
        return run(["limactl", *args], env=environment, **kwargs)

    def require_lima(self):
        if platform.system() != "Darwin":
            raise LocalError("This launcher currently targets macOS with Apple's Virtualization.framework.")
        if not shutil.which("limactl"):
            raise LocalError("Lima is missing. Install it with: brew install lima")
        version = run(["limactl", "--version"], capture=True).stdout
        match = re.search(r"(\d+)\.(\d+)", version)
        if not match or tuple(map(int, match.groups())) < (2, 0):
            raise LocalError("Lima 2.0 or newer is required. Run: brew upgrade lima")

    def instances(self):
        self.require_lima()
        return parse_instances(self.lima(["list", "--json"], capture=True, timeout=20).stdout)

    @contextmanager
    def lock(self):
        self.require_storage()
        self.state.mkdir(mode=0o700, parents=True, exist_ok=True)
        with (self.state / "launcher.lock").open("a") as stream:
            try:
                fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError as exc:
                raise LocalError("Another local launcher operation is in progress. Retry when it finishes.") from exc
            try:
                yield
            finally:
                fcntl.flock(stream, fcntl.LOCK_UN)

    def records(self):
        return [json.loads(p.read_text()) for p in sorted((self.state / "projects").glob("*.json"))]

    def record_path(self, project):
        return self.state / "projects" / f"{project.name}.json"

    def record(self, project):
        path = self.record_path(project)
        return json.loads(path.read_text()) if path.exists() else None

    def check_ownership(self, project, instances):
        record = self.record(project)
        if project.name in instances and not record:
            raise LocalError(f"{project.name} exists but is not owned by this launcher. Refusing to modify it.")
        if record and (record["relative"] != project.relative or record["name"] != project.name):
            raise LocalError("Project metadata does not match. Refusing to modify this VM.")
        return record

    def update_ssh(self):
        lines = ["# Generated by canonical-local; Lima manages SSH keys and connection settings."]
        for record in self.records():
            config = self.lima_home / record["name"] / "ssh.config"
            if config.is_file():
                if self.git.get("enabled"):
                    from git_identity import agent_socket
                    lines.extend([f"Host lima-{record['name']}",
                                  f"  ForwardAgent {ssh_quote(agent_socket())}",
                                  "  ControlMaster no", "  ControlPath none",
                                  "  ServerAliveInterval 30", "  ServerAliveCountMax 3"])
                lines.append(f"Include {ssh_quote(config)}")
        # Included files leave a Host context active; reset it before the user's config.
        lines.append("Host *")
        include = self.state / "ssh/config"
        atomic_write(include, "\n".join(lines) + "\n")
        install_ssh_include(self.ssh_config, include)

    def up(self, project):
        instances = self.instances()
        record = self.check_ownership(project, instances)
        options = load_options(self.root, project.relative, self.overrides)
        if record and record["source"] != str(project.source):
            raise LocalError("This project's source path changed. The existing VM still mounts the old path.\n"
                             "Run make local-remount PROJECT=" + shlex.quote(project.relative) + " to update the mount and retain the VM disk.")
        if self.git.get("enabled"):
            from git_identity import ensure_agent
            if not options["forward_agent"]:
                raise LocalError("Shared Git signing requires forward_agent=true in local-vms.toml.")
            ensure_agent(self.git)
            instance = instances.get(project.name)
            if instance and instance.get("config", {}).get("ssh", {}).get("forwardAgent", False):
                if instance["status"] != "Stopped":
                    self.lima(["stop", project.name], timeout=180)
                self.lima(["edit", "--tty=false", "--set", ".ssh.forwardAgent = false", project.name])
                instance["status"] = "Stopped"
        if project.name not in instances:
            config_path = self.state / "configs" / f"{project.name}.yaml"
            atomic_write(config_path, render_config(project, options, work_git=bool(self.git.get("enabled"))))
            # Record ownership before creation so interrupted provisioning can be resumed.
            record = {"name": project.name, "relative": project.relative, "source": str(project.source),
                      "resources": {k: options[k] for k in ("cpus", "memory", "disk")}}
            atomic_write(self.record_path(project), json.dumps(record, indent=2) + "\n")
            print(f"Creating Ubuntu 24.04 for {project.relative} ({options['cpus']} CPU, {options['memory']} RAM)…", flush=True)
            if self.storage and shutil.disk_usage(self.lima_home.parent).free < 10 * 1024**3:
                raise LocalError("Nebula has less than 10 GiB free. Free space before creating another VM.")
            self.lima(["start", "--tty=false", f"--name={project.name}", str(config_path)], timeout=1800)
        elif instances[project.name]["status"] != "Running":
            print(f"Starting {project.relative}…", flush=True)
            self.lima(["start", "--tty=false", project.name], timeout=600)
        if record and record.get("resources") != {k: options[k] for k in ("cpus", "memory", "disk")}:
            print(f"Existing resources retained. To resize: limactl stop {project.name}; limactl edit {project.name}")
        self.provision(project, options)
        self.update_ssh()
        if self.git.get("enabled"):
            self.configure_git(project)
        # This also verifies the generated Include as used by VS Code, not just limactl shell.
        run(["ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=15", project.host,
             "test -d /workspace && test -w /workspace"], timeout=30)
        self.show_connection(project)

    def provision(self, project, options):
        script = (ROOT / ".local-launcher/provision.sh").read_text()
        settings = [options[k] for k in ("lxd_channel", "juju_channel", "go_channel", "rockcraft", "charmcraft")]
        digest = hashlib.sha256((script + json.dumps(settings)).encode()).hexdigest()
        old = self.lima(["shell", "--workdir=/", project.name, "cat",
                   "/var/lib/canonical-local/provision.sha256"], capture=True, check=False).stdout.strip()
        if old == digest:
            return
        user = self.lima(["shell", "--workdir=/", project.name, "id", "-un"], capture=True).stdout.strip()
        args = [str(v).lower() if isinstance(v, bool) else str(v) for v in settings]
        self.lima(["shell", "--workdir=/", project.name, "sudo", "bash", "-c", GUEST_SCRIPT, "--", user, *args, digest],
            input=script, timeout=2400)
        # Lima authenticated its shared SSH session before usermod added the user
        # to lxd/docker. Reauthenticate so ordinary SSH and VS Code get those groups.
        run(["ssh", "-F", self.lima_home / project.name / "ssh.config", "-O", "exit", project.host],
            capture=True, check=False, timeout=15)

    def guest_python(self, project, filename, arguments=()):
        script = (ROOT / ".local-launcher" / filename).read_text()
        wrapper = "import sys; exec(compile(sys.stdin.read(), 'canonical-local', 'exec'))"
        command = shlex.join(["python3", "-c", wrapper, *arguments])
        return run(["ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=15", project.host, command],
                   input=script, timeout=90)

    def configure_git(self, project):
        from git_identity import guest_payload
        self.guest_python(project, "git-guest.py", [json.dumps(guest_payload(self.git))])

    def setup_git(self):
        from git_identity import setup
        with self.lock():
            setup(self.git)
            instances = self.instances()
            for record in self.records():
                if record["name"] not in instances:
                    continue
                project = project_at(self.root, record["relative"])
                was_stopped = instances[project.name]["status"] == "Stopped"
                try:
                    self.up(project)
                    self.guest_python(project, "git-check.py", ["--offline"])
                finally:
                    if was_stopped:
                        self.stop(project)
        print("Work Git is configured for existing VMs and will apply automatically to new ones.")

    def show_connection(self, project):
        print(f"\nProject  {project.relative}\nSource   {project.source}\nGuest    /workspace\n")
        print(f"SSH      ssh {project.host}")
        print(f"VS Code  code --remote ssh-remote+{project.host} /workspace")
        print(f"URI      vscode://vscode-remote/ssh-remote+{project.host}/workspace\n", flush=True)

    def stop(self, project):
        instances = self.instances()
        self.check_ownership(project, instances)
        if project.name not in instances:
            print("No VM has been created for this folder.")
        elif instances[project.name]["status"] == "Stopped":
            print("Already stopped. Guest data is preserved.")
        else:
            self.lima(["stop", project.name], timeout=180)
            print("Stopped. Guest data is preserved.")

    def remount(self, project):
        instances = self.instances()
        record = self.check_ownership(project, instances)
        if not record or project.name not in instances:
            raise LocalError("No VM exists to remount. Use local-up to create it first.")
        mounts = project_mounts(project)
        if instances[project.name]["status"] != "Stopped":
            self.lima(["stop", project.name], timeout=180)
        self.lima(["edit", "--tty=false", "--set", ".mounts = " + json.dumps(mounts), project.name])
        record["source"] = str(project.source)
        atomic_write(self.record_path(project), json.dumps(record, indent=2) + "\n")
        config_path = self.state / "configs" / f"{project.name}.yaml"
        if config_path.exists():
            config = json.loads(config_path.read_text())
            config["mounts"] = mounts
            atomic_write(config_path, json.dumps(config, indent=2) + "\n")
        self.up(project)

    def delete(self, project, confirmed=False):
        instances = self.instances()
        self.check_ownership(project, instances)
        if not self.record(project):
            print("No managed VM exists for this folder.")
            return
        if not confirmed:
            if not sys.stdin.isatty():
                raise LocalError("Deleting destroys the VM's disk. Pass CONFIRM=delete to explicitly confirm.")
            answer = input(f"Delete {project.name} and all data on its VM disk? Host source files remain.\nType the VM name: ")
            if answer != project.name:
                print("Cancelled.")
                return
        if project.name in instances:
            if instances[project.name]["status"] != "Stopped":
                self.lima(["stop", project.name], timeout=180)
            self.lima(["delete", "--tty=false", project.name], timeout=180)
        self.record_path(project).unlink(missing_ok=True)
        (self.state / "configs" / f"{project.name}.yaml").unlink(missing_ok=True)
        self.update_ssh()
        print("Deleted the VM. The host project folder was preserved.")

    def list(self):
        instances = self.instances()
        records = self.records()
        if not records:
            print("No project VMs yet. Run make local to choose a folder.")
        for record in records:
            status = instances.get(record["name"], {}).get("status", "Not created")
            print(f"{status:12} {record['relative']}\n{'':12} lima-{record['name']}")

    def info(self, project):
        row = self.instances().get(project.name)
        print(f"State    {row['status'] if row else 'Not created'}")
        self.show_connection(project)
        print(f"VM files {self.lima_home / project.name}")
        print(f"Host log {self.lima_home / project.name / 'ha.stderr.log'}")
        print(f"Settings {self.root / 'local-vms.toml'}")

    def doctor(self):
        print(f"Host: {platform.system()} {platform.machine()}\nWorkspace: {self.root}")
        self.require_lima()
        print(run(["limactl", "--version"], capture=True).stdout.strip())
        print(f"Lima state: {self.lima_home}")
        print(f"SSH: {shutil.which('ssh') or 'MISSING'}")
        print(f"VS Code: {shutil.which('code') or 'CLI not installed (use Remote-SSH from VS Code)'}")
        print(f"Configured VMs: {len(self.records())}")
        self.instances()
        print("Host checks passed. Use local-verify PROJECT=… for a real guest/container check.")

    def action(self, action, project, *, confirmed=False, port=None, guest_port=None, offline=False):
        if action == "render":
            print(render_config(project, load_options(self.root, project.relative, self.overrides),
                                work_git=bool(self.git.get("enabled"))), end="")
            return
        if action == "info":
            return self.info(project)
        # Release the lock before a long-lived shell, tunnel, or editor connection.
        with self.lock():
            if action in {"up", "ssh", "code", "verify", "forward", "git-check"}:
                self.up(project)
            elif action == "stop":
                return self.stop(project)
            elif action == "delete":
                return self.delete(project, confirmed)
            elif action == "remount":
                return self.remount(project)
        if action == "ssh":
            run(["ssh", project.host], timeout=None)
        elif action == "code":
            if shutil.which("code"):
                run(["code", "--remote", f"ssh-remote+{project.host}", "/workspace"])
            else:
                run(["open", f"vscode://vscode-remote/ssh-remote+{project.host}/workspace"])
        elif action == "verify":
            script = (ROOT / ".local-launcher/verify.sh").read_text()
            run(["ssh", "-o", "BatchMode=yes", project.host, "bash -c " + shlex.quote(GUEST_SCRIPT) + " --"],
                input=script, timeout=900)
        elif action == "git-check":
            self.guest_python(project, "git-check.py", ["--offline"] if offline else [])
        elif action == "forward":
            port = valid_port(port)
            guest_port = valid_port(guest_port or port)
            print(f"Forwarding localhost:{port} → guest:{guest_port}. Ctrl-C closes the tunnel.", flush=True)
            run(["ssh", "-N", "-o", "ExitOnForwardFailure=yes", "-L",
                 f"127.0.0.1:{port}:127.0.0.1:{guest_port}", project.host], timeout=None)


def valid_port(value):
    try:
        number = int(value)
    except (ValueError, TypeError) as exc:
        raise LocalError("Provide PORT=8080 (and optionally GUEST_PORT=3000).") from exc
    if not 1 <= number <= 65535:
        raise LocalError("Ports must be between 1 and 65535.")
    return number


def main():
    parser = argparse.ArgumentParser(description="Ubuntu project VMs. Run make local for the tree picker.",
                                     epilog="Example: make local-code PROJECT=github/ucgeorge-canonical/canonical-contracts")
    parser.add_argument("action", nargs="?", default="pick", choices=["pick", "up", "ssh", "code", "stop",
                        "list", "info", "doctor", "verify", "delete", "render", "forward", "remount", "ssh-config",
                        "git-setup", "git-public-key", "git-check"])
    parser.add_argument("--project", default=os.environ.get("PROJECT") or None, help="Folder relative to local/")
    parser.add_argument("--confirm-delete", action="store_true", default=os.environ.get("CONFIRM") == "delete")
    parser.add_argument("--port", default=os.environ.get("PORT"))
    parser.add_argument("--guest-port", default=os.environ.get("GUEST_PORT"))
    parser.add_argument("--offline", action="store_true", help="Check Git signing without contacting GitHub")
    args = parser.parse_args()
    manager = Manager()
    if args.action in {"list", "doctor"}:
        return getattr(manager, args.action)()
    if args.action == "ssh-config":
        with manager.lock():
            manager.update_ssh()
        print(f"SSH Include configured in {manager.ssh_config}")
        return
    if args.action == "git-setup":
        return manager.setup_git()
    if args.action == "git-public-key":
        from git_identity import show_key
        return show_key(manager.git)
    if args.action == "pick" or not args.project:
        if not sys.stdin.isatty() or not sys.stdout.isatty():
            raise LocalError("The tree picker needs an interactive terminal. Use PROJECT=path with make local-up/ssh/code.")
        from tui import pick
        while True:
            choice = pick(manager, initial=args.project)
            if not choice:
                return
            action, relative = choice
            if args.action != "pick":
                action = args.action
            try:
                manager.action(action, project_at(manager.root, relative, must_exist=action not in {"delete", "stop", "info"}),
                               confirmed=args.confirm_delete, port=args.port, guest_port=args.guest_port, offline=args.offline)
            except (LocalError, OSError) as exc:
                if args.action != "pick":
                    raise
                print(f"\nError: {exc}", file=sys.stderr)
            if args.action != "pick":
                return
            input("\nPress Enter to return to the project tree…")
            args.project = relative
    else:
        project = project_at(manager.root, args.project, must_exist=args.action not in {"delete", "stop", "info"})
        manager.action(args.action, project, confirmed=args.confirm_delete, port=args.port, guest_port=args.guest_port, offline=args.offline)


if __name__ == "__main__":
    sys.modules.setdefault("local", sys.modules[__name__])
    try:
        main()
    except KeyboardInterrupt:
        print("\nInterrupted. Any existing VM and its disk are preserved.", file=sys.stderr)
        sys.exit(130)
    except (LocalError, OSError, ValueError, KeyError, TypeError) as error:
        print(f"local: {error}", file=sys.stderr)
        sys.exit(1)

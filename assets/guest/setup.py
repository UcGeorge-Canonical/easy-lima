#!/usr/bin/env python3
"""Stage effects and independent postconditions for Ubuntu 24.04 guests."""
import hashlib
import fcntl
import ipaddress
import json
import os
from pathlib import Path
import pwd
import re
import selectors
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import urllib.request

BASE = ["git", "openssh-client", "ca-certificates", "curl", "build-essential", "python3", "python3-venv", "jq", "rsync", "tar", "gzip", "bzip2", "xz-utils", "unzip", "zip", "iptables", "uidmap", "dbus-user-session", "slirp4netns", "pigz", "libltdl7"]
DEV = ["snapd", "apparmor", "apparmor-utils", "squashfs-tools"]
ROOT = Path("/var/lib/easy-lima")


def run(args, capture=False, check=True, timeout=1200):
    if not capture:
        return subprocess.run(args, check=check, timeout=timeout).returncode
    # Bound output while reading: checking communicate() afterwards lets a
    # malformed package/tool response consume unbounded guest memory first.
    process = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, start_new_session=True)
    output = bytearray()
    deadline = time.monotonic() + timeout
    try:
        with selectors.DefaultSelector() as selector:
            selector.register(process.stdout, selectors.EVENT_READ)
            while selector.get_map():
                remaining = deadline - time.monotonic()
                if remaining <= 0: raise subprocess.TimeoutExpired(args, timeout)
                for key, _ in selector.select(min(remaining, 1)):
                    chunk = os.read(key.fileobj.fileno(), 16384)
                    if not chunk:
                        selector.unregister(key.fileobj)
                        continue
                    if len(output) + len(chunk) > 256 * 1024: raise ValueError("response too large")
                    output.extend(chunk)
        status = process.wait(timeout=max(0.001, deadline - time.monotonic()))
        if check and status: raise subprocess.CalledProcessError(status, args)
        return output.decode().strip()
    finally:
        if process.poll() is None:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait()
        process.stdout.close()


def sha(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            h.update(chunk)
    return h.hexdigest()


def atomic(path, data, mode=0o600):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd, temporary = tempfile.mkstemp(prefix=".easy-lima-", dir=path.parent)
    try:
        os.fchmod(fd, mode)
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        fd = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def save(path, value):
    atomic(path, json.dumps(value, sort_keys=True, separators=(",", ":")).encode() + b"\n")


def owned(path, data, owner, mode=0o644):
    path = Path(path)
    index = ROOT / "owned" / (owner + ".json")
    state = json.loads(index.read_bytes()) if index.exists() else {}
    entry = state.get(str(path))
    expected = entry.get("old") if isinstance(entry, dict) else entry
    previous_target = entry.get("target") if isinstance(entry, dict) else expected
    desired = hashlib.sha256(data).hexdigest()
    if path.is_symlink():
        raise ValueError("owned destination is a symlink")
    current = sha(path) if path.exists() else None
    if isinstance(entry, dict) and previous_target != desired:
        raise ValueError("a different file publication is pending")
    if current not in (expected, desired):
        raise ValueError("owned destination was edited")
    # Intent before publication permits exact target-byte recovery.
    state[str(path)] = {"old": expected, "target": desired}
    save(index, state)
    if current != desired:
        atomic(path, data, mode)
    state[str(path)] = desired
    save(index, state)


def package_resolution(data, group, roots):
    import apt
    directory = ROOT / "environments" / data["environment_revision_id"]
    path = directory / (group + ".packages.json")
    if path.exists():
        validate_resolution(path)
        return
    previous = data["parameters"].get("previous_revision_id")
    presets = data["parameters"].get("previous_presets", [])
    reuse = previous and not data["parameters"].get("refresh_tools") and (group == "base" or group == "dev" and "development" in presets)
    if reuse:
        oldpath = ROOT / "environments" / previous / (group + ".packages.json")
        # Resource-only revisions may refer to the last actual provisioning run.
        old = validate_resolution(oldpath)
        archive = directory / (group + "-archives")
        archive.mkdir(parents=True, mode=0o700, exist_ok=True)
        for package in old["packages"]:
            source = Path(package["file"])
            target = archive / source.name
            if not target.exists(): os.link(source, target)
            package["file"] = str(target)
        save(path, old)
        return
    cache = apt.Cache()
    cache.update()
    cache.open(None)
    for name in roots:
        cache[name].mark_install(auto_fix=True, from_user=True)
    changes = cache.get_changes()
    if len(changes) > 512 or any(p.marked_delete for p in changes):
        raise ValueError("unapproved package removal or transaction size")
    archive = directory / (group + "-archives")
    archive.mkdir(parents=True, mode=0o700, exist_ok=True)
    selected = []
    for package in changes:
        version = package.candidate
        origins = version.origins
        if not any(o.origin == "Ubuntu" and o.trusted for o in origins):
            raise ValueError("unapproved package source")
        record = version.record
        digest = record.get("SHA256")
        if not digest or not re.fullmatch(r"[a-f0-9]{64}", digest):
            raise ValueError("package digest unavailable")
        file = Path(version.fetch_binary(str(archive)))
        if sha(file) != digest:
            raise ValueError("package hash mismatch")
        selected.append({"name": package.name, "version": version.version, "before": package.installed.version if package.installed else None,
                         "sha256": digest, "size": file.stat().st_size, "file": str(file)})
    required = [{"name": name, "version": cache[name].candidate.version if cache[name].marked_install or cache[name].marked_upgrade else cache[name].installed.version} for name in roots]
    save(path, {"schema_version": 1, "roots": roots, "required": required, "packages": selected})


def validate_resolution(path):
    data = json.loads(path.read_bytes())
    if data["schema_version"] != 1 or len(data["packages"]) > 512:
        raise ValueError("resolution invalid")
    for p in data["packages"]:
        file = Path(p["file"])
        if not file.is_relative_to(path.parent) or file.is_symlink() or sha(file) != p["sha256"]:
            raise ValueError("frozen archive unavailable")
    return data


def installed(name):
    result = subprocess.run(["dpkg-query", "-W", "-f=${db:Status-Status}\t${Version}", name], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=15)
    text = result.stdout.decode()
    status, _, version = text.partition("\t")
    return version if result.returncode == 0 and status == "installed" else None


def package_version(name):
    result = subprocess.run(["dpkg-query", "-W", "-f=${Version}", name], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=15)
    return result.stdout.decode() if result.returncode == 0 else None


def package_manager_idle():
    for path in ("/var/lib/dpkg/lock", "/var/lib/dpkg/lock-frontend"):
        fd = os.open(path, os.O_RDWR | os.O_NOFOLLOW)
        try: fcntl.lockf(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        finally: os.close(fd)


def install_resolution(path):
    data = validate_resolution(path)
    pending = []
    for p in data["packages"]:
        value = installed(p["name"])
        if value == p["version"]:
            continue
        if value != p["before"] and package_version(p["name"]) != p["version"]:
            raise ValueError("package state changed after resolution")
        pending.append(p["file"])
    if pending:
        available = os.statvfs("/").f_bavail * os.statvfs("/").f_frsize
        if available < sum(Path(p).stat().st_size * 4 for p in pending) + 64*1024*1024: raise ValueError("insufficient guest package capacity")
        run(["dpkg", "--force-confdef", "--force-confold", "--install", *pending])


def package_evidence(path):
    data = validate_resolution(path)
    evidence = []
    for p in data["packages"] + data["required"]:
        value = installed(p["name"])
        if value != p["version"]:
            raise ValueError("package postcondition failed")
        evidence.append({"kind": "package", "name": p["name"], "version": value, "configured": True})
    return evidence


def snap_state(name):
    # snapd's local API returns structured exact revision and hold evidence.
    text = run(["curl", "--fail", "--silent", "--unix-socket", "/run/snapd.socket", "http://localhost/v2/snaps/" + name], capture=True, timeout=20)
    return json.loads(text)["result"]


def pending_snap_changes():
    raw = run(["curl", "--fail", "--silent", "--unix-socket", "/run/snapd.socket", "http://localhost/v2/changes?select=in-progress"], capture=True, timeout=20)
    changes = json.loads(raw)["result"]
    if any(not change.get("ready", False) for change in changes):
        raise ValueError("snapd still has an in-progress change")


def set_snap(data, name):
    spec = data["parameters"]["snaps"][name]
    marker = ROOT / "owned" / ("snap-" + name + ".json")
    prior = json.loads(marker.read_bytes()) if marker.exists() else None
    pending_snap_changes()
    try:
        existing = snap_state(name)
    except subprocess.CalledProcessError:
        existing = None
    if existing and existing["id"] != spec["id"]: raise ValueError("snap identity mismatch")
    if existing and prior is None: raise ValueError("external snap must not be changed")
    before = str(existing["revision"]) if existing else None
    if prior:
        allowed = {str(prior.get("revision")), str(prior.get("before")), str(spec["revision"])}
        if str(before) not in allowed: raise ValueError("snap revision was changed externally")
    # Ownership and before/target evidence precede snapd dispatch.
    save(marker, {"id":spec["id"], "before":before, "revision":spec["revision"]})
    if before != str(spec["revision"]):
        args = ["snap", "refresh" if existing else "install", name, "--revision=" + str(spec["revision"]), "--channel=" + spec["channel"]]
        if name == "juju": args.append("--classic")
        run(args)
    run(["snap", "refresh", "--hold=forever", name])


def lxd_json(*args):
    if "show" in args:
        # The API returns JSON on every supported LXD revision; CLI show is YAML.
        resource, _, name = args
        endpoint = {"storage": "storage-pools", "network": "networks", "profile": "profiles"}[resource]
        return json.loads(run(["lxc", "query", "/1.0/" + endpoint + "/" + name], capture=True))
    return json.loads(run(["lxc", *args, "--format=json"], capture=True))


def network_conflicts(cidr, except_interface=None):
    desired = ipaddress.ip_network(cidr)
    routes = json.loads(run(["ip", "-j", "route", "show", "table", "all"], capture=True))
    for route in routes:
        destination = route.get("dst", "default")
        if destination == "default" or route.get("dev") == except_interface:
            continue
        try:
            other = ipaddress.ip_network(destination, strict=False)
        except ValueError:
            continue
        if desired.overlaps(other):
            raise ValueError("network overlaps an existing route")


def network_allocation(data, create=False):
    path = ROOT / "owned" / ("networks-" + data["registration_id"] + ".json")
    if path.exists():
        allocation = json.loads(path.read_bytes())
        if allocation.get("owner") != data["registration_id"]: raise ValueError("network owner mismatch")
        selected = allocation["selected"]
        candidates = data["parameters"].get("network_candidates")
        if candidates and selected not in candidates: raise ValueError("the existing allocation conflicts with current host routes")
        return selected
    if not create: return None
    p = data["parameters"]
    # An explicit update can preserve a pre-manifest allocation only after
    # proving the existing owned services still use its recorded subnets.
    legacy_lxd = ROOT / "owned/lxd-bridge.json"
    legacy_docker = ROOT / "owned/docker-config.json"
    if legacy_lxd.exists() or legacy_docker.exists():
        selected = {"lxd": p["lxd_subnet"], "docker": p["docker_subnet"], "pool": p["docker_pool"]}
        if selected not in p.get("network_candidates", []): raise ValueError("old networks conflict with host routes")
        if legacy_lxd.exists():
            bridge = lxd_json("network", "show", "elbr0")
            if bridge["config"].get("ipv4.address") != bridge_config(data)["ipv4.address"]: raise ValueError("owned LXD allocation changed")
        if legacy_docker.exists():
            daemon = json.loads(Path("/etc/docker/daemon.json").read_bytes())
            expected = str(ipaddress.ip_network(p["docker_subnet"]).network_address + 1) + "/24"
            if daemon.get("bip") != expected or daemon.get("default-address-pools") != [{"base": p["docker_pool"], "size":24}]: raise ValueError("owned Docker allocation changed")
        save(path, {"owner":data["registration_id"],"selected":selected})
        return selected
    for candidate in data["parameters"].get("network_candidates", []):
        try:
            for value in candidate.values(): network_conflicts(value)
        except ValueError:
            continue
        save(path, {"owner": data["registration_id"], "selected": candidate})
        return candidate
    raise ValueError("no admitted subnet candidate avoids guest routes")


def bind_networks(data):
    selected = network_allocation(data)
    if selected:
        data["parameters"].update(lxd_subnet=selected["lxd"], docker_subnet=selected["docker"], docker_pool=selected["pool"])


def bridge_config(data):
    subnet = ipaddress.ip_network(data["parameters"]["lxd_subnet"])
    return {"ipv4.address": str(subnet.network_address + 1) + "/24", "ipv4.nat": "true", "ipv4.dhcp": "true", "ipv4.firewall": "true", "ipv6.address": "none"}


def apply(stage, data):
    bind_networks(data)
    p = data["parameters"]
    envdir = ROOT / "environments" / data["environment_revision_id"]
    owner = data["registration_id"]
    if stage in ("env.inspect.v1", "env.verify.v1"):
        return
    if stage == "network.resolve.v1":
        network_allocation(data, create=True)
    elif stage == "base.resolve.v1":
        package_resolution(data, "base", BASE)
    elif stage == "base.install.v1":
        install_resolution(envdir / "base.packages.json")
    elif stage == "base.runtime.v1":
        # Lima supplies a per-boot cloud-init instance. Ubuntu's default SSH
        # module otherwise replaces the host keys already enrolled by the host.
        owned("/etc/cloud/cloud.cfg.d/99-easy-lima-host-keys.cfg", b"ssh_deletekeys: false\n", owner)
        user = pwd.getpwnam(p["user"])
        runtime = Path("/run/easy-lima") / owner
        runtime.mkdir(parents=True, mode=0o700, exist_ok=True)
        os.chown(runtime, user.pw_uid, user.pw_gid)
        os.chmod(runtime, 0o700)
        rule = f"d /run/easy-lima 0755 root root -\nd {runtime} 0700 {user.pw_uid} {user.pw_gid} -\n"
        owned("/etc/tmpfiles.d/easy-lima-" + owner + ".conf", rule.encode(), owner)
    elif stage == "dev.resolve.v1":
        package_resolution(data, "dev", DEV)
    elif stage == "dev.snapd.v1":
        install_resolution(envdir / "dev.packages.json")
        run(["systemctl", "enable", "--now", "apparmor", "snapd.socket"])
        run(["snap", "wait", "system", "seed.loaded"], timeout=600)
    elif stage in ("dev.lxd.v1", "dev.juju.v1"):
        set_snap(data, stage.split(".")[1])
    elif stage == "dev.pool.v1":
        pools = lxd_json("storage", "list")
        matches = [v for v in pools if v["name"] == "easy-lima"]
        if matches:
            if not (ROOT / "owned" / "lxd-pool.json").exists() or matches[0]["driver"] != "dir":
                raise ValueError("LXD pool ownership conflict")
        else:
            save(ROOT / "owned" / "lxd-pool.json", {"name": "easy-lima", "driver": "dir", "owner": owner})
            run(["lxc", "storage", "create", "easy-lima", "dir"])
    elif stage == "dev.bridge.v1":
        network_conflicts(p["lxd_subnet"], "elbr0")
        owned("/etc/sysctl.d/70-easy-lima-forwarding.conf", b"net.ipv4.ip_forward=1\n", owner)
        run(["sysctl", "-w", "net.ipv4.ip_forward=1"])
        bridges = lxd_json("network", "list")
        found = [v for v in bridges if v["name"] == "elbr0"]
        if found:
            if not (ROOT / "owned" / "lxd-bridge.json").exists() or any(found[0]["config"].get(k) != v for k, v in bridge_config(data).items()):
                raise ValueError("LXD network ownership conflict")
        else:
            save(ROOT / "owned" / "lxd-bridge.json", {"name": "elbr0", "config": bridge_config(data), "owner": owner})
            run(["lxc", "network", "create", "elbr0", *[k + "=" + v for k, v in bridge_config(data).items()]])
    elif stage == "dev.profile.v1":
        profile = lxd_json("profile", "show", "default")
        for name, kind, fields in [("root", "disk", {"path": "/", "pool": "easy-lima"}), ("eth0", "nic", {"network": "elbr0", "name": "eth0"})]:
            desired = {"type": kind, **fields}
            if name in profile["devices"]:
                if profile["devices"][name] != desired or not (ROOT / "owned" / ("lxd-device-" + name + ".json")).exists():
                    raise ValueError("existing LXD device conflict")
            else:
                save(ROOT / "owned" / ("lxd-device-" + name + ".json"), desired)
                run(["lxc", "profile", "device", "add", "default", name, kind, *[k + "=" + v for k, v in fields.items()]])
    elif stage == "docker.source.v1":
        for name in ("docker.io", "docker-compose", "containerd", "runc"):
            if installed(name):
                raise ValueError("preexisting distro Docker conflict")
        owned("/etc/systemd/system/docker.service.d/20-easy-lima.conf", b"[Unit]\nConditionPathExists=/var/lib/easy-lima/docker/configured\n", owner)
        run(["systemctl", "daemon-reload"])
        key = p["docker"]["key"].encode()
        if hashlib.sha256(key).hexdigest() != p["docker"]["key_sha256"]:
            raise ValueError("Docker publisher key mismatch")
        Path("/etc/apt/keyrings").mkdir(mode=0o755, exist_ok=True)
        owned("/etc/apt/keyrings/easy-lima-docker.asc", key, owner)
        source = f"Types: deb\nURIs: https://download.docker.com/linux/ubuntu\nSuites: noble\nComponents: stable\nArchitectures: {p['architecture']}\nSigned-By: /etc/apt/keyrings/easy-lima-docker.asc\n"
        owned("/etc/apt/sources.list.d/easy-lima-docker.sources", source.encode(), owner)
    elif stage == "docker.config.v1":
        network_conflicts(p["docker_subnet"], "docker0")
        policy = run(["iptables", "-S", "FORWARD"], capture=True)
        if "-P FORWARD DROP" in policy and not (ROOT / "owned" / "docker-config.json").exists():
            raise ValueError("conflicting forwarding policy")
        owned("/etc/sysctl.d/70-easy-lima-forwarding.conf", b"net.ipv4.ip_forward=1\n", owner)
        run(["sysctl", "-w", "net.ipv4.ip_forward=1"])
        path = Path("/etc/docker/daemon.json")
        settings = json.loads(path.read_bytes()) if path.exists() else {}
        subnet = ipaddress.ip_network(p["docker_subnet"])
        values = {"firewall-backend": "iptables", "iptables": True, "ip6tables": True, "ip-forward-no-drop": True,
                  "bip": str(subnet.network_address + 1) + "/24", "default-address-pools": [{"base": p["docker_pool"], "size": 24}]}
        marker = ROOT / "owned" / "docker-config.json"
        recorded = json.loads(marker.read_bytes()) if marker.exists() else {}
        if "target" in recorded:
            if recorded["target"] != values: raise ValueError("a different Docker configuration publication is pending")
            old = recorded["old"]
        else:
            old = recorded
        for k, v in values.items():
            if k in settings and settings[k] not in (v, old.get(k)):
                raise ValueError("Docker user configuration conflict")
        settings.update(values)
        save(marker, {"old": old, "target": values})
        atomic(path, json.dumps(settings, sort_keys=True, indent=2).encode() + b"\n", 0o644)
        save(marker, values)
    elif stage == "docker.install.v1":
        target = p["docker"]["packages"]
        publisher = envdir / "docker.publisher.json"
        if publisher.exists():
            if json.loads(publisher.read_bytes()) != target:
                raise ValueError("Docker publisher resolution changed")
        else:
            # APT authenticates InRelease and Packages using the source's
            # scoped Signed-By key; TLS/catalog hashes alone are insufficient.
            import apt
            cache = apt.Cache()
            cache.update()
            cache.open(None)
            for package in target:
                versions = [v for v in cache[package["name"]].versions if v.version == package["version"]]
                if not any(v.record.get("SHA256") == package["sha256"] and any(o.trusted and o.site == "download.docker.com" for o in v.origins) for v in versions):
                    raise ValueError("Pinned Docker package is absent from authenticated publisher metadata")
            save(publisher, target)
        before_path = envdir / "docker.before.json"
        if not before_path.exists(): save(before_path, {v["name"]: package_version(v["name"]) for v in target})
        before = json.loads(before_path.read_bytes())
        for package in target:
            if package_version(package["name"]) not in (before.get(package["name"]), package["version"]): raise ValueError("Docker package changed after admission")
        archive = envdir / "docker-archives"
        archive.mkdir(parents=True, mode=0o700, exist_ok=True)
        paths = []
        for package in target:
            path = archive / (package["name"] + ".deb")
            if not path.exists():
                with urllib.request.urlopen(package["url"], timeout=90) as response:
                    content = response.read(package["size"] + 1)
                if len(content) != package["size"] or hashlib.sha256(content).hexdigest() != package["sha256"]:
                    raise ValueError("Docker package mismatch")
                atomic(path, content)
            if sha(path) != package["sha256"]:
                raise ValueError("Docker archive changed")
            paths.append(str(path))
        run(["dpkg", "--force-confdef", "--force-confold", "--install", *paths])
        run(["apt-mark", "hold", *[v["name"] for v in target]])
    elif stage == "docker.service.v1":
        marker = ROOT / "docker" / "configured"
        atomic(marker, hashlib.sha256(json.dumps(p["docker"], sort_keys=True).encode()).hexdigest().encode() + b"\n")
        run(["systemctl", "enable", "--now", "docker"])
    elif stage == "access.groups.v1":
        groups = []
        if "development" in p["presets"]:
            groups.append("lxd")
        if "docker" in p["presets"]:
            groups.append("docker")
        if groups:
            run(["usermod", "-aG", ",".join(groups), p["user"]])
    else:
        raise ValueError("unrecognized stage")


def verify(stage, data):
    bind_networks(data)
    p = data["parameters"]
    directory = ROOT / "environments" / data["environment_revision_id"]
    if stage == "network.resolve.v1":
        allocation = network_allocation(data)
        if allocation is None: raise ValueError("network allocation missing")
        return [{"kind": "network_allocation", **allocation}]
    if stage.startswith("native."):
        return [{"kind": "native_script", "sha256": data["script_sha256"], "completion": "successful_exit_observed"}]
    if stage == "env.inspect.v1":
        release = Path("/etc/os-release").read_text()
        if 'VERSION_ID="24.04"' not in release or 'ID=ubuntu' not in release:
            raise ValueError("unsupported image")
        run(["systemctl", "is-system-running"], capture=True, check=False, timeout=15)
        if not Path("/run/systemd/system").is_dir() or not Path("/workspace").is_dir():
            raise ValueError("guest prerequisites missing")
        return [{"kind": "guest", "os": "ubuntu", "release": "24.04", "systemd": True}]
    if stage in ("base.resolve.v1", "dev.resolve.v1"):
        path = directory / (stage.split(".")[0] + ".packages.json")
        validate_resolution(path)
        return [{"kind": "resolution", "path": str(path), "sha256": sha(path)}]
    if stage in ("base.install.v1", "dev.snapd.v1"):
        return package_evidence(directory / (stage.split(".")[0] + ".packages.json"))
    if stage == "base.runtime.v1":
        if Path("/etc/cloud/cloud.cfg.d/99-easy-lima-host-keys.cfg").read_bytes() != b"ssh_deletekeys: false\n":
            raise ValueError("persistent host key policy missing")
        path = Path("/run/easy-lima") / data["registration_id"]
        user = pwd.getpwnam(p["user"])
        st = path.stat()
        if st.st_uid != user.pw_uid or st.st_mode & 0o777 != 0o700:
            raise ValueError("runtime permissions mismatch")
        return [{"kind": "runtime", "path": str(path), "uid": st.st_uid, "mode": 448}]
    if stage in ("dev.lxd.v1", "dev.juju.v1"):
        name = stage.split(".")[1]
        value = snap_state(name)
        target = p["snaps"][name]
        if value["id"] != target["id"] or str(value["revision"]) != str(target["revision"]) or not value.get("hold"):
            raise ValueError("snap target/hold mismatch")
        return [{"kind": "snap", "name": name, "revision": str(value["revision"]), "hold": value["hold"]}]
    if stage == "dev.pool.v1":
        value = lxd_json("storage", "show", "easy-lima")
        if value["driver"] != "dir":
            raise ValueError("pool driver mismatch")
        return [{"kind": "lxd_pool", "name": "easy-lima", "driver": "dir"}]
    if stage == "dev.bridge.v1":
        value = lxd_json("network", "show", "elbr0")
        if any(value["config"].get(k) != v for k, v in bridge_config(data).items()):
            raise ValueError("bridge mismatch")
        return [{"kind": "lxd_bridge", "name": "elbr0", "config": bridge_config(data)}]
    if stage == "dev.profile.v1":
        devices = lxd_json("profile", "show", "default")["devices"]
        if devices.get("root") != {"type": "disk", "path": "/", "pool": "easy-lima"} or devices.get("eth0") != {"type": "nic", "network": "elbr0", "name": "eth0"}:
            raise ValueError("profile device mismatch")
        return [{"kind": "lxd_profile", "name": "default", "devices": devices}]
    if stage == "docker.install.v1":
        held = set(run(["apt-mark", "showhold"], capture=True).splitlines())
        result = []
        for package in p["docker"]["packages"]:
            if installed(package["name"]) != package["version"] or package["name"] not in held:
                raise ValueError("Docker package or hold mismatch")
            result.append({"kind": "package", "name": package["name"], "version": package["version"], "held": True})
        return result
    if stage == "docker.service.v1":
        run(["systemctl", "is-active", "docker"], capture=True)
        run(["docker", "info", "--format", "{{json .ServerVersion}}"], capture=True)
        return [{"kind": "service", "name": "docker", "state": "active"}]
    if stage == "docker.source.v1":
        gate = Path("/etc/systemd/system/docker.service.d/20-easy-lima.conf")
        if gate.read_bytes() != b"[Unit]\nConditionPathExists=/var/lib/easy-lima/docker/configured\n":
            raise ValueError("Docker start gate missing")
        return [{"kind": "file", "path": str(gate), "sha256": sha(gate)}]
    if stage == "docker.config.v1":
        values = json.loads((ROOT / "owned" / "docker-config.json").read_bytes())
        if "target" in values: values = values["target"]
        effective = json.loads(Path("/etc/docker/daemon.json").read_bytes())
        if any(effective.get(k) != v for k, v in values.items()) or Path("/proc/sys/net/ipv4/ip_forward").read_text().strip() != "1":
            raise ValueError("Docker networking mismatch")
        return [{"kind": "docker_configuration", "backend": "iptables", "ip_forward_no_drop": True}]
    if stage == "access.groups.v1":
        groups = run(["id", "-nG", p["user"]], capture=True).split()
        expected = (["lxd"] if "development" in p["presets"] else []) + (["docker"] if "docker" in p["presets"] else [])
        if not all(g in groups for g in expected):
            raise ValueError("group membership mismatch")
        return [{"kind": "groups", "user": p["user"], "groups": expected}]
    if stage == "env.verify.v1":
        for command in ("git", "ssh", "python3", "make", "jq", "rsync"):
            if not shutil.which(command):
                raise ValueError("base executable missing")
        return [{"kind": "environment", "base_tools": True, "workspace": Path("/workspace").is_dir()}]
    raise ValueError("unrecognized stage")


def reconcile(stage, data):
    # Called only after the host proves the old unit and its entire cgroup
    # inactive. This inspection authorizes only frozen, owned builtin effects.
    allowed = {"env.inspect.v1", "env.verify.v1", "base.resolve.v1", "base.install.v1", "base.runtime.v1",
               "network.resolve.v1", "dev.resolve.v1", "dev.snapd.v1", "dev.lxd.v1", "dev.juju.v1",
               "dev.pool.v1", "dev.bridge.v1", "dev.profile.v1", "docker.source.v1", "docker.config.v1",
               "docker.install.v1", "docker.service.v1", "access.groups.v1"}
    if stage not in allowed: raise ValueError("arbitrary script replay is forbidden")
    bind_networks(data)
    directory = ROOT / "environments" / data["environment_revision_id"]
    if stage in ("base.install.v1", "dev.snapd.v1"):
        package_manager_idle()
        group = "base" if stage.startswith("base.") else "dev"
        selection = validate_resolution(directory / (group + ".packages.json"))
        for package in selection["packages"]:
            if package_version(package["name"]) not in (package["before"], package["version"]): raise ValueError("unexpected package revision")
    if stage.startswith("dev.") and stage not in ("dev.resolve.v1", "dev.snapd.v1"):
        pending_snap_changes()
    if stage == "docker.install.v1":
        package_manager_idle()
        before_path = directory / "docker.before.json"
        if not before_path.exists(): raise ValueError("Docker dispatch before-state is unavailable")
        before = json.loads(before_path.read_bytes())
        for package in data["parameters"]["docker"]["packages"]:
            if package_version(package["name"]) not in (before.get(package["name"]), package["version"]): raise ValueError("unexpected Docker revision")
    # File helpers themselves check exact old/target digests before each write.
    index = ROOT / "owned" / (data["registration_id"] + ".json")
    if index.exists():
        for name, entry in json.loads(index.read_bytes()).items():
            path = Path(name)
            current = sha(path) if path.exists() else None
            accepted = (entry.get("old"), entry.get("target")) if isinstance(entry, dict) else (entry,)
            if path.is_symlink() or current not in accepted: raise ValueError("owned file was edited")
    return {"disposition":"resume_owned_substeps", "stage_id":stage, "input_sha256":sha(Path(os.environ["EASY_LIMA_INPUT"])) if "EASY_LIMA_INPUT" in os.environ else None}


def main():
    mode, input_path = sys.argv[1:]
    data = json.loads(Path(input_path).read_bytes())
    if mode == "reconcile":
        print(json.dumps(reconcile(data["stage_id"], data), separators=(",", ":")))
    elif mode == "verify":
        print(json.dumps(verify(data["stage_id"], data), separators=(",", ":")))
    else:
        apply(mode, data)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.stderr.write("easy-lima setup: stage postconditions are incomplete\n")
        sys.exit(75)

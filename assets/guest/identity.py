#!/usr/bin/env python3
"""Apply/remove only the guest identity fragment owned by easy-lima."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile


def digest(data):
    return hashlib.sha256(data).hexdigest()


def atomic(path, data, mode=0o600):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd, temporary = tempfile.mkstemp(dir=path.parent)
    try:
        os.fchmod(fd, mode)
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        fd = os.open(path.parent, os.O_DIRECTORY)
        os.fsync(fd)
        os.close(fd)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def main():
    raw = sys.stdin.buffer.read(256 * 1024 + 1)
    if len(raw) > 256 * 1024:
        raise ValueError("bundle too large")
    data = json.loads(raw)
    home = Path.home()
    base = home / ".config/easy-lima"
    manifest = base / "identity.json"
    old = json.loads(manifest.read_bytes()) if manifest.exists() else {"files": {}, "include": None}
    intent_path = base / "identity.intent.json"
    pending = json.loads(intent_path.read_bytes()) if intent_path.exists() else None
    allowed = {"gitconfig", "public_key", "allowed_signers", "ssh_config", "github_known_hosts", "signing-wrapper"}
    if pending is not None:
        if data["action"] != "check":
            requested = {name: digest(value.encode()) for name, value in data["files"].items()}
            if pending["action"] != data["action"] or pending["target"] != requested or pending["revision_id"] != data["revision_id"]:
                raise ValueError("another identity publication is pending")
        old = pending["old"]
    if manifest.is_symlink() or base.is_symlink() or base.parent.is_symlink() or intent_path.is_symlink():
        raise ValueError("identity directory is a symlink")
    if not set(old["files"]).issubset(allowed):
        raise ValueError("invalid owned paths")
    for name in set(old["files"]) | (set(pending["target"]) if pending else set()):
        p = base / name
        expected = {old["files"].get(name)}
        if pending:
            expected.add(pending["target"].get(name))
        actual = digest(p.read_bytes()) if p.exists() else None
        if p.is_symlink() or actual not in expected:
            raise ValueError("owned identity setting changed")
    config = home / ".gitconfig"
    if config.is_symlink():
        raise ValueError("guest global Git config is a symlink")
    original = config.read_bytes() if config.exists() else b""
    include = ('\n# BEGIN easy-lima identity\n[include]\n\tpath = "' + str(base / "gitconfig").replace('\\', '\\\\').replace('"', '\\"') + '"\n# END easy-lima identity\n').encode()
    count = original.count(include)
    if count > 1:
        raise ValueError("duplicate Git include")
    if old["include"] is not None and count != 1 and not (pending and pending["action"] == "remove" and count == 0):
        raise ValueError("owned Git include was edited")
    if old["include"] is None and b"# BEGIN easy-lima identity" in original and not (pending and pending["action"] == "apply" and count == 1):
        raise ValueError("unrecorded identity include")
    owned_keys = ["user.name", "user.email", "user.useConfigOnly", "user.signingKey", "gpg.format", "gpg.ssh.program", "gpg.ssh.allowedSignersFile", "commit.gpgSign", "tag.gpgSign", "core.sshCommand", "url.git@github.com:.insteadOf"]
    # Without includes, this checks only user-owned settings in the guest file.
    if config.exists():
        for key in owned_keys:
            r = subprocess.run(["git", "config", "--file", str(config), "--no-includes", "--get-all", key], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
            if r.returncode == 0:
                raise ValueError("user-owned Git identity setting conflicts")
    if data["action"] == "check":
        print('{"ready":true,"checked_only":true}')
        return
    if data["action"] not in {"remove", "apply"}:
        raise ValueError("invalid identity action")
    if data["action"] == "remove":
        atomic(intent_path, json.dumps({"old": old, "target": {}, "revision_id": data["revision_id"], "action": "remove"}, sort_keys=True).encode())
        if old["include"] is not None:
            atomic(config, original.replace(include, b"", 1))
        for name in old["files"]:
            (base / name).unlink(missing_ok=True)
        manifest.unlink(missing_ok=True)
        intent_path.unlink(missing_ok=True)
        print('{"ready":true,"profile_free":true}')
        return
    files = data["files"]
    if set(files) != allowed:
        raise ValueError("invalid bundle")
    proposed = {name: digest(content.encode()) for name, content in files.items()}
    for name, content in files.items():
        path = base / name
        if path.exists() and name not in old["files"] and path.read_bytes() != content.encode():
            raise ValueError("unowned file conflicts")
    if config.exists() and old["include"] is None:
        backup = base / "gitconfig.before"
        if not backup.exists():
            atomic(backup, original)
    # Persist target intent before individual files; interrupted target bytes can
    # be reconciled against this exact bundle, never an edited desired profile.
    atomic(intent_path, json.dumps({"old": old, "target": proposed, "revision_id": data["revision_id"], "action": "apply"}, sort_keys=True).encode())
    for name, content in files.items():
        atomic(base / name, content.encode(), 0o700 if name == "signing-wrapper" else 0o600)
    if count == 0:
        atomic(config, original + include)
    atomic(manifest, json.dumps({"schema_version": 1, "revision_id": data["revision_id"], "files": proposed, "include": digest(include)}, sort_keys=True).encode())
    (base / "identity.intent.json").unlink(missing_ok=True)
    print('{"ready":true,"profile_free":false}')


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.stderr.write("easy-lima identity: settings conflict or incomplete publication\n")
        sys.exit(76)

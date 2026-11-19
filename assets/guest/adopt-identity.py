#!/usr/bin/env python3
"""Retire only the exact public identity settings of the pinned prototype."""
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def atomic(path, raw):
    path.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
    fd, temporary = tempfile.mkstemp(dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "wb") as stream:
            stream.write(raw)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        fd = os.open(path.parent, os.O_DIRECTORY)
        os.fsync(fd)
        os.close(fd)
    finally:
        if os.path.exists(temporary): os.unlink(temporary)


def main():
    expected = json.loads(sys.stdin.buffer.read(256 * 1024))
    home = Path.home()
    base = home / ".config/easy-lima"
    oldbase = home / ".config/canonical-local"
    ssh = home / ".ssh"
    journal = base / "legacy-retirement.json"
    config = home / ".gitconfig"
    ssh_main = ssh / "config"
    for p in (base, base.parent, oldbase, ssh, config, ssh_main, journal):
        if p.is_symlink(): raise ValueError("symlink conflict")
    if journal.exists():
        state = json.loads(journal.read_bytes())
        if state["identity"] != expected: raise ValueError("identity changed")
    else:
        metadata = oldbase / "work-identity.json"
        if metadata.is_symlink() or metadata.stat().st_size > 128 * 1024: raise ValueError("invalid metadata")
        old = json.loads(metadata.read_bytes())
        for key in ("name", "email", "github_user", "public_key"):
            if old[key].strip() != expected[key].strip(): raise ValueError("legacy identity changed")
        pub, known, ssh_config = [ssh / name for name in ("canonical-work-git.pub", "canonical-work-known-hosts", "canonical-work-git.conf")]
        allowed = oldbase / "allowed_signers"
        expected_files = {pub: old["public_key"] + "\n", known: old["known_hosts"], allowed: old["email"] + ' namespaces="git" ' + old["public_key"] + "\n",
                          ssh_config: f'''Host github.com
  User git
  IdentityFile "{pub}"
  IdentitiesOnly yes
  UserKnownHostsFile "{known}"
  GlobalKnownHostsFile /dev/null
  StrictHostKeyChecking yes
  HostKeyAlgorithms ssh-ed25519
  BatchMode yes
Host *
'''}
        for p, content in expected_files.items():
            if p.is_symlink() or p.read_bytes() != content.encode(): raise ValueError("legacy public file edited")
        keys = {"user.name": old["name"], "user.email": old["email"], "user.useConfigOnly": "true", "gpg.format": "ssh",
                "user.signingkey": str(pub), "gpg.ssh.allowedSignersFile": str(allowed), "commit.gpgsign": "true", "tag.gpgsign": "true",
                "core.sshCommand": "ssh -F " + shlex.quote(str(ssh_config)), "url.git@github.com:.insteadOf": "https://github.com/"}
        original = config.read_bytes()
        fd, temporary = tempfile.mkstemp(dir=home)
        try:
            with os.fdopen(fd, "wb") as stream: stream.write(original)
            for key, value in keys.items():
                args = ["git", "config", "--file", temporary, "--no-includes"]
                result = subprocess.run([*args, "--get-all", key], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=True)
                if result.stdout != (value + "\n").encode(): raise ValueError("legacy global setting edited")
                subprocess.run([*args, "--unset-all", key], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            target = Path(temporary).read_bytes()
        finally:
            os.unlink(temporary)
        ssh_before = ssh_main.read_bytes()
        include = ('Include "' + str(ssh_config) + '"\n').encode()
        if ssh_before.splitlines(keepends=True).count(include) != 1: raise ValueError("legacy SSH include changed")
        ssh_after = b"".join(line for line in ssh_before.splitlines(keepends=True) if line != include)
        state = {"identity": expected, "files": [{"path": str(config), "old": digest(original), "target": target.decode()},
                                               {"path": str(ssh_main), "old": digest(ssh_before), "target": ssh_after.decode()}]}
        atomic(base / "legacy-gitconfig.before", original)
        atomic(base / "legacy-ssh-config.before", ssh_before)
        atomic(journal, json.dumps(state, sort_keys=True).encode())
    for item in state["files"]:
        p = Path(item["path"])
        if p not in (config, ssh_main) or p.is_symlink(): raise ValueError("invalid target")
        raw = p.read_bytes()
        if digest(raw) not in (item["old"], digest(item["target"].encode())): raise ValueError("user edit during retirement")
        if raw != item["target"].encode(): atomic(p, item["target"].encode())
    print('{"retired":true}')


if __name__ == "__main__":
    try: main()
    except Exception:
        sys.stderr.write("easy-lima adoption: legacy identity has conflicting edits\n")
        sys.exit(76)

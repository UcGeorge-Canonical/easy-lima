#!/usr/bin/env python3
"""Configure the work identity inside a VM. Receives public settings only."""
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile


def write(path, content):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(dir=path.parent)
    try:
        with os.fdopen(fd, "w") as stream:
            stream.write(content)
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def git_config(key, value):
    subprocess.run(["git", "config", "--global", "--replace-all", key, value],
                   stdin=subprocess.DEVNULL, check=True)


def configure(data):
    home = Path.home()
    ssh = home / ".ssh"
    config_dir = home / ".config/canonical-local"
    pub = ssh / "canonical-work-git.pub"
    known_hosts = ssh / "canonical-work-known-hosts"
    ssh_config = ssh / "canonical-work-git.conf"
    allowed = config_dir / "allowed_signers"
    write(pub, data["public_key"] + "\n")
    write(known_hosts, data["known_hosts"])
    # Use this exact configuration for Git; user SSH defaults cannot add personal identities.
    write(ssh_config, f'''Host github.com
  User git
  IdentityFile "{pub}"
  IdentitiesOnly yes
  UserKnownHostsFile "{known_hosts}"
  GlobalKnownHostsFile /dev/null
  StrictHostKeyChecking yes
  HostKeyAlgorithms ssh-ed25519
  BatchMode yes
Host *
''')
    main_ssh = ssh / "config"
    original = main_ssh.read_text() if main_ssh.exists() else ""
    include = f'Include "{ssh_config}"'
    if include not in original.splitlines():
        if main_ssh.exists() and not (ssh / "config.before-canonical-work").exists():
            write(ssh / "config.before-canonical-work", original)
        write(main_ssh, include + "\n" + original)
    write(allowed, data["email"] + ' namespaces="git" ' + data["public_key"] + "\n")
    global_config = home / ".gitconfig"
    backup = home / ".gitconfig.before-canonical-work"
    if global_config.exists() and not backup.exists():
        write(backup, global_config.read_text())
    for key, value in {
        "user.name": data["name"], "user.email": data["email"], "user.useConfigOnly": "true",
        "gpg.format": "ssh", "user.signingkey": str(pub),
        "gpg.ssh.allowedSignersFile": str(allowed), "commit.gpgsign": "true", "tag.gpgsign": "true",
        "core.sshCommand": "ssh -F " + shlex.quote(str(ssh_config)),
        "url.git@github.com:.insteadOf": "https://github.com/",
    }.items():
        git_config(key, value)
    write(config_dir / "work-identity.json", json.dumps(data, indent=2) + "\n")
    print(f"Work Git: {data['name']} <{data['email']}>; expected GitHub account: {data['github_user']}")


if __name__ == "__main__":
    configure(json.loads(sys.argv[1]))

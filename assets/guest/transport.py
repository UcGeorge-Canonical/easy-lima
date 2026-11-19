#!/usr/bin/env python3
"""One-time owned transport setup, including adopted guests; no tool changes."""
import hashlib
import json
import os
from pathlib import Path
import pwd
import re
import sys
import tempfile


def atomic(path, raw, mode=0o644):
    path.parent.mkdir(parents=True, mode=0o755, exist_ok=True)
    fd, temporary = tempfile.mkstemp(dir=path.parent)
    try:
        os.fchmod(fd, mode)
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
    vm_id, username = sys.argv[1:]
    if not re.fullmatch("[a-f0-9]{32}", vm_id): raise ValueError("invalid VM")
    user = pwd.getpwnam(username)
    runtime = Path("/run/easy-lima") / vm_id
    if runtime.is_symlink(): raise ValueError("runtime symlink")
    if runtime.exists() and runtime.stat().st_uid not in (0, user.pw_uid): raise ValueError("runtime owner")
    runtime.mkdir(parents=True, mode=0o700, exist_ok=True)
    os.chown(runtime, user.pw_uid, user.pw_gid)
    os.chmod(runtime, 0o700)
    desired = {"/etc/cloud/cloud.cfg.d/99-easy-lima-host-keys.cfg": b"ssh_deletekeys: false\n",
               "/etc/tmpfiles.d/easy-lima-" + vm_id + ".conf": f"d /run/easy-lima 0755 root root -\nd {runtime} 0700 {user.pw_uid} {user.pw_gid} -\n".encode()}
    for name, raw in desired.items():
        p = Path(name)
        if p.is_symlink() or p.exists() and p.read_bytes() != raw: raise ValueError("transport setting edited")
    # All conflicts are checked before publication. Exact target bytes allow
    # completion after an interruption, without replacing any third value.
    for name, raw in desired.items():
        p = Path(name)
        if not p.exists(): atomic(p, raw)
    print(json.dumps({"ready": True, "uid": user.pw_uid}))


if __name__ == "__main__":
    try: main()
    except Exception:
        sys.stderr.write("easy-lima transport: owned setting conflict\n")
        sys.exit(76)

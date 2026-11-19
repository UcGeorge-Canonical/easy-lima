#!/usr/bin/env python3
"""Durable, bounded guest stage execution. No raw output is persisted by default."""
import argparse
import collections
import datetime
import fcntl
import hashlib
import json
import os
import pwd
from pathlib import Path
import re
import selectors
import socket
import subprocess
import sys
import tempfile
import errno
import stat
import time


def verifier(helper, input_path, env, timeout=90):
    """Bound evidence while draining; a broken verifier cannot exhaust memory."""
    child = subprocess.Popen(["/usr/bin/python3", str(helper), "verify", str(input_path)],
                             stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, env=env)
    output = bytearray()
    deadline = time.monotonic() + timeout
    try:
        with selectors.DefaultSelector() as selector:
            os.set_blocking(child.stdout.fileno(), False)
            selector.register(child.stdout, selectors.EVENT_READ)
            while True:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    return None
                if not selector.select(min(remaining, 0.2)):
                    continue
                chunk = os.read(child.stdout.fileno(), 8192)
                if not chunk:
                    break
                output.extend(chunk)
                if len(output) > LIMIT:
                    return None
        if child.wait(timeout=max(0.01, deadline - time.monotonic())) != 0:
            return None
        value = json.loads(output)
        return value if isinstance(value, list) and len(value) <= 512 else None
    except (ValueError, UnicodeError, subprocess.TimeoutExpired):
        return None
    finally:
        if child.poll() is None:
            child.kill()
        child.wait()
        child.stdout.close()

LIMIT = 256 * 1024
ID = re.compile(r"[a-f0-9]{32}\Z")
STAGE = re.compile(r"[a-z][a-z0-9.-]{0,79}\Z")
ROOT = Path("/var/lib/easy-lima/operations")
RUN = Path("/run/easy-lima/operations")
FIELDS = {"schema_version", "registration_id", "operation_id", "environment_revision_id",
          "stage_id", "stage_ordinal", "attempt", "script_sha256", "runner_sha256",
          "helper_sha256", "parameters"}


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def read_input(path):
    raw = path.read_bytes()
    if len(raw) > LIMIT:
        raise ValueError("input too large")
    data = json.loads(raw)
    if set(data) != FIELDS or data["schema_version"] != 1:
        raise ValueError("unsupported input")
    for key in ("registration_id", "operation_id", "environment_revision_id"):
        if not isinstance(data[key], str) or not ID.fullmatch(data[key]):
            raise ValueError("invalid identifier")
    if not STAGE.fullmatch(data["stage_id"]):
        raise ValueError("invalid stage")
    for key in ("stage_ordinal", "attempt"):
        if type(data[key]) is not int or not 1 <= data[key] <= 10000:
            raise ValueError("invalid ordinal")
    return data, hashlib.sha256(raw).hexdigest()


def atomic(path, value):
    raw = json.dumps(value, sort_keys=True, separators=(",", ":")).encode() + b"\n"
    if len(raw) > LIMIT:
        raise ValueError("receipt too large")
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    fd, name = tempfile.mkstemp(prefix="." + path.name + ".", dir=path.parent)
    tmp = Path(name)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(raw)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(tmp, path)
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        tmp.unlink(missing_ok=True)


def execute(input_path, script, helper):
    data, input_hash = read_input(input_path)
    if digest(script) != data["script_sha256"] or digest(Path(__file__)) != data["runner_sha256"] or digest(helper) != data["helper_sha256"]:
        raise ValueError("artifact digest mismatch")
    operation_dir = ROOT / data["operation_id"]
    operation_dir.mkdir(parents=True, mode=0o700, exist_ok=True)
    lock = os.open(operation_dir / "executor.lock", os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        directory = operation_dir / data["stage_id"] / str(data["attempt"])
        directory.mkdir(parents=True, mode=0o700, exist_ok=True)
        receipt_path = directory / "receipt.json"
        if receipt_path.exists():
            old = json.loads(receipt_path.read_bytes())
            if old["input_sha256"] != input_hash:
                raise ValueError("attempt input changed")
            # Never replay an ambiguous script, even if the prior runner died.
            return 0 if old["phase"] == "verified" else 76
        atomic(directory / "input.json", data)
        receipt = {"schema_version": 1, **{k: data[k] for k in ("registration_id", "operation_id", "environment_revision_id", "stage_id", "stage_ordinal", "attempt", "script_sha256", "runner_sha256")},
                   "input_sha256": input_hash, "boot_id": Path("/proc/sys/kernel/random/boot_id").read_text().strip(),
                   "unit": f'el-{data["operation_id"]}-s{data["stage_ordinal"]}-a{data["attempt"]}.service',
                   "invocation_id": os.environ.get("INVOCATION_ID", ""), "phase": "prepared",
                   "started_at": now(), "finished_at": None, "exit_code": None, "result_code": "stage.prepared",
                   "substeps": [], "postconditions": [], "output_dropped_bytes": 0}
        atomic(receipt_path, receipt)
        runtime = RUN / data["operation_id"]
        runtime.mkdir(parents=True, mode=0o700, exist_ok=True)
        sock_path = runtime / (str(data["stage_ordinal"]) + ".sock")
        if sock_path.exists():
            old = sock_path.lstat()
            if not stat.S_ISSOCK(old.st_mode) or old.st_uid != os.getuid(): raise ValueError("foreign output socket")
            peer = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            peer.settimeout(1)
            try:
                peer.connect(str(sock_path))
                raise ValueError("prior runner remains active")
            except OSError as error:
                if error.errno not in (errno.ECONNREFUSED, errno.ENOENT): raise
            finally: peer.close()
            if sock_path.exists(): sock_path.unlink()
        server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        server.bind(str(sock_path))
        os.chmod(sock_path, 0o600)
        server.listen(2)
        server.setblocking(False)
        selector = selectors.DefaultSelector()
        selector.register(server, selectors.EVENT_READ, "listener")
        clients = []
        ring = collections.deque()
        ring_size = 0
        env = {"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "LANG": "C.UTF-8", "DEBIAN_FRONTEND": "noninteractive",
               "EASY_LIMA_STAGE": data["stage_id"], "EASY_LIMA_INPUT": str(input_path), "EASY_LIMA_HELPER": str(helper)}
        receipt["phase"] = "may_have_effects"
        receipt["result_code"] = "stage.dispatched"
        atomic(receipt_path, receipt)
        command = ["/bin/bash", str(script)]
        source = None
        if data["parameters"].get("native_mode") == "user":
            user = pwd.getpwnam(data["parameters"]["user"])
            env.update(HOME=user.pw_dir, USER=user.pw_name, LOGNAME=user.pw_name)
            command = ["runuser", "-u", user.pw_name, "--", "/bin/bash"]
            source = script.open("rb")
        child = subprocess.Popen(command, stdin=source if source else subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env)
        if source: source.close()
        for stream in (child.stdout, child.stderr):
            os.set_blocking(stream.fileno(), False)
            selector.register(stream, selectors.EVENT_READ, "output")
        streams = 2
        while streams:
            for key, _ in selector.select(0.2):
                if key.data == "listener":
                    peer, _ = server.accept()
                    peer.setblocking(False)
                    if len(clients) >= 2:
                        peer.close()
                        continue
                    try:
                        peer.send(b"".join(ring))
                        clients.append(peer)
                    except (BlockingIOError, BrokenPipeError):
                        peer.close()
                    continue
                chunk = os.read(key.fileobj.fileno(), 8192)
                if not chunk:
                    selector.unregister(key.fileobj)
                    key.fileobj.close()
                    streams -= 1
                    continue
                ring.append(chunk)
                ring_size += len(chunk)
                while ring_size > 65536:
                    removed = ring.popleft()
                    ring_size -= len(removed)
                    receipt["output_dropped_bytes"] += len(removed)
                for peer in clients[:]:
                    try:
                        sent = peer.send(chunk)
                        receipt["output_dropped_bytes"] += len(chunk) - sent
                    except (BlockingIOError, BrokenPipeError, ConnectionResetError):
                        receipt["output_dropped_bytes"] += len(chunk)
                        clients.remove(peer)
                        peer.close()
        code = child.wait()
        selector.close()
        server.close()
        sock_path.unlink(missing_ok=True)
        for peer in clients:
            peer.close()
        # The helper independently inspects postconditions; exit 0 alone is not
        # a success receipt. Verification stdout is bounded structured evidence.
        evidence = verifier(helper, input_path, env) if code == 0 else None
        verified = evidence is not None
        if verified:
            receipt["postconditions"] = evidence
        receipt.update(phase="verified" if verified else "failed", exit_code=code, finished_at=now(), result_code="stage.verified" if verified else "stage.failed")
        atomic(receipt_path, receipt)
        return 0 if verified else 75
    finally:
        os.close(lock)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["run", "inspect", "stream"])
    parser.add_argument("input", type=Path)
    parser.add_argument("script", nargs="?", type=Path)
    parser.add_argument("helper", nargs="?", type=Path)
    args = parser.parse_args()
    data, _ = read_input(args.input)
    if args.action == "run":
        return execute(args.input, args.script, args.helper)
    if args.action == "inspect":
        path = ROOT / data["operation_id"] / data["stage_id"] / str(data["attempt"]) / "receipt.json"
        if not path.exists():
            print(json.dumps({"schema_version": 1, "phase": "absent"}))
            return 0
        raw = path.read_bytes()
        if len(raw) > LIMIT:
            return 75
        sys.stdout.buffer.write(raw)
        return 0
    peer = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    peer.settimeout(10)
    peer.connect(str(RUN / data["operation_id"] / (str(data["stage_ordinal"]) + ".sock")))
    while chunk := peer.recv(8192):
        sys.stdout.buffer.write(chunk)
        sys.stdout.buffer.flush()
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        # Exception text may contain script output or user data.
        sys.stderr.write("easy-lima stage runner: operation incomplete\n")
        sys.exit(75)

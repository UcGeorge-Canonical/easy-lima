"""Real PTY first-run regression. No VM is created or started."""
import errno
import fcntl
import os
from pathlib import Path
import pty
import re
import select
import signal
import shutil
import struct
import sys
import tempfile
import termios
import time
import unittest

BINARY = Path(__file__).resolve().parents[2] / "bin/easy-lima"
ANSI = re.compile(rb"\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07]*(?:\x07|\x1b\\)")

class Session:
    def __init__(self, home, cwd):
        self.raw = bytearray()
        # Keep one executable inode for the session even while a developer rebuilds.
        executable = Path(home) / "easy-lima-test"
        shutil.copy2(BINARY, executable)
        self.pid, self.fd = pty.fork()
        if not self.pid:
            os.chdir(cwd)
            env = dict(os.environ, HOME=str(home), TERM="xterm-256color")
            for key in ("XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"):
                env.pop(key, None)
            os.execve(executable, [str(executable)], env)
        fcntl.ioctl(self.fd, termios.TIOCSWINSZ, struct.pack("HHHH", 36, 120, 0, 0))

    def send(self, data):
        os.write(self.fd, data)

    def expect(self, text, timeout=30):
        print("PTY: waiting for " + text, flush=True)
        end = time.monotonic() + timeout
        while time.monotonic() < end:
            if text in ANSI.sub(b"", bytes(self.raw)).decode("utf-8", "replace"):
                return
            if select.select([self.fd], [], [], .1)[0]:
                try:
                    chunk = os.read(self.fd, 65536)
                except OSError as error:
                    if error.errno == errno.EIO:
                        break
                    raise
                self.raw.extend(chunk)
                # Answer terminal feature probes without requiring a GUI terminal.
                if b"\x1b[6n" in chunk:
                    self.send(b"\x1b[1;1R")
                if b"\x1b[c" in chunk:
                    self.send(b"\x1b[?1;2c")
        raise AssertionError(f"Missing {text!r}; terminal tail:\n" + ANSI.sub(b"", bytes(self.raw[-6000:])).decode("utf-8", "replace"))

    def close(self):
        # Closing the master also wakes a child blocked on terminal teardown.
        os.close(self.fd)
        for signum in (signal.SIGTERM, signal.SIGKILL):
            try:
                os.kill(self.pid, signum)
            except ProcessLookupError:
                break
            end = time.monotonic() + 2
            while time.monotonic() < end:
                if os.waitpid(self.pid, os.WNOHANG)[0]:
                    return
                time.sleep(.05)
        os.waitpid(self.pid, 0)

class FirstRunTests(unittest.TestCase):
    def test_picker_preview_cancel_and_complete(self):
        with tempfile.TemporaryDirectory(prefix="el-ui-", dir="/tmp") as temp:
            root = Path(temp)
            home = root / "home"
            project = root / "project"
            home.mkdir(); project.mkdir()
            vmroot = root / "chosen vm storage"
            for approve in (False, True):
                session = Session(home, project)
                try:
                    session.expect("GiB free")
                    session.expect("Enter another path")
                    session.send(b"\x1b[B" * 100 + b"\r")
                    session.expect("Directory path")
                    session.send(b"\x1b[200~" + str(vmroot).encode() + b"\x1b[201~\r")
                    session.expect("How should Enter open")
                    session.send(b"\x1b[B\r")
                    session.expect("Apply this plan? (yes/no)")
                    self.assertIn("storage.image_cache", session.raw.decode("utf-8", "replace"))
                    self.assertFalse(vmroot.exists(), "picker changed disk before approval")
                    session.send(b"yes\r" if approve else b"no\r")
                    if approve:
                        session.expect("Reusable Git identity")
                        session.send(b"\r")
                        session.expect("Hidden:")
                        session.expect("Profiles [p]")
                        self.assertTrue((vmroot / "_easy-lima/namespace.json").is_file())
                        session.send(b"q")
                    else:
                        session.expect("declined")
                        self.assertFalse(vmroot.exists())
                finally:
                    session.close()

if __name__ == "__main__":
    unittest.main()

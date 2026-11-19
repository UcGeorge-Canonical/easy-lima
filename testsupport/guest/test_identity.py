import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[2] / "assets/guest/identity.py"


class IdentityTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="eli-", dir="/tmp")
        self.addCleanup(self.tmp.cleanup)
        self.home = Path(self.tmp.name)
        self.bundle = {"action": "apply", "revision_id": "a" * 32, "files": {
            "gitconfig": '[user]\n name = Test Developer\n email = test@example.invalid\n',
            "public_key": "ssh-ed25519 placeholder", "allowed_signers": "test", "ssh_config": "Host github.com\n",
            "github_known_hosts": "", "signing-wrapper": "#!/bin/sh\nexit 0\n"}}

    def run_bundle(self, bundle=None):
        env = {"PATH": os.environ["PATH"], "HOME": str(self.home), "GIT_CONFIG_NOSYSTEM": "1"}
        return subprocess.run([sys.executable, str(SCRIPT)], input=json.dumps(bundle or self.bundle).encode(),
                              env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def test_preserves_unowned_config_and_rejects_owned_edit(self):
        original = b"[alias]\n st = status\n"
        (self.home / ".gitconfig").write_bytes(original)
        self.assertEqual(self.run_bundle().returncode, 0)
        self.assertTrue((self.home / ".gitconfig").read_bytes().startswith(original))
        path = self.home / ".config/easy-lima/gitconfig"
        path.write_text("edited")
        self.assertNotEqual(self.run_bundle().returncode, 0)
        self.assertEqual(path.read_text(), "edited")

    def test_unassign_removes_only_owned_settings(self):
        original = b"[alias]\n st = status\n"
        (self.home / ".gitconfig").write_bytes(original)
        self.assertEqual(self.run_bundle().returncode, 0)
        result = self.run_bundle({"action": "remove", "revision_id": "", "files": {}})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.home / ".gitconfig").read_bytes(), original)
        self.assertEqual(self.run_bundle({"action": "remove", "revision_id": "", "files": {}}).returncode, 0)

    def test_user_identity_and_symlinks_are_conflicts(self):
        (self.home / ".gitconfig").write_text('[user]\n name = Someone Else\n')
        self.assertNotEqual(self.run_bundle().returncode, 0)
        (self.home / ".gitconfig").unlink()
        (self.home / ".config").symlink_to(self.home)
        self.assertNotEqual(self.run_bundle().returncode, 0)


if __name__ == "__main__":
    unittest.main()

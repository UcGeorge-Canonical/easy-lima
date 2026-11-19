import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[2] / "assets/guest/git-check.py"

class GitCheckTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="el-git-check-")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.env = {"PATH": os.environ["PATH"], "HOME": str(self.root), "GIT_CONFIG_NOSYSTEM": "1"}
        self.expected = {"user.name": "Fixture Developer", "user.email": "fixture@example.invalid",
                         "user.signingkey": "/fixture/key.pub", "gpg.format": "ssh",
                         "gpg.ssh.program": "/fixture/signing-wrapper", "commit.gpgsign": "true",
                         "tag.gpgsign": "true", "core.sshCommand": "ssh -F /fixture/config"}
        for key, value in self.expected.items():
            self.git("config", "--global", key, value)

    def git(self, *args):
        return subprocess.run(["git", "-C", str(self.root), *args], env=self.env, check=True,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def check(self):
        result = subprocess.run([sys.executable, str(SCRIPT)], env=self.env,
                                input=json.dumps({"expected": self.expected, "repositories": [str(self.root)]}).encode(),
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True)
        return json.loads(result.stdout)["repositories"][0]

    def test_non_git_folder_is_valid_and_diagnostics_do_not_write(self):
        before = (self.root / ".gitconfig").read_bytes()
        result = self.check()
        self.assertFalse(result["repository"])
        self.assertEqual(result["conflicts"], [])
        self.assertEqual(before, (self.root / ".gitconfig").read_bytes())
        self.assertFalse((self.root / ".git").exists())

    def test_effective_repository_overrides_report_origin_without_values(self):
        self.git("init", "-q")
        self.git("config", "user.signingkey", "private-value-must-not-be-printed")
        self.git("config", "author.email", "wrong@example.invalid")
        original = (self.root / ".git/config").read_bytes()
        result = self.check()
        self.assertTrue(result["repository"])
        self.assertEqual({c["field"] for c in result["conflicts"]}, {"user.signingkey", "author.email"})
        self.assertTrue(all(".git/config" in c["origin"] for c in result["conflicts"]))
        self.assertNotIn("private-value", json.dumps(result))
        self.assertEqual(original, (self.root / ".git/config").read_bytes())

if __name__ == "__main__":
    unittest.main()

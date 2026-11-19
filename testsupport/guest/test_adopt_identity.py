import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[2] / "assets/guest/adopt-identity.py"


class LegacyIdentityTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="el-adopt-")
        self.addCleanup(self.tmp.cleanup)
        self.home = Path(self.tmp.name)
        self.env = dict(os.environ, HOME=str(self.home), GIT_CONFIG_NOSYSTEM="1")
        self.ssh = self.home / ".ssh"
        self.oldbase = self.home / ".config/canonical-local"
        self.ssh.mkdir()
        self.oldbase.mkdir(parents=True)
        old = {"name":"Legacy Developer", "email":"legacy@example.invalid", "github_user":"LegacyWork", "public_key":"ssh-ed25519 fixture", "known_hosts":"known host fixture\n"}
        self.expected = {key:old[key] for key in ("name","email","github_user","public_key")}
        (self.oldbase / "work-identity.json").write_text(json.dumps(old))
        pub = self.ssh / "canonical-work-git.pub"
        known = self.ssh / "canonical-work-known-hosts"
        conf = self.ssh / "canonical-work-git.conf"
        allowed = self.oldbase / "allowed_signers"
        pub.write_text(old["public_key"]+"\n")
        known.write_text(old["known_hosts"])
        allowed.write_text(old["email"]+' namespaces="git" '+old["public_key"]+"\n")
        conf.write_text(f'''Host github.com
  User git
  IdentityFile "{pub}"
  IdentitiesOnly yes
  UserKnownHostsFile "{known}"
  GlobalKnownHostsFile /dev/null
  StrictHostKeyChecking yes
  HostKeyAlgorithms ssh-ed25519
  BatchMode yes
Host *
''')
        (self.ssh / "config").write_text(f'Include "{conf}"\nHost private-host\n  User someone\n')
        self.config = self.home / ".gitconfig"
        keys = {"user.name":old["name"], "user.email":old["email"], "user.useConfigOnly":"true", "gpg.format":"ssh", "user.signingkey":str(pub), "gpg.ssh.allowedSignersFile":str(allowed), "commit.gpgsign":"true", "tag.gpgsign":"true", "core.sshCommand":"ssh -F "+shlex.quote(str(conf)), "url.git@github.com:.insteadOf":"https://github.com/", "alias.st":"status"}
        for k,v in keys.items():
            subprocess.run(["git","config","--file",str(self.config),k,v],env=self.env,check=True)

    def run_retirement(self):
        return subprocess.run([sys.executable,str(SCRIPT)],input=json.dumps(self.expected).encode(),env=self.env,capture_output=True)

    def test_retirement_preserves_user_settings_and_is_recoverable(self):
        original = self.config.read_bytes()
        self.assertEqual(self.run_retirement().returncode,0)
        self.assertIn(b"st = status",self.config.read_bytes())
        self.assertEqual((self.ssh / "config").read_text(),"Host private-host\n  User someone\n")
        self.assertEqual((self.home / ".config/easy-lima/legacy-gitconfig.before").read_bytes(),original)
        # Restore one admitted old file to model an interruption between writes.
        self.config.write_bytes(original)
        self.assertEqual(self.run_retirement().returncode,0)
        self.assertNotIn(b"Legacy Developer",self.config.read_bytes())
        self.assertEqual(self.run_retirement().returncode,0)

    def test_user_edit_blocks_retirement_without_overwriting(self):
        subprocess.run(["git","config","--file",str(self.config),"user.name","User edit"],env=self.env,check=True)
        before=self.config.read_bytes()
        self.assertNotEqual(self.run_retirement().returncode,0)
        self.assertEqual(self.config.read_bytes(),before)


if __name__ == "__main__": unittest.main()

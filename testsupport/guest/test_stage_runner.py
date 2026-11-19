"""Exercise the real guest runner with harmless scripts and private temp roots."""
import hashlib
import importlib.util
import json
import pathlib
import tempfile
import unittest
from unittest import mock

ASSETS = pathlib.Path(__file__).resolve().parents[2] / "assets/guest"


def load(name):
    spec = importlib.util.spec_from_file_location(name, ASSETS / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class RunnerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="elr-", dir="/tmp")
        self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name)
        self.runner = load("stage-runner")
        self.runner.ROOT = self.root / "ops"
        self.runner.RUN = self.root / "run"
        boot = self.root / "boot"
        boot.write_text("00000000-0000-0000-0000-000000000000")
        self.runner.Path = lambda p: boot if str(p) == "/proc/sys/kernel/random/boot_id" else pathlib.Path(p)

    def execute(self, script, helper):
        sp, hp, ip = [self.root / name for name in ("script.sh", "helper.py", "input.json")]
        sp.write_text(script)
        hp.write_text(helper)
        data = {"schema_version": 1, "registration_id": "a" * 32, "operation_id": "b" * 32,
                "environment_revision_id": "c" * 32, "stage_id": "fixture.v1", "stage_ordinal": 1, "attempt": 1,
                "script_sha256": self.runner.digest(sp), "runner_sha256": self.runner.digest(ASSETS / "stage-runner.py"),
                "helper_sha256": self.runner.digest(hp), "parameters": {}}
        ip.write_text(json.dumps(data))
        return self.runner.execute(ip, sp, hp)

    def receipt(self):
        return json.loads((self.runner.ROOT / ("b" * 32) / "fixture.v1/1/receipt.json").read_text())

    def test_verified_receipt_and_no_default_raw_capture(self):
        script = 'printf "password=super-secret\\n"\nhead -c 2000000 /dev/zero\n'
        self.assertEqual(self.execute(script, 'print("[]")'), 0)
        receipt = self.receipt()
        self.assertEqual(receipt["phase"], "verified")
        self.assertGreater(receipt["output_dropped_bytes"], 1000000)
        for file in self.runner.ROOT.rglob("*"):
            if file.is_file():
                self.assertNotIn(b"super-secret", file.read_bytes())
        self.assertEqual(self.execute(script, 'print("[]")'), 0)

    def test_invalid_verifier_output_is_durable_failure_and_never_replayed(self):
        marker = self.root / "effect"
        script = "printf x >> '" + str(marker) + "'\n"
        helper = 'print("running\\n[]")'
        self.assertEqual(self.execute(script, helper), 75)
        self.assertEqual(self.receipt()["phase"], "failed")
        self.assertEqual(self.execute(script, helper), 76)
        self.assertEqual(marker.read_text(), "x")

    def test_changed_frozen_input_is_rejected(self):
        self.assertEqual(self.execute("true", 'print("[]")'), 0)
        with self.assertRaisesRegex(ValueError, "input changed"):
            self.execute("false", 'print("[]")')

    def test_oversized_or_stalled_verifier_is_a_durable_failure(self):
        self.assertEqual(self.execute("true", 'print("x" * 2000000)'), 75)
        self.assertEqual(self.receipt()["phase"], "failed")
        helper = self.root / "sleep.py"
        helper.write_text("import time; time.sleep(60)")
        self.assertIsNone(self.runner.verifier(helper, self.root / "input.json", {}, timeout=0.05))


class OwnedFilesTests(unittest.TestCase):
    def test_interrupted_owned_file_accepts_old_or_target_only(self):
        setup = load("setup")
        with tempfile.TemporaryDirectory(prefix="elo-", dir="/tmp") as directory:
            root = pathlib.Path(directory)
            setup.ROOT = root / "state"
            target = root / "owned.conf"
            setup.owned(target, b"old", "fixture")
            index = setup.ROOT / "owned/fixture.json"
            state = {str(target): {"old": setup.sha(target), "target": hashlib.sha256(b"new").hexdigest()}}
            setup.save(index, state)
            setup.owned(target, b"new", "fixture")
            self.assertEqual(target.read_bytes(), b"new")
            target.write_bytes(b"user edit")
            with self.assertRaisesRegex(ValueError, "edited"):
                setup.owned(target, b"new", "fixture")

    def test_builtin_reconciliation_checks_partial_packages_and_rejects_scripts(self):
        setup = load("setup")
        with tempfile.TemporaryDirectory(prefix="elo-", dir="/tmp") as directory:
            setup.ROOT = pathlib.Path(directory)
            data = {"registration_id": "a" * 32, "environment_revision_id": "b" * 32, "parameters": {}}
            selection = {"packages": [{"name": "fixture", "before": "1", "version": "2"}]}
            with mock.patch.object(setup, "package_manager_idle"), mock.patch.object(setup, "validate_resolution", return_value=selection), mock.patch.object(setup, "package_version", return_value="2"):
                self.assertEqual(setup.reconcile("base.install.v1", data)["disposition"], "resume_owned_substeps")
                with mock.patch.object(setup, "package_version", return_value="3"):
                    with self.assertRaisesRegex(ValueError, "unexpected package"):
                        setup.reconcile("base.install.v1", data)
            with self.assertRaisesRegex(ValueError, "arbitrary script"):
                setup.reconcile("native.0001.v1", data)

    def test_snap_dispatch_records_before_state_and_reuses_exact_revision(self):
        setup = load("setup")
        with tempfile.TemporaryDirectory(prefix="elo-", dir="/tmp") as directory:
            setup.ROOT = pathlib.Path(directory)
            spec = {"id": "publisher-id", "revision": "123", "channel": "3/stable"}
            data = {"parameters": {"snaps": {"juju": spec}}}
            marker = setup.ROOT / "owned/snap-juju.json"
            def dispatch(args, **kwargs):
                self.assertEqual(json.loads(marker.read_bytes())["revision"], "123")
                if args[1] == "install":
                    raise RuntimeError("lost connection after snap dispatch")
            with mock.patch.object(setup, "pending_snap_changes"), mock.patch.object(setup, "snap_state", side_effect=setup.subprocess.CalledProcessError(22, "curl")), mock.patch.object(setup, "run", side_effect=dispatch):
                with self.assertRaisesRegex(RuntimeError, "lost connection"):
                    setup.set_snap(data, "juju")
            with mock.patch.object(setup, "pending_snap_changes"), mock.patch.object(setup, "snap_state", return_value={"id": "publisher-id", "revision": "123"}), mock.patch.object(setup, "run") as run:
                setup.set_snap(data, "juju")
                run.assert_called_once_with(["snap", "refresh", "--hold=forever", "juju"])


if __name__ == "__main__":
    unittest.main()

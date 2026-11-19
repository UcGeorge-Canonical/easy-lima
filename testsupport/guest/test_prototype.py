"""Use the actual pinned legacy ownership gate to test downgrade refusal."""
import hashlib
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest


class PrototypeOwnershipTests(unittest.TestCase):
    def test_retired_record_cannot_reacquire_existing_vm(self):
        path = Path(__file__).resolve().parents[2] / "testdata/prototype/v1/local.py"
        self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), "55999dc949a226ad73cf3bd0084fa6eb4164162e9f296445c14f6b01e6d0333f")
        spec = importlib.util.spec_from_file_location("legacy_fixture", path)
        legacy = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = legacy
        spec.loader.exec_module(legacy)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "project").mkdir()
            project = legacy.Project(root, "project", root / "project")
            launcher = legacy.Manager(root, lima_home=root / "vms", ssh_config=root / "ssh")
            record = launcher.record_path(project)
            record.parent.mkdir(parents=True)
            record.write_text(json.dumps({"name":project.name, "relative":project.relative, "source":str(project.source)}))
            self.assertIsNotNone(launcher.check_ownership(project, {project.name:{"status":"Stopped"}}))
            archived = root / ".local-state/retired/handoff/project.json"
            archived.parent.mkdir(parents=True)
            record.rename(archived)
            for status in ("Running", "Stopped", "Broken"):
                with self.assertRaisesRegex(legacy.LocalError, "not owned"):
                    launcher.check_ownership(project, {project.name:{"status":status}})

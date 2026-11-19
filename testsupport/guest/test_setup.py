import importlib.util
from pathlib import Path
import subprocess
import sys
import unittest

spec = importlib.util.spec_from_file_location("guest_setup", Path(__file__).resolve().parents[2] / "assets/guest/setup.py")
setup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(setup)


class CaptureTests(unittest.TestCase):
    def test_capture_bounds_and_deadline(self):
        with self.assertRaises(ValueError):
            setup.run([sys.executable, "-c", "import sys; sys.stdout.write('x'*300000)"], capture=True, timeout=5)
        with self.assertRaises(subprocess.TimeoutExpired):
            setup.run([sys.executable, "-c", "import time; time.sleep(30)"], capture=True, timeout=0.05)
        self.assertEqual(setup.run([sys.executable, "-c", "print('bounded')"], capture=True), "bounded")


if __name__ == "__main__": unittest.main()

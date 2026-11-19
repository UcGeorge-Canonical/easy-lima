"""Read-only effective Git identity checks; never echo unknown config values."""
import json
import os
from pathlib import Path
import subprocess
import sys

request = json.loads(sys.stdin.buffer.read(65537))
expected = request["expected"]
paths = request.get("repositories") or ["/workspace"]
results = []
truncated = False

def git(directory, *args):
    return subprocess.run(["git", "-C", directory, *args], stdout=subprocess.PIPE,
                          stderr=subprocess.DEVNULL, timeout=10)

if request.get("discover"):
    paths = []
    remaining = 256
    for directory, folders, _ in os.walk("/workspace", followlinks=False):
        remaining -= 1
        if remaining < 0:
            truncated = True
            break
        candidates = sorted(d for d in folders if d not in {".git", "node_modules", ".venv", "venv", "vendor", ".cache", "__pycache__"} and not Path(directory, d).is_symlink())
        truncated = truncated or len(candidates) > 256
        folders[:] = candidates[:256]
        if (Path(directory) / ".git").exists():
            paths.append(directory)
            if len(paths) >= 32:
                truncated = True
                break
    if not paths:
        paths = ["/workspace"]

for directory in paths[:32]:
    item = {"path": directory, "repository": False, "conflicts": [], "unavailable": False}
    if not os.path.isabs(directory) or not Path(directory).is_dir():
        item["unavailable"] = True
        results.append(item)
        continue
    item["repository"] = git(directory, "rev-parse", "--is-inside-work-tree").returncode == 0
    for key, want in expected.items():
        result = git(directory, "config", "--show-origin", "--get", key)
        text = result.stdout.decode("utf-8", "replace").rstrip("\n")[:65536]
        origin, _, value = text.partition("\t")
        if result.returncode != 0 or value != want:
            item["conflicts"].append({"field": key, "origin": origin if result.returncode == 0 else "unset"})
    for role in ("author", "committer"):
        for field in ("name", "email"):
            result = git(directory, "config", "--show-origin", "--get", role + "." + field)
            if result.returncode == 0:
                origin, _, value = result.stdout.decode("utf-8", "replace").rstrip("\n").partition("\t")
                if value != expected["user." + field]:
                    item["conflicts"].append({"field": role + "." + field, "origin": origin[:4096]})
    results.append(item)
print(json.dumps({"repositories": results, "truncated": truncated or len(paths) > 32}, separators=(",", ":")))

#!/usr/bin/env python3
"""Validate local links in current guides; frozen review snapshots are archival."""
from pathlib import Path
import argparse
import re
import sys
from urllib.parse import unquote, urlsplit

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1], help="repository or installed documentation root")
root = parser.parse_args().root.resolve()
guides = [root / "README.md", *sorted((root / "docs").glob("*.md")), *sorted((root / "docs/reference").glob("*.md")), *sorted((root / "docs/examples").glob("*.md")), root / "docs/design/README.md", root / "docs/reviews/README.md", *sorted((root / ".github").glob("*.md"))]
errors = []
for guide in guides:
    for target in re.findall(r"\[[^\]]*\]\(([^)]+)\)", guide.read_text()):
        url = urlsplit(target)
        if url.scheme or url.netloc or not url.path: continue
        path = (guide.parent / unquote(url.path)).resolve()
        if not path.is_relative_to(root) or not path.exists():
            errors.append(f"{guide.relative_to(root)}: missing local link {target}")
        elif url.fragment:
            headings = re.findall(r"^#+\s+(.+)$", path.read_text(), re.M) if path.suffix == ".md" else []
            anchors = {re.sub(r"[^\w\- ]", "", h.replace("`", "").lower()).replace(" ", "-") for h in headings}
            if unquote(url.fragment) not in anchors:
                errors.append(f"{guide.relative_to(root)}: missing anchor {target}")
if errors:
    sys.exit("\n".join(errors))
print(f"Checked local links in {len(guides)} guides.")

#!/usr/bin/env python3
"""Build one release archive; does not publish, sign, or notarize a release."""
import argparse
import gzip
import hashlib
import io
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--target", required=True, choices=["darwin/arm64", "darwin/amd64", "linux/arm64", "linux/amd64"])
    args = parser.parse_args()
    if not re.fullmatch(r"v?[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9.-]+)?|dev", args.version):
        parser.error("use a semantic version or dev")
    system, arch = args.target.split("/")
    go = os.environ.get("GO", "go")
    host = subprocess.check_output([go, "env", "GOHOSTOS"]).decode().strip()
    if system == "darwin" and host != "darwin":
        parser.error("macOS releases require a macOS SDK and native Keychain build")
    env = dict(os.environ, GOOS=system, GOARCH=arch, CGO_ENABLED="1" if system == "darwin" else "0")
    destination = ROOT / "dist"
    destination.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="el-release-") as temporary:
        binary = Path(temporary) / "easy-lima"
        subprocess.run([go, "build", "-trimpath", "-ldflags", "-s -w -X github.com/UcGeorge-Canonical/easy-lima/internal/model.Version=" + args.version, "-o", str(binary), "./cmd/easy-lima"], cwd=ROOT, env=env, check=True)
        archive = destination / ("easy-lima_" + args.version + "_" + system + "_" + arch + ".tar.gz")
        with archive.open("wb") as output, gzip.GzipFile(filename="", fileobj=output, mode="wb", mtime=0) as compressed, tarfile.open(fileobj=compressed, mode="w") as tar:
            entries = [(binary, "easy-lima")]
            entries += [(ROOT / name, name) for name in ("README.md", "LICENSE", "NOTICE", "THIRD_PARTY_NOTICES.txt")]
            entries += [(path, str(path.relative_to(ROOT))) for path in sorted((ROOT / "docs").rglob("*")) if path.is_file()]
            entries += [(ROOT / ".github" / name, ".github/" + name) for name in ("CONTRIBUTING.md", "SECURITY.md")]
            entries += [(path, "completions/" + path.name) for path in sorted((ROOT / "packaging/completions").iterdir()) if path.is_file()]
            for path, name in entries:
                data = path.read_bytes()
                info = tarfile.TarInfo(name)
                info.size = len(data)
                info.mode = 0o755 if path == binary else 0o644
                info.mtime = int(os.environ.get("SOURCE_DATE_EPOCH", "0"))
                tar.addfile(info, io.BytesIO(data))
        checksum = hashlib.sha256(archive.read_bytes()).hexdigest()
        archive.with_suffix(archive.suffix + ".sha256").write_text(checksum + "  " + archive.name + "\n")
        print(archive)


if __name__ == "__main__":
    main()

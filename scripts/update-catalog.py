#!/usr/bin/env python3
"""Maintainer-only catalog refresh; never invoked by the installed application.

Fetch publisher metadata, pin exact artifacts, and clear runtime qualification.
Review the resulting diff and run the acceptance matrix before a release.
"""
import argparse
import concurrent.futures
import gzip
import hashlib
import json
from pathlib import Path
import re
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def fetch(url, headers=None, limit=32 * 1024 * 1024):
    request = urllib.request.Request(url, headers={"User-Agent": "easy-lima-catalog/1", **(headers or {})})
    with urllib.request.urlopen(request, timeout=45) as response:
        data = response.read(limit + 1)
    if len(data) > limit:
        raise ValueError("publisher response exceeded its limit")
    return data


def version(value):
    return tuple(int(v) for v in re.findall(r"\d+", value.split("~")[0]))


def docker(arch, key):
    url = f"https://download.docker.com/linux/ubuntu/dists/noble/stable/binary-{arch}/Packages.gz"
    metadata = fetch(url)
    packages = []
    for paragraph in gzip.decompress(metadata).decode().split("\n\n"):
        row = {}
        for line in paragraph.splitlines():
            if ": " in line and not line.startswith(" "):
                field, value = line.split(": ", 1)
                row[field] = value
        if row:
            packages.append(row)
    names = ["docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"]
    selected = []
    for name in names:
        choices = [p for p in packages if p.get("Package") == name and p.get("Architecture") == arch]
        if name in ("docker-ce", "docker-ce-cli"):
            choices = [p for p in choices if re.match(r"(?:\d+:)?29\.", p["Version"])]
        item = max(choices, key=lambda p: version(p["Version"]))
        selected.append({"name": name, "version": item["Version"], "url": "https://download.docker.com/linux/ubuntu/" + item["Filename"], "sha256": item["SHA256"], "size": int(item["Size"])})
    return {"key": key.decode(), "key_sha256": hashlib.sha256(key).hexdigest(), "packages": selected}, {"url": url, "sha256": hashlib.sha256(metadata).hexdigest()}


def snaps(arch, channels):
    result, evidence = {}, []
    for name, channel in channels.items():
        url = "https://api.snapcraft.io/v2/snaps/info/" + name
        metadata = fetch(url, {"Snap-Device-Series": "16", "Snap-Device-Architecture": arch})
        data = json.loads(metadata)
        matches = [v for v in data["channel-map"] if v["channel"]["name"] == channel and v["channel"]["architecture"] == arch]
        if len(matches) != 1:
            raise ValueError(f"Expected one {name} {channel} {arch} publisher revision")
        result[name] = {"id": data["snap"]["snap-id"], "revision": str(matches[0]["revision"]), "channel": channel}
        evidence.append({"url": url, "architecture": arch, "sha256": hashlib.sha256(metadata).hexdigest()})
    return result, evidence


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true", help="replace assets/catalog.json after reviewable publisher resolution")
    args = parser.parse_args()
    path = ROOT / "assets/catalog.json"
    catalog = json.loads(path.read_bytes())
    key = fetch("https://download.docker.com/linux/ubuntu/gpg", limit=64 * 1024)
    evidence = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        tasks = {(arch, kind): pool.submit(fn, arch, value) for arch in ("arm64", "amd64") for kind, fn, value in (("docker", docker, key), ("snaps", snaps, catalog["snap_channels"]))}
        for (arch, kind), future in tasks.items():
            data, sources = future.result()
            catalog[kind][arch] = data
            evidence.extend(sources if isinstance(sources, list) else [sources])
    catalog["qualification"] = []
    if args.write:
        path.write_text(json.dumps(catalog, indent=2) + "\n")
        (ROOT / "assets/catalog-sources.json").write_text(json.dumps({"schema_version": 1, "sources": evidence}, indent=2) + "\n")
    else:
        print(json.dumps({"snaps": catalog["snaps"], "docker_versions": {arch: {p["name"]: p["version"] for p in entry["packages"]} for arch, entry in catalog["docker"].items()}}, indent=2))


if __name__ == "__main__":
    main()

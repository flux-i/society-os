#!/usr/bin/env python3
"""Retain an already-verified binary/assets together; never copy data or keys."""
import argparse
import os
from pathlib import Path
import re
import shutil
import tempfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--name", required=True, help="A unique verified checkpoint name, e.g. 0.6")
args = parser.parse_args()
if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,48}", args.name):
    parser.error("Use a simple checkpoint name, without paths.")
root = Path(__file__).resolve().parent.parent
binary, web = root / "build/society-server", root / "build/web"
if not binary.is_file() or not (web / "index.html").is_file():
    parser.error("Build and verify this checkpoint before pinning its preview.")
parent = root / "var/preview-releases"
parent.mkdir(parents=True, exist_ok=True, mode=0o700)
destination = parent / args.name
if destination.exists():
    parser.error("That checkpoint already exists; choose a new name. It will not be overwritten.")
with tempfile.TemporaryDirectory(prefix=".pin-", dir=parent) as temporary:
    staged = Path(temporary) / "release"
    staged.mkdir(mode=0o700)
    shutil.copy2(binary, staged / "society-server")
    shutil.copytree(web, staged / "web")
    os.rename(staged, destination)
print(f"Verified preview retained at var/preview-releases/{args.name}")

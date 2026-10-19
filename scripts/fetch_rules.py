"""Fetch only source-commit-pinned rule snapshots and verify before staging."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys

repo = Path(__file__).resolve().parents[1]
destination = Path(sys.argv[1])
cache = repo / ".tools"
cache.mkdir(exist_ok=True)
lock = json.loads((repo / "tools/rules.lock.json").read_text())
for name, pin in lock.items():
    path = cache / f"{name}.srs"
    if not path.exists():
        pending = path.with_suffix(".pending")
        subprocess.run(["curl", "-fsSL", "--connect-timeout", "30", "--max-time", "120", pin["url"], "-o", str(pending)], check=True)
        pending.replace(path)
    data = path.read_bytes()
    if len(data) != pin["bytes"] or hashlib.sha256(data).hexdigest() != pin["sha256"]:
        raise SystemExit(f"Rule snapshot checksum mismatch: {name}")
    (destination / f"{name}.srs").write_bytes(data)

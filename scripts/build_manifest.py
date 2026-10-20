"""Create deterministic release metadata from staged files and pinned rule sources."""
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import sys

stage, repo = map(Path, sys.argv[1:3])
epoch = int(sys.argv[3])
generation = stage / "rules/generations/initial"
lock = json.loads((repo / "tools/rules.lock.json").read_text())
rule_files = []
for p in sorted(generation.glob("*.srs")):
    data = p.read_bytes()
    source = lock.get(p.stem, {}).get("url", "local")
    rule_files.append(dict(name=p.stem, source_url=source, sha256=hashlib.sha256(data).hexdigest(), bytes=len(data), fetched_at=datetime.fromtimestamp(epoch, timezone.utc).isoformat().replace("+00:00", "Z")))
(generation / "manifest.json").write_text(json.dumps(dict(generation="initial", files=rule_files), sort_keys=True, indent=2) + "\n")
files = {str(p.relative_to(stage)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(stage.rglob("*")) if p.is_file() and not p.is_symlink()}
(stage / "manifest.json").write_text(json.dumps(dict(version=(repo / "VERSION").read_text().strip(), source_date_epoch=epoch, files=files), sort_keys=True, indent=2) + "\n")

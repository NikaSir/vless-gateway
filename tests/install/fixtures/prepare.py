"""Build synthetic installation inputs without tracking usable credentials."""
import base64
import json
from pathlib import Path
import shutil
import sys
import uuid

repo, target = map(Path, sys.argv[1:])
target.mkdir(parents=True, exist_ok=True)
for name in ("settings", "hardware", "secrets"):
    shutil.copyfile(repo / "config/examples" / f"{name}.example.json", target / f"{name}.json")
secret = json.loads((target / "secrets.json").read_text())
for p in secret.values():
    p.update(uuid=str(uuid.uuid4()), public_key=base64.urlsafe_b64encode(bytes([1] * 32)).decode().rstrip("="), short_id="ab", server_name="example.com")
(target / "secrets.json").write_text(json.dumps(secret))
hardware = json.loads((target / "hardware.json").read_text())
inventory = dict(board="friendlyarm,nanopi-r5s", distribution="FriendlyWrt", version="24.10.2", arch="arm64", sing_box_version="1.12.22", clash_support=True, links=[{"ifname": p["interface"], "address": p["mac"]} for p in hardware["ports"]], routes=[], rule_priorities=[], nft_tables=[])
(target / "inventory.json").write_text(json.dumps(inventory))

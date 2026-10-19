"""Scan release inputs without printing any suspected secret value."""
from pathlib import Path
import re
import subprocess
import sys

KEYS = re.compile(rb'"(?:uuid|public_key|short_id|api_token|clash_token)"\s*:\s*"([^"\n]+)"')
UUID = re.compile(rb'(?i)\b[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\b')

def violations(data):
    result = []
    for match in KEYS.finditer(data):
        if not match.group(1).startswith(b"REPLACE_WITH_"):
            result.append("credential value")
    if UUID.search(data):
        result.append("UUID")
    if re.search(rb'(?:vless|vmess|trojan)://', data):
        result.append("subscription URI")
    return result

def main():
    root = Path(__file__).resolve().parents[1]
    if len(sys.argv) > 1:
        base = Path(sys.argv[1])
        paths = (p for p in base.rglob("*") if p.is_file() and not p.is_symlink())
    else:
        names = subprocess.check_output(["git", "ls-files", "-z"], cwd=root).split(b"\0")
        paths = (root / n.decode() for n in names if n)
    count = 0
    for path in paths:
        if not path.is_file():
            continue
        found = violations(path.read_bytes())
        if found:
            print(f"Prohibited material in {path.name}: {', '.join(sorted(set(found)))}", file=sys.stderr)
            count += 1
    if count:
        raise SystemExit(1)
    print("Secret scan passed")

if __name__ == "__main__":
    main()

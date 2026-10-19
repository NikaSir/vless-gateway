import hashlib
import json
from pathlib import Path, PurePosixPath
import struct
import sys
import tarfile

archive = Path(sys.argv[1])
assert archive.name == "nikas-vless-gateway-1.0.0-beta001-aarch64.tar.gz"
expected = archive.with_suffix(archive.suffix + ".sha256").read_text().split()[0]
assert hashlib.sha256(archive.read_bytes()).hexdigest() == expected
with tarfile.open(archive) as tar:
    members = {}
    for m in tar.getmembers():
        name = m.name.removeprefix("./")
        assert not name.startswith("/") and ".." not in PurePosixPath(name).parts
        assert m.uid == 0 and m.gid == 0
        assert name not in members
        members[name] = m
    manifest = json.load(tar.extractfile(members["manifest.json"]))
    for m in members.values():
        assert m.mtime == manifest["source_date_epoch"]
    for path, checksum in manifest["files"].items():
        assert hashlib.sha256(tar.extractfile(members[path]).read()).hexdigest() == checksum
    for path in ("bin/gatewayd", "bin/gatewayctl"):
        m = members[path]
        assert m.mode & 0o111
        data = tar.extractfile(m).read()
        assert data[:4] == b"\x7fELF" and struct.unpack_from("<H", data, 18)[0] == 183
        assert b"1.0.0-beta001" in data
    assert members["rules/current"].linkname == "generations/initial"
    assert "config/examples/secrets.example.json" in members
    assert "etc/nikas-vless/secrets.json" not in members
print("ARM64 archive content, metadata and checksums valid")

"""Validate tracked JSON, examples, and API contract fixtures."""
import json
from pathlib import Path
import jsonschema

root = Path(__file__).resolve().parents[1]
for path in sorted((root / "config").rglob("*.json")):
    json.loads(path.read_text())
for path in sorted((root / "config/examples").glob("*.example.json")):
    schema = root / "config/schema" / path.name.replace(".example", ".schema")
    if schema.exists():
        jsonschema.validate(json.loads(path.read_text()), json.loads(schema.read_text()))
for path in sorted((root / "tests/contract/api-v1").glob("*.json")):
    endpoint = path.name.split("-")[0]
    schema = root / "config/schema" / f"api-v1-{endpoint}.schema.json"
    jsonschema.validate(json.loads(path.read_text()), json.loads(schema.read_text()))
print("JSON examples and API contract fixtures valid")

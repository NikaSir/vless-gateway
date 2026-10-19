import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("scanner", Path(__file__).resolve().parents[2] / "scripts/scan_secrets.py")
scanner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scanner)

class ScanTests(unittest.TestCase):
    def test_detects_credentials_and_allows_placeholders(self):
        for key in ("uuid", "public_key", "short_id", "api_token", "clash_token"):
            self.assertTrue(scanner.violations((f'{{"{key}":"sensitive-canary"}}').encode()))
            self.assertFalse(scanner.violations((f'{{"{key}":"REPLACE_WITH_VALUE"}}').encode()))
        self.assertTrue(scanner.violations(b"vless" + b"://private-material"))
        self.assertFalse(scanner.violations(b'{"server":"192.0.2.1"}'))

if __name__ == "__main__":
    unittest.main()

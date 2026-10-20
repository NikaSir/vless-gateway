"""DNS wire regression: dnsmasq adds EDNS to forwarded queries."""
import runpy
from pathlib import Path
import struct
import unittest

resolver = runpy.run_path(str(Path(__file__).parent / "fixtures/dns-resolver.py"))["dns_response"]


class DNSWireTests(unittest.TestCase):
    def test_edns_query_has_a_real_a_answer_not_an_echoed_opt_record(self):
        question = b"\x07example\x03com\x00\x00\x01\x00\x01"
        query = b"\x12\x34\x01\x00\x00\x01\x00\x00\x00\x00\x00\x01" + question
        query += b"\x00\x00\x29\x04\xd0\x00\x00\x00\x00\x00\x00"
        reply = resolver(query, "192.0.2.10")
        answer = 12 + len(question)
        self.assertEqual(reply[answer:answer+2], b"\xc0\x0c")
        self.assertEqual(struct.unpack("!H", reply[answer+2:answer+4])[0], 1)
        self.assertEqual(len(reply), answer + 16)


if __name__ == "__main__":
    unittest.main()

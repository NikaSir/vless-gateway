"""Assert an actual DNS reply, retrying while dnsmasq detects server loss."""
import socket
import sys
import time

expected = socket.inet_aton(sys.argv[1])
deadline = time.monotonic() + 20
last_reply = None
query = b"\x12\x34\x01\x00\x00\x01\x00\x00\x00\x00\x00\x00\x07example\x03com\x00\x00\x01\x00\x01"
with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as client:
    client.settimeout(1)
    while time.monotonic() < deadline:
        client.sendto(query, ("127.0.0.1", 1053))
        try:
            reply = client.recv(4096)
        except TimeoutError:
            continue
        last_reply = reply.hex()
        if reply[:2] == query[:2] and reply[3] & 15 == 0 and reply[-4:] == expected:
            sys.exit(0)
        time.sleep(0.05)
raise SystemExit(f"DNS did not return the expected resolver answer; last reply: {last_reply}")

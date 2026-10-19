"""Small distinguishable UDP resolver for the real dnsmasq failure test."""
import os
from pathlib import Path
import socket
import struct
import sys

def dns_response(query, answer):
    end = 12
    while query[end]:
        end += query[end] + 1
    end += 5  # Root label plus QTYPE and QCLASS; don't echo the EDNS OPT record.
    response = query[:2] + b"\x81\x80\x00\x01\x00\x01\x00\x00\x00\x00" + query[12:end]
    response += b"\xc0\x0c" + struct.pack("!HHIH", 1, 1, 0, 4) + socket.inet_aton(answer)
    return response


if __name__ == "__main__":
    address, answer = sys.argv[1:3]
    server = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    server.bind((address, int(sys.argv[4]) if len(sys.argv) > 4 else 53))
    if len(sys.argv) > 3:
        Path(sys.argv[3]).write_text(str(os.getpid()))
    while True:
        query, peer = server.recvfrom(4096)
        server.sendto(dns_response(query, answer), peer)

"""Small distinguishable UDP resolver for the real dnsmasq failure test."""
import os
from pathlib import Path
import socket
import struct
import sys

address, answer = sys.argv[1:3]
server = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
server.bind((address, 53))
if len(sys.argv) > 3:
    Path(sys.argv[3]).write_text(str(os.getpid()))
while True:
    query, peer = server.recvfrom(4096)
    response = query[:2] + b"\x81\x80\x00\x01\x00\x01\x00\x00\x00\x00" + query[12:]
    response += b"\xc0\x0c" + struct.pack("!HHIH", 1, 1, 0, 4) + socket.inet_aton(answer)
    server.sendto(response, peer)

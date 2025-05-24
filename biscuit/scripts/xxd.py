#!/usr/bin/env python3
"""Minimal replacement for `xxd -i` used during Biscuit build.

This script outputs a C-style hex dump of the input binary similar to
`xxd -i`. Only the subset required by `bin2go.sh` is implemented.
"""

import sys
from pathlib import Path

if len(sys.argv) != 2:
    sys.stderr.write("usage: xxd.py <file>\n")
    sys.exit(1)

path = sys.argv[1]
name = Path(path).name.replace('-', '_').replace('.', '_')

with open(path, 'rb') as f:
    data = f.read()

# Print array header
print(f"unsigned char {name}[] = {{")

for i, b in enumerate(data):
    end = "\n" if (i + 1) % 12 == 0 else ""
    sep = " " if i % 12 == 0 else ""
    print(f"{sep}0x{b:02x},", end=end)

if len(data) % 12 != 0:
    print()
print("};")
print(f"unsigned int {name}_len = {len(data)};")

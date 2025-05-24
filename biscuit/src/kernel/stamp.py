#!/usr/bin/env python3

import os
import sys

fn = sys.argv[1]
sz = os.path.getsize(fn)
# both bootmain.c and boot.S also need to know the size of the bootloader in
# blocks (see BOOTBLOCKS)
numblocks = 10
left = numblocks*512 - sz
if left < 0:
    sb = sz/512
    if sz % 512 != 0:
      sb += 1
    s = 'boot sector is bigger than numblocks (is %d, should be %d)' % (numblocks, sb)
    raise ValueError(s)

# Append zero bytes until the bootloader is exactly numblocks*512 bytes.
# Use binary mode to avoid text encoding issues.
with open(fn, 'ab') as f:
    f.write(b"\x00" * left)

# Verify the boot signature. Read in binary mode for the same reason as above.
with open(fn, 'rb') as f:
    d = f.read(512)

# When opened in binary mode, d is a bytes object. Check the boot signature by
# looking at the integer values of the last two bytes.
if d[510] != 0x55 or d[511] != 0xaa:
    raise ValueError('sig is wrong! fix damn text ordering somehow')

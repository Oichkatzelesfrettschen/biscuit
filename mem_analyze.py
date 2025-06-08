import os
# Simple memory analysis script.
# Reads /proc/meminfo and prints key statistics.

with open('/proc/meminfo') as f:
    lines = [line.strip() for line in f]

meminfo = {key: value for (key, value) in [line.split(':') for line in lines]}

for field in ('MemTotal', 'MemFree', 'Buffers', 'Cached', 'SwapTotal', 'SwapFree'):
    print(f"{field}: {meminfo.get(field, '').strip()}")

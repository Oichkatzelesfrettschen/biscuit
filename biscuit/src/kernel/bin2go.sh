#!/bin/sh
set -e -u

[ ! $# -eq 1 ] && { echo "usage: $0 <binfile>" 1>&2; exit 1; }

F=$1
NF=`basename $F| sed "s/[^[:alnum:]]/_/g"`
[ ! -r $F ] && { echo "cannot read $F" 1>&2; exit 1; }

UTIL="xxd"
if which $UTIL > /dev/null 2>&1; then
    X=`which $UTIL`
    CMD="$X -i $F"
else
    # Use the fallback Python implementation shipped with Biscuit.
    # bin2go.sh lives in src/kernel. The helper script resides in the top-level
    # scripts directory. Construct the path relative to this script so that the
    # build works regardless of the current working directory.
    X="$(dirname "$0")/../../scripts/xxd.py"
    if [ ! -x "$X" ]; then
        echo "cannot find $UTIL and fallback $X" 1>&2
        exit 1
    fi
    CMD="$X $F"
fi
echo "var _bin_$NF = []uint8{"
$CMD | tail -n+2 | sed "s/ \(0x[[:xdigit:]][[:xdigit:]]\)$/\1,/" \
        | sed "s/^unsigned int.*\(=.*\);/var _bin_${NF}_len int \1/"

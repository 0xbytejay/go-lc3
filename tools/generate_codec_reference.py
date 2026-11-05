#!/usr/bin/env python3
# Copyright 2026 go-lc3 contributors
# SPDX-License-Identifier: Apache-2.0
"""Regenerate synthetic C oracle vectors; runtime library/tests need no C tools."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import struct
import subprocess
import tempfile

PIN = "48bbd3eacd36e99a57317a0a4867002e0b09e183"

def main():
    p = argparse.ArgumentParser()
    p.add_argument("liblc3", type=Path, help="liblc3 checkout at the pinned commit")
    p.add_argument("--cc", default="clang")
    args = p.parse_args()
    source = args.liblc3.resolve()
    commit = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
    if commit != PIN:
        p.error(f"expected liblc3 {PIN}; found {commit}")
    root = Path(__file__).resolve().parent.parent
    with tempfile.TemporaryDirectory(prefix="lc3-oracle-") as tmp:
        exe = Path(tmp) / "reference_codec"
        flags = ["-O2", "-ffast-math", "-fno-associative-math", "-ffp-contract=off", "-U__ARM_NEON", "-Wno-overriding-option"]
        subprocess.run([args.cc, *flags, "-I"+str(source/"include"), "-I"+str(source/"src"), str(root/"tools/reference_codec.c"), *map(str, sorted((source/"src").glob("*.c"))), "-lm", "-o", str(exe)], check=True)
        raw = subprocess.check_output([str(exe)])
    pos = 0
    def read(n):
        nonlocal pos
        data = raw[pos:pos+n]
        if len(data) != n:
            raise ValueError("truncated oracle output")
        pos += n
        return data
    def word():
        return struct.unpack("<I", read(4))[0]
    def block():
        return read(word())
    if read(9) != b"GOLC3REF1":
        raise ValueError("bad oracle magic")
    count, frames = word(), word()
    streams = []
    for _ in range(count):
        stream = dict(dt=word(), sr=word(), pcm=word(), format=word(), frames=[])
        for _ in range(frames):
            fmt, stride, status = word(), word(), word()
            data, pcm = block(), block()
            stream["frames"].append(dict(format=fmt, stride=stride, status=status, data=data.hex(), pcmSHA256=hashlib.sha256(pcm).hexdigest()))
        streams.append(stream)
    if pos != len(raw):
        raise ValueError("trailing oracle output")
    report = dict(upstream=PIN, flags=flags, streams=streams)
    destination = root / "testdata/codec_reference.json.gz"
    payload = json.dumps(report, separators=(",", ":")).encode()
    destination.write_bytes(gzip.compress(payload, mtime=0))
    print(f"{count} streams, {count*frames} frame pairs; {len(raw)} raw oracle bytes; {destination.stat().st_size} compressed fixture bytes")

if __name__ == "__main__":
    main()

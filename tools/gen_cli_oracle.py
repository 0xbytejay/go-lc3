#!/usr/bin/env python3
# Copyright 2026 The go-lc3 Authors
# SPDX-License-Identifier: Apache-2.0
"""Regenerate the reference CLI golden hashes from synthetic PCM input."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shlex
import struct
import subprocess
import tempfile

from gen_transform_oracle import FLAGS, PIN

# rate, channels, samples/channel, milliseconds, bitrate, HR, encoder rate,
# decoder rate, input bit depth, bytes/sample, output bit depth
CASES = [
    (16000, 1, 173, 10, 32000, False, 0, 0, 16, 2, 16),
    (48000, 2, 733, 7.5, 128000, False, 0, 0, 16, 2, 16),
    (48000, 1, 257, 2.5, 96000, False, 0, 0, 16, 2, 16),
    (96000, 2, 2001, 5, 400000, True, 0, 0, 16, 2, 16),
    (48000, 2, 1763, 10, 64000, False, 16000, 48000, 16, 2, 16),
    (96000, 1, 1081, 2.5, 300000, True, 48000, 96000, 16, 2, 16),
    (32000, 2, 451, 7.5, 128000, False, 0, 0, 24, 3, 24),
    (48000, 1, 997, 10, 96000, False, 0, 0, 24, 4, 24),
    (96000, 1, 577, 2.5, 300000, True, 0, 0, 24, 3, 24),
]


def wave_bytes(rate, channels, samples, bits, width):
    seed = 0x13471
    pcm = bytearray()
    for _ in range(samples * channels):
        seed = (seed * 1664525 + 1013904223) & 0xffffffff
        value = (seed >> (32 - bits)) - (1 << (bits - 1))
        pcm.extend(struct.pack("<i", value)[:width])
    return struct.pack(
        "<4sI4s4sIHHIIHH4sI", b"RIFF", 36 + len(pcm), b"WAVE", b"fmt ",
        16, 1, channels, rate, rate * channels * width, channels * width,
        bits, b"data", len(pcm),
    ) + pcm


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True, help="liblc3 checkout at " + PIN)
    parser.add_argument("--cc", default=os.environ.get("CC", "clang"))
    parser.add_argument("--output", type=Path, help="JSON destination; defaults to stdout")
    args = parser.parse_args()
    source = args.source.resolve()
    if not (source / "src/lc3.c").is_file():
        parser.error("--source must point to a liblc3 source tree")
    if (source / ".git").exists():
        commit = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
        if commit != PIN:
            parser.error("reference checkout must be pinned to " + PIN)
    records = []
    with tempfile.TemporaryDirectory(prefix="go-lc3-cli-oracle-") as name:
        temp = Path(name)
        for tool in ("elc3", "dlc3"):
            command = shlex.split(args.cc) + FLAGS + ["-I" + str(source / "include")]
            command += [str(p) for p in sorted((source / "src").glob("*.c"))]
            command += [str(source / "tools" / f"{p}.c") for p in (tool, "wave", "lc3bin")]
            command += ["-o", str(temp / tool), "-lm"]
            subprocess.run(command, check=True)
        for index, case in enumerate(CASES):
            rate, channels, samples, duration, bitrate, hr, enc, dec, bits, width, out_bits = case
            original, compressed, decoded = (temp / (str(index) + ext) for ext in (".wav", ".lc3", ".decoded.wav"))
            original.write_bytes(wave_bytes(rate, channels, samples, bits, width))
            options = ["-b", str(bitrate), "-m", str(duration)]
            if hr:
                options.append("-H")
            if enc:
                options += ["-r", str(enc)]
            subprocess.run([str(temp / "elc3"), *options, str(original), str(compressed)], check=True, capture_output=True)
            options = ["-b", str(out_bits)]
            if dec:
                options += ["-r", str(dec)]
            subprocess.run([str(temp / "dlc3"), *options, str(compressed), str(decoded)], check=True, capture_output=True)
            records.append(dict(
                rate=rate, channels=channels, samples=samples, milliseconds=duration,
                bitrate=bitrate, high_resolution=hr, encoder_rate=enc, decoder_rate=dec,
                input_bits=bits, input_width=width, output_bits=out_bits,
                encoded_sha256=hashlib.sha256(compressed.read_bytes()).hexdigest(),
                decoded_sha256=hashlib.sha256(decoded.read_bytes()).hexdigest(),
            ))
    output = json.dumps(records, indent=2) + "\n"
    if args.output:
        args.output.write_text(output)
    else:
        print(output, end="")


if __name__ == "__main__":
    main()

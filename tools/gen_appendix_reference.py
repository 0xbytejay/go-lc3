#!/usr/bin/env python3
# Copyright 2022 Google LLC
# Copyright 2026 go-lc3 contributors
# SPDX-License-Identifier: Apache-2.0
# Modified from liblc3: extract the Appendix C vectors without NumPy.
"""Extract upstream Appendix C examples and independent scalar C decode output."""
import argparse
import ast
import ctypes
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

PIN = "48bbd3eacd36e99a57317a0a4867002e0b09e183"

def constant(node):
    """Allow only literal arrays and the two constructors used by these vectors."""
    if isinstance(node, ast.Call):
        if isinstance(node.func, ast.Attribute) and isinstance(node.func.value, ast.Name) and node.func.value.id == "np" and node.func.attr == "array":
            return constant(node.args[0])
        if isinstance(node.func, ast.Name) and node.func.id == "bytearray":
            return bytearray(constant(node.args[0]))
        raise ValueError("unexpected constructor")
    if isinstance(node, (ast.List, ast.Tuple)):
        return [constant(item) for item in node.elts]
    return ast.literal_eval(node)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("liblc3", type=Path)
    parser.add_argument("--cc", default="clang")
    args = parser.parse_args()
    source = args.liblc3.resolve()
    commit = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
    if commit != PIN:
        parser.error(f"expected liblc3 {PIN}; found {commit}")
    root = Path(__file__).resolve().parent.parent
    appendix = (source/"test/appendix_c.py").read_bytes()
    wanted = {prefix+suffix for prefix in ["X_PCM_", "BYTES_AC_", "X_HAT_CLIP_"] for suffix in ["7M5", "10M"]}
    values = {}
    for node in ast.parse(appendix).body:
        if isinstance(node, ast.Assign) and len(node.targets) == 1 and isinstance(node.targets[0], ast.Name) and node.targets[0].id in wanted:
            values[node.targets[0].id] = constant(node.value)
    if set(values) != wanted:
        raise ValueError("missing Appendix C arrays")
    flags = ["-O2", "-ffast-math", "-fno-associative-math", "-ffp-contract=off", "-U__ARM_NEON", "-Wno-overriding-option"]
    streams = []
    with tempfile.TemporaryDirectory(prefix="lc3-appendix-") as tmp:
        library = Path(tmp)/"liblc3.so"
        subprocess.run([args.cc, "-shared", "-fPIC", *flags, "-I"+str(source/"include"), "-I"+str(source/"src"), *map(str, sorted((source/"src").glob("*.c"))), "-lm", "-o", str(library)], check=True)
        lib = ctypes.CDLL(str(library))
        for direction in ["encoder", "decoder"]:
            size = getattr(lib, "lc3_"+direction+"_size")
            size.argtypes = [ctypes.c_int, ctypes.c_int]
            size.restype = ctypes.c_uint
            setup = getattr(lib, "lc3_setup_"+direction)
            setup.argtypes = [ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_void_p]
            setup.restype = ctypes.c_void_p
        lib.lc3_encode.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_void_p, ctypes.c_int, ctypes.c_int, ctypes.c_void_p]
        lib.lc3_decode.argtypes = [ctypes.c_void_p, ctypes.c_void_p, ctypes.c_int, ctypes.c_int, ctypes.c_void_p, ctypes.c_int]
        for dt, suffix in [(7500,"7M5"), (10000,"10M")]:
            emem = ctypes.create_string_buffer(lib.lc3_encoder_size(dt,16000))
            dmem = ctypes.create_string_buffer(lib.lc3_decoder_size(dt,16000))
            encoder = lib.lc3_setup_encoder(dt,16000,0,emem)
            decoder = lib.lc3_setup_decoder(dt,16000,0,dmem)
            if not encoder or not decoder:
                raise ValueError("C state initialization failed")
            frames = []
            for pcm, encoded, clipped in zip(values["X_PCM_"+suffix], values["BYTES_AC_"+suffix], values["X_HAT_CLIP_"+suffix], strict=True):
                input_pcm = (ctypes.c_int16*len(pcm))(*pcm)
                output_bits = ctypes.create_string_buffer(len(encoded))
                if lib.lc3_encode(encoder,0,input_pcm,1,len(encoded),output_bits) != 0 or output_bits.raw != encoded:
                    raise ValueError(f"scalar C encoder differs from Appendix C at {dt} us frame {len(frames)}")
                input_bits = ctypes.create_string_buffer(bytes(encoded))
                output_pcm = (ctypes.c_int16*len(pcm))()
                if lib.lc3_decode(decoder,input_bits,len(encoded),0,output_pcm,1) != 0:
                    raise ValueError("C decoder concealed an Appendix C frame")
                if max(abs(a-b) for a,b in zip(output_pcm,clipped,strict=True)) >= 1:
                    raise ValueError("C decoder exceeds upstream Appendix C output tolerance")
                frames.append(dict(input=pcm,bitstream=encoded.hex(),decodedS16=list(output_pcm),referenceClipped=clipped))
            streams.append(dict(durationUS=dt,sampleRate=16000,frames=frames))
    result = dict(upstream=PIN,source="test/appendix_c.py",sourceSHA256=hashlib.sha256(appendix).hexdigest(),license="Apache-2.0",copyright=["2022 Google LLC","2026 go-lc3 contributors"],flags=flags,streams=streams)
    destination = root/"testdata/appendix_reference.json"
    destination.write_text(json.dumps(result, separators=(",", ":"))+"\n")
    print(f"{len(streams)} Appendix C streams; {sum(len(s['frames']) for s in streams)} frames; {destination.stat().st_size} fixture bytes")

if __name__ == "__main__":
    main()

#!/usr/bin/env python3
# Copyright 2026 The go-lc3 Authors
# SPDX-License-Identifier: Apache-2.0
"""Generate scalar MDCT/LTPF oracle digests from an explicit liblc3 checkout.

Only this optional development tool needs a C compiler. The Go library, tools,
normal tests, and published module have no C or Python dependency.
"""
import argparse
import ctypes as C
import hashlib
import json
import os
from pathlib import Path
import shlex
import struct
import subprocess
import sys
import tempfile

PIN = "48bbd3eacd36e99a57317a0a4867002e0b09e183"
FLAGS = ["-O2", "-ffast-math", "-fno-associative-math", "-ffp-contract=off", "-U__ARM_NEON"]


def generate(lib):
    P=C.POINTER(C.c_float)
    for name in ['lc3_mdct_forward','lc3_mdct_inverse']:
     getattr(lib,name).argtypes=[C.c_int]*3+[P]*3
    ns2=[20,40,60,80,120,120,240];ns4=[32,64,96,128,192,192,384]
    F=lambda xs:(C.c_float*len(xs))(*xs)
    refs=[]
    for dt in range(4):
     for sr in range(7):
      if dt==2 and sr>4:continue
      for src in range(sr+1):
       if (sr>=5)!=(src>=5):continue
       if sr>=5 and src<5:continue
       ns=ns2[sr]*(1+dt);dst=ns2[src]*(1+dt);nd=(ns+(ns4[sr] if dt==2 else ns2[sr]))//2
       d=F([0]*nd);di=F([0]*nd);raw=bytearray();state=0x13471
       for frame in range(4):
        ins=[]
        for i in range(ns):state=(state*1664525+1013904223)&0xffffffff;ins.append((state>>16)-32768)
        x=F(ins);y=F([0]*ns);z=F([0]*ns)
        lib.lc3_mdct_forward(dt,sr,src,x,d,y)
        padded=F(list(y)[:dst]+[0]*(ns-dst))
        lib.lc3_mdct_inverse(dt,sr,src,padded,di,z)
        for a in [y,d,z,di]:raw.extend(bytes(a))
       refs.append(dict(kind='mdct',dt=dt,sr=sr,src=src,sha=hashlib.sha256(raw).hexdigest(),raw=raw.hex()))
    class HP(C.Structure):_fields_=[('s1',C.c_int64),('s2',C.c_int64)]
    class AN(C.Structure):_fields_=[('active',C.c_bool),('pitch',C.c_int),('nc',C.c_float*2),('hp50',HP),('x12',C.c_int16*384),('x6',C.c_int16*178),('tc',C.c_int)]
    class DATA(C.Structure):_fields_=[('active',C.c_bool),('pitch_index',C.c_int)]
    lib.lc3_ltpf_analyse.argtypes=[C.c_int]*2+[C.POINTER(AN),C.POINTER(C.c_int16),C.POINTER(DATA)]
    lib.lc3_ltpf_analyse.restype=C.c_bool
    for dt in range(4):
     for sr in range(7):
      if dt==2 and sr>4:continue
      ns=ns2[sr]*(1+dt);nt=ns2[sr]//2;state=AN();raw=bytearray();hist=[0]*nt;data=DATA()
      for frame in range(20):
       period=ns2[sr]//2
       xs=[((frame*ns+i)%period)*24000//period-12000 for i in range(ns)]
       x=(C.c_int16*(nt+ns))(*(hist+xs));xp=C.cast(C.byref(x,2*nt),C.POINTER(C.c_int16))
       present=lib.lc3_ltpf_analyse(dt,sr,C.byref(state),xp,C.byref(data))
       raw.extend(struct.pack('<iiiiiffqq',int(present),int(data.active),data.pitch_index,int(state.active),state.pitch,*state.nc,state.hp50.s1,state.hp50.s2))
       raw.extend(bytes(state.x12));raw.extend(bytes(state.x6));raw.extend(struct.pack('<i',state.tc));hist=xs[-nt:]
      refs.append(dict(kind='ltpf_an',dt=dt,sr=sr,src=0,sha=hashlib.sha256(raw).hexdigest(),raw=raw.hex()))
    class SYN(C.Structure):_fields_=[('active',C.c_bool),('pitch',C.c_int),('c',C.c_float*24),('x',C.c_float*12)]
    lib.lc3_ltpf_synthesize.argtypes=[C.c_int]*3+[C.POINTER(SYN),C.POINTER(DATA),P,P]
    for dt in range(4):
     for sr in range(5):
      ns=ns2[sr]*(1+dt);nh=(8+(dt==2))*ns2[sr];ring=F([0]*(nh+ns));xoff=nh;s=SYN();state=0x13471;raw=bytearray()
      for frame in range(24):
       for i in range(ns):state=(state*1664525+1013904223)&0xffffffff;ring[xoff+i]=(state>>16)-32768
       data=DATA(frame%5!=4,(frame//4)*57%512)
       ptr=C.cast(C.byref(ring,4*xoff),P)
       lib.lc3_ltpf_synthesize(dt,sr,40,C.byref(s),C.byref(data) if frame%7 else None,ring,ptr)
       raw.extend(bytes(ring));raw.extend(struct.pack('<ii',int(s.active),s.pitch));raw.extend(bytes(s.c));raw.extend(bytes(s.x))
       xoff=xoff+ns if xoff<nh else 0
      refs.append(dict(kind='ltpf_syn',dt=dt,sr=sr,src=0,sha=hashlib.sha256(raw).hexdigest(),raw=raw.hex()))
    return refs


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True, help="liblc3 checkout at " + PIN)
    parser.add_argument("--cc", default=os.environ.get("CC", "clang"), help="C compiler command")
    parser.add_argument("--output", type=Path, help="JSON destination; defaults to stdout")
    parser.add_argument("--raw", action="store_true", help="include full byte oracles for mismatch debugging")
    parser.add_argument("--go", action="store_true", help="emit Go golden-table rows instead of JSON")
    args = parser.parse_args()
    source = args.source.resolve()
    if not (source / "src/lc3.c").is_file():
        parser.error("--source must point to a liblc3 source tree")
    if sys.byteorder != "little":
        parser.error("the optional ctypes oracle generator requires a little-endian host")
    if os.name == "nt":
        parser.error("run the optional reference generator on Linux or macOS")
    # A release archive need not contain .git; check revisions when available.
    if (source / ".git").exists():
        commit = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
        if commit != PIN:
            parser.error("reference checkout must be pinned to " + PIN)
    with tempfile.TemporaryDirectory(prefix="go-lc3-oracle-") as tmp:
        library = Path(tmp) / ("liblc3.dylib" if sys.platform == "darwin" else "liblc3.so")
        mode = ["-dynamiclib"] if sys.platform == "darwin" else ["-fPIC", "-shared"]
        command = shlex.split(args.cc) + FLAGS + mode + ["-I" + str(source / "include")]
        command += [str(p) for p in sorted((source / "src").glob("*.c"))]
        command += ["-o", str(library), "-lm"]
        subprocess.run(command, check=True)
        records = generate(C.CDLL(str(library)))
    if args.go:
        output = "\n".join('{"%s", %d, %d, %d, "%s"},' % (r["kind"], r["dt"], r["sr"], r["src"], r["sha"]) for r in records) + "\n"
    else:
        if not args.raw:
            for r in records:
                r.pop("raw", None)
        output = json.dumps(records, indent=2) + "\n"
    if args.output:
        args.output.write_text(output)
    else:
        print(output, end="")


if __name__ == "__main__":
    main()

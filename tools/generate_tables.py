#!/usr/bin/env python3
# Copyright 2026 go-lc3 contributors. SPDX-License-Identifier: Apache-2.0
"""Regenerate tables.go from liblc3's Clang AST, retaining IEEE binary32 values.

Usage: python3 tools/generate_tables.py /path/to/liblc3
Requires clang and Python 3 for development only; consumers need only Go.
"""
import argparse
import hashlib
import json
import pathlib
import re
import struct
import subprocess

PIN = '48bbd3eacd36e99a57317a0a4867002e0b09e183'
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('source', type=pathlib.Path)
parser.add_argument('--output', type=pathlib.Path, default=pathlib.Path('tables.go'))
args = parser.parse_args()
source = args.source.resolve()
actual = subprocess.check_output(['git', '-C', str(source), 'rev-parse', 'HEAD'], text=True).strip()
if actual != PIN:
    parser.error('expected liblc3 commit ' + PIN)
ast = json.loads(subprocess.check_output(['clang', '-Iinclude', '-Isrc', '-U__ARM_NEON', '-Xclang', '-ast-dump=json', '-fsyntax-only', 'src/tables.c'], cwd=source))
variables = [n for n in ast['inner'] if n['kind']=='VarDecl' and n.get('init')]
variables = [v for v in variables if v['loc'].get('line', 0) >= 26 and v['loc'].get('offset', 0) < (source/'src/tables.c').stat().st_size]
byname = {v['name']:v for v in variables}

def typ(t):
    t = t.replace('const ', '').replace('struct ', '').strip()
    dims = re.findall(r'\[(\d+)\]', t)
    base = re.sub(r'\[\d+\]', '', t).strip()
    pointers = base.endswith('*')
    if pointers: base=base[:-1].strip()
    base = {'float':'float32','int32_t':'int32','uint16_t':'uint16','uint8_t':'uint8','lc3_sns_vq_gains':'lc3_sns_vq_gains_def'}.get(base,base)
    if pointers: base = ('*' if base.startswith('lc3_') else '[]') + base
    return ''.join('['+d+']' for d in dims)+base

def scalar(n):
    k=n['kind']; ch=n.get('inner',[])
    if k in ('IntegerLiteral','FloatingLiteral','ConstantExpr') and 'value' in n:
        s=n['value']
        return float(s) if k=='FloatingLiteral' or n['type']['qualType'] in ('float','double') else int(s,0)
    if k in ('ImplicitCastExpr','ParenExpr','ConstantExpr'):
        v=scalar(ch[0])
        if n['type']['qualType']=='float':return struct.unpack('f',struct.pack('f',v))[0]
        return v
    if k=='UnaryOperator':
        v=scalar(ch[0]);return -v if n['opcode']=='-' else v
    if k=='BinaryOperator':
        a,b=map(scalar,ch);op=n['opcode']
        return {'+':lambda:a+b,'-':lambda:a-b,'*':lambda:a*b,'/':lambda:(a/b if n['type']['qualType'] in ('float','double') else int(a/b)),'<<':lambda:a<<b,'>>':lambda:a>>b,'%':lambda:a%b}[op]()
    if k=='UnaryExprOrTypeTraitExpr' and n['name']=='sizeof':
        t=n.get('argType',{}).get('qualType') or ch[0]['type']['qualType']
        ds=re.findall(r'\[(\d+)\]',t);bt=re.sub(r'\[\d+\]','',t).replace('const ','').strip()
        size={'float':4,'int':4,'int32_t':4,'uint16_t':2,'uint8_t':1}[bt]
        for d in ds:size*=int(d)
        return size
    if k=='DeclRefExpr':
        d=n['referencedDecl']
        if d['kind']=='EnumConstantDecl':
            enums={'LC3_NUM_DT':4,'LC3_NUM_SRATE':7}
            return enums[d['name']]
    raise ValueError(('scalar',k,n.get('opcode'),n.get('type'), json.dumps(n)[:800]))

def floatlit(v):
    v=struct.unpack('f',struct.pack('f',v))[0]
    bits=struct.unpack('I',struct.pack('f',v))[0]
    if bits==0x80000000:return 'math.Float32frombits(0x80000000)'
    return v.hex()

def expr(n, want=None):
    k=n['kind'];ch=n.get('inner',[]);t=n['type']['qualType'];gt=typ(t)
    if k=='InitListExpr':
        entries = ch
        if 'array_filler' in n: entries=n['array_filler'][1:]
        values=[expr(x) for x in entries]
        if '[' in t:
            if any('{' in v or '\n' in v for v in values):
                return gt+'{\n'+',\n'.join(values)+(',\n' if values else '')+'}'
            if len(values)>8:
                rows=[','.join(values[i:i+4]) for i in range(0,len(values),4)]
                return gt+'{\n'+',\n'.join(rows)+',\n}'
        return gt+'{'+','.join(values)+'}'
    if k=='ImplicitValueInitExpr':
        if '*' in t:return 'nil'
        if '[' in t or 'struct' in t:return gt+'{}'
        return '0'
    if k=='ImplicitCastExpr' and n.get('castKind')=='NullToPointer':return 'nil'
    if k=='ImplicitCastExpr' and n.get('castKind')=='ArrayToPointerDecay':
        v=ch[0]
        while v['kind']=='ParenExpr':v=v['inner'][0]
        if v['kind']=='DeclRefExpr':return v['referencedDecl']['name']+'[:]'
        if v['kind']=='CompoundLiteralExpr':
            s=expr(v['inner'][0]); return re.sub(r'^\[\d+\]','[]',s)
        raise ValueError(('decay',v['kind']))
    if (k=='ConstantExpr' and 'value' not in n) or k in ('ParenExpr','CompoundLiteralExpr') or (k=='ImplicitCastExpr' and n.get('castKind') in ('NoOp','LValueToRValue')):return expr(ch[0],want)
    if k=='UnaryOperator' and n['opcode']=='&':return '&'+expr(ch[0])
    if k=='DeclRefExpr' and n['referencedDecl']['kind']=='VarDecl':return n['referencedDecl']['name']
    v=scalar(n)
    return floatlit(v) if t.replace('const ','')=='float' else str(int(v))

body='// Code generated by tools/generate_tables.py; DO NOT EDIT.\n'
body+='// Copyright 2022 Google LLC\n// Copyright 2026 go-lc3 contributors\n// SPDX-License-Identifier: Apache-2.0\n'
body+='// Modified from liblc3 '+PIN+': exact binary32 Go tables.\n'
body+='// tables.c SHA-256: '+hashlib.sha256((source/'src/tables.c').read_bytes()).hexdigest()+'\n\npackage lc3\n\nimport "math"\n\n'
for v in variables:
    init=next(n for n in v['inner'] if n['kind']=='InitListExpr')
    body+='var '+v['name']+' = '+expr(init)+'\n\n'
if 'math.Float32frombits' not in body: body=body.replace('import "math"\n\n','')
args.output.write_text(body)
subprocess.run(['gofmt','-s','-w',str(args.output)],check=True)
print('Generated',args.output,'from',len(variables),'tables at',PIN)

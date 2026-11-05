# Transform and file-tool reference vectors

`transform_oracle_test.go` contains 115 SHA-256 digests generated from the pinned
[liblc3](https://github.com/google/liblc3) revision
`48bbd3eacd36e99a57317a0a4867002e0b09e183`. The vectors use synthetic, deterministic
PCM generated in the tests. They contain no recordings or external audio assets.

Coverage includes every supported duration and sample rate, all valid
PCM/encoded-rate pairs, delayed MDCT state, LTPF resampling and pitch analysis,
and postfilter activation, deactivation, and pitch changes. MDCT output samples,
delay state, and all semantically significant LTPF state fields are serialized
in little-endian order; struct padding is excluded.

To reproduce the digest table using a separate liblc3 checkout:

```sh
python3 tools/gen_transform_oracle.py --source /path/to/liblc3 --go
```

To retain full oracle bytes and report the first differing word on failure:

```sh
python3 tools/gen_transform_oracle.py --source /path/to/liblc3 --raw --output /tmp/transform-oracle.json
LC3_TRANSFORM_ORACLE=/tmp/transform-oracle.json go test -run TestScalarTransforms
```

The CLI golden tests in `cmd/internal/cli/cli_test.go` compare complete encoded
container bytes and complete decoded WAV bytes against the scalar reference
tools. They cover mono/stereo, 16-bit PCM, packed 24-bit PCM, 24-bit PCM in
32-bit words, 2.5/5/7.5/10ms frames, high resolution, rate conversion, and exact
sample counts after delay and tail trimming. Their nine golden cases can be
regenerated as follows:

```sh
python3 tools/gen_cli_oracle.py --source /path/to/liblc3 --output /tmp/cli-oracle.json
```

These optional development scripts require Python 3 and a C compiler (Clang by
default, or `--cc`). The transform ctypes generator supports little-endian Linux
and macOS. They build the reference with `-O2 -ffast-math
-fno-associative-math -ffp-contract=off -U__ARM_NEON`: scalar operations, without
floating-point reassociation or contraction. Run normal Go tests without these
dependencies: the digests are already checked in.

A port and these regression vectors do not confer the original implementation's
Bluetooth qualification or replace independent specification conformance tests.

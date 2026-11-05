# Compatibility and validation

## Reference and numerical profile

The codec is adapted from Google's liblc3 at revision
`48bbd3eacd36e99a57317a0a4867002e0b09e183` (version 1.1.2 source). The optional
reference generators require that exact checkout. Normal builds and tests use
only the Go toolchain and checked-in synthetic fixtures.

Reference builds use the scalar implementation with:

```text
-O2 -ffast-math -fno-associative-math -ffp-contract=off -U__ARM_NEON
```

The Go code keeps binary32 intermediate rounding explicit. This prevents fused
multiply-add or reassociation from changing quantization decisions or integer
PCM rounding. Fast-math approximations, coefficient tables, integer wraparound,
filter history, frame delay, and packet-loss state follow the reference.

The target is the pinned scalar profile. Different C compilers, options, SIMD
paths, or liblc3 versions can produce different rounding. In particular, this
is not a claim that every upstream build has the same PCM CRC.

## Checked-in differential tests

| Area | Cases | Comparison |
| --- | ---: | --- |
| MDCT and LTPF | 115 | SHA-256 of exact float/state bytes, including rate conversion and filter transitions |
| Spectral quantization and arithmetic coding | 312 | Exact compressed bytes, float bits, and analysis state |
| Energy, bandwidth, attack detection, PLC | 312 | Exact state and numerical output |
| SNS and TNS | 78 | Side data, float output, and bitstream; all supported duration/rate modes |
| Complete codec | 276 streams × 32 frames | Exact encoded bytes and SHA-256 of decoded PCM bytes |
| Appendix C published examples | 4 frames | Exact 140 encoded bytes; 560 decoded samples match scalar C and the published reference within 1 LSB |
| File tools | 9 golden files | Complete `.lc3` and WAV bytes against the C tools |

The complete-codec suite covers all **69 valid duration / encoded-rate /
PCM-rate combinations**, including LC3plus and high resolution, across all four
input PCM formats. The 8,832 frame pairs include dynamic byte budgets, stride
1–3, silence, tonal signals, speech-like synthetic waveforms, noise, quiet
signals, full-scale impulses, nine consecutive missing packets, and corrupt
frames. Both typed PCM methods and byte-buffer methods are tested. Output
formats include S16, S24, packed S24LE, and IEEE float32, including untouched
interleaved channel slots.

Additional tests cover 8,000 pulse-vector enumeration round trips, state reset,
invalid-input state preservation, saturation, helper-query overflow, zero-value
contexts, file parsing, short readers/writers, channels 1/2/8, very short files,
frame boundaries, truncation, and delay/tail trimming. Long-loss tests exercise
1,000 consecutive concealed frames for all 69 rate combinations and four output
formats (276,000 PLC calls), checking finite, decaying output and recovery.
Power-of-two scaling also has explicit signed-zero, subnormal, and overflow
edge-case tests.

`FuzzDecode` exercises every supported configuration and rate conversion,
valid seeds, malformed payloads, consecutive frames, and packet-loss state.
The initial 30-second expanded run completed over 2.5 million executions without
a failure. `FuzzEncode` also covers all 69 rate configurations, four PCM formats,
dynamic frame sizes, valid round trips, and invalid-input state preservation. Its
initial 30-second run completed over 800,000 executions. These are bounded
regression exercises, not proof that every input has been exhausted.

## Reproduce Go checks

```sh
CGO_ENABLED=0 go test ./...
go test -race ./...
go vet ./...
go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime 30s -parallel 2 .
go test -run '^$' -fuzz '^FuzzEncode$' -fuzztime 30s -parallel 2 .
go test -run '^$' -bench . -benchmem .
```

The initial suite was executed on macOS arm64 using Go 1.23.12 and Go 1.27.2,
and on macOS amd64 through Rosetta. Windows/amd64 and Linux/386 builds were
cross-compiled. The GitHub Actions matrix runs native Linux, macOS, and Windows
with the minimum Go version and the current stable release.

Encoder, decoder, and PLC tests assert zero allocations per frame after
construction at 16 kHz/10 ms, 48 kHz/7.5 ms, and 96 kHz HR/10 ms. Timing depends
on hardware and configuration; the repository includes benchmarks rather than
a portable throughput guarantee.

## Regenerate the reference data

Obtain a separate liblc3 checkout and select the pinned revision:

```sh
git clone https://github.com/google/liblc3.git /tmp/liblc3-reference
git -C /tmp/liblc3-reference checkout 48bbd3eacd36e99a57317a0a4867002e0b09e183
python3 tools/generate_tables.py /tmp/liblc3-reference
python3 tools/generate_codec_reference.py /tmp/liblc3-reference
python3 tools/gen_appendix_reference.py /tmp/liblc3-reference
python3 tools/gen_transform_oracle.py --source /tmp/liblc3-reference --go
python3 tools/gen_cli_oracle.py --source /tmp/liblc3-reference --output /tmp/cli-oracle.json
```

The transform command prints the digest table for comparison; it does not
rewrite the tests. See [tools/transform-oracles.md](tools/transform-oracles.md)
for full-byte transform diagnostics. Commands for the smaller C generators are
included in `tools/reference_spectrum.c`, `tools/reference_analysis.c`, and
`testdata/reference_sns_tns.c`. These are optional developer tools and are never
linked into the Go library or command-line programs.

## Scope and boundaries

The library provides the codec features of the pinned implementation, using a
Go API rather than C's raw pointers and memory-size interfaces. Context
allocation is managed by Go. Invalid buffers and unsupported configurations
return errors before changing state. Encoders reject non-finite/out-of-range
float PCM and out-of-range 24-bit samples, where the C API relies on its input
contract. Integer output saturates safely before float-to-integer conversion. Power-of-two
scaling falls back to IEEE arithmetic outside the fast helper's normal exponent
range, preventing very quiet PCM or prolonged packet loss from wrapping a
subnormal exponent into full-scale output or NaN. This is an intentional
robustness improvement over relying on a C flush-to-zero environment.

The core decoder preserves liblc3's concealment behavior for missing or corrupt
frames. The file decoder reports corrupt frames and truncated files as errors
rather than silently publishing concealed audio. Named CLI output is written
to a temporary file in the destination directory and replaces the target only
after successful conversion and flush/close; failed conversion preserves an
existing target. Standard output remains a streaming interface and can contain
partial data when a later error occurs. Very short file trimming uses
the declared remaining sample count to avoid negative sample-count behavior.

This project has not undergone independent Bluetooth qualification or a complete
standards conformance campaign. The upstream implementation's QDID and
conformance reports apply to that implementation, not automatically to this
port. The `.lc3` file container is the reference tools' non-standard format;
Bluetooth transport, media players, general-purpose resampling, and other audio
codecs belong outside this library.

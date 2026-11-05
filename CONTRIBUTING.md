# Contributing

Use Go 1.23 or later. Keep the codec usable with `CGO_ENABLED=0` and avoid
adding runtime dependencies for codec math, file conversion, or playback.

Before submitting a change:

```sh
gofmt -w .
CGO_ENABLED=0 go test ./...
go test -race ./...
go vet ./...
```

For numerical changes, compare the affected stages and complete frame output
against the pinned liblc3 revision. Preserve explicit float32 operation order;
removing a conversion can enable fused multiply-add and change encoded bytes
or PCM rounding. Keep integer wraparound behavior explicit where C relies on
32-bit arithmetic.

Tests should use deterministic synthetic signals or redistributable public
vectors with attribution. Do not commit personal recordings, credentials,
local paths, compiled tools, profiling outputs, or generated build directories.

Report malformed input bugs with a minimal reproducer and a codec configuration.
Changes to error handling should test whether state advances and whether the
next valid frame remains correct. Include a benchmark when changing a hot path.

Contributions are provided under the project's Apache-2.0 license.

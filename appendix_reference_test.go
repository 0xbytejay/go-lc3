// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: standalone Go verification of the Appendix C examples.
package lc3

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

type appendixReference struct {
	Upstream     string `json:"upstream"`
	SourceSHA256 string `json:"sourceSHA256"`
	Streams      []struct {
		DurationUS int `json:"durationUS"`
		SampleRate int `json:"sampleRate"`
		Frames     []struct {
			Input            []int16   `json:"input"`
			Bitstream        string    `json:"bitstream"`
			DecodedS16       []int16   `json:"decodedS16"`
			ReferenceClipped []float64 `json:"referenceClipped"`
		} `json:"frames"`
	} `json:"streams"`
}

// TestAppendixC verifies the published intermediate-verification examples as
// reproduced in upstream's Apache-2.0-licensed test/appendix_c.py. It preserves
// the upstream strict <1 LSB decoder tolerance and additionally checks exact
// signed-16 output from the canonical scalar C build. These few examples are
// regression vectors; they are not a full LC3 conformance or qualification test.
func TestAppendixC(t *testing.T) {
	data, err := os.ReadFile("testdata/appendix_reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference appendixReference
	if err = json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	if reference.Upstream != UpstreamVersion || len(reference.Streams) != 2 || len(reference.SourceSHA256) != 64 {
		t.Fatal("unexpected Appendix C reference provenance")
	}
	for _, stream := range reference.Streams {
		t.Run(fmt.Sprintf("%dus", stream.DurationUS), func(t *testing.T) {
			config := Config{SampleRate: stream.SampleRate, FrameDuration: time.Duration(stream.DurationUS) * time.Microsecond}
			encoder, err := NewEncoder(config)
			if err != nil {
				t.Fatal(err)
			}
			decoder, err := NewDecoder(config)
			if err != nil {
				t.Fatal(err)
			}
			if len(stream.Frames) != 2 {
				t.Fatal("unexpected Appendix C frame count")
			}
			for index, frame := range stream.Frames {
				expected, err := hex.DecodeString(frame.Bitstream)
				if err != nil {
					t.Fatal(err)
				}
				encoded := make([]byte, len(expected))
				if err = encoder.Encode(encoded, frame.Input); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(encoded, expected) {
					t.Fatalf("frame%d encoder differs from published Appendix C bitstream: got%x want%x", index, encoded, expected)
				}
				decoded := make([]int16, config.FrameSamples())
				concealed, err := decoder.Decode(decoded, expected)
				if err != nil || concealed {
					t.Fatalf("frame%d decoder concealed=%t err=%v", index, concealed, err)
				}
				if len(decoded) != len(frame.DecodedS16) || len(decoded) != len(frame.ReferenceClipped) {
					t.Fatal("invalid Appendix C sample count")
				}
				maximum := float64(0)
				for i, actual := range decoded {
					if actual != frame.DecodedS16[i] {
						t.Fatalf("frame%d sample%d differs from canonical scalar C: got%d want%d", index, i, actual, frame.DecodedS16[i])
					}
					difference := math.Abs(float64(actual) - frame.ReferenceClipped[i])
					maximum = max(maximum, difference)
					if difference >= 1 {
						t.Fatalf("frame%d sample%d exceeds upstream <1LSB tolerance: error%g", index, i, difference)
					}
				}
				t.Logf("frame%d: %d exact encoded bytes, %d exact scalar C PCM samples, Appendix C maximum error %.8f LSB", index, len(encoded), len(decoded), maximum)
			}
		})
	}
}

// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

package lc3

import "testing"

var benchmarkConfigurations = []struct {
	name  string
	c     Config
	bytes int
}{
	{"16k_10ms", Config{SampleRate: 16000, FrameDuration: FrameDuration10M}, 40},
	{"48k_7.5ms", Config{SampleRate: 48000, FrameDuration: FrameDuration7M5}, 60},
	{"96k_HR_10ms", Config{SampleRate: 96000, HighResolution: true, FrameDuration: FrameDuration10M}, 250},
}

func BenchmarkEncoder(b *testing.B) {
	for _, v := range benchmarkConfigurations {
		b.Run(v.name, func(b *testing.B) {
			e := apiEncoder(b, v.c)
			pcm := apiPCM(e.FrameSamples(), 3)
			frame := make([]byte, v.bytes)
			b.ReportAllocs()
			b.SetBytes(int64(2 * len(pcm)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := e.Encode(frame, pcm); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
func BenchmarkDecoder(b *testing.B) {
	for _, v := range benchmarkConfigurations {
		b.Run(v.name, func(b *testing.B) {
			e, d := apiEncoder(b, v.c), apiDecoder(b, v.c)
			pcm := apiPCM(e.FrameSamples(), 3)
			frame := apiFrame(b, e, pcm, v.bytes)
			b.ReportAllocs()
			b.SetBytes(int64(2 * len(pcm)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := d.Decode(pcm, frame); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
func BenchmarkPLC(b *testing.B) {
	for _, v := range benchmarkConfigurations {
		b.Run(v.name, func(b *testing.B) {
			d := apiDecoder(b, v.c)
			pcm := make([]int16, d.FrameSamples())
			b.ReportAllocs()
			b.SetBytes(int64(2 * len(pcm)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := d.Decode(pcm, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestNoPerFrameAllocations(t *testing.T) {
	for _, v := range benchmarkConfigurations {
		t.Run(v.name, func(t *testing.T) {
			e, d := apiEncoder(t, v.c), apiDecoder(t, v.c)
			pcm := apiPCM(e.FrameSamples(), 3)
			frame := apiFrame(t, e, pcm, v.bytes)
			if n := testing.AllocsPerRun(20, func() {
				if err := e.Encode(frame, pcm); err != nil {
					panic(err)
				}
			}); n != 0 {
				t.Errorf("Encode allocates %g times per frame", n)
			}
			if n := testing.AllocsPerRun(20, func() {
				if _, err := d.Decode(pcm, frame); err != nil {
					panic(err)
				}
			}); n != 0 {
				t.Errorf("Decode allocates %g times per frame", n)
			}
			if n := testing.AllocsPerRun(20, func() {
				if _, err := d.Decode(pcm, nil); err != nil {
					panic(err)
				}
			}); n != 0 {
				t.Errorf("PLC allocates %g times per frame", n)
			}
		})
	}
}

// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

package lc3

import (
	"bytes"
	"fmt"
	"math"
	"testing"
)

func TestLongPacketLossAllModes(t *testing.T) {
	normalRates := []int{8000, 16000, 24000, 32000, 48000}
	hrRates := []int{48000, 96000}
	durations := []struct {
		name   string
		config Config
	}{
		{"2.5ms", Config{FrameDuration: FrameDuration2M5}},
		{"5ms", Config{FrameDuration: FrameDuration5M}},
		{"7.5ms", Config{FrameDuration: FrameDuration7M5}},
		{"10ms", Config{FrameDuration: FrameDuration10M}},
	}
	modes := 0
	for _, hr := range []bool{false, true} {
		rates := normalRates
		if hr {
			rates = hrRates
		}
		for _, duration := range durations {
			if hr && duration.config.FrameDuration == FrameDuration7M5 {
				continue
			}
			for srIndex, sr := range rates {
				for _, pcmRate := range rates[srIndex:] {
					c := duration.config
					c.SampleRate = sr
					c.PCMSampleRate = pcmRate
					c.HighResolution = hr
					modes++
					t.Run(fmt.Sprintf("%s_%d_pcm%d_hr%v", duration.name, sr, pcmRate, hr), func(t *testing.T) {
						e := apiEncoder(t, c)
						d16, d24, dp, df := apiDecoder(t, c), apiDecoder(t, c), apiDecoder(t, c), apiDecoder(t, c)
						ns := e.FrameSamples()
						pcm, s16 := make([]int16, ns), make([]int16, ns)
						s24, packed, floats := make([]int32, ns), make([]byte, 3*ns), make([]float32, ns)
						lo, hi, _ := c.FrameSizeRange()
						encoded := make([]byte, (lo+hi)/2)
						// Prime every decoder with identical, continuous audio.
						for frame := 0; frame < 5; frame++ {
							for i := range pcm {
								pcm[i] = int16(12000 * math.Sin(2*math.Pi*200*float64(frame*ns+i)/float64(pcmRate)))
							}
							if err := e.Encode(encoded, pcm); err != nil {
								t.Fatal(err)
							}
							if plc, err := d16.Decode(s16, encoded); plc || err != nil {
								t.Fatalf("valid S16 frame: concealed=%v err=%v", plc, err)
							}
							if plc, err := d24.DecodeS24(s24, encoded, 1); plc || err != nil {
								t.Fatalf("valid S24 frame: concealed=%v err=%v", plc, err)
							}
							if plc, err := dp.DecodeS24LE(packed, encoded, 1); plc || err != nil {
								t.Fatalf("valid packed frame: concealed=%v err=%v", plc, err)
							}
							if plc, err := df.DecodeFloat32(floats, encoded, 1); plc || err != nil {
								t.Fatalf("valid float frame: concealed=%v err=%v", plc, err)
							}
						}
						// Normal PLC attenuation eventually enters IEEE subnormal
						// ranges. It must keep decaying without exponent wrap,
						// full-scale bursts, or NaNs in any output format.
						for frame := 0; frame < 1000; frame++ {
							if plc, err := d16.Decode(s16, nil); !plc || err != nil {
								t.Fatalf("PLC S16 frame%d: %v,%v", frame, plc, err)
							}
							if plc, err := d24.DecodeS24(s24, nil, 1); !plc || err != nil {
								t.Fatalf("PLC S24 frame%d: %v,%v", frame, plc, err)
							}
							if plc, err := dp.DecodeS24LE(packed, nil, 1); !plc || err != nil {
								t.Fatalf("PLC packed frame%d: %v,%v", frame, plc, err)
							}
							if plc, err := df.DecodeFloat32(floats, nil, 1); !plc || err != nil {
								t.Fatalf("PLC float frame%d: %v,%v", frame, plc, err)
							}
							for i, v := range floats {
								if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < -1 || v > 1 {
									t.Fatalf("PLC frame%d sample%d: invalid float PCM%g", frame, i, v)
								}
								if frame >= 400 && (v < -1e-12 || v > 1e-12) {
									t.Fatalf("PLC frame%d sample%d amplified after attenuation: %g", frame, i, v)
								}
								if s24[i] < -8388608 || s24[i] > 8388607 {
									t.Fatalf("PLC S24 out of range %d", s24[i])
								}
								if frame >= 400 && (s16[i] != 0 || s24[i] != 0) {
									t.Fatalf("PLC frame%d sample%d not silent: S16=%d S24=%d", frame, i, s16[i], s24[i])
								}
							}
							if frame >= 400 {
								for _, b := range packed {
									if b != 0 {
										t.Fatalf("PLC packed output not silent at frame%d", frame)
									}
								}
							}
						}
						// Recovery after a long burst must remain usable.
						if err := e.Encode(encoded, pcm); err != nil {
							t.Fatal(err)
						}
						if plc, err := df.DecodeFloat32(floats, encoded, 1); plc || err != nil {
							t.Fatalf("recovery frame: %v,%v", plc, err)
						}
						for _, v := range floats {
							if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < -1 || v > 1 {
								t.Fatalf("invalid recovery PCM%g", v)
							}
						}
					})
				}
			}
		}
	}
	if modes != 69 {
		t.Fatalf("tested %d modes, want69", modes)
	}
}

func TestBinary32ScalingAtExponentLimits(t *testing.T) {
	values := []float32{0, math.Float32frombits(0x80000000), math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32,
		math.Float32frombits(0x007fffff), -math.Float32frombits(0x007fffff), math.Float32frombits(0x00800000), -math.Float32frombits(0x00800000),
		0x1p-120, -0x1p-120, 0.5, -0.5, math.MaxFloat32, -math.MaxFloat32, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())}
	for _, x := range values {
		for _, exp := range []int{-150, -15, -1, 0, 8, 15, 150} {
			want := float32(math.Ldexp(float64(x), exp))
			got := lc3_ldexpf(x, exp)
			if math.IsNaN(float64(want)) {
				if !math.IsNaN(float64(got)) {
					t.Errorf("scale(%g,%d)=%g wantNaN", x, exp, got)
				}
				continue
			}
			if math.Float32bits(got) != math.Float32bits(want) {
				t.Errorf("scale(%g,%d)=%g/%08x want%g/%08x", x, exp, got, math.Float32bits(got), want, math.Float32bits(want))
			}
		}
	}
}

func TestEncodeSubnormalFloatPCM(t *testing.T) {
	for _, c := range []Config{{SampleRate: 16000, FrameDuration: FrameDuration10M}, {SampleRate: 96000, FrameDuration: FrameDuration10M, HighResolution: true}} {
		t.Run(fmt.Sprint(c), func(t *testing.T) {
			e, silence := apiEncoder(t, c), apiEncoder(t, c)
			pcm, zero := make([]float32, e.FrameSamples()), make([]float32, e.FrameSamples())
			for i := range pcm {
				bits := uint32(i%8 + 1)
				if i&1 != 0 {
					bits |= 1 << 31
				}
				pcm[i] = math.Float32frombits(bits)
			}
			n, _ := c.FrameBytes(320000)
			got, want := make([]byte, n), make([]byte, n)
			for frame := 0; frame < 10; frame++ {
				if err := e.EncodeFloat32(got, pcm, 1); err != nil {
					t.Fatal(err)
				}
				if err := silence.EncodeFloat32(want, zero, 1); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("subnormal input did not quantize to silence at frame%d", frame)
				}
			}
		})
	}
}

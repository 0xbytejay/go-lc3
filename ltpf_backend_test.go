// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0

package lc3

import (
	"fmt"
	"math"
	"testing"
)

func ltpfBackendSamples(n, pattern int, seed uint32) []int16 {
	x := make([]int16, n)
	for i := range x {
		switch pattern {
		case 0:
			x[i] = -32768
		case 1:
			x[i] = 32767
		case 2:
			if i%2 == 0 {
				x[i] = -32768
			} else {
				x[i] = 32767
			}
		default:
			seed = seed*1664525 + 1013904223
			x[i] = int16(seed >> 16)
		}
	}
	return x
}

func TestLTPFBackendDotExact(t *testing.T) {
	for _, n := range []int{-1, 0, 1, 15, 16, 17, 31, 32, 63, 64, 127, 128, 191, 192, 255, 256, 513, 1024} {
		for patternA := 0; patternA < 4; patternA++ {
			for patternB := 0; patternB < 4; patternB++ {
				for _, offset := range []int{0, 1, 3, 7} {
					// Exact input extent also detects reading the ignored final
					// n%16 elements or loading beyond the last vector.
					length := max(0, (n>>4)*16)
					a := ltpfBackendSamples(length+offset, patternA, 0xa1b2c3d4)[offset:]
					b := ltpfBackendSamples(length+offset, patternB, 0x98abcdef)[offset:]
					want, got := ltpf_dot_scalar(a, b, n), ltpf_dot(a, b, n)
					if math.Float32bits(got) != math.Float32bits(want) {
						t.Fatalf("n=%d patterns=%d/%d offset=%d got=%08x want=%08x", n, patternA, patternB, offset, math.Float32bits(got), math.Float32bits(want))
					}
				}
			}
		}
	}
}

func TestLTPFBackendCorrelateExact(t *testing.T) {
	for _, n := range []int{0, 15, 16, 31, 128, 192} {
		for _, count := range []int{0, 1, 5, 64} {
			for pattern := 0; pattern < 4; pattern++ {
				a, b := ltpfBackendSamples(n+5, pattern, 42), ltpfBackendSamples(n+count+7, pattern, 314159)
				y := make([]float32, count+2)
				y[0], y[len(y)-1] = 0.25, -0.75
				ltpf_correlate(a, 3, b, count+3, n, y[1:len(y)-1], count)
				for i := 0; i < count; i++ {
					want := ltpf_dot_scalar(a[3:], b[count+3-i:], n)
					if math.Float32bits(y[i+1]) != math.Float32bits(want) {
						t.Fatalf("n=%d count=%d pattern=%d index=%d got=%08x want=%08x", n, count, pattern, i, math.Float32bits(y[i+1]), math.Float32bits(want))
					}
				}
				if y[0] != 0.25 || y[len(y)-1] != -0.75 {
					t.Fatal("correlation overwrote output guard")
				}
			}
		}
	}
}

func TestLTPFBackendResampleExact(t *testing.T) {
	cases := []struct {
		p, step, w int
		h          []int16
	}{
		{8, 5, 10, h_8k_12k8_q15[:]},
		{4, 5, 20, h_16k_12k8_q15[:]},
		{8, 15, 30, h_24k_12k8_q15[:]},
		{2, 5, 40, h_32k_12k8_q15[:]},
		{4, 15, 60, h_48k_12k8_q15[:]},
		{2, 15, 120, h_96k_12k8_q15[:]},
	}
	for _, tc := range cases {
		for _, n := range []int{0, 1, 3, 17, 128} {
			for pattern := 0; pattern < 4; pattern++ {
				xoff := tc.w + 3
				length := xoff + max(1, tc.step*n/tc.p) + tc.w
				x := ltpfBackendSamples(length, pattern, 20261106)
				want, got := make([]int16, n+2), make([]int16, n+2)
				want[0], want[n+1], got[0], got[n+1] = -123, 456, -123, 456
				stateWant := lc3_ltpf_hp50_state{s1: 123456789, s2: -987654321}
				stateGot := stateWant
				ltpf_resample_scalar(tc.p, tc.step, tc.w, tc.h, &stateWant, x, xoff, want[1:n+1], n)
				ltpf_resample(tc.p, tc.step, tc.w, tc.h, &stateGot, x, xoff, got[1:n+1], n)
				for i := range got {
					if got[i] != want[i] {
						t.Fatalf("w=%d n=%d pattern=%d index=%d got=%d want=%d", tc.w, n, pattern, i, got[i], want[i])
					}
				}
				if stateGot != stateWant {
					t.Fatalf("w=%d n=%d pattern=%d state=%+v want=%+v", tc.w, n, pattern, stateGot, stateWant)
				}
			}
		}
	}
}

var ltpfBackendBenchmarkSink float32

func BenchmarkLTPFBackendDot(b *testing.B) {
	for _, n := range []int{128, 192, 384} {
		a, c := ltpfBackendSamples(n, 3, 1), ltpfBackendSamples(n, 3, 2)
		b.Run(fmt.Sprintf("%d/scalar", n), func(b *testing.B) {
			var result float32
			for i := 0; i < b.N; i++ {
				result = ltpf_dot_scalar(a, c, n)
			}
			ltpfBackendBenchmarkSink = result
		})
		b.Run(fmt.Sprintf("%d/backend", n), func(b *testing.B) {
			var result float32
			for i := 0; i < b.N; i++ {
				result = ltpf_dot(a, c, n)
			}
			ltpfBackendBenchmarkSink = result
		})
	}
}

func BenchmarkLTPFBackendResample(b *testing.B) {
	for _, tc := range []struct {
		p, step, w int
		h          []int16
	}{{4, 5, 20, h_16k_12k8_q15[:]}, {4, 15, 60, h_48k_12k8_q15[:]}, {2, 15, 120, h_96k_12k8_q15[:]}} {
		x := ltpfBackendSamples(2048, 3, 1)
		y := make([]int16, 128)
		b.Run(fmt.Sprintf("%d/scalar", tc.w), func(b *testing.B) {
			var hp lc3_ltpf_hp50_state
			for i := 0; i < b.N; i++ {
				ltpf_resample_scalar(tc.p, tc.step, tc.w, tc.h, &hp, x, tc.w, y, len(y))
			}
		})
		b.Run(fmt.Sprintf("%d/backend", tc.w), func(b *testing.B) {
			var hp lc3_ltpf_hp50_state
			for i := 0; i < b.N; i++ {
				ltpf_resample(tc.p, tc.step, tc.w, tc.h, &hp, x, tc.w, y, len(y))
			}
		})
	}
}

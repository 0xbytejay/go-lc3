// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

package lc3

import (
	"fmt"
	"math"
	"testing"
	"unsafe"
)

func fftTestInput(n, pattern int) []lc3_complex {
	x := make([]lc3_complex, n)
	seed := uint32(0x15c3)
	edge := []uint32{0, 0x80000000, 1, 0x80000001, 0x007fffff, 0x807fffff, 0x00800000, 0x80800000, 0x3f800000, 0xbf800000, 0x39800000, 0xb9800000, 0x67800000, 0xe7800000}
	next := func() float32 { seed = seed*1664525 + 1013904223; return float32(int32(seed)) / float32(65536) }
	for i := range x {
		if pattern == 0 {
			x[i] = lc3_complex{next(), next()}
		} else {
			x[i] = lc3_complex{math.Float32frombits(edge[(2*i)%len(edge)]), math.Float32frombits(edge[(2*i+1)%len(edge)])}
		}
	}
	return x
}

func checkFFTKernel(t *testing.T, n int, pattern int, scalar, selected func([]lc3_complex, []lc3_complex)) {
	t.Helper()
	input := fftTestInput(n, pattern)
	before := append([]lc3_complex(nil), input...)
	want := make([]lc3_complex, n)
	marker := lc3_complex{math.Float32frombits(0x7fc01234), math.Float32frombits(0xffc05678)}
	padded := make([]lc3_complex, n+2)
	padded[0], padded[n+1] = marker, marker
	selected(input[:n:n], padded[1:n+1:n+1])
	scalar(input[:n:n], want)
	same := func(a, b lc3_complex) bool {
		return math.Float32bits(a.re) == math.Float32bits(b.re) && math.Float32bits(a.im) == math.Float32bits(b.im)
	}
	if !same(padded[0], marker) || !same(padded[n+1], marker) {
		t.Fatal("kernel wrote outside the output")
	}
	for i := range input {
		if !same(input[i], before[i]) {
			t.Fatalf("input changed at %d", i)
		}
		if !same(padded[i+1], want[i]) {
			t.Fatalf("backend=%s index=%d: got=(%08x,%08x) want=(%08x,%08x)", fftBackendName, i, math.Float32bits(padded[i+1].re), math.Float32bits(padded[i+1].im), math.Float32bits(want[i].re), math.Float32bits(want[i].im))
		}
	}
}

func TestFFTBackendMatchesScalar(t *testing.T) {
	t.Logf("FFT backend: %s", fftBackendName)
	for pattern := 0; pattern < 2; pattern++ {
		for _, n := range []int{0, 1, 2, 3, 4, 5, 7, 8, 16, 24, 32, 48, 96} {
			t.Run(fmt.Sprintf("fft5/n%d/p%d", n, pattern), func(t *testing.T) {
				checkFFTKernel(t, 5*n, pattern, func(x, y []lc3_complex) { fft_5_scalar(x, y, n) }, func(x, y []lc3_complex) { fft_5(x, y, n) })
			})
		}
		for _, table := range lc3_fft_twiddles_bf3 {
			for _, n := range []int{0, 1, 2, 3, 5, 8} {
				t.Run(fmt.Sprintf("bf3/n3_%d/n%d/p%d", table.n3, n, pattern), func(t *testing.T) {
					checkFFTKernel(t, 3*n*table.n3, pattern, func(x, y []lc3_complex) { fft_bf3_scalar(table, x, y, n) }, func(x, y []lc3_complex) { fft_bf3(table, x, y, n) })
				})
			}
		}
		seen := map[*lc3_fft_bf2_twiddles]bool{}
		for _, row := range lc3_fft_twiddles_bf2 {
			for _, table := range row {
				if table == nil || seen[table] {
					continue
				}
				seen[table] = true
				for _, n := range []int{0, 1, 2, 3, 5, 8} {
					t.Run(fmt.Sprintf("bf2/n2_%d/n%d/p%d", table.n2, n, pattern), func(t *testing.T) {
						checkFFTKernel(t, 2*n*table.n2, pattern, func(x, y []lc3_complex) { fft_bf2_scalar(table, x, y, n) }, func(x, y []lc3_complex) { fft_bf2(table, x, y, n) })
					})
				}
			}
		}
	}
}

func BenchmarkFFTBackend(b *testing.B) {
	for _, n := range []int{16, 48, 96} {
		input := fftTestInput(5*n, 0)
		out := make([]lc3_complex, len(input))
		for _, v := range []struct {
			name string
			f    func([]lc3_complex, []lc3_complex, int)
		}{{"scalar", fft_5_scalar}, {"selected", fft_5}} {
			b.Run(fmt.Sprintf("fft5/n%d/%s", n, v.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					v.f(input, out, n)
				}
			})
		}
	}
	for _, table := range []*lc3_fft_bf2_twiddles{&fft_twiddles_10, &fft_twiddles_40, &fft_twiddles_120} {
		const n = 2
		input := fftTestInput(2*n*table.n2, 0)
		out := make([]lc3_complex, len(input))
		for _, v := range []struct {
			name string
			f    func(*lc3_fft_bf2_twiddles, []lc3_complex, []lc3_complex, int)
		}{{"scalar", fft_bf2_scalar}, {"selected", fft_bf2}} {
			b.Run(fmt.Sprintf("bf2/n2_%d/%s", table.n2, v.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					v.f(table, input, out, n)
				}
			})
		}
	}
	for _, table := range lc3_fft_twiddles_bf3 {
		const n = 2
		input := fftTestInput(3*n*table.n3, 0)
		out := make([]lc3_complex, len(input))
		for _, v := range []struct {
			name string
			f    func(*lc3_fft_bf3_twiddles, []lc3_complex, []lc3_complex, int)
		}{{"scalar", fft_bf3_scalar}, {"selected", fft_bf3}} {
			b.Run(fmt.Sprintf("bf3/n3_%d/%s", table.n3, v.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					v.f(table, input, out, n)
				}
			})
		}
	}
}

func TestFFTComplexLayout(t *testing.T) {
	if unsafe.Sizeof(lc3_complex{}) != 8 || unsafe.Offsetof(lc3_complex{}.re) != 0 || unsafe.Offsetof(lc3_complex{}.im) != 4 {
		t.Fatal("FFT complex layout is not two adjacent float32 values")
	}
}

func TestFFTBackendMisalignedComplexSlices(t *testing.T) {
	// Adjacent candidates differ by 4 modulo 8. Select and verify an actual
	// 4-byte-aligned address instead of assuming the allocator's base alignment.
	// Vector accesses must not require 8- or 16-byte aligned slice addresses.
	type storage struct {
		prefix float32
		values [82]lc3_complex
	}
	var source, output [2]storage
	pick := func(candidates *[2]storage) []lc3_complex {
		for i := range candidates {
			values := candidates[i].values[:]
			if uintptr(unsafe.Pointer(&values[0]))%8 == 4 {
				return values
			}
		}
		t.Fatal("test did not obtain a 4-byte-aligned complex slice")
		return nil
	}
	src, dst := pick(&source), pick(&output)
	const n = 16
	x, y := src[:80:80], dst[1:81:81]
	if uintptr(unsafe.Pointer(&x[0]))%8 != 4 || uintptr(unsafe.Pointer(&y[0]))%8 != 4 {
		t.Fatal("test slices are not 4-byte aligned")
	}
	copy(x, fftTestInput(len(x), 0))
	expected := make([]lc3_complex, len(y))
	marker := lc3_complex{123, -456}
	dst[0], dst[81] = marker, marker
	fft_5_scalar(x, expected, n)
	fft_5(x, y, n)
	for i := range y {
		if math.Float32bits(y[i].re) != math.Float32bits(expected[i].re) || math.Float32bits(y[i].im) != math.Float32bits(expected[i].im) {
			t.Fatalf("unaligned FFT differs at %d", i)
		}
	}
	if dst[0] != marker || dst[81] != marker {
		t.Fatal("unaligned FFT changed guard samples")
	}
}

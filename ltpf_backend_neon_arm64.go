// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0

//go:build arm64 && go1.27 && goexperiment.simd && !purego

package lc3

import "simd/archsimd"

// ltpf_dot preserves the scalar implementation's 16-sample truncation and
// signed 64-bit accumulation. Widen every product before adding: combining
// pairs in int32 first would overflow for full-scale negative input.
func ltpf_dot(a, b []int16, n int) float32 {
	limit := (n >> 4) * 16
	if limit <= 0 {
		return 0
	}
	a, b = a[:limit], b[:limit]
	var lo, hi archsimd.Int64x2
	for i := 0; i < limit; i += 8 {
		x, y := archsimd.LoadInt16x8(a[i:]), archsimd.LoadInt16x8(b[i:])
		p0 := x.MulWidenLo(y)
		p1 := x.HiToLo().MulWidenLo(y.HiToLo())
		lo = lo.Add(p0.ExtendLo2ToInt64()).Add(p0.HiToLo().ExtendLo2ToInt64())
		hi = hi.Add(p1.ExtendLo2ToInt64()).Add(p1.HiToLo().ExtendLo2ToInt64())
	}
	sum := lo.Add(hi)
	v := sum.GetElem(0) + sum.GetElem(1)
	return float32(int32((v + 32) >> 6))
}

// ltpfResampleDot uses modulo-int32 addition, matching the scalar convolution.
// The hp50 filter remains serial in ltpf_resample because each output depends
// on the filter state from the previous output.
func ltpfResampleDot(a, b []int16, n int) int32 {
	var lo, hi archsimd.Int32x4
	i := 0
	for ; i+8 <= n; i += 8 {
		x, y := archsimd.LoadInt16x8(a[i:]), archsimd.LoadInt16x8(b[i:])
		lo = lo.Add(x.MulWidenLo(y))
		hi = hi.Add(x.HiToLo().MulWidenLo(y.HiToLo()))
	}
	v := lo.Add(hi).ReduceSum()
	for ; i < n; i++ {
		v += int32(a[i]) * int32(b[i])
	}
	return v
}

func ltpf_resample(p, step, w int, h []int16, hp *lc3_ltpf_hp50_state, x []int16, xoff int, y []int16, n int) {
	if w < 16 {
		ltpf_resample_scalar(p, step, w, h, hp, x, xoff, y, n)
		return
	}
	xoff -= w - 1
	for i := 0; i < step*n; i += step {
		hi := (i % p) * w
		xi := xoff + i/p
		un := ltpfResampleDot(x[xi:xi+w], h[hi:hi+w], w)
		yn := ltpf_filter_hp50(hp, un)
		y[i/step] = int16((yn + (1 << 15)) >> 16)
	}
}

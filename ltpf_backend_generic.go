// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !arm64 || !go1.27 || !goexperiment.simd || purego

package lc3

func ltpf_dot(a, b []int16, n int) float32 {
	return ltpf_dot_scalar(a, b, n)
}

func ltpf_resample(p, step, w int, h []int16, hp *lc3_ltpf_hp50_state, x []int16, xoff int, y []int16, n int) {
	ltpf_resample_scalar(p, step, w, h, hp, x, xoff, y, n)
}

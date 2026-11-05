// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

//go:build !arm64 || !go1.27 || !goexperiment.simd || purego

package lc3

const fftBackendName = "scalar"

func fft_5(x, y []lc3_complex, n int) { fft_5_scalar(x, y, n) }
func fft_bf3(t *lc3_fft_bf3_twiddles, x, y []lc3_complex, n int) {
	fft_bf3_scalar(t, x, y, n)
}
func fft_bf2(t *lc3_fft_bf2_twiddles, x, y []lc3_complex, n int) {
	fft_bf2_scalar(t, x, y, n)
}

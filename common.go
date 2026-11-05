// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: ported to Go, preserving the scalar operation order.

package lc3

type lc3_dt = int
type lc3_srate = int
type lc3_bandwidth = int

const (
	lc3_DT_2M5 = iota
	lc3_DT_5M
	lc3_DT_7M5
	lc3_DT_10M
	lc3_NUM_DT
)
const (
	lc3_SRATE_8K = iota
	lc3_SRATE_16K
	lc3_SRATE_24K
	lc3_SRATE_32K
	lc3_SRATE_48K
	lc3_SRATE_48K_HR
	lc3_SRATE_96K_HR
	lc3_NUM_SRATE
)
const (
	lc3_BANDWIDTH_NB       = lc3_SRATE_8K
	lc3_BANDWIDTH_WB       = lc3_SRATE_16K
	lc3_BANDWIDTH_SSWB     = lc3_SRATE_24K
	lc3_BANDWIDTH_SWB      = lc3_SRATE_32K
	lc3_BANDWIDTH_FB       = lc3_SRATE_48K
	lc3_BANDWIDTH_FB_HR    = lc3_SRATE_48K_HR
	lc3_BANDWIDTH_UB_HR    = lc3_SRATE_96K_HR
	lc3_NUM_BANDWIDTH      = lc3_NUM_SRATE
	lc3_MIN_FRAME_BYTES    = 20
	lc3_MAX_FRAME_BYTES    = 400
	lc3_HR_MAX_FRAME_BYTES = 625
	lc3_MAX_NS             = 960
	lc3_MAX_NE             = 960
	lc3_MAX_BANDS          = 64
)

type lc3_complex struct{ re, im float32 }
type lc3_fft_bf3_twiddles struct {
	n3 int
	t  [][2]lc3_complex
}
type lc3_fft_bf2_twiddles struct {
	n2 int
	t  []lc3_complex
}
type lc3_mdct_rot_def struct {
	n4 int
	w  []lc3_complex
}
type lc3_sns_vq_gains_def struct {
	count int
	v     []float32
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func lc3_hr(sr int) bool    { return sr >= lc3_SRATE_48K_HR }
func lc3_ns(dt, sr int) int { return lc3_ns_2m5[sr] * (1 + dt) }
func lc3_ne(dt, sr int) int { return lc3_ne_2m5[sr] * (1 + dt) }
func lc3_nd(dt, sr int) int {
	n := lc3_ns_2m5[sr]
	if dt == lc3_DT_7M5 {
		n = lc3_ns_4m[sr]
	}
	return (lc3_ns(dt, sr) + n) >> 1
}
func lc3_nh(dt, sr int) int {
	if sr > lc3_SRATE_48K_HR {
		return 0
	}
	return (8 + boolInt(dt == lc3_DT_7M5)) * lc3_ns_2m5[sr]
}
func lc3_nt(sr int) int { return lc3_ns_2m5[sr] >> 1 }
func lc3_min_frame_bytes(dt, sr int) int {
	if !lc3_hr(sr) {
		return lc3_MIN_FRAME_BYTES
	}
	return lc3_frame_bytes_hr_lim[dt][sr-lc3_SRATE_48K_HR][0]
}
func lc3_max_frame_bytes(dt, sr int) int {
	if !lc3_hr(sr) {
		return lc3_MAX_FRAME_BYTES
	}
	return lc3_frame_bytes_hr_lim[dt][sr-lc3_SRATE_48K_HR][1]
}

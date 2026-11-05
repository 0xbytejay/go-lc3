// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: translated bandwidth detection to Go.

package lc3

type bw_region struct{ is, ie int }

var bw_regions = [4][4][4]bw_region{
	{{{24, 35}}, {{24, 33}, {35, 40}}, {{24, 32}, {33, 39}, {39, 43}}, {{22, 30}, {31, 36}, {37, 41}, {41, 44}}},
	{{{39, 50}}, {{35, 45}, {47, 52}}, {{34, 43}, {44, 50}, {50, 54}}, {{32, 41}, {42, 47}, {48, 52}, {52, 55}}},
	{{{51, 64}}, {{45, 56}, {58, 64}}, {{42, 52}, {53, 59}, {60, 64}}, {{40, 49}, {51, 56}, {57, 61}, {61, 64}}},
	{{{53, 64}}, {{47, 57}, {59, 64}}, {{44, 53}, {54, 60}, {60, 64}}, {{41, 50}, {51, 56}, {57, 61}, {61, 64}}},
}
var bw_l = [4][4]int{{4, 4, 3, 1}, {4, 4, 3, 1}, {4, 4, 3, 2}, {4, 4, 3, 1}}

func lc3_bwdet_run(dt, sr int, e []float32) int {
	bw0, bwn := lc3_BANDWIDTH_NB, sr
	if bwn <= bw0 || lc3_hr(sr) {
		return bwn
	}
	bwr := bw_regions[dt][bwn-1]
	for bw := bw0; bw < bwn; bw++ {
		i, ie := bwr[bw].is, bwr[bw].ie
		n := ie - i
		se := e[i]
		for i++; i < ie; i++ {
			se = float32(se + e[i])
		}
		threshold := 10
		if bw == lc3_BANDWIDTH_NB {
			threshold <<= 1
		}
		if se >= float32(threshold*n) {
			bw0 = bw + 1
		}
	}
	hold := bw0 >= bwn
	if !hold {
		i0, l := bwr[bw0].is, bw_l[dt][bw0]
		tc := [4]float32{31.62277660, 199.52623150, 100, 100}[bw0]
		for i := i0 - l + 1; !hold && i <= i0+1; i++ {
			hold = e[i-l] > float32(tc*e[i])
		}
	}
	if hold {
		return bw0
	}
	return bwn
}
func lc3_bwdet_get_nbits(sr int) int {
	if lc3_hr(sr) {
		return 0
	}
	n := 0
	if sr > 0 {
		n++
	}
	if sr > 1 {
		n++
	}
	if sr > 3 {
		n++
	}
	return n
}
func lc3_bwdet_put_bw(bits *lc3_bits_t, sr, bw int) {
	if n := lc3_bwdet_get_nbits(sr); n > 0 {
		lc3_put_bits(bits, bw, n)
	}
}
func lc3_bwdet_get_bw(bits *lc3_bits_t, sr int, bw *int) int {
	n := lc3_bwdet_get_nbits(sr)
	*bw = sr
	if n > 0 {
		*bw = lc3_get_bits(bits, n)
	}
	if *bw > sr {
		*bw = sr
		return -1
	}
	return 0
}

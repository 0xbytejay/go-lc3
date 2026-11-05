// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: translated attack detection to Go.

package lc3

type lc3_attdet_analysis_t struct {
	en1, an1 int32
	p_att    int
}

// x includes lc3_nt(sr) historical samples before the current frame.
func lc3_attdet_run(dt, sr, nbytes int, attdet *lc3_attdet_analysis_t, x []int16) bool {
	ranges := [2][2][2]int{{{61, 149}, {75, 149}}, {{81, 2147483647}, {100, 2147483647}}}
	if dt < lc3_DT_7M5 || sr < lc3_SRATE_32K || lc3_hr(sr) || nbytes < ranges[dt-lc3_DT_7M5][sr-lc3_SRATE_32K][0] || nbytes > ranges[dt-lc3_DT_7M5][sr-lc3_SRATE_32K][1] {
		return false
	}
	nblk := 4
	if dt == lc3_DT_7M5 {
		nblk--
	}
	var e [4]int32
	pos := lc3_nt(sr)
	for i := 0; i < nblk; i++ {
		if sr == lc3_SRATE_32K {
			xn2 := int16((int(x[pos-4]) + int(x[pos-3])) >> 1)
			xn1 := int16((int(x[pos-2]) + int(x[pos-1])) >> 1)
			for j := 0; j < 40; j++ {
				xn := int16((int(x[pos]) + int(x[pos+1])) >> 1)
				xf := int16((3*int(xn) - 4*int(xn1) + int(xn2)) >> 3)
				e[i] += int32((int(xf) * int(xf)) >> 5)
				pos += 2
				xn2, xn1 = xn1, xn
			}
		} else {
			xn2 := int16((int(x[pos-6]) + int(x[pos-5]) + int(x[pos-4])) >> 2)
			xn1 := int16((int(x[pos-3]) + int(x[pos-2]) + int(x[pos-1])) >> 2)
			for j := 0; j < 40; j++ {
				xn := int16((int(x[pos]) + int(x[pos+1]) + int(x[pos+2])) >> 2)
				xf := int16((3*int(xn) - 4*int(xn1) + int(xn2)) >> 3)
				e[i] += int32((int(xf) * int(xf)) >> 5)
				pos += 3
				xn2, xn1 = xn1, xn
			}
		}
	}
	p := 0
	for i := 0; i < nblk; i++ {
		a := max(attdet.an1>>2, attdet.en1)
		attdet.en1, attdet.an1 = e[i], a
		if e[i]>>3 > a+(a>>4) {
			p = i + 1
		}
	}
	att := attdet.p_att >= 1+(nblk>>1) || p > 0
	attdet.p_att = p
	return att
}

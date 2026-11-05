// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: ported fastmath.h to Go with explicit binary32 rounding.

package lc3

import "math"

func lc3_ldexpf(x float32, exp int) float32 {
	bits := math.Float32bits(x)
	e := int((bits >> 23) & 255)
	// liblc3's bit shortcut assumes a normal finite result (and commonly FTZ).
	// Go preserves subnormals: prolonged PLC or very quiet float PCM can leave
	// that range. Scale correctly instead of wrapping the exponent into NaN/Inf.
	if e == 0 || e == 255 || exp <= -e || exp >= 255-e {
		return float32(math.Ldexp(float64(x), exp))
	}
	return math.Float32frombits(bits + uint32(int32(exp)<<23))
}
func lc3_frexpf(x float32, exp *int) float32 {
	bits := math.Float32bits(x)
	*exp = int((bits&0x7f800000)>>23) - 126
	return math.Float32frombits((bits &^ 0x7f800000) | (126 << 23))
}
func lc3_exp2f(x float32) float32 {
	e := [8]float32{1, 1.09050773, 1.18920712, 1.29683955, 1.41421356, 1.54221083, 1.68179283, 1.83400809}
	p := [4]float32{1.00448128e-2, 5.54563260e-2, 2.40228756e-1, 6.93147140e-1}
	v := float32(x + float32(0x1.8p20))
	k := int32(math.Float32bits(v))
	x = float32(x - float32(v-float32(0x1.8p20)))
	y := float32(p[0] * x)
	y = float32(float32(y+p[1]) * x)
	y = float32(float32(y+p[2]) * x)
	y = float32(float32(y+p[3]) * x)
	y = float32(float32(y+1) * e[k&7])
	return math.Float32frombits(math.Float32bits(y) + uint32((k>>3)<<23))
}
func lc3_log2f(x float32) float32 {
	c := [5]float32{-1.29479677, 5.11769018, -8.42295281, 8.10557963, -3.50567360}
	var e int
	x = lc3_frexpf(x, &e)
	y := float32(c[0] * x)
	y = float32(float32(y+c[1]) * x)
	y = float32(float32(y+c[2]) * x)
	y = float32(float32(y+c[3]) * x)
	y = float32(y + c[4])
	return float32(float32(e) + y)
}
func lc3_log10f(x float32) float32 { return float32(float32(0.30102999566398119521) * lc3_log2f(x)) }
func lc3_db_q16(x float32) int32 {
	t := [32][2]uint16{
		{0, 4379}, {4379, 4248}, {8627, 4125}, {12753, 4009}, {16762, 3899}, {20661, 3795}, {24456, 3697}, {28153, 3603},
		{31755, 3514}, {35269, 3429}, {38699, 3349}, {42047, 3272}, {45319, 3198}, {48517, 3128}, {51645, 3061}, {54705, 2996},
		{8381, 2934}, {11315, 2875}, {14190, 2818}, {17008, 2763}, {19772, 2711}, {22482, 2660}, {25142, 2611}, {27754, 2564},
		{30318, 2519}, {32837, 2475}, {35312, 2433}, {37744, 2392}, {40136, 2352}, {42489, 2314}, {44803, 2277}, {47080, 2241},
	}
	bits := math.Float32bits(float32(x * x))
	e2 := int32(bits>>22) - 254
	hi := (bits >> 18) & 31
	lo := int32((bits >> 2) & 0xffff)
	return e2*49321 + int32(t[hi][0]) + ((int32(t[hi][1]) * lo) >> 16)
}

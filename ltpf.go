// Copyright 2022 Google LLC
// Copyright 2026 The go-lc3 Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Modified from liblc3: long-term postfilter translated to Go. Explicit binary32
// rounding preserves the scalar non-reassociated, non-contracted arithmetic.

package lc3

import "math"

type lc3_ltpf_hp50_state struct{ s1, s2 int64 }
type lc3_ltpf_analysis_t struct {
	active bool
	pitch  int
	nc     [2]float32
	hp50   lc3_ltpf_hp50_state
	x_12k8 [384]int16
	x_6k4  [178]int16
	tc     int
}
type lc3_ltpf_synthesis_t struct {
	active bool
	pitch  int
	c      [24]float32
	x      [12]float32
}
type lc3_ltpf_data_t struct {
	active      bool
	pitch_index int
}

var h_8k_12k8_q15 = [...]int16{
	214, 417, -1052, -4529, 26233, -4529, -1052, 417, 214, 0,
	180, 0, -1522, -2427, 24506, -5289, 0, 763, 156, -28,
	92, -323, -1361, 0, 19741, -3885, 1317, 861, 0, -61,
	0, -457, -752, 1873, 13068, 0, 2389, 598, -213, -79,
	-61, -398, 0, 2686, 5997, 5997, 2686, 0, -398, -61,
	-79, -213, 598, 2389, 0, 13068, 1873, -752, -457, 0,
	-61, 0, 861, 1317, -3885, 19741, 0, -1361, -323, 92,
	-28, 156, 763, 0, -5289, 24506, -2427, -1522, 0, 180,
}
var h_16k_12k8_q15 = [...]int16{
	-61, 214, -398, 417, 0, -1052, 2686, -4529, 5997, 26233,
	5997, -4529, 2686, -1052, 0, 417, -398, 214, -61, 0,

	-79, 180, -213, 0, 598, -1522, 2389, -2427, 0, 24506,
	13068, -5289, 1873, 0, -752, 763, -457, 156, 0, -28,

	-61, 92, 0, -323, 861, -1361, 1317, 0, -3885, 19741,
	19741, -3885, 0, 1317, -1361, 861, -323, 0, 92, -61,

	-28, 0, 156, -457, 763, -752, 0, 1873, -5289, 13068,
	24506, 0, -2427, 2389, -1522, 598, 0, -213, 180, -79,
}
var h_32k_12k8_q15 = [...]int16{
	-30, -31, 46, 107, 0, -199, -162, 209, 430, 0,
	-681, -526, 658, 1343, 0, -2264, -1943, 2999, 9871, 13116,
	9871, 2999, -1943, -2264, 0, 1343, 658, -526, -681, 0,
	430, 209, -162, -199, 0, 107, 46, -31, -30, 0,

	-14, -39, 0, 90, 78, -106, -229, 0, 382, 299,
	-376, -761, 0, 1194, 937, -1214, -2644, 0, 6534, 12253,
	12253, 6534, 0, -2644, -1214, 937, 1194, 0, -761, -376,
	299, 382, 0, -229, -106, 78, 90, 0, -39, -14,
}
var h_24k_12k8_q15 = [...]int16{
	-50, 19, 143, -93, -290, 278, 485, -658, -701, 1396,
	901, -3019, -1042, 10276, 17488, 10276, -1042, -3019, 901, 1396,
	-701, -658, 485, 278, -290, -93, 143, 19, -50, 0,

	-46, 0, 141, -45, -305, 185, 543, -501, -854, 1153,
	1249, -2619, -1908, 8712, 17358, 11772, 0, -3319, 480, 1593,
	-504, -796, 399, 367, -261, -142, 138, 40, -52, -5,

	-41, -17, 133, 0, -304, 91, 574, -334, -959, 878,
	1516, -2143, -2590, 7118, 16971, 13161, 1202, -3495, 0, 1731,
	-267, -908, 287, 445, -215, -188, 125, 62, -52, -12,

	-34, -30, 120, 41, -291, 0, 577, -164, -1015, 585,
	1697, -1618, -3084, 5534, 16337, 14406, 2544, -3526, -523, 1800,
	0, -985, 152, 509, -156, -230, 104, 83, -48, -19,

	-26, -41, 103, 76, -265, -83, 554, 0, -1023, 288,
	1791, -1070, -3393, 3998, 15474, 15474, 3998, -3393, -1070, 1791,
	288, -1023, 0, 554, -83, -265, 76, 103, -41, -26,

	-19, -48, 83, 104, -230, -156, 509, 152, -985, 0,
	1800, -523, -3526, 2544, 14406, 16337, 5534, -3084, -1618, 1697,
	585, -1015, -164, 577, 0, -291, 41, 120, -30, -34,

	-12, -52, 62, 125, -188, -215, 445, 287, -908, -267,
	1731, 0, -3495, 1202, 13161, 16971, 7118, -2590, -2143, 1516,
	878, -959, -334, 574, 91, -304, 0, 133, -17, -41,

	-5, -52, 40, 138, -142, -261, 367, 399, -796, -504,
	1593, 480, -3319, 0, 11772, 17358, 8712, -1908, -2619, 1249,
	1153, -854, -501, 543, 185, -305, -45, 141, 0, -46,
}
var h_48k_12k8_q15 = [...]int16{
	-13, -25, -20, 10, 51, 71, 38, -47, -133, -145,
	-42, 139, 277, 242, 0, -329, -511, -351, 144, 698,
	895, 450, -535, -1510, -1697, -521, 1999, 5138, 7737, 8744,
	7737, 5138, 1999, -521, -1697, -1510, -535, 450, 895, 698,
	144, -351, -511, -329, 0, 242, 277, 139, -42, -145,
	-133, -47, 38, 71, 51, 10, -20, -25, -13, 0,

	-9, -23, -24, 0, 41, 71, 52, -23, -115, -152,
	-78, 92, 254, 272, 76, -251, -493, -427, 0, 576,
	900, 624, -262, -1309, -1763, -954, 1272, 4356, 7203, 8679,
	8169, 5886, 2767, 0, -1542, -1660, -809, 240, 848, 796,
	292, -252, -507, -398, -82, 199, 288, 183, 0, -130,
	-145, -71, 20, 69, 60, 20, -15, -26, -17, -3,

	-6, -20, -26, -8, 31, 67, 62, 0, -94, -152,
	-108, 45, 223, 287, 143, -167, -454, -480, -134, 439,
	866, 758, 0, -1071, -1748, -1295, 601, 3559, 6580, 8485,
	8485, 6580, 3559, 601, -1295, -1748, -1071, 0, 758, 866,
	439, -134, -480, -454, -167, 143, 287, 223, 45, -108,
	-152, -94, 0, 62, 67, 31, -8, -26, -20, -6,

	-3, -17, -26, -15, 20, 60, 69, 20, -71, -145,
	-130, 0, 183, 288, 199, -82, -398, -507, -252, 292,
	796, 848, 240, -809, -1660, -1542, 0, 2767, 5886, 8169,
	8679, 7203, 4356, 1272, -954, -1763, -1309, -262, 624, 900,
	576, 0, -427, -493, -251, 76, 272, 254, 92, -78,
	-152, -115, -23, 52, 71, 41, 0, -24, -23, -9,
}
var h_96k_12k8_q15 = [...]int16{
	-3, -7, -10, -13, -13, -10, -4, 5, 15, 26,
	33, 36, 31, 19, 0, -23, -47, -66, -76, -73,
	-54, -21, 23, 70, 111, 139, 143, 121, 72, 0,
	-84, -165, -227, -256, -240, -175, -67, 72, 219, 349,
	433, 448, 379, 225, 0, -268, -536, -755, -874, -848,
	-648, -260, 301, 1000, 1780, 2569, 3290, 3869, 4243, 4372,
	4243, 3869, 3290, 2569, 1780, 1000, 301, -260, -648, -848,
	-874, -755, -536, -268, 0, 225, 379, 448, 433, 349,
	219, 72, -67, -175, -240, -256, -227, -165, -84, 0,
	72, 121, 143, 139, 111, 70, 23, -21, -54, -73,
	-76, -66, -47, -23, 0, 19, 31, 36, 33, 26,
	15, 5, -4, -10, -13, -13, -10, -7, -3, 0,

	-1, -5, -8, -12, -13, -12, -8, 0, 10, 21,
	30, 35, 34, 26, 10, -11, -35, -58, -73, -76,
	-65, -39, 0, 46, 92, 127, 144, 136, 100, 38,
	-41, -125, -199, -246, -254, -214, -126, 0, 146, 288,
	398, 450, 424, 312, 120, -131, -405, -655, -830, -881,
	-771, -477, 0, 636, 1384, 2178, 2943, 3601, 4084, 4340,
	4340, 4084, 3601, 2943, 2178, 1384, 636, 0, -477, -771,
	-881, -830, -655, -405, -131, 120, 312, 424, 450, 398,
	288, 146, 0, -126, -214, -254, -246, -199, -125, -41,
	38, 100, 136, 144, 127, 92, 46, 0, -39, -65,
	-76, -73, -58, -35, -11, 10, 26, 34, 35, 30,
	21, 10, 0, -8, -12, -13, -12, -8, -5, -1,
}

func ltpf_filter_hp50(s *lc3_ltpf_hp50_state, xn int32) int32 {
	const a1, a2 int64 = -2110217691, 1037111617
	const b1, b2 int64 = -2110535566, 1055267782
	yn := int32((s.s1 + int64(xn)*b2) >> 30)
	s.s1 = s.s2 + int64(xn)*b1 - int64(yn)*a1
	s.s2 = int64(xn)*b2 - int64(yn)*a2
	return yn
}

// x includes history before xoff; samples before the present frame participate
// in each phase's convolution, as in the reference resampling filter.
func ltpf_resample_scalar(p, step, w int, h []int16, hp *lc3_ltpf_hp50_state, x []int16, xoff int, y []int16, n int) {
	xoff -= w - 1
	for i := 0; i < step*n; i += step {
		hi := (i % p) * w
		xi := xoff + i/p
		var un int32
		for k := 0; k < w; k++ {
			un += int32(x[xi+k]) * int32(h[hi+k])
		}
		yn := ltpf_filter_hp50(hp, un)
		y[i/step] = int16((yn + (1 << 15)) >> 16)
	}
}
func ltpf_resample_12k8(sr int, hp *lc3_ltpf_hp50_state, x []int16, xoff int, y []int16, n int) {
	switch sr {
	case lc3_SRATE_8K:
		ltpf_resample(8, 5, 10, h_8k_12k8_q15[:], hp, x, xoff, y, n)
	case lc3_SRATE_16K:
		ltpf_resample(4, 5, 20, h_16k_12k8_q15[:], hp, x, xoff, y, n)
	case lc3_SRATE_24K:
		ltpf_resample(8, 15, 30, h_24k_12k8_q15[:], hp, x, xoff, y, n)
	case lc3_SRATE_32K:
		ltpf_resample(2, 5, 40, h_32k_12k8_q15[:], hp, x, xoff, y, n)
	case lc3_SRATE_48K, lc3_SRATE_48K_HR:
		ltpf_resample(4, 15, 60, h_48k_12k8_q15[:], hp, x, xoff, y, n)
	case lc3_SRATE_96K_HR:
		ltpf_resample(2, 15, 120, h_96k_12k8_q15[:], hp, x, xoff, y, n)
	}
}
func ltpf_resample_6k4(x []int16, xoff int, y []int16, n int) {
	xoff--
	for i := 0; i < n; i++ {
		k := xoff + 2*i
		v := int32(x[k])*18477 + (int32(x[k-1])+int32(x[k+1]))*15424 + (int32(x[k-2])+int32(x[k+2]))*8105
		y[i] = int16(v >> 16)
	}
}
func ltpf_dot_scalar(a, b []int16, n int) float32 {
	var v int64
	for i := 0; i < (n>>4)*16; i++ {
		v += int64(int32(a[i]) * int32(b[i]))
	}
	return float32(int32((v + 32) >> 6))
}
func ltpf_correlate(a []int16, ao int, b []int16, bo, n int, y []float32, nc int) {
	for i := 0; i < nc; i++ {
		y[i] = ltpf_dot(a[ao:], b[bo-i:], n)
	}
}
func ltpf_argmax(x []float32, n int, xmax *float32) int {
	arg := 0
	*xmax = x[0]
	for i := 1; i < n; i++ {
		if *xmax < x[i] {
			*xmax = x[i]
			arg = i
		}
	}
	return arg
}
func ltpf_argmax_weighted(x []float32, n int, incr float32, xmax *float32) int {
	arg := 0
	*xmax = x[0]
	xw := x[0]
	w := float32(1 + incr)
	for i := 1; i < n; i++ {
		v := float32(x[i] * w)
		if xw < v {
			*xmax = x[i]
			arg = i
			xw = float32(x[i] * w)
		}
		w = float32(w + incr)
	}
	return arg
}
func ltpf_interpolate(x []int16, xoff, n, d int, y []int16) {
	h4 := [4][4]int32{{6877, 19121, 6877, 0}, {3506, 18025, 11000, 220}, {1300, 15048, 15048, 1300}, {220, 11000, 18025, 3506}}
	h := h4[d]
	// Each output uses samples x[i-2],x[i-1],x[i],x[i+1].
	for i := 0; i < n; i++ {
		k := xoff + i
		v := int32(x[k+1])*h[0] + int32(x[k])*h[1] + int32(x[k-1])*h[2] + int32(x[k-2])*h[3]
		y[i] = int16(v >> 15)
	}
}
func ltpf_interpolate_corr(x []float32, xoff, d int) float32 {
	h4 := [4][8]float32{
		{1.53572770e-02, -4.72963246e-02, 8.35788573e-02, 8.98638285e-01, 8.35788573e-02, -4.72963246e-02, 1.53572770e-02, 0},
		{2.74547165e-03, 4.59833449e-03, -7.54404636e-02, 8.17488686e-01, 3.30182571e-01, -1.05835916e-01, 2.86823405e-02, -2.87456116e-03},
		{-3.00125103e-03, 2.95038503e-02, -1.30305021e-01, 6.03297008e-01, 6.03297008e-01, -1.30305021e-01, 2.95038503e-02, -3.00125103e-03},
		{-2.87456116e-03, 2.86823405e-02, -1.05835916e-01, 3.30182571e-01, 8.17488686e-01, -7.54404636e-02, 4.59833449e-03, 2.74547165e-03},
	}
	h := h4[(4+d)%4]
	hi := 0
	var y float32
	if d < 0 {
		y = float32(x[xoff-4] * h[0])
		hi = 1
	} else if d > 0 {
		y = float32(x[xoff+4] * h[7])
	}
	// The seven-term sum is evaluated independently before adding y.
	v := float32(x[xoff-3] * h[hi])
	for i := 1; i < 7; i++ {
		v = float32(v + float32(x[xoff-3+i]*h[hi+i]))
	}
	return float32(y + v)
}
func ltpf_detect_pitch(s *lc3_ltpf_analysis_t, x []int16, xoff, n int, tc *int) bool {
	var r [98]float32
	var rm1, rm2 float32
	k0 := max(0, s.tc-4)
	nk := min(97, s.tc+4) - k0 + 1
	ltpf_correlate(x, xoff, x, xoff-17, n, r[:], 98)
	t1 := ltpf_argmax_weighted(r[:], 98, float32(-.5/97), &rm1)
	t2 := k0 + ltpf_argmax(r[k0:], nk, &rm2)
	a := ltpf_dot(x[xoff:], x[xoff:], n)
	var nc1, nc2 float32
	if rm1 > 0 {
		b := ltpf_dot(x[xoff-17-t1:], x[xoff-17-t1:], n)
		nc1 = float32(rm1 / float32(math.Sqrt(float64(float32(a*b)))))
	}
	if rm2 > 0 {
		b := ltpf_dot(x[xoff-17-t2:], x[xoff-17-t2:], n)
		nc2 = float32(rm2 / float32(math.Sqrt(float64(float32(a*b)))))
	}
	s.tc = t2
	nc := nc2
	if nc2 <= float32(.85*nc1) {
		s.tc = t1
		nc = nc1
	}
	*tc = 17 + s.tc
	return nc > .6
}
func ltpf_refine_pitch(x []int16, xoff, n, tc int, pitch *int) int {
	var r [17]float32
	var rm float32
	r0 := max(32, 2*tc-4)
	nr := min(228, 2*tc+4) - r0 + 1
	ltpf_correlate(x, xoff, x, xoff-(r0-4), n, r[:], nr+8)
	e := r0 + ltpf_argmax(r[4:], nr, &rm)
	re := e - (r0 - 4)
	f := 0
	dm := ltpf_interpolate_corr(r[:], re, 0)
	for i := 1; i <= 3; i++ {
		if e >= 127 && (i&1 != 0 || e >= 157) {
			continue
		}
		if d := ltpf_interpolate_corr(r[:], re, i); d > dm {
			dm = d
			f = i
		}
		if e > 32 {
			if d := ltpf_interpolate_corr(r[:], re, -i); d > dm {
				dm = d
				f = -i
			}
		}
	}
	if f < 0 {
		e--
		f += 4
	}
	*pitch = 4*e + f
	if e < 127 {
		return 4*e + f - 128
	}
	if e < 157 {
		return 2*e + (f >> 1) + 126
	}
	return e + 283
}

// lc3_ltpf_analyse consumes a temporal buffer containing nt(sr) history
// samples followed by lc3_ns(dt,sr) input samples.
func lc3_ltpf_analyse(dt, sr int, s *lc3_ltpf_analysis_t, x []int16, data *lc3_ltpf_data_t) bool {
	n12 := (1 + dt) * 32
	copy(s.x_12k8[:], s.x_12k8[n12:])
	o12 := len(s.x_12k8) - n12
	ltpf_resample_12k8(sr, &s.hp50, x, lc3_nt(sr), s.x_12k8[o12:], n12)
	if dt == lc3_DT_7M5 {
		o12 -= 44
	} else {
		o12 -= 24
	}
	n6 := n12 >> 1
	copy(s.x_6k4[:], s.x_6k4[n6:])
	o6 := len(s.x_6k4) - n6
	ltpf_resample_6k4(s.x_12k8[:], o12, s.x_6k4[o6:], n6)
	if dt == lc3_DT_2M5 {
		o12 -= n12
		o6 -= n6
		n12 *= 2
		n6 *= 2
	}
	tc, pitch := 0, 0
	var nc float32
	present := ltpf_detect_pitch(s, s.x_6k4[:], o6, n6, &tc)
	if present {
		var u, v [128]int16
		data.pitch_index = ltpf_refine_pitch(s.x_12k8[:], o12, n12, tc, &pitch)
		ltpf_interpolate(s.x_12k8[:], o12, n12, 0, u[:])
		ltpf_interpolate(s.x_12k8[:], o12-(pitch>>2), n12, pitch&3, v[:])
		a, b := ltpf_dot(u[:], u[:], n12), ltpf_dot(v[:], v[:], n12)
		nc = float32(ltpf_dot(u[:], v[:], n12) / float32(math.Sqrt(float64(float32(a*b)))))
	}
	if s.active {
		pd := max(pitch, s.pitch) - min(pitch, s.pitch)
		nd := float32(nc - s.nc[0])
		data.active = !lc3_hr(sr) && present && (nc > .9 || (nc > .84 && pd < 8 && nd > -.1))
	} else {
		data.active = !lc3_hr(sr) && present && (dt == lc3_DT_10M || s.nc[1] > .94) && (s.nc[0] > .94 && nc > .94)
	}
	s.active = data.active
	s.pitch = pitch
	s.nc[1] = s.nc[0]
	s.nc[0] = nc
	return present
}

func ltpf_synthesize_template(xh []float32, nh, lag int, x0 []float32, xoff, n int, c []float32, w, fade int) {
	var g float32
	if fade <= 0 {
		g = 1
	}
	var sign float32
	if fade > 0 {
		sign = 1
	} else if fade < 0 {
		sign = -1
	}
	incr := float32(sign / float32(n))
	var u [24]float32
	lag += w >> 1
	yi := xoff - lag
	if xoff < lag {
		yi = xoff + nh - lag
	}
	for j := 0; j < w-1; j++ {
		yy, xx := xh[yi], x0[j]
		yi++
		if yi >= nh {
			yi = 0
		}
		for k := 0; k <= j; k++ {
			u[j-k] = float32(u[j-k] - float32(yy*c[k]))
		}
		for k := 0; k <= j; k++ {
			u[j-k] = float32(u[j-k] + float32(xx*c[w+k]))
		}
	}
	for i := 0; i < n; i += w {
		for j := 0; j < w; j++ {
			yy, xx := xh[yi], xh[xoff]
			yi++
			if yi >= nh {
				yi = 0
			}
			for k := 0; k < w; k++ {
				z := (j + w - 1 - k) % w
				u[z] = float32(u[z] - float32(yy*c[k]))
			}
			for k := 0; k < w; k++ {
				z := (j + w - 1 - k) % w
				u[z] = float32(u[z] + float32(xx*c[w+k]))
			}
			xh[xoff] = float32(xx - float32(g*u[j]))
			xoff++
			u[j] = 0
			g = float32(g + incr)
		}
	}
}

// xh is the entire history ring; xoff identifies the current output frame.
func lc3_ltpf_synthesize(dt, sr, nbytes int, s *lc3_ltpf_synthesis_t, data *lc3_ltpf_data_t, xh []float32, xoff int) {
	nh := lc3_ns(dt, sr) + lc3_nh(dt, sr)
	idx := 0
	if data != nil {
		idx = data.pitch_index
	}
	pitch := ((idx>>2)+32)*4 + (idx & 3)
	if idx >= 440 {
		pitch = (idx - 283) * 4
	} else if idx >= 380 {
		pitch = ((idx>>1)-63)*4 + (idx&1)*2
	}
	pitch = (pitch*lc3_ns(lc3_DT_10M, sr) + 64) / 128
	nbits := (nbytes * 8 * (1 + lc3_DT_10M)) / (1 + dt)
	if dt == lc3_DT_2M5 {
		nbits = (6*nbits + 5) / 10
	}
	if dt == lc3_DT_5M {
		nbits -= 160
	}
	gi := max(nbits/80, 3+sr) - (3 + sr)
	active := data != nil && data.active && gi < 4
	w := max(4, lc3_ns_4m[sr]>>4)
	var c [48]float32
	for i := 0; i < w; i++ {
		var g float32
		if active {
			g = float32(float32(.4) - float32(.05*float32(gi)))
		}
		c[i] = float32(g * lc3_ltpf_cden[sr][pitch&3][w-1-i])
		c[w+i] = float32(float32(.85*g) * lc3_ltpf_cnum[sr][min(gi, 3)][w-1-i])
	}
	ns := lc3_ns(dt, sr)
	nt := ns / (1 + dt)
	var x0 [2][24]float32
	copy(x0[0][:w-1], s.x[:w-1])
	copy(s.x[:w-1], xh[xoff+ns-(w-1):xoff+ns])
	if active {
		copy(x0[1][:w-1], xh[xoff+nt-(w-1):xoff+nt])
	}
	if !s.active && active {
		ltpf_synthesize_template(xh, nh, pitch/4, x0[0][:], xoff, nt, c[:], w, 1)
	} else if s.active && !active {
		ltpf_synthesize_template(xh, nh, s.pitch/4, x0[0][:], xoff, nt, s.c[:], w, -1)
	} else if s.active && active && s.pitch == pitch {
		ltpf_synthesize_template(xh, nh, pitch/4, x0[0][:], xoff, nt, c[:], w, 0)
	} else if s.active && active {
		ltpf_synthesize_template(xh, nh, s.pitch/4, x0[0][:], xoff, nt, s.c[:], w, -1)
		prev := xoff
		if prev <= 0 {
			prev += nh
		}
		prev -= w - 1
		ltpf_synthesize_template(xh, nh, pitch/4, xh[prev:prev+w-1], xoff, nt, c[:], w, 1)
	}
	if active && ns > nt {
		ltpf_synthesize_template(xh, nh, pitch/4, x0[1][:], xoff+nt, ns-nt, c[:], w, 0)
	}
	s.active = active
	s.pitch = pitch
	copy(s.c[:2*w], c[:2*w])
}
func lc3_ltpf_disable(data *lc3_ltpf_data_t) { data.active = false }
func lc3_ltpf_get_nbits(pitch bool) int {
	if pitch {
		return 11
	}
	return 1
}
func lc3_ltpf_put_data(bits *lc3_bits_t, data *lc3_ltpf_data_t) {
	v := 0
	if data.active {
		v = 1
	}
	lc3_put_bit(bits, v)
	lc3_put_bits(bits, data.pitch_index, 9)
}
func lc3_ltpf_get_data(bits *lc3_bits_t, data *lc3_ltpf_data_t) {
	data.active = lc3_get_bit(bits) != 0
	data.pitch_index = lc3_get_bits(bits, 9)
}

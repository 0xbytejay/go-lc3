// Copyright 2022 Google LLC
// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0
//
// Temporal noise shaping, adapted from liblc3's scalar implementation.

package lc3

type lc3_tns_data_t struct {
	nfilters      int
	lpc_weighting bool
	rc_order      [2]int
	rc            [2][8]int
}

func tns_resolve_lpc_weighting(dt, nbytes int) bool { return nbytes*8 < 120*(1+dt) }

func tns_dot(a, b []float32, n int) float32 {
	var v float32
	for i := 0; i < n; i++ {
		v = float32(v + float32(a[i]*b[i]))
	}
	return v
}

var tns_subdivisions = [4][7][]int{
	{{3, 10, 20}, {3, 20, 40}, {3, 30, 60}, {3, 40, 80}, {3, 51, 100}, {3, 51, 100}, {3, 51, 100}},
	{{6, 23, 40}, {6, 43, 80}, {6, 63, 120}, {6, 43, 80, 120, 160}, {6, 53, 100, 150, 200}, {6, 53, 100, 150, 200}, {6, 53, 100, 150, 200}},
	{{9, 26, 43, 60}, {9, 46, 83, 120}, {9, 66, 123, 180}, {9, 46, 82, 120, 159, 200, 240}, {9, 56, 103, 150, 200, 250, 300}, nil, nil},
	{{12, 34, 57, 80}, {12, 61, 110, 160}, {12, 88, 164, 240}, {12, 61, 110, 160, 213, 266, 320}, {12, 74, 137, 200, 266, 333, 400}, {12, 74, 137, 200, 266, 333, 400}, {12, 74, 137, 200, 266, 333, 400}},
}
var tns_lag_window = [...]float32{
	1.00000000e+00, 9.98028026e-01, 9.92135406e-01, 9.82391584e-01,
	9.68910791e-01, 9.51849807e-01, 9.31404933e-01, 9.07808230e-01, 8.81323137e-01,
}

func tns_compute_lpc_coeffs(dt, bw, maxorder int, x []float32, gain []float32, a *[2][9]float32) {
	sub := tns_subdivisions[dt][bw]
	nfilters, nsub := 1, 2
	if dt >= lc3_DT_5M && bw >= lc3_BANDWIDTH_SWB {
		nfilters++
	}
	if dt >= lc3_DT_7M5 {
		nsub++
	}
	var r [2][9]float32
	si, xe := 0, sub[0]
	for f := 0; f < nfilters; f++ {
		var c [9][3]float32
		for s := 0; s < nsub; s++ {
			xs := xe
			si++
			xe = sub[si]
			for k := 0; k <= maxorder; k++ {
				c[k][s] = tns_dot(x[xs:], x[xs+k:], xe-xs-k)
			}
		}
		r[f][0] = float32(nsub)
		e0, e1 := c[0][0], c[0][1]
		if nsub == 2 {
			for k := 1; k <= maxorder; k++ {
				if e0 != 0 && e1 != 0 {
					r[f][k] = float32(float32(float32(c[k][0]/e0)+float32(c[k][1]/e1)) * tns_lag_window[k])
				}
			}
		} else {
			e2 := c[0][2]
			for k := 1; k <= maxorder; k++ {
				if e0 != 0 && e1 != 0 && e2 != 0 {
					r[f][k] = float32(float32(float32(float32(c[k][0]/e0)+float32(c[k][1]/e1))+float32(c[k][2]/e2)) * tns_lag_window[k])
				}
			}
		}
	}
	for f := 0; f < nfilters; f++ {
		a0 := a[f][:]
		var a1 [9]float32
		err := r[f][0]
		gain[f] = err
		a0[0] = 1
		for k := 1; k <= maxorder; {
			rc := -r[f][k]
			for i := 1; i < k; i++ {
				rc = float32(rc - float32(a0[i]*r[f][k-i]))
			}
			rc = float32(rc / err)
			err = float32(err * float32(1-float32(rc*rc)))
			for i := 1; i < k; i++ {
				a1[i] = float32(a0[i] + float32(rc*a0[k-i]))
			}
			a1[k] = rc
			k++
			rc = -r[f][k]
			for i := 1; i < k; i++ {
				rc = float32(rc - float32(a1[i]*r[f][k-i]))
			}
			rc = float32(rc / err)
			err = float32(err * float32(1-float32(rc*rc)))
			for i := 1; i < k; i++ {
				a0[i] = float32(a1[i] + float32(rc*a1[k-i]))
			}
			a0[k] = rc
			k++
		}
		gain[f] = float32(gain[f] / err)
	}
}

func tns_lpc_weighting(predGain float32, a []float32) {
	gamma := float32(1 - float32(float32(float32(float32(1)-float32(0.85))*float32(2-predGain))/float32(2-1.5)))
	g := float32(1)
	for i := 1; i < 9; i++ {
		g = float32(g * gamma)
		a[i] = float32(a[i] * g)
	}
}

func tns_lpc_reflection(a []float32, maxorder int, rc []float32) {
	var b [2][7]float32
	rc[maxorder-1] = a[maxorder]
	e := float32(1 - float32(rc[maxorder-1]*rc[maxorder-1]))
	b1 := b[1][:]
	for i := 0; i < maxorder-1; i++ {
		b1[i] = float32(float32(a[1+i]-float32(rc[maxorder-1]*a[maxorder-1-i])) / e)
	}
	for k := maxorder - 2; k > 0; k-- {
		b0 := b1
		b1 = b[k&1][:]
		rc[k] = b0[k]
		e = float32(1 - float32(rc[k]*rc[k]))
		for i := 0; i < k; i++ {
			b1[i] = float32(float32(b0[i]-float32(rc[k]*b0[k-1-i])) / e)
		}
	}
	rc[0] = b1[0]
}

var tns_q_thr = [...]float32{0x0bcfp-15, 0x2307p-15, 0x390ep-15, 0x4d23p-15, 0x5e98p-15, 0x6cd4p-15, 0x775bp-15, 0x7dd2p-15}
var tns_q_inv = [...]float32{0x0000p-15, 0x1785p-15, 0x2e3dp-15, 0x4362p-15, 0x563cp-15, 0x6625p-15, 0x7295p-15, 0x7b1dp-15, 0x7f74p-15}

func tns_quantize_rc(rc []float32, maxorder int, order *int, rcq []int) {
	*order = maxorder
	for i := 0; i < maxorder; i++ {
		m := rc[i]
		if m < 0 {
			m = -m
		}
		rcq[i] = 0
		if m >= tns_q_thr[4] {
			rcq[i] = 4
		}
		for j := 0; j < 4 && m >= tns_q_thr[rcq[i]]; j++ {
			rcq[i]++
		}
		if rc[i] < 0 {
			rcq[i] = -rcq[i]
		}
		if rcq[i] != 0 {
			*order = maxorder
		} else {
			*order--
		}
	}
}
func tns_unquantize_rc(rcq []int, order int, rc []float32) {
	for i := 0; i < order; i++ {
		q := rcq[i]
		if q < 0 {
			rc[i] = -tns_q_inv[-q]
		} else {
			rc[i] = tns_q_inv[q]
		}
	}
}

func tns_forward_filtering(dt, bw int, orders [2]int, rc *[2][8]float32, x []float32) {
	nfilters := 1
	if dt >= lc3_DT_5M && bw >= lc3_BANDWIDTH_SWB {
		nfilters++
	}
	nf := lc3_ne(dt, min(bw, lc3_BANDWIDTH_FB)) >> (nfilters - 1)
	ie := 3 * (1 + dt)
	var s [8]float32
	for f := 0; f < nfilters; f++ {
		i0 := ie
		ie = nf * (1 + f)
		if orders[f] == 0 {
			continue
		}
		for i := i0; i < ie; i++ {
			xi := x[i]
			s1 := xi
			for k := 0; k < orders[f]; k++ {
				s0 := s[k]
				s[k] = s1
				s1 = float32(float32(rc[f][k]*xi) + s0)
				xi = float32(xi + float32(rc[f][k]*s0))
			}
			x[i] = xi
		}
	}
}
func tns_inverse_filtering(dt, bw int, orders [2]int, rc *[2][8]float32, x []float32) {
	nfilters := 1
	if dt >= lc3_DT_5M && bw >= lc3_BANDWIDTH_SWB {
		nfilters++
	}
	nf := lc3_ne(dt, min(bw, lc3_BANDWIDTH_FB)) >> (nfilters - 1)
	ie := 3 * (1 + dt)
	var s [8]float32
	for f := 0; f < nfilters; f++ {
		i0 := ie
		ie = nf * (1 + f)
		if orders[f] == 0 {
			continue
		}
		for i := i0; i < ie; i++ {
			xi := float32(x[i] - float32(s[7]*rc[f][7]))
			for k := 6; k >= 0; k-- {
				xi = float32(xi - float32(s[k]*rc[f][k]))
				s[k+1] = float32(s[k] + float32(rc[f][k]*xi))
			}
			s[0] = xi
			x[i] = xi
		}
		for k := 7; k >= orders[f]; k-- {
			s[k] = 0
		}
	}
}

func lc3_tns_analyze(dt, bw int, nnFlag bool, nbytes int, data *lc3_tns_data_t, x []float32) {
	var predGain [2]float32
	var a [2][9]float32
	var rc [2][8]float32
	data.lpc_weighting = tns_resolve_lpc_weighting(dt, nbytes)
	data.nfilters = 1
	if dt >= lc3_DT_5M && bw >= lc3_BANDWIDTH_SWB {
		data.nfilters++
	}
	maxorder := 8
	if dt <= lc3_DT_5M {
		maxorder = 4
	}
	tns_compute_lpc_coeffs(dt, bw, maxorder, x, predGain[:], &a)
	for f := 0; f < data.nfilters; f++ {
		data.rc_order[f] = 0
		if nnFlag || predGain[f] <= 1.5 {
			continue
		}
		if data.lpc_weighting && predGain[f] < 2 {
			tns_lpc_weighting(predGain[f], a[f][:])
		}
		tns_lpc_reflection(a[f][:], maxorder, rc[f][:])
		tns_quantize_rc(rc[f][:], maxorder, &data.rc_order[f], data.rc[f][:])
		tns_unquantize_rc(data.rc[f][:], data.rc_order[f], rc[f][:])
	}
	tns_forward_filtering(dt, bw, data.rc_order, &rc, x)
}

func lc3_tns_synthesize(dt, bw int, data *lc3_tns_data_t, x []float32) {
	var rc [2][8]float32
	for f := 0; f < data.nfilters; f++ {
		if data.rc_order[f] > 0 {
			tns_unquantize_rc(data.rc[f][:], data.rc_order[f], rc[f][:])
		}
	}
	tns_inverse_filtering(dt, bw, data.rc_order, &rc, x)
}

func lc3_tns_get_nbits(data *lc3_tns_data_t) int {
	nbits := 0
	w := 0
	if data.lpc_weighting {
		w = 1
	}
	for f := 0; f < data.nfilters; f++ {
		n := 2048
		order := data.rc_order[f]
		if order > 0 {
			n += int(lc3_tns_order_bits[w][order-1])
		}
		for i := 0; i < order; i++ {
			n += int(lc3_tns_coeffs_bits[i][8+data.rc[f][i]])
		}
		nbits += (n + (1 << 11) - 1) >> 11
	}
	return nbits
}

func lc3_tns_put_data(bits *lc3_bits_t, data *lc3_tns_data_t) {
	w := 0
	if data.lpc_weighting {
		w = 1
	}
	for f := 0; f < data.nfilters; f++ {
		order := data.rc_order[f]
		present := 0
		if order > 0 {
			present = 1
		}
		lc3_put_bits(bits, present, 1)
		if order <= 0 {
			continue
		}
		lc3_put_symbol(bits, &lc3_tns_order_models[w], order-1)
		for i := 0; i < order; i++ {
			lc3_put_symbol(bits, &lc3_tns_coeffs_models[i], 8+data.rc[f][i])
		}
	}
}

func lc3_tns_get_data(bits *lc3_bits_t, dt, bw, nbytes int, data *lc3_tns_data_t) int {
	data.nfilters = 1
	if dt >= lc3_DT_5M && bw >= lc3_BANDWIDTH_SWB {
		data.nfilters++
	}
	data.lpc_weighting = tns_resolve_lpc_weighting(dt, nbytes)
	w := 0
	if data.lpc_weighting {
		w = 1
	}
	for f := 0; f < data.nfilters; f++ {
		data.rc_order[f] = lc3_get_bit(bits)
		if data.rc_order[f] == 0 {
			continue
		}
		data.rc_order[f] += lc3_get_symbol(bits, &lc3_tns_order_models[w])
		if dt <= lc3_DT_5M && data.rc_order[f] > 4 {
			return -1
		}
		for i := 0; i < data.rc_order[f]; i++ {
			data.rc[f][i] = lc3_get_symbol(bits, &lc3_tns_coeffs_models[i]) - 8
		}
	}
	return 0
}

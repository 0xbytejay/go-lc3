// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: translated scalar spectral quantization and coding to Go.

package lc3

import "math"

type lc3_spec_analysis_t struct {
	nbits_off   float32
	nbits_spare int
}
type lc3_spec_side_t struct {
	g_idx, nq int
	lsb_mode  bool
}

func spec_bool(v bool) int {
	if v {
		return 1
	}
	return 0
}
func spec_abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
func spec_resolve_gain_offset(sr, nbytes int) int {
	ind := sr
	if lc3_hr(sr) {
		ind = 4 + (sr - lc3_SRATE_48K_HR)
	}
	off := nbytes * 8 / (10 * (1 + ind))
	lim := 255
	if sr >= lc3_SRATE_96K_HR {
		lim = 181
	}
	return min(lim, 105+5*(1+ind)+min(off, 115))
}

var spec_iq_table = [...]float32{
	1.00000000e+00, 1.08571112e+00, 1.17876863e+00, 1.27980221e+00,
	1.38949549e+00, 1.50859071e+00, 1.63789371e+00, 1.77827941e+00,
	1.93069773e+00, 2.09617999e+00, 2.27584593e+00, 2.47091123e+00,
	2.68269580e+00, 2.91263265e+00, 3.16227766e+00, 3.43332002e+00,
	3.72759372e+00, 4.04708995e+00, 4.39397056e+00, 4.77058270e+00,
	5.17947468e+00, 5.62341325e+00, 6.10540230e+00, 6.62870316e+00,
	7.19685673e+00, 7.81370738e+00, 8.48342898e+00, 9.21055318e+00,
}

func spec_unquantize_gain(gint int) float32 {
	g := float32(1)
	for ; gint < 0; gint += 28 {
		g = float32(g * 0.1)
	}
	for ; gint >= 28; gint -= 28 {
		g = float32(g * 10)
	}
	return float32(g * spec_iq_table[gint])
}
func spec_estimate_gain(dt, sr int, x []float32, nbytes, budget int, off float32, goff int, reset *bool, gmin *int) int {
	n4 := lc3_ne(dt, sr) / 4
	var e [lc3_MAX_NE / 4]float32
	var eq [lc3_MAX_NE / 4]int
	regBits := 0
	lowBits := float32(0)
	if lc3_hr(sr) {
		regC := [4][2]int{{-6, -6}, {0, 0}, {0, 0}, {2, 5}}[dt][sr-lc3_SRATE_48K_HR]
		regBits = min(max(8*nbytes*4/(125*(1+dt))+regC, 6), 23)
		m0, m1, k := float32(1e-5), float32(1e-5), float32(0)
		for i := 0; i < n4; i++ {
			for j := 0; j < 4; j++ {
				v := spec_abs(x[4*i+j])
				m0 = float32(m0 + v)
				m1 = float32(m1 + float32(v*k))
				k = float32(k + 1)
			}
		}
		m := int(math.Round(float64(float32(float32(1.6*m0) / float32(float32(1+dt)*m1)))))
		lowBits = float32(8 - min(m, 8))
	}
	xmax2 := float32(0)
	for i := 0; i < n4; i++ {
		x0 := float32(x[4*i] * x[4*i])
		x1 := float32(x[4*i+1] * x[4*i+1])
		x2 := float32(x[4*i+2] * x[4*i+2])
		x3 := float32(x[4*i+3] * x[4*i+3])
		xmax2 = max(xmax2, x0)
		xmax2 = max(xmax2, x1)
		xmax2 = max(xmax2, x2)
		xmax2 = max(xmax2, x3)
		e[i] = float32(float32(float32(x0+x1)+x2) + x3)
	}
	xmax := float32(math.Sqrt(float64(xmax2)))
	nf := float32(0)
	if lc3_hr(sr) {
		nf = float32(lc3_ldexpf(xmax, -regBits) * lc3_exp2f(-lowBits))
	}
	for i := 0; i < n4; i++ {
		eq[i] = int(lc3_db_q16(max(float32(e[i]+nf), float32(1e-10))))
	}
	nbits := int(float32(float32(float32(budget)+off) + 0.5))
	gint := 255 - goff
	const k20_28 = 46811
	const k2u7 = 176947
	const k1u4 = 91750
	j0 := n4 - 1
	for i := 128; i > 0; i >>= 1 {
		gn := (gint - i) * k20_28
		v := int32(0)
		j := j0
		for ; j >= 0 && eq[j] < gn; j-- {
		}
		j1 := j
		for ; j >= 0; j-- {
			diff := eq[j] - gn
			if diff < 0 {
				v += k2u7
			} else if diff < 43<<16 {
				v += int32(diff + (7 << 16))
			} else {
				v += int32(2*diff - (36 << 16))
			}
		}
		if v > int32(nbits*k1u4) {
			j0 = j1
		} else {
			gint -= i
		}
	}
	xlim := float32(32767)
	if lc3_hr(sr) {
		xlim = 32767 * 256
	}
	*gmin = 255 - goff
	for i := 128; i > 0; i >>= 1 {
		if float32(xlim*spec_unquantize_gain(*gmin-i)) > xmax {
			*gmin -= i
		}
	}
	*reset = gint < *gmin || xmax == 0
	if *reset {
		gint = *gmin
	}
	return gint
}
func spec_adjust_gain(dt, sr, gidx, nbits, budget, gmin int) int {
	t := [7][3]int{{80, 500, 850}, {230, 1025, 1700}, {380, 1550, 2550}, {530, 2075, 3400}, {680, 2600, 4250}, {680, 2600, 4250}, {830, 3125, 5100}}[sr]
	delta, den := 0, 48
	if nbits < t[0] {
		delta = 3 * (nbits + 48)
	} else if nbits < t[1] {
		n0, r := 3*(t[0]+48), t[1]-t[0]
		delta = n0*r + (nbits-t[0])*(t[1]-n0)
		den *= r
	} else {
		delta = min(nbits, t[2])
	}
	delta = (delta + den/2) / den
	if lc3_hr(sr) && nbits > budget {
		factor := 1 + spec_bool(dt <= lc3_DT_5M) + spec_bool(dt <= lc3_DT_2M5)*(1+spec_bool(nbits >= 520))
		incr := factor + factor*(nbits-budget)/delta
		return min(gidx+incr, 255) - gidx
	}
	if !lc3_hr(sr) && nbits < budget-(delta+2) {
		return -spec_bool(gidx > gmin)
	}
	if !lc3_hr(sr) && nbits > budget {
		return spec_bool(gidx < 255) + spec_bool(gidx < 254 && nbits >= budget+delta)
	}
	return 0
}
func spec_quantize(dt, sr, gint int, x []float32, n *int) {
	inv := spec_unquantize_gain(-gint)
	ne := lc3_ne(dt, sr)
	*n = ne
	lim := float32(10.0 / 16)
	if lc3_hr(sr) {
		lim = 0.5
	}
	for i := 0; i < ne; i += 2 {
		x[i] = float32(x[i] * inv)
		x[i+1] = float32(x[i+1] * inv)
		if spec_abs(x[i]) >= lim || spec_abs(x[i+1]) >= lim {
			*n = ne
		} else {
			*n -= 2
		}
	}
}
func spec_unquantize(dt, sr, gint int, x []float32, nq int) float32 {
	g := spec_unquantize_gain(gint)
	for i := 0; i < nq; i++ {
		x[i] = float32(x[i] * g)
	}
	clear(x[nq:lc3_ne(dt, sr)])
	return g
}
func spec_resolve_modes(sr, nbytes int, lsb *bool) bool {
	ind := sr
	if lc3_hr(sr) {
		ind = 4 + (sr - lc3_SRATE_48K_HR)
	}
	if lsb != nil {
		*lsb = nbytes >= 20*(3+ind) && sr < lc3_SRATE_96K_HR
	}
	return nbytes > 20*(1+ind) && sr < lc3_SRATE_96K_HR
}
func spec_compute_nbits(dt, sr, nbytes int, x []float32, n *int, budget int, outLSB *bool) int {
	lsb := false
	high := spec_resolve_modes(sr, nbytes, &lsb)
	ne := lc3_ne(dt, sr)
	nbits, nbitsLSB, nbitsEnd, nEnd := 0, 0, 0, 0
	state := uint8(0)
	if budget != 0 {
		budget *= 2048
	} else {
		budget = 2147483647
	}
	i := 0
	for h := 0; h < 2; h++ {
		coeff := &lc3_spectrum_lookup[spec_bool(high)][h]
		for ; i < min(*n, (ne+2)>>(1-h)) && nbits <= budget; i += 2 {
			off := float32(6.0 / 16)
			if lc3_hr(sr) {
				off = 0.5
			}
			a, b := uint32(float32(spec_abs(x[i])+off)), uint32(float32(spec_abs(x[i+1])+off))
			lut := coeff[state]
			s := spec_bool(a != 0) + spec_bool(b != 0)
			nbits += s * 2048
			m, k := (a|b)>>2, 0
			if m != 0 {
				if lsb {
					nbits += int(lc3_spectrum_bits[lut[k]][16]) - 2*2048
					k++
					nbitsLSB += 2 + spec_bool(a == 1) + spec_bool(b == 1)
				}
				for m >>= spec_bool(lsb); m != 0; m, k = m>>1, k+1 {
					nbits += int(lc3_spectrum_bits[lut[min(k, 3)]][16])
				}
				nbits += k * 2 * 2048
				a >>= k
				b >>= k
				k = min(k, 3)
			}
			nbits += int(lc3_spectrum_bits[lut[k]][a+4*b])
			if s != 0 && nbits <= budget {
				nEnd = i + 2
				nbitsEnd = nbits
			}
			v := 12 + k
			if k <= 1 {
				v = 1 + (int(a)+int(b))*(k+1)
			}
			state = (state << 4) + uint8(v)
		}
	}
	*n = nEnd
	if outLSB != nil {
		*outLSB = lsb && nbitsEnd+nbitsLSB*2048 > budget
	}
	if budget >= 2147483647 {
		nbitsEnd += nbitsLSB * 2048
	}
	return (nbitsEnd + 2047) / 2048
}
func spec_put_quantized(bits *lc3_bits_t, dt, sr, nbytes int, x []float32, nq int, lsb bool) {
	high := spec_resolve_modes(sr, nbytes, nil)
	ne := lc3_ne(dt, sr)
	state := uint8(0)
	i := 0
	for h := 0; h < 2; h++ {
		coeff := &lc3_spectrum_lookup[spec_bool(high)][h]
		for ; i < min(nq, (ne+2)>>(1-h)); i += 2 {
			off := float32(6.0 / 16)
			if lc3_hr(sr) {
				off = 0.5
			}
			a, b := uint32(float32(spec_abs(x[i])+off)), uint32(float32(spec_abs(x[i+1])+off))
			lut := coeff[state]
			m, k, shr := (a|b)>>2, 0, 0
			if m != 0 {
				if lsb {
					lc3_put_symbol(bits, &lc3_spectrum_models[lut[k]], 16)
					k++
				}
				for m >>= spec_bool(lsb); m != 0; m, k = m>>1, k+1 {
					lc3_put_bit(bits, int((a>>k)&1))
					lc3_put_bit(bits, int((b>>k)&1))
					lc3_put_symbol(bits, &lc3_spectrum_models[lut[min(k, 3)]], 16)
				}
				a >>= spec_bool(lsb)
				b >>= spec_bool(lsb)
				shr = k - spec_bool(lsb)
				k = min(k, 3)
			}
			if a != 0 {
				lc3_put_bit(bits, spec_bool(x[i] < 0))
			}
			if b != 0 {
				lc3_put_bit(bits, spec_bool(x[i+1] < 0))
			}
			a >>= shr
			b >>= shr
			lc3_put_symbol(bits, &lc3_spectrum_models[lut[k]], int(a+4*b))
			v := 12 + k
			if k <= 1 {
				v = 1 + (int(a)+int(b))*(k+1)
			}
			state = (state << 4) + uint8(v)
		}
	}
}
func spec_get_quantized(bits *lc3_bits_t, dt, sr, nbytes, nq int, lsb bool, x []float32, seed *uint16) int {
	high := spec_resolve_modes(sr, nbytes, nil)
	ne := lc3_ne(dt, sr)
	*seed = 0
	state := uint8(0)
	i := 0
	for h := 0; h < 2; h++ {
		coeff := &lc3_spectrum_lookup[spec_bool(high)][h]
		for ; i < min(nq, (ne+2)>>(1-h)); i += 2 {
			lut := coeff[state]
			maxshl := 14
			if lc3_hr(sr) {
				maxshl = 22
			}
			u, v, k, shl := 0, 0, 0, 0
			s := lc3_get_symbol(bits, &lc3_spectrum_models[lut[k]])
			if lsb && s >= 16 {
				k++
				s = lc3_get_symbol(bits, &lc3_spectrum_models[lut[k]])
				shl++
			}
			for ; s >= 16 && shl < maxshl; shl++ {
				u |= lc3_get_bit(bits) << shl
				v |= lc3_get_bit(bits) << shl
				if k < 3 {
					k++
				}
				s = lc3_get_symbol(bits, &lc3_spectrum_models[lut[k]])
			}
			if s >= 16 {
				return -1
			}
			a, b := s%4, s/4
			u |= a << shl
			v |= b << shl
			xu, xv := u, v
			if u != 0 && lc3_get_bit(bits) != 0 {
				xu = -u
			}
			if v != 0 && lc3_get_bit(bits) != 0 {
				xv = -v
			}
			x[i], x[i+1] = float32(xu), float32(xv)
			*seed = uint16((int(*seed) + (u&0x7fff)*i + (v&0x7fff)*(i+1)) & 0xffff)
			next := 12 + k
			if k <= 1 {
				next = 1 + (a+b)*(k+1)
			}
			state = (state << 4) + uint8(next)
		}
	}
	return 0
}
func spec_put_residual(bits *lc3_bits_t, nbits int, hr bool, x []float32, n int) {
	lim := float32(10.0 / 16)
	iters := 1
	if hr {
		lim = 0.5
		iters = 20
	}
	off := float32(lim / 2)
	for iter := 0; iter < iters && nbits > 0; iter++ {
		for i := 0; i < n && nbits > 0; i++ {
			r := spec_abs(x[i])
			if r < lim {
				continue
			}
			b := (float32(r-float32(math.Trunc(float64(r)))) < lim) != (x[i] < 0)
			lc3_put_bit(bits, spec_bool(b))
			nbits--
			if b {
				x[i] = float32(x[i] - off)
			} else {
				x[i] = float32(x[i] + off)
			}
		}
		off = float32(off * lim)
	}
}
func spec_get_residual(bits *lc3_bits_t, nbits int, hr bool, x []float32, n int) {
	off1, off2 := float32(5.0/16), float32(3.0/16)
	iters := 1
	if hr {
		off1, off2, iters = 0.25, 0.25, 20
	}
	for iter := 0; iter < iters && nbits > 0; iter++ {
		for i := 0; i < n && nbits > 0; i++ {
			if x[i] == 0 {
				continue
			}
			if lc3_get_bit(bits) == 0 {
				v := off2
				if x[i] < 0 {
					v = off1
				}
				x[i] = float32(x[i] - v)
			} else {
				v := off2
				if x[i] > 0 {
					v = off1
				}
				x[i] = float32(x[i] + v)
			}
			nbits--
		}
		off1 = float32(off1 * 0.5)
		off2 = float32(off2 * 0.5)
	}
}
func spec_put_lsb(bits *lc3_bits_t, nbits int, hr bool, x []float32, n int) {
	for i := 0; i < n && nbits > 0; i += 2 {
		off := float32(6.0 / 16)
		if hr {
			off = 0.5
		}
		a, b := uint32(float32(spec_abs(x[i])+off)), uint32(float32(spec_abs(x[i+1])+off))
		if (a|b)>>2 == 0 {
			continue
		}
		if nbits > 0 {
			lc3_put_bit(bits, int(a&1))
		}
		nbits--
		if a == 1 {
			if nbits > 0 {
				lc3_put_bit(bits, spec_bool(x[i] < 0))
			}
			nbits--
		}
		if nbits > 0 {
			lc3_put_bit(bits, int(b&1))
		}
		nbits--
		if b == 1 {
			if nbits > 0 {
				lc3_put_bit(bits, spec_bool(x[i+1] < 0))
			}
			nbits--
		}
	}
}
func spec_get_lsb(bits *lc3_bits_t, nbits int, x []float32, nq int, seed *uint16) {
	for i := 0; i < nq && nbits > 0; i += 2 {
		a, b := spec_abs(x[i]), spec_abs(x[i+1])
		if max(a, b) < 4 {
			continue
		}
		if nbits > 0 && lc3_get_bit(bits) != 0 {
			if a != 0 {
				if x[i] < 0 {
					x[i]--
				} else {
					x[i]++
				}
				*seed = uint16((int(*seed) + i) & 0xffff)
			} else {
				if nbits-1 > 0 {
					if lc3_get_bit(bits) != 0 {
						x[i] = -1
					} else {
						x[i] = 1
					}
					*seed = uint16((int(*seed) + i) & 0xffff)
				}
				nbits--
			}
		}
		nbits--
		if nbits > 0 && lc3_get_bit(bits) != 0 {
			if b != 0 {
				if x[i+1] < 0 {
					x[i+1]--
				} else {
					x[i+1]++
				}
				*seed = uint16((int(*seed) + i + 1) & 0xffff)
			} else {
				if nbits-1 > 0 {
					if lc3_get_bit(bits) != 0 {
						x[i+1] = -1
					} else {
						x[i+1] = 1
					}
					*seed = uint16((int(*seed) + i + 1) & 0xffff)
				}
				nbits--
			}
		}
		nbits--
	}
}
func spec_estimate_noise(dt, bw int, hr bool, x []float32, n int) int {
	stop := lc3_ne(dt, min(bw, lc3_BANDWIDTH_FB))
	w := 1 + spec_bool(dt >= lc3_DT_7M5) + spec_bool(dt >= lc3_DT_10M)
	lim := float32(10.0 / 16)
	if hr {
		lim = 0.5
	}
	sum := float32(0)
	ns, z := 0, 0
	i := 6*(1+dt) - w
	for ; i < min(n, stop); i++ {
		if spec_abs(x[i]) < lim {
			z++
		} else {
			z = 0
		}
		if z > 2*w {
			sum = float32(sum + spec_abs(x[i-w]))
			ns++
		}
	}
	for ; i < stop+w; i++ {
		z++
		if z > 2*w {
			sum = float32(sum + spec_abs(x[i-w]))
			ns++
		}
	}
	nf := 8
	if ns != 0 {
		nf = 8 - int(float32(float32(float32(16*sum)/float32(ns))+0.5))
	}
	return min(max(nf, 0), 7)
}
func spec_fill_noise(dt, bw, nf int, seed uint16, g float32, x []float32, nq int) {
	stop := lc3_ne(dt, min(bw, lc3_BANDWIDTH_FB))
	w := 1 + spec_bool(dt >= lc3_DT_7M5) + spec_bool(dt >= lc3_DT_10M)
	s := float32(float32(g*float32(8-nf)) / 16)
	z := 0
	i := 6*(1+dt) - w
	fill := func(j int) {
		seed = uint16((13849 + uint32(seed)*31821) & 0xffff)
		if seed&0x8000 != 0 {
			x[j] = -s
		} else {
			x[j] = s
		}
	}
	for ; i < min(nq, stop); i++ {
		if x[i] != 0 {
			z = 0
		} else {
			z++
		}
		if z > 2*w {
			fill(i - w)
		}
	}
	for ; i < stop+w; i++ {
		z++
		if z > 2*w {
			fill(i - w)
		}
	}
}
func spec_get_nbits_nq(dt, sr int) int {
	ne := lc3_ne(dt, sr)
	return 4 + spec_bool(ne > 32) + spec_bool(ne > 64) + spec_bool(ne > 128) + spec_bool(ne > 256) + spec_bool(ne > 512)
}
func spec_get_nbits_ac(dt, sr, nbytes int) int {
	return spec_get_nbits_nq(dt, sr) + 3 + spec_bool(lc3_hr(sr)) + min((nbytes-1)/160, 2)
}
func lc3_spec_analyze(dt, sr, nbytes int, pitch bool, tns *lc3_tns_data_t, spec *lc3_spec_analysis_t, x []float32, side *lc3_spec_side_t) {
	budget := 8*nbytes - spec_get_nbits_ac(dt, sr, nbytes) - lc3_bwdet_get_nbits(sr) - lc3_ltpf_get_nbits(pitch) - lc3_sns_get_nbits() - lc3_tns_get_nbits(tns) - 8 - 3
	off := float32(spec.nbits_off + float32(spec.nbits_spare))
	off = min(max(off, float32(-40)), float32(40))
	off = float32(float32(0.8*spec.nbits_off) + float32(0.2*off))
	goff := spec_resolve_gain_offset(sr, nbytes)
	gmin := 0
	reset := false
	gint := spec_estimate_gain(dt, sr, x, nbytes, budget, off, goff, &reset, &gmin)
	spec_quantize(dt, sr, gint, x, &side.nq)
	nbits := spec_compute_nbits(dt, sr, nbytes, x, &side.nq, 0, nil)
	if reset {
		spec.nbits_off = 0
		spec.nbits_spare = 0
	} else {
		spec.nbits_off = off
		spec.nbits_spare = budget - nbits
	}
	adj := spec_adjust_gain(dt, sr, goff+gint, nbits, budget, goff+gmin)
	if adj != 0 {
		spec_quantize(dt, sr, adj, x, &side.nq)
	}
	side.g_idx = gint + adj + goff
	spec_compute_nbits(dt, sr, nbytes, x, &side.nq, budget, &side.lsb_mode)
}
func lc3_spec_put_side(bits *lc3_bits_t, dt, sr int, side *lc3_spec_side_t) {
	lc3_put_bits(bits, max(side.nq>>1, 1)-1, spec_get_nbits_nq(dt, sr))
	lc3_put_bits(bits, spec_bool(side.lsb_mode), 1)
	lc3_put_bits(bits, side.g_idx, 8)
}
func lc3_spec_encode(bits *lc3_bits_t, dt, sr, bw, nbytes int, side *lc3_spec_side_t, x []float32) {
	lc3_put_bits(bits, spec_estimate_noise(dt, bw, lc3_hr(sr), x, side.nq), 3)
	spec_put_quantized(bits, dt, sr, nbytes, x, side.nq, side.lsb_mode)
	left := lc3_get_bits_left(bits)
	if side.lsb_mode {
		spec_put_lsb(bits, left, lc3_hr(sr), x, side.nq)
	} else {
		spec_put_residual(bits, left, lc3_hr(sr), x, side.nq)
	}
}
func lc3_spec_get_side(bits *lc3_bits_t, dt, sr int, side *lc3_spec_side_t) int {
	side.nq = (lc3_get_bits(bits, spec_get_nbits_nq(dt, sr)) + 1) << 1
	side.lsb_mode = lc3_get_bit(bits) != 0
	side.g_idx = lc3_get_bits(bits, 8)
	if side.nq > lc3_ne(dt, sr) {
		side.nq = lc3_ne(dt, sr)
		return -1
	}
	return 0
}
func lc3_spec_decode(bits *lc3_bits_t, dt, sr, bw, nbytes int, side *lc3_spec_side_t, x []float32) int {
	nf := lc3_get_bits(bits, 3)
	seed := uint16(0)
	if ret := spec_get_quantized(bits, dt, sr, nbytes, side.nq, side.lsb_mode, x, &seed); ret < 0 {
		return ret
	}
	left := lc3_get_bits_left(bits)
	if side.lsb_mode {
		spec_get_lsb(bits, left, x, side.nq, &seed)
	} else {
		spec_get_residual(bits, left, lc3_hr(sr), x, side.nq)
	}
	g := spec_unquantize(dt, sr, side.g_idx-spec_resolve_gain_offset(sr, nbytes), x, side.nq)
	if side.nq > 2 || x[0] != 0 || x[1] != 0 || side.g_idx > 0 || nf < 7 {
		spec_fill_noise(dt, bw, nf, seed, g, x, side.nq)
	}
	return 0
}

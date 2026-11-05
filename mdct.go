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
// Modified from liblc3: scalar transforms translated to Go. Explicit binary32
// rounding preserves the scalar non-reassociated, non-contracted arithmetic.

package lc3

import "math"

func fft_5(x, y []lc3_complex, n int) {
	const cos1, cos2 float32 = 0.3090169944, -0.8090169944
	const sin1, sin2 float32 = -0.9510565163, -0.5877852523
	for i := 0; i < n; i++ {
		a, b, c, d := x[i+n], x[i+2*n], x[i+3*n], x[i+4*n]
		s14 := lc3_complex{float32(a.re + d.re), float32(a.im + d.im)}
		d14 := lc3_complex{float32(a.re - d.re), float32(a.im - d.im)}
		s23 := lc3_complex{float32(b.re + c.re), float32(b.im + c.im)}
		d23 := lc3_complex{float32(b.re - c.re), float32(b.im - c.im)}
		z := x[i]
		o := y[5*i:]
		o[0] = lc3_complex{float32(float32(z.re+s14.re) + s23.re), float32(float32(z.im+s14.im) + s23.im)}
		o[1].re = float32(float32(float32(float32(z.re+float32(s14.re*cos1))-float32(d14.im*sin1))+float32(s23.re*cos2)) - float32(d23.im*sin2))
		o[1].im = float32(float32(float32(float32(z.im+float32(s14.im*cos1))+float32(d14.re*sin1))+float32(s23.im*cos2)) + float32(d23.re*sin2))
		o[2].re = float32(float32(float32(float32(z.re+float32(s14.re*cos2))-float32(d14.im*sin2))+float32(s23.re*cos1)) + float32(d23.im*sin1))
		o[2].im = float32(float32(float32(float32(z.im+float32(s14.im*cos2))+float32(d14.re*sin2))+float32(s23.im*cos1)) - float32(d23.re*sin1))
		o[3].re = float32(float32(float32(float32(z.re+float32(s14.re*cos2))+float32(d14.im*sin2))+float32(s23.re*cos1)) - float32(d23.im*sin1))
		o[3].im = float32(float32(float32(float32(z.im+float32(s14.im*cos2))-float32(d14.re*sin2))+float32(s23.im*cos1)) + float32(d23.re*sin1))
		o[4].re = float32(float32(float32(float32(z.re+float32(s14.re*cos1))+float32(d14.im*sin1))+float32(s23.re*cos2)) + float32(d23.im*sin2))
		o[4].im = float32(float32(float32(float32(z.im+float32(s14.im*cos1))-float32(d14.re*sin1))+float32(s23.im*cos2)) - float32(d23.re*sin2))
	}
}

func fft_bf3(t *lc3_fft_bf3_twiddles, x, y []lc3_complex, n int) {
	n3 := t.n3
	for i := 0; i < n; i++ {
		for j := 0; j < n3; j++ {
			z0, z1, z2 := x[i*n3+j], x[n*n3+i*n3+j], x[2*n*n3+i*n3+j]
			for q := 0; q < 3; q++ {
				w := t.t[q*n3+j]
				k := i*3*n3 + q*n3 + j
				y[k].re = float32(float32(float32(float32(z0.re+float32(z1.re*w[0].re))-float32(z1.im*w[0].im))+float32(z2.re*w[1].re)) - float32(z2.im*w[1].im))
				y[k].im = float32(float32(float32(float32(z0.im+float32(z1.im*w[0].re))+float32(z1.re*w[0].im))+float32(z2.im*w[1].re)) + float32(z2.re*w[1].im))
			}
		}
	}
}
func fft_bf2(t *lc3_fft_bf2_twiddles, x, y []lc3_complex, n int) {
	n2 := t.n2
	for i := 0; i < n; i++ {
		for j := 0; j < n2; j++ {
			z0, z1, w := x[i*n2+j], x[n*n2+i*n2+j], t.t[j]
			k := i*2*n2 + j
			y[k].re = float32(float32(z0.re+float32(z1.re*w.re)) - float32(z1.im*w.im))
			y[k].im = float32(float32(z0.im+float32(z1.im*w.re)) + float32(z1.re*w.im))
			y[k+n2].re = float32(float32(z0.re-float32(z1.re*w.re)) + float32(z1.im*w.im))
			y[k+n2].im = float32(float32(z0.im-float32(z1.im*w.re)) - float32(z1.re*w.im))
		}
	}
}
func fft(x []lc3_complex, n int, y0, y1 []lc3_complex) []lc3_complex {
	y := [2][]lc3_complex{y1, y0}
	is := 0
	n /= 5
	fft_5(x, y[is], n)
	i3 := 0
	for ; n&(n-1) != 0; i3++ {
		n /= 3
		fft_bf3(lc3_fft_twiddles_bf3[i3], y[is], y[is^1], n)
		is ^= 1
	}
	for i2 := 0; n > 1; i2++ {
		n >>= 1
		fft_bf2(lc3_fft_twiddles_bf2[i2][i3], y[is], y[is^1], n)
		is ^= 1
	}
	return y[is]
}

func mdct_window(dt, sr int, x, d, y []float32) {
	win := lc3_mdct_win[dt][sr]
	ns, nd := lc3_ns(dt, sr), lc3_nd(dt, sr)
	w0, w1, w2, w3 := 0, ns, ns, ns+nd
	x0, x1 := ns-nd, ns-nd
	y0, y1 := ns/2, ns/2
	d0, d1 := 0, nd
	for x1 > 0 {
		for q := 0; q < 2; q++ {
			y0--
			x1--
			w1--
			y[y0] = float32(float32(d[d0]*win[w0]) - float32(x[x1]*win[w1]))
			w0++
			d[d0] = x[x0]
			y[y1] = float32(d[d0] * win[w2])
			d0++
			x0++
			y1++
			w2++
		}
	}
	x1 += ns
	for x0 < x1 {
		for q := 0; q < 2; q++ {
			y0--
			d1--
			w1--
			y[y0] = float32(float32(d[d0]*win[w0]) - float32(d[d1]*win[w1]))
			w0++
			d[d0] = x[x0]
			x0++
			x1--
			w3--
			d[d1] = x[x1]
			y[y1] = float32(float32(d[d0]*win[w2]) + float32(d[d1]*win[w3]))
			d0++
			y1++
			w2++
		}
	}
}
func mdct_pre_fft(def *lc3_mdct_rot_def, x []float32, y []lc3_complex) {
	n4 := def.n4
	x0, x1, w0, w1, y0, y1 := 0, 2*n4, 0, n4, 0, n4
	for x0 < x1 {
		uw := def.w[w0]
		w0++
		x1--
		u := lc3_complex{float32(float32(-x[x1]*uw.re) + float32(x[x0]*uw.im)), float32(float32(x[x0]*uw.re) + float32(x[x1]*uw.im))}
		x0++
		w1--
		vw := def.w[w1]
		x1--
		v := lc3_complex{float32(float32(-x[x1]*vw.im) + float32(x[x0]*vw.re)), float32(float32(-x[x0]*vw.im) - float32(x[x1]*vw.re))}
		x0++
		y[y0] = u
		y0++
		y1--
		y[y1] = v
	}
}
func mdct_post_fft(def *lc3_mdct_rot_def, x []lc3_complex, y []float32) {
	n4 := def.n4
	n8 := n4 >> 1
	w0, w1, x0, x1, y0, y1 := n8, n8-1, n8, n8-1, n4, n4
	for y1 > 0 {
		a, b, u, v := x[x0], x[x1], def.w[w0], def.w[w1]
		u0 := float32(float32(a.im*u.im) + float32(a.re*u.re))
		u1 := float32(float32(b.re*v.im) - float32(b.im*v.re))
		v0 := float32(float32(a.re*u.im) - float32(a.im*u.re))
		v1 := float32(float32(b.im*v.im) + float32(b.re*v.re))
		y[y0] = u0
		y[y0+1] = u1
		y0 += 2
		y1--
		y[y1] = v0
		y1--
		y[y1] = v1
		x0++
		x1--
		w0++
		w1--
	}
}
func imdct_pre_fft(def *lc3_mdct_rot_def, x []float32, y []lc3_complex) {
	n4 := def.n4
	x0, x1, w0, w1, y0, y1 := 0, 2*n4, 0, n4, 0, n4
	for x0 < x1 {
		u0 := x[x0]
		x0++
		x1--
		u1 := x[x1]
		v0 := x[x0]
		x0++
		x1--
		v1 := x[x1]
		uw := def.w[w0]
		w0++
		w1--
		vw := def.w[w1]
		y[y0] = lc3_complex{float32(float32(-u0*uw.re) - float32(u1*uw.im)), float32(float32(-u1*uw.re) + float32(u0*uw.im))}
		y0++
		y1--
		y[y1] = lc3_complex{float32(float32(-v1*vw.re) - float32(v0*vw.im)), float32(float32(-v0*vw.re) + float32(v1*vw.im))}
	}
}
func imdct_post_fft(def *lc3_mdct_rot_def, x []lc3_complex, y []float32) {
	n4 := def.n4
	x0, x1, w0, w1, y0, y1 := 0, n4, 0, n4, 0, 2*n4
	for x0 < x1 {
		uz := x[x0]
		x0++
		x1--
		vz := x[x1]
		uw := def.w[w0]
		w0++
		w1--
		vw := def.w[w1]
		y[y0] = float32(float32(uz.re*uw.im) - float32(uz.im*uw.re))
		y0++
		y1--
		y[y1] = float32(float32(uz.re*uw.re) + float32(uz.im*uw.im))
		y1--
		y[y1] = float32(float32(vz.re*vw.im) - float32(vz.im*vw.re))
		y[y0] = float32(float32(vz.re*vw.re) + float32(vz.im*vw.im))
		y0++
	}
}
func imdct_window(dt, sr int, x, d, y []float32) {
	win := lc3_mdct_win[dt][sr]
	n4, nd := lc3_ns(dt, sr)>>1, lc3_nd(dt, sr)
	w2, w0, w1 := 0, 3*n4, 3*n4
	x0, x1, xi := nd-n4, nd-n4, 0
	y0, y1, y2, y3 := nd-n4, nd-n4, nd, 0
	for y0 > 0 {
		for q := 0; q < 2; q++ {
			y0--
			x0--
			y[y0] = float32(d[x0] - float32(x[xi]*win[w1]))
			w1++
			w0--
			y[y1] = float32(d[x1] + float32(x[xi]*win[w0]))
			x1++
			xi++
			y1++
		}
	}
	for y1 < nd {
		for q := 0; q < 2; q++ {
			w0--
			y[y1] = float32(d[x1] + float32(x[xi]*win[w0]))
			x1++
			xi++
			y1++
		}
	}
	for y1 < 2*n4 {
		for q := 0; q < 2; q++ {
			w0--
			y[y1] = float32(x[xi] * win[w0])
			y1++
			y2--
			d[y2] = float32(x[xi] * win[w2])
			xi++
			w2++
		}
	}
	for y2 > y3 {
		for q := 0; q < 2; q++ {
			w0--
			d[y3] = float32(x[xi] * win[w0])
			y3++
			y2--
			d[y2] = float32(x[xi] * win[w2])
			xi++
			w2++
		}
	}
}
func rescale(x []float32, n int, f float32) {
	for i := 0; i < (n>>2)*4; i++ {
		x[i] = float32(x[i] * f)
	}
}

func lc3_mdct_forward(dt, sr, sr_dst int, x, d, y []float32) {
	ns, nsDst := lc3_ns(dt, sr), lc3_ns(dt, sr_dst)
	var window [960]float32
	var a, b [480]lc3_complex
	rot := lc3_mdct_rot[dt][sr]
	mdct_window(dt, sr, x, d, window[:ns])
	mdct_pre_fft(rot, window[:ns], a[:ns/2])
	z := fft(a[:ns/2], ns/2, a[:ns/2], b[:ns/2])
	mdct_post_fft(rot, z, y)
	if ns != nsDst {
		rescale(y, nsDst, float32(math.Sqrt(float64(float32(float32(nsDst)/float32(ns))))))
	}
}
func lc3_mdct_inverse(dt, sr, sr_src int, x, d, y []float32) {
	ns, nsSrc := lc3_ns(dt, sr), lc3_ns(dt, sr_src)
	var a, b [480]lc3_complex
	var half [960]float32
	// Input spectra shorter than the output transform are zero-padded.
	var spectrum [960]float32
	copy(spectrum[:ns], x[:nsSrc])
	rot := lc3_mdct_rot[dt][sr]
	imdct_pre_fft(rot, spectrum[:ns], a[:ns/2])
	z := fft(a[:ns/2], ns/2, a[:ns/2], b[:ns/2])
	imdct_post_fft(rot, z, half[:ns])
	if ns != nsSrc {
		rescale(half[:ns], ns, float32(math.Sqrt(float64(float32(float32(ns)/float32(nsSrc))))))
	}
	imdct_window(dt, sr, half[:ns], d, y)
}

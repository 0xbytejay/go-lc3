// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

//go:build arm64 && go1.27 && goexperiment.simd && !purego

package lc3

import (
	"simd/archsimd"
	"unsafe"
)

const fftBackendName = "arm64-neon"

// SIMD lanes carry independent butterflies. Separate Mul/Add/Sub operations
// preserve the scalar reference's intermediate binary32 rounding; MulAdd would
// fuse operations and change that contract.
//
// lc3_complex contains exactly two adjacent float32 fields and no pointers.
// The checked view is confined to the length of the supplied slice; callers
// pass only complete four-complex groups. Layout assertions are compile-time
// checks so this representation can never silently become incompatible.
var (
	_ [8 - unsafe.Sizeof(lc3_complex{})]byte
	_ [unsafe.Sizeof(lc3_complex{}) - 8]byte
	_ [4 - unsafe.Offsetof(lc3_complex{}.im)]byte
	_ [unsafe.Offsetof(lc3_complex{}.im) - 4]byte
)

func neonComplexFloats(x []lc3_complex) []float32 {
	if len(x) == 0 {
		return nil
	}
	return unsafe.Slice((*float32)(unsafe.Pointer(&x[0])), len(x)*2)
}

func neonLoadComplex4(x []lc3_complex) (archsimd.Float32x4, archsimd.Float32x4) {
	p := neonComplexFloats(x[:4:4])
	a := archsimd.LoadFloat32x4(p[:4]).ToBits()
	b := archsimd.LoadFloat32x4(p[4:]).ToBits()
	return a.ConcatEven(b).BitsToFloat32(), a.ConcatOdd(b).BitsToFloat32()
}

func neonStoreComplex4(y []lc3_complex, stride int, re, im archsimd.Float32x4) {
	_ = y[3*stride]
	if stride == 1 {
		r, i := re.ToBits(), im.ToBits()
		p := neonComplexFloats(y[:4:4])
		r.InterleaveLo(i).BitsToFloat32().Store(p[:4])
		r.InterleaveHi(i).BitsToFloat32().Store(p[4:])
		return
	}
	y[0] = lc3_complex{re.GetElem(0), im.GetElem(0)}
	y[stride] = lc3_complex{re.GetElem(1), im.GetElem(1)}
	y[2*stride] = lc3_complex{re.GetElem(2), im.GetElem(2)}
	y[3*stride] = lc3_complex{re.GetElem(3), im.GetElem(3)}
}

func neonLoadTwiddles4(x [][2]lc3_complex, which int) (archsimd.Float32x4, archsimd.Float32x4) {
	x = x[:4:4]
	p := unsafe.Slice((*float32)(unsafe.Pointer(&x[0])), 16)
	a := archsimd.LoadFloat32x4(p[0:4]).ToBits().ReshapeToUint64s()
	b := archsimd.LoadFloat32x4(p[4:8]).ToBits().ReshapeToUint64s()
	c := archsimd.LoadFloat32x4(p[8:12]).ToBits().ReshapeToUint64s()
	d := archsimd.LoadFloat32x4(p[12:16]).ToBits().ReshapeToUint64s()
	if which == 0 {
		u, v := a.ConcatEven(b).ReshapeToUint32s(), c.ConcatEven(d).ReshapeToUint32s()
		return u.ConcatEven(v).BitsToFloat32(), u.ConcatOdd(v).BitsToFloat32()
	}
	u, v := a.ConcatOdd(b).ReshapeToUint32s(), c.ConcatOdd(d).ReshapeToUint32s()
	return u.ConcatEven(v).BitsToFloat32(), u.ConcatOdd(v).BitsToFloat32()
}

func fft_5(x, y []lc3_complex, n int) {
	if n < 4 {
		fft_5_scalar(x, y, n)
		return
	}
	c1 := archsimd.BroadcastFloat32x4(0.3090169944)
	c2 := archsimd.BroadcastFloat32x4(-0.8090169944)
	s1 := archsimd.BroadcastFloat32x4(-0.9510565163)
	s2 := archsimd.BroadcastFloat32x4(-0.5877852523)
	i := 0
	for ; i+4 <= n; i += 4 {
		zr, zi := neonLoadComplex4(x[i:])
		ar, ai := neonLoadComplex4(x[i+n:])
		br, bi := neonLoadComplex4(x[i+2*n:])
		cr, ci := neonLoadComplex4(x[i+3*n:])
		dr, di := neonLoadComplex4(x[i+4*n:])
		s14r, s14i := ar.Add(dr), ai.Add(di)
		d14r, d14i := ar.Sub(dr), ai.Sub(di)
		s23r, s23i := br.Add(cr), bi.Add(ci)
		d23r, d23i := br.Sub(cr), bi.Sub(ci)
		neonStoreComplex4(y[5*i:], 5, zr.Add(s14r).Add(s23r), zi.Add(s14i).Add(s23i))
		neonStoreComplex4(y[5*i+1:], 5,
			zr.Add(s14r.Mul(c1)).Sub(d14i.Mul(s1)).Add(s23r.Mul(c2)).Sub(d23i.Mul(s2)),
			zi.Add(s14i.Mul(c1)).Add(d14r.Mul(s1)).Add(s23i.Mul(c2)).Add(d23r.Mul(s2)))
		neonStoreComplex4(y[5*i+2:], 5,
			zr.Add(s14r.Mul(c2)).Sub(d14i.Mul(s2)).Add(s23r.Mul(c1)).Add(d23i.Mul(s1)),
			zi.Add(s14i.Mul(c2)).Add(d14r.Mul(s2)).Add(s23i.Mul(c1)).Sub(d23r.Mul(s1)))
		neonStoreComplex4(y[5*i+3:], 5,
			zr.Add(s14r.Mul(c2)).Add(d14i.Mul(s2)).Add(s23r.Mul(c1)).Sub(d23i.Mul(s1)),
			zi.Add(s14i.Mul(c2)).Sub(d14r.Mul(s2)).Add(s23i.Mul(c1)).Add(d23r.Mul(s1)))
		neonStoreComplex4(y[5*i+4:], 5,
			zr.Add(s14r.Mul(c1)).Add(d14i.Mul(s1)).Add(s23r.Mul(c2)).Add(d23i.Mul(s2)),
			zi.Add(s14i.Mul(c1)).Sub(d14r.Mul(s1)).Add(s23i.Mul(c2)).Sub(d23r.Mul(s2)))
	}
	fft5ScalarRange(x, y, n, i, n)
}

func fft_bf3(t *lc3_fft_bf3_twiddles, x, y []lc3_complex, n int) {
	n3 := t.n3
	stop := n3 &^ 3
	for i := 0; i < n; i++ {
		for j := 0; j < stop; j += 4 {
			z0r, z0i := neonLoadComplex4(x[i*n3+j:])
			z1r, z1i := neonLoadComplex4(x[n*n3+i*n3+j:])
			z2r, z2i := neonLoadComplex4(x[2*n*n3+i*n3+j:])
			for q := 0; q < 3; q++ {
				w0r, w0i := neonLoadTwiddles4(t.t[q*n3+j:], 0)
				w1r, w1i := neonLoadTwiddles4(t.t[q*n3+j:], 1)
				re := z0r.Add(z1r.Mul(w0r)).Sub(z1i.Mul(w0i)).Add(z2r.Mul(w1r)).Sub(z2i.Mul(w1i))
				im := z0i.Add(z1i.Mul(w0r)).Add(z1r.Mul(w0i)).Add(z2i.Mul(w1r)).Add(z2r.Mul(w1i))
				neonStoreComplex4(y[i*3*n3+q*n3+j:], 1, re, im)
			}
		}
	}
	fftBf3ScalarRange(t, x, y, n, stop, n3)
}

func fft_bf2(t *lc3_fft_bf2_twiddles, x, y []lc3_complex, n int) {
	n2 := t.n2
	stop := n2 &^ 3
	for i := 0; i < n; i++ {
		for j := 0; j < stop; j += 4 {
			z0r, z0i := neonLoadComplex4(x[i*n2+j:])
			z1r, z1i := neonLoadComplex4(x[n*n2+i*n2+j:])
			wr, wi := neonLoadComplex4(t.t[j:])
			re0 := z0r.Add(z1r.Mul(wr)).Sub(z1i.Mul(wi))
			im0 := z0i.Add(z1i.Mul(wr)).Add(z1r.Mul(wi))
			re1 := z0r.Sub(z1r.Mul(wr)).Add(z1i.Mul(wi))
			im1 := z0i.Sub(z1i.Mul(wr)).Sub(z1r.Mul(wi))
			k := i*2*n2 + j
			neonStoreComplex4(y[k:], 1, re0, im0)
			neonStoreComplex4(y[k+n2:], 1, re1, im1)
		}
	}
	fftBf2ScalarRange(t, x, y, n, stop, n2)
}

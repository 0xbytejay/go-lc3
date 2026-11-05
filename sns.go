// Copyright 2022 Google LLC
// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0
//
// Spectral noise shaping, adapted from liblc3's scalar implementation.

package lc3

import "math"

type lc3_sns_data_t struct {
	lfcb, hfcb   int
	shape, gain  int
	idx_a, idx_b int
	ls_a, ls_b   bool
}

var sns_dct16_m = [16][16]float32{

	{2.50000000e-01, 3.51850934e-01, 3.46759961e-01, 3.38329500e-01,
		3.26640741e-01, 3.11806253e-01, 2.93968901e-01, 2.73300467e-01,
		2.50000000e-01, 2.24291897e-01, 1.96423740e-01, 1.66663915e-01,
		1.35299025e-01, 1.02631132e-01, 6.89748448e-02, 3.46542923e-02},

	{2.50000000e-01, 3.38329500e-01, 2.93968901e-01, 2.24291897e-01,
		1.35299025e-01, 3.46542923e-02, -6.89748448e-02, -1.66663915e-01,
		-2.50000000e-01, -3.11806253e-01, -3.46759961e-01, -3.51850934e-01,
		-3.26640741e-01, -2.73300467e-01, -1.96423740e-01, -1.02631132e-01},

	{2.50000000e-01, 3.11806253e-01, 1.96423740e-01, 3.46542923e-02,
		-1.35299025e-01, -2.73300467e-01, -3.46759961e-01, -3.38329500e-01,
		-2.50000000e-01, -1.02631132e-01, 6.89748448e-02, 2.24291897e-01,
		3.26640741e-01, 3.51850934e-01, 2.93968901e-01, 1.66663915e-01},

	{2.50000000e-01, 2.73300467e-01, 6.89748448e-02, -1.66663915e-01,
		-3.26640741e-01, -3.38329500e-01, -1.96423740e-01, 3.46542923e-02,
		2.50000000e-01, 3.51850934e-01, 2.93968901e-01, 1.02631132e-01,
		-1.35299025e-01, -3.11806253e-01, -3.46759961e-01, -2.24291897e-01},

	{2.50000000e-01, 2.24291897e-01, -6.89748448e-02, -3.11806253e-01,
		-3.26640741e-01, -1.02631132e-01, 1.96423740e-01, 3.51850934e-01,
		2.50000000e-01, -3.46542923e-02, -2.93968901e-01, -3.38329500e-01,
		-1.35299025e-01, 1.66663915e-01, 3.46759961e-01, 2.73300467e-01},

	{2.50000000e-01, 1.66663915e-01, -1.96423740e-01, -3.51850934e-01,
		-1.35299025e-01, 2.24291897e-01, 3.46759961e-01, 1.02631132e-01,
		-2.50000000e-01, -3.38329500e-01, -6.89748448e-02, 2.73300467e-01,
		3.26640741e-01, 3.46542923e-02, -2.93968901e-01, -3.11806253e-01},

	{2.50000000e-01, 1.02631132e-01, -2.93968901e-01, -2.73300467e-01,
		1.35299025e-01, 3.51850934e-01, 6.89748448e-02, -3.11806253e-01,
		-2.50000000e-01, 1.66663915e-01, 3.46759961e-01, 3.46542923e-02,
		-3.26640741e-01, -2.24291897e-01, 1.96423740e-01, 3.38329500e-01},

	{2.50000000e-01, 3.46542923e-02, -3.46759961e-01, -1.02631132e-01,
		3.26640741e-01, 1.66663915e-01, -2.93968901e-01, -2.24291897e-01,
		2.50000000e-01, 2.73300467e-01, -1.96423740e-01, -3.11806253e-01,
		1.35299025e-01, 3.38329500e-01, -6.89748448e-02, -3.51850934e-01},

	{2.50000000e-01, -3.46542923e-02, -3.46759961e-01, 1.02631132e-01,
		3.26640741e-01, -1.66663915e-01, -2.93968901e-01, 2.24291897e-01,
		2.50000000e-01, -2.73300467e-01, -1.96423740e-01, 3.11806253e-01,
		1.35299025e-01, -3.38329500e-01, -6.89748448e-02, 3.51850934e-01},

	{2.50000000e-01, -1.02631132e-01, -2.93968901e-01, 2.73300467e-01,
		1.35299025e-01, -3.51850934e-01, 6.89748448e-02, 3.11806253e-01,
		-2.50000000e-01, -1.66663915e-01, 3.46759961e-01, -3.46542923e-02,
		-3.26640741e-01, 2.24291897e-01, 1.96423740e-01, -3.38329500e-01},

	{2.50000000e-01, -1.66663915e-01, -1.96423740e-01, 3.51850934e-01,
		-1.35299025e-01, -2.24291897e-01, 3.46759961e-01, -1.02631132e-01,
		-2.50000000e-01, 3.38329500e-01, -6.89748448e-02, -2.73300467e-01,
		3.26640741e-01, -3.46542923e-02, -2.93968901e-01, 3.11806253e-01},

	{2.50000000e-01, -2.24291897e-01, -6.89748448e-02, 3.11806253e-01,
		-3.26640741e-01, 1.02631132e-01, 1.96423740e-01, -3.51850934e-01,
		2.50000000e-01, 3.46542923e-02, -2.93968901e-01, 3.38329500e-01,
		-1.35299025e-01, -1.66663915e-01, 3.46759961e-01, -2.73300467e-01},

	{2.50000000e-01, -2.73300467e-01, 6.89748448e-02, 1.66663915e-01,
		-3.26640741e-01, 3.38329500e-01, -1.96423740e-01, -3.46542923e-02,
		2.50000000e-01, -3.51850934e-01, 2.93968901e-01, -1.02631132e-01,
		-1.35299025e-01, 3.11806253e-01, -3.46759961e-01, 2.24291897e-01},

	{2.50000000e-01, -3.11806253e-01, 1.96423740e-01, -3.46542923e-02,
		-1.35299025e-01, 2.73300467e-01, -3.46759961e-01, 3.38329500e-01,
		-2.50000000e-01, 1.02631132e-01, 6.89748448e-02, -2.24291897e-01,
		3.26640741e-01, -3.51850934e-01, 2.93968901e-01, -1.66663915e-01},

	{2.50000000e-01, -3.38329500e-01, 2.93968901e-01, -2.24291897e-01,
		1.35299025e-01, -3.46542923e-02, -6.89748448e-02, 1.66663915e-01,
		-2.50000000e-01, 3.11806253e-01, -3.46759961e-01, 3.51850934e-01,
		-3.26640741e-01, 2.73300467e-01, -1.96423740e-01, 1.02631132e-01},

	{2.50000000e-01, -3.51850934e-01, 3.46759961e-01, -3.38329500e-01,
		3.26640741e-01, -3.11806253e-01, 2.93968901e-01, -2.73300467e-01,
		2.50000000e-01, -2.24291897e-01, 1.96423740e-01, -1.66663915e-01,
		1.35299025e-01, -1.02631132e-01, 6.89748448e-02, -3.46542923e-02},
}

var sns_ge_14 = [64]float32{
	1.00000000e+00, 1.05250029e+00, 1.10775685e+00, 1.16591440e+00,
	1.22712524e+00, 1.29154967e+00, 1.35935639e+00, 1.43072299e+00,
	1.50583635e+00, 1.58489319e+00, 1.66810054e+00, 1.75567629e+00,
	1.84784980e+00, 1.94486244e+00, 2.04696827e+00, 2.15443469e+00,
	2.26754313e+00, 2.38658979e+00, 2.51188643e+00, 2.64376119e+00,
	2.78255940e+00, 2.92864456e+00, 3.08239924e+00, 3.24422608e+00,
	3.41454887e+00, 3.59381366e+00, 3.78248991e+00, 3.98107171e+00,
	4.19007911e+00, 4.41005945e+00, 4.64158883e+00, 4.88527357e+00,
	5.14175183e+00, 5.41169527e+00, 5.69581081e+00, 5.99484250e+00,
	6.30957344e+00, 6.64082785e+00, 6.98947321e+00, 7.35642254e+00,
	7.74263683e+00, 8.14912747e+00, 8.57695899e+00, 9.02725178e+00,
	9.50118507e+00, 1.00000000e+01, 1.05250029e+01, 1.10775685e+01,
	1.16591440e+01, 1.22712524e+01, 1.29154967e+01, 1.35935639e+01,
	1.43072299e+01, 1.50583635e+01, 1.58489319e+01, 1.66810054e+01,
	1.75567629e+01, 1.84784980e+01, 1.94486244e+01, 2.04696827e+01,
	2.15443469e+01, 2.26754313e+01, 2.38658979e+01, 2.51188643e+01}

var sns_ge_18 = [64]float32{
	1.00000000e+00, 1.06800043e+00, 1.14062492e+00, 1.21818791e+00,
	1.30102522e+00, 1.38949549e+00, 1.48398179e+00, 1.58489319e+00,
	1.69266662e+00, 1.80776868e+00, 1.93069773e+00, 2.06198601e+00,
	2.20220195e+00, 2.35195264e+00, 2.51188643e+00, 2.68269580e+00,
	2.86512027e+00, 3.05994969e+00, 3.26802759e+00, 3.49025488e+00,
	3.72759372e+00, 3.98107171e+00, 4.25178630e+00, 4.54090961e+00,
	4.84969343e+00, 5.17947468e+00, 5.53168120e+00, 5.90783791e+00,
	6.30957344e+00, 6.73862717e+00, 7.19685673e+00, 7.68624610e+00,
	8.20891416e+00, 8.76712387e+00, 9.36329209e+00, 1.00000000e+01,
	1.06800043e+01, 1.14062492e+01, 1.21818791e+01, 1.30102522e+01,
	1.38949549e+01, 1.48398179e+01, 1.58489319e+01, 1.69266662e+01,
	1.80776868e+01, 1.93069773e+01, 2.06198601e+01, 2.20220195e+01,
	2.35195264e+01, 2.51188643e+01, 2.68269580e+01, 2.86512027e+01,
	3.05994969e+01, 3.26802759e+01, 3.49025488e+01, 3.72759372e+01,
	3.98107171e+01, 4.25178630e+01, 4.54090961e+01, 4.84969343e+01,
	5.17947468e+01, 5.53168120e+01, 5.90783791e+01, 6.30957344e+01}

var sns_ge_22 = [64]float32{
	1.00000000e+00, 1.08372885e+00, 1.17446822e+00, 1.27280509e+00,
	1.37937560e+00, 1.49486913e+00, 1.62003281e+00, 1.75567629e+00,
	1.90267705e+00, 2.06198601e+00, 2.23463373e+00, 2.42173704e+00,
	2.62450630e+00, 2.84425319e+00, 3.08239924e+00, 3.34048498e+00,
	3.62017995e+00, 3.92329345e+00, 4.25178630e+00, 4.60778348e+00,
	4.99358789e+00, 5.41169527e+00, 5.86481029e+00, 6.35586411e+00,
	6.88803330e+00, 7.46476041e+00, 8.08977621e+00, 8.76712387e+00,
	9.50118507e+00, 1.02967084e+01, 1.11588399e+01, 1.20931568e+01,
	1.31057029e+01, 1.42030283e+01, 1.53922315e+01, 1.66810054e+01,
	1.80776868e+01, 1.95913107e+01, 2.12316686e+01, 2.30093718e+01,
	2.49359200e+01, 2.70237760e+01, 2.92864456e+01, 3.17385661e+01,
	3.43959997e+01, 3.72759372e+01, 4.03970086e+01, 4.37794036e+01,
	4.74450028e+01, 5.14175183e+01, 5.57226480e+01, 6.03882412e+01,
	6.54444792e+01, 7.09240702e+01, 7.68624610e+01, 8.32980665e+01,
	9.02725178e+01, 9.78309319e+01, 1.06022203e+02, 1.14899320e+02,
	1.24519708e+02, 1.34945600e+02, 1.46244440e+02, 1.58489319e+02}

var sns_ge_26 = [64]float32{
	1.00000000e+00, 1.09968890e+00, 1.20931568e+00, 1.32987103e+00,
	1.46244440e+00, 1.60823388e+00, 1.76855694e+00, 1.94486244e+00,
	2.13874364e+00, 2.35195264e+00, 2.58641621e+00, 2.84425319e+00,
	3.12779366e+00, 3.43959997e+00, 3.78248991e+00, 4.15956216e+00,
	4.57422434e+00, 5.03022373e+00, 5.53168120e+00, 6.08312841e+00,
	6.68954879e+00, 7.35642254e+00, 8.08977621e+00, 8.89623710e+00,
	9.78309319e+00, 1.07583590e+01, 1.18308480e+01, 1.30102522e+01,
	1.43072299e+01, 1.57335019e+01, 1.73019574e+01, 1.90267705e+01,
	2.09235283e+01, 2.30093718e+01, 2.53031508e+01, 2.78255940e+01,
	3.05994969e+01, 3.36499270e+01, 3.70044512e+01, 4.06933843e+01,
	4.47500630e+01, 4.92111475e+01, 5.41169527e+01, 5.95118121e+01,
	6.54444792e+01, 7.19685673e+01, 7.91430346e+01, 8.70327166e+01,
	9.57089124e+01, 1.05250029e+02, 1.15742288e+02, 1.27280509e+02,
	1.39968963e+02, 1.53922315e+02, 1.69266662e+02, 1.86140669e+02,
	2.04696827e+02, 2.25102829e+02, 2.47543082e+02, 2.72220379e+02,
	2.99357729e+02, 3.29200372e+02, 3.62017995e+02, 3.98107171e+02}

var sns_ge_30 = [64]float32{
	1.00000000e+00, 1.11588399e+00, 1.24519708e+00, 1.38949549e+00,
	1.55051578e+00, 1.73019574e+00, 1.93069773e+00, 2.15443469e+00,
	2.40409918e+00, 2.68269580e+00, 2.99357729e+00, 3.34048498e+00,
	3.72759372e+00, 4.15956216e+00, 4.64158883e+00, 5.17947468e+00,
	5.77969288e+00, 6.44946677e+00, 7.19685673e+00, 8.03085722e+00,
	8.96150502e+00, 1.00000000e+01, 1.11588399e+01, 1.24519708e+01,
	1.38949549e+01, 1.55051578e+01, 1.73019574e+01, 1.93069773e+01,
	2.15443469e+01, 2.40409918e+01, 2.68269580e+01, 2.99357729e+01,
	3.34048498e+01, 3.72759372e+01, 4.15956216e+01, 4.64158883e+01,
	5.17947468e+01, 5.77969288e+01, 6.44946677e+01, 7.19685673e+01,
	8.03085722e+01, 8.96150502e+01, 1.00000000e+02, 1.11588399e+02,
	1.24519708e+02, 1.38949549e+02, 1.55051578e+02, 1.73019574e+02,
	1.93069773e+02, 2.15443469e+02, 2.40409918e+02, 2.68269580e+02,
	2.99357729e+02, 3.34048498e+02, 3.72759372e+02, 4.15956216e+02,
	4.64158883e+02, 5.17947468e+02, 5.77969288e+02, 6.44946677e+02,
	7.19685673e+02, 8.03085722e+02, 8.96150502e+02, 1.00000000e+03}

var sns_ge_34 = [64]float32{
	1.00000000e+00, 1.13231759e+00, 1.28214312e+00, 1.45179321e+00,
	1.64389099e+00, 1.86140669e+00, 2.10770353e+00, 2.38658979e+00,
	2.70237760e+00, 3.05994969e+00, 3.46483486e+00, 3.92329345e+00,
	4.44241419e+00, 5.03022373e+00, 5.69581081e+00, 6.44946677e+00,
	7.30284467e+00, 8.26913948e+00, 9.36329209e+00, 1.06022203e+01,
	1.20050806e+01, 1.35935639e+01, 1.53922315e+01, 1.74288945e+01,
	1.97350438e+01, 2.23463373e+01, 2.53031508e+01, 2.86512027e+01,
	3.24422608e+01, 3.67349426e+01, 4.15956216e+01, 4.70994540e+01,
	5.33315403e+01, 6.03882412e+01, 6.83786677e+01, 7.74263683e+01,
	8.76712387e+01, 9.92716858e+01, 1.12407076e+02, 1.27280509e+02,
	1.44121960e+02, 1.63191830e+02, 1.84784980e+02, 2.09235283e+02,
	2.36920791e+02, 2.68269580e+02, 3.03766364e+02, 3.43959997e+02,
	3.89471955e+02, 4.41005945e+02, 4.99358789e+02, 5.65432741e+02,
	6.40249439e+02, 7.24965701e+02, 8.20891416e+02, 9.29509790e+02,
	1.05250029e+03, 1.19176459e+03, 1.34945600e+03, 1.52801277e+03,
	1.73019574e+03, 1.95913107e+03, 2.21835857e+03, 2.51188643e+03}

var sns_ge_table = [7][]float32{sns_ge_14[:], sns_ge_18[:], sns_ge_22[:], sns_ge_26[:], sns_ge_30[:], sns_ge_30[:], sns_ge_34[:]}

func sns_dct16_forward(x, y []float32) {
	for i := 0; i < 16; i++ {
		y[i] = 0
		for j := 0; j < 16; j++ {
			y[i] = float32(y[i] + float32(x[j]*sns_dct16_m[j][i]))
		}
	}
}
func sns_dct16_inverse(x, y []float32) {
	for i := 0; i < 16; i++ {
		y[i] = 0
		for j := 0; j < 16; j++ {
			y[i] = float32(y[i] + float32(x[j]*sns_dct16_m[i][j]))
		}
	}
}

func sns_compute_scale_factors(dt, sr, nbytes int, eb []float32, att bool, scf []float32) {
	var e [64]float32
	nb := lc3_num_bands[dt][sr]
	n4, n2 := 0, 64-nb
	if nb < 32 {
		n4 = 32 % nb
		n2 = nb - n4
	}
	for i := 0; i < n4; i++ {
		for j := 0; j < 4; j++ {
			e[4*i+j] = eb[i]
		}
	}
	for i := n4; i < n4+n2; i++ {
		e[2*(n4+i)] = eb[i]
		e[2*(n4+i)+1] = eb[i]
	}
	copy(e[4*n4+2*n2:], eb[n4+n2:nb])
	ge := sns_ge_table[sr]
	e0, e1 := e[0], e[0]
	var e2, eSum float32
	for i := 0; i < 63; {
		e2 = e[i+1]
		e[i] = float32(float32(float32(float32(e0*0.25)+float32(e1*0.5))+float32(e2*0.25)) * ge[i])
		eSum = float32(eSum + e[i])
		i++
		e0 = e[i+1]
		e[i] = float32(float32(float32(float32(e1*0.25)+float32(e2*0.5))+float32(e0*0.25)) * ge[i])
		eSum = float32(eSum + e[i])
		i++
		e1 = e[i+1]
		e[i] = float32(float32(float32(float32(e2*0.25)+float32(e0*0.5))+float32(e1*0.25)) * ge[i])
		eSum = float32(eSum + e[i])
		i++
	}
	e[63] = float32(float32(float32(e0*0.25)+float32(e1*0.75)) * ge[63])
	eSum = float32(eSum + e[63])
	noiseFloor := max(float32(eSum*float32(1e-4/64)), float32(0x1p-32))
	for i := 0; i < 64; i++ {
		e[i] = float32(lc3_log2f(max(e[i], noiseFloor)) * 0.5)
	}

	// Keep the scalar reference's individual rounded weighting terms.
	const div12 = float32(1.0 / 12)
	scf[0] = float32(float32(float32(float32(e[0]+e[4])*div12)+float32(float32(float32(e[0]+e[3])*2)*div12)) + float32(float32(float32(e[1]+e[2])*3)*div12))
	scfSum := scf[0]
	for i := 1; i < 15; i++ {
		scf[i] = float32(float32(float32(float32(e[4*i-1]+e[4*i+4])*div12)+float32(float32(float32(e[4*i]+e[4*i+3])*2)*div12)) + float32(float32(float32(e[4*i+1]+e[4*i+2])*3)*div12))
		scfSum = float32(scfSum + scf[i])
	}
	scf[15] = float32(float32(float32(float32(e[59]+e[63])*div12)+float32(float32(float32(e[60]+e[63])*2)*div12)) + float32(float32(float32(e[61]+e[62])*3)*div12))
	scfSum = float32(scfSum + scf[15])
	cf := float32(0.85)
	if lc3_hr(sr) {
		cf = 0.6
	}
	threshold := 4400
	if dt < lc3_DT_10M {
		threshold = 1150 * (1 + dt)
	}
	if lc3_hr(sr) && 8*nbytes > threshold {
		if dt < lc3_DT_10M {
			cf = float32(cf * 0.25)
		} else {
			cf = float32(cf * 0.35)
		}
	}
	for i := 0; i < 16; i++ {
		scf[i] = float32(cf * float32(scf[i]-float32(scfSum*(1.0/16))))
	}
	if !att {
		return
	}

	s1, s2, s3, s4 := scf[0], scf[1], scf[2], scf[3]
	sn := float32(s1 + s2)
	sn = float32(sn + s3)
	scf[0] = float32(sn * float32(1.0/3))
	sn = float32(sn + s4)
	scf[1] = float32(sn * (1.0 / 4))
	scfSum = float32(scf[0] + scf[1])
	for i := 2; i < 14; i++ {
		s0 := s1
		s1 = s2
		s2 = s3
		s3 = s4
		s4 = scf[i+2]
		sn = float32(sn + s4)
		scf[i] = float32(sn * float32(1.0/5))
		scfSum = float32(scfSum + scf[i])
		sn = float32(sn - s0)
	}
	scf[14] = float32(sn * (1.0 / 4))
	sn = float32(sn - s1)
	scf[15] = float32(sn * float32(1.0/3))
	scfSum = float32(scfSum + float32(scf[14]+scf[15]))
	cf = 0.5
	if dt == lc3_DT_7M5 {
		cf = 0.3
	}
	for i := 0; i < 16; i++ {
		scf[i] = float32(cf * float32(scf[i]-float32(scfSum*(1.0/16))))
	}
}

func sns_resolve_codebooks(scf []float32, lfcbIdx, hfcbIdx *int) {
	var dlfMax, dhfMax float32
	*lfcbIdx = 0
	*hfcbIdx = 0
	for cb := 0; cb < 32; cb++ {
		var dlf, dhf float32
		for i := 0; i < 8; i++ {
			l := float32(scf[i] - lc3_sns_lfcb[cb][i])
			h := float32(scf[8+i] - lc3_sns_hfcb[cb][i])
			dlf = float32(dlf + float32(l*l))
			dhf = float32(dhf + float32(h*h))
		}
		if cb == 0 || dlf < dlfMax {
			*lfcbIdx = cb
			dlfMax = dlf
		}
		if cb == 0 || dhf < dhfMax {
			*hfcbIdx = cb
			dhfMax = dhf
		}
	}
}

func sns_normalize(c []int, cn []float32) {
	c2Sum := 0
	for i := 0; i < 16; i++ {
		c2Sum += c[i] * c[i]
	}
	cNorm := float32(1 / float32(math.Sqrt(float64(float32(c2Sum)))))
	for i := 0; i < 16; i++ {
		cn[i] = float32(float32(c[i]) * cNorm)
	}
}

func sns_add_pulse(x []float32, y []int, n, start, end int, corr, energy *float32) {
	for k := start; k < end; k++ {
		z := float32(*corr + x[0])
		bestC2 := float32(z * z)
		bestE := float32(float32(*energy+float32(2*y[0])) + 1)
		best := 0
		for i := 1; i < n; i++ {
			z = float32(*corr + x[i])
			c2 := float32(z * z)
			e := float32(float32(*energy+float32(2*y[i])) + 1)
			if float32(c2*bestE) > float32(e*bestC2) {
				bestC2 = c2
				bestE = e
				best = i
			}
		}
		*corr = float32(*corr + x[best])
		*energy = float32(*energy + float32(2*y[best]+1))
		y[best]++
	}
}

func sns_quantize(scf []float32, lfcbIdx, hfcbIdx int, c *[4][16]int, cn *[4][16]float32, shapeIdx, gainIdx *int) {
	var r, x, xm [16]float32
	for i := 0; i < 8; i++ {
		r[i] = float32(scf[i] - lc3_sns_lfcb[lfcbIdx][i])
		r[8+i] = float32(scf[8+i] - lc3_sns_hfcb[hfcbIdx][i])
	}
	sns_dct16_forward(r[:], x[:])
	var xmSum float32
	for i := 0; i < 16; i++ {
		xm[i] = x[i]
		if xm[i] < 0 {
			xm[i] = -xm[i]
		}
		xmSum = float32(xmSum + xm[i])
	}
	proj := float32(5 / max(xmSum, float32(1e-31)))
	var corr, energy float32
	npulses := 0
	for i := 0; i < 16; i++ {
		// Both factors are nonnegative, so truncation equals floorf.
		c[3][i] = int(float32(xm[i] * proj))
		npulses += c[3][i]
		corr = float32(corr + float32(float32(c[3][i])*xm[i]))
		energy = float32(energy + float32(c[3][i]*c[3][i]))
	}
	sns_add_pulse(xm[:], c[3][:], 16, npulses, 6, &corr, &energy)
	npulses = 6
	c[2] = c[3]
	sns_add_pulse(xm[:], c[2][:], 16, npulses, 8, &corr, &energy)
	npulses = 8
	c[1] = c[2]
	for i := 10; i < 16; i++ {
		c[1][i] = 0
		npulses -= c[2][i]
		corr = float32(corr - float32(float32(c[2][i])*xm[i]))
		energy = float32(energy - float32(c[2][i]*c[2][i]))
	}
	sns_add_pulse(xm[:], c[1][:], 10, npulses, 10, &corr, &energy)
	c[0] = c[1]
	sns_add_pulse(xm[10:], c[0][10:], 6, 0, 1, &corr, &energy)
	for j := 0; j < 16; j++ {
		if x[j] < 0 {
			for i := 0; i < 4; i++ {
				c[i][j] = -c[i][j]
			}
		}
	}
	for i := 0; i < 4; i++ {
		sns_normalize(c[i][:], cn[i][:])
	}
	mseMin := float32(math.MaxFloat32)
	*shapeIdx = 0
	*gainIdx = 0
	for ic := 0; ic < 4; ic++ {
		gains := &lc3_sns_vq_gains[ic]
		cmseMin := float32(math.MaxFloat32)
		cgainIdx := 0
		for ig := 0; ig < gains.count; ig++ {
			g := gains.v[ig]
			var mse float32
			for i := 0; i < 16; i++ {
				z := float32(x[i] - float32(g*cn[ic][i]))
				mse = float32(mse + float32(z*z))
			}
			if mse < cmseMin {
				cgainIdx = ig
				cmseMin = mse
			}
		}
		if cmseMin < mseMin {
			*shapeIdx = ic
			*gainIdx = cgainIdx
			mseMin = cmseMin
		}
	}
}

func sns_unquantize(lfcbIdx, hfcbIdx int, c []float32, shape, gain int, scf []float32) {
	g := lc3_sns_vq_gains[shape].v[gain]
	sns_dct16_inverse(c, scf)
	for i := 0; i < 8; i++ {
		scf[i] = float32(lc3_sns_lfcb[lfcbIdx][i] + float32(g*scf[i]))
	}
	for i := 8; i < 16; i++ {
		scf[i] = float32(lc3_sns_hfcb[hfcbIdx][i-8] + float32(g*scf[i]))
	}
}

func sns_enum_mvpq(c []int, n int, idx *int, ls *bool) {
	i, ci := 0, 0
	for {
		ci = c[n-1-i]
		if ci != 0 || i >= 15 {
			break
		}
		i++
	}
	*idx = 0
	*ls = ci < 0
	j := ci
	if j < 0 {
		j = -j
	}
	for i++; i < n; i++ {
		ci = c[n-1-i]
		if ci != 0 {
			sign := 0
			if *ls {
				sign = 1
			}
			*idx = (*idx << 1) | sign
			*ls = ci < 0
		}
		*idx += int(lc3_sns_mpvq_offsets[i][min(j, 10)])
		if ci < 0 {
			j -= ci
		} else {
			j += ci
		}
	}
}

func sns_deenum_mvpq(idx int, ls bool, npulses int, c []int, n int) {
	p, i := 0, n-1
	for ; i >= 0 && idx != 0; i-- {
		ci := 0
		for idx < int(lc3_sns_mpvq_offsets[i][npulses-ci]) {
			ci++
		}
		idx -= int(lc3_sns_mpvq_offsets[i][npulses-ci])
		c[p] = ci
		if ls {
			c[p] = -ci
		}
		p++
		npulses -= ci
		if ci > 0 {
			ls = idx&1 != 0
			idx >>= 1
		}
	}
	if i >= 0 {
		c[p] = npulses
		if ls {
			c[p] = -npulses
		}
		p++
	}
	i--
	for ; i >= 0; i-- {
		c[p] = 0
		p++
	}
}

func sns_enumerate(shape int, c []int, idxA *int, lsA *bool, idxB *int, lsB *bool) {
	n := 16
	if shape < 2 {
		n = 10
	}
	sns_enum_mvpq(c, n, idxA, lsA)
	if shape == 0 {
		sns_enum_mvpq(c[10:], 6, idxB, lsB)
	}
}
func sns_deenumerate(shape, idxA int, lsA bool, idxB int, lsB bool, c []int) {
	n := 16
	if shape < 2 {
		n = 10
	}
	npulses := [...]int{10, 10, 8, 6}[shape]
	sns_deenum_mvpq(idxA, lsA, npulses, c, n)
	if shape == 0 {
		sns_deenum_mvpq(idxB, lsB, 1, c[10:], 6)
	} else if shape == 1 {
		clear(c[10:16])
	}
}

func sns_spectral_shaping(dt, sr int, scfQ []float32, inv bool, x, y []float32) {
	var scf [64]float32
	var s0 float32
	s1 := scfQ[0]
	if inv {
		s1 = -s1
	}
	scf[0] = s1
	scf[1] = s1
	for i := 0; i < 15; i++ {
		s0 = s1
		s1 = scfQ[i+1]
		if inv {
			s1 = -s1
		}
		delta := float32(s1 - s0)
		scf[4*i+2] = float32(s0 + float32(0.125*delta))
		scf[4*i+3] = float32(s0 + float32(0.375*delta))
		scf[4*i+4] = float32(s0 + float32(0.625*delta))
		scf[4*i+5] = float32(s0 + float32(0.875*delta))
	}
	scf[62] = float32(s1 + float32(0.125*float32(s1-s0)))
	scf[63] = float32(s1 + float32(0.375*float32(s1-s0)))
	nb := lc3_num_bands[dt][sr]
	n4, n2 := 0, 64-nb
	if nb < 32 {
		n4 = 32 % nb
		n2 = nb - n4
	}
	for i := 0; i < n4; i++ {
		scf[i] = float32(0.25 * float32(float32(float32(scf[4*i]+scf[4*i+1])+scf[4*i+2])+scf[4*i+3]))
	}
	for i := n4; i < n4+n2; i++ {
		scf[i] = float32(0.5 * float32(scf[2*(n4+i)]+scf[2*(n4+i)+1]))
	}
	copy(scf[n4+n2:nb], scf[4*n4+2*n2:4*n4+2*n2+(nb-n4-n2)])
	lim := lc3_band_lim[dt][sr]
	i := 0
	for ib := 0; ib < nb; ib++ {
		g := lc3_exp2f(-scf[ib])
		for ; i < lim[ib+1]; i++ {
			y[i] = float32(x[i] * g)
		}
	}
}

func lc3_sns_analyze(dt, sr, nbytes int, eb []float32, att bool, data *lc3_sns_data_t, x, y []float32) {
	var scf [16]float32
	var cn [4][16]float32
	var c [4][16]int
	sns_compute_scale_factors(dt, sr, nbytes, eb, att, scf[:])
	sns_resolve_codebooks(scf[:], &data.lfcb, &data.hfcb)
	sns_quantize(scf[:], data.lfcb, data.hfcb, &c, &cn, &data.shape, &data.gain)
	sns_unquantize(data.lfcb, data.hfcb, cn[data.shape][:], data.shape, data.gain, scf[:])
	sns_enumerate(data.shape, c[data.shape][:], &data.idx_a, &data.ls_a, &data.idx_b, &data.ls_b)
	sns_spectral_shaping(dt, sr, scf[:], false, x, y)
}
func lc3_sns_synthesize(dt, sr int, data *lc3_sns_data_t, x, y []float32) {
	var scf, cn [16]float32
	var c [16]int
	sns_deenumerate(data.shape, data.idx_a, data.ls_a, data.idx_b, data.ls_b, c[:])
	sns_normalize(c[:], cn[:])
	sns_unquantize(data.lfcb, data.hfcb, cn[:], data.shape, data.gain, scf[:])
	sns_spectral_shaping(dt, sr, scf[:], true, x, y)
}
func lc3_sns_get_nbits() int { return 38 }

func lc3_sns_put_data(bits *lc3_bits_t, data *lc3_sns_data_t) {
	lc3_put_bits(bits, data.lfcb, 5)
	lc3_put_bits(bits, data.hfcb, 5)
	shapeMSB := data.shape >> 1
	lc3_put_bit(bits, shapeMSB)
	lsA, lsB := 0, 0
	if data.ls_a {
		lsA = 1
	}
	if data.ls_b {
		lsB = 1
	}
	if shapeMSB == 0 {
		const sizeA = 2390004
		submode := data.shape & 1
		muxHigh := data.gain & 1
		if submode == 0 {
			muxHigh = 2*(data.idx_b+1) + lsB
		}
		muxCode := muxHigh*sizeA + data.idx_a
		lc3_put_bits(bits, data.gain>>submode, 1)
		lc3_put_bits(bits, lsA, 1)
		lc3_put_bits(bits, muxCode, 25)
	} else {
		const sizeA = 15158272
		submode := data.shape & 1
		muxCode := data.idx_a
		if submode != 0 {
			muxCode = sizeA + 2*data.idx_a + (data.gain & 1)
		}
		lc3_put_bits(bits, data.gain>>submode, 2)
		lc3_put_bits(bits, lsA, 1)
		lc3_put_bits(bits, muxCode, 24)
	}
}

func lc3_sns_get_data(bits *lc3_bits_t, data *lc3_sns_data_t) int {
	*data = lc3_sns_data_t{lfcb: lc3_get_bits(bits, 5), hfcb: lc3_get_bits(bits, 5)}
	shapeMSB := lc3_get_bit(bits)
	data.gain = lc3_get_bits(bits, 1+shapeMSB)
	data.ls_a = lc3_get_bit(bits) != 0
	muxCode := lc3_get_bits(bits, 25-shapeMSB)
	if shapeMSB == 0 {
		const sizeA = 2390004
		if muxCode >= sizeA*14 {
			return -1
		}
		data.idx_a = muxCode % sizeA
		muxCode /= sizeA
		data.shape = 0
		if muxCode < 2 {
			data.shape = 1
		}
		if data.shape == 0 {
			data.idx_b = (muxCode - 2) / 2
			data.ls_b = (muxCode-2)%2 != 0
		} else {
			data.gain = (data.gain << 1) + (muxCode % 2)
		}
	} else {
		const sizeA = 15158272
		if muxCode >= sizeA+1549824 {
			return -1
		}
		data.shape = 2
		if muxCode >= sizeA {
			data.shape++
		}
		if data.shape == 2 {
			data.idx_a = muxCode
		} else {
			muxCode -= sizeA
			data.idx_a = muxCode / 2
			data.gain = (data.gain << 1) + (muxCode % 2)
		}
	}
	return 0
}

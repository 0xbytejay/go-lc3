// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: translated band energy estimation to Go.

package lc3

func lc3_energy_compute(dt, sr int, x, e []float32) bool {
	nb := lc3_num_bands[dt][sr]
	lim := lc3_band_lim[dt][sr]
	var sums [2]float32
	high := nb - [4]int{2, 3, 4, 2}[dt]
	i := lim[0]
	for band := 0; band < nb; band++ {
		end := lim[band+1]
		n := end - i
		s := float32(x[i] * x[i])
		for i++; i < end; i++ {
			s = float32(s + float32(x[i]*x[i]))
		}
		e[band] = float32(s / float32(n))
		h := 0
		if band >= high {
			h = 1
		}
		sums[h] = float32(sums[h] + e[band])
	}
	return sums[1] > float32(30*sums[0])
}

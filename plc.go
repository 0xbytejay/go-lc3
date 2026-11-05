// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: translated packet loss concealment to Go.

package lc3

type lc3_plc_state_t struct {
	seed  uint16
	count int
	alpha float32
}

func lc3_plc_reset(plc *lc3_plc_state_t)   { plc.seed = 24607; lc3_plc_suspend(plc) }
func lc3_plc_suspend(plc *lc3_plc_state_t) { plc.count = 1; plc.alpha = 1 }
func lc3_plc_synthesize(dt, sr int, plc *lc3_plc_state_t, x, y []float32) {
	seed, alpha := plc.seed, plc.alpha
	factor := float32(1)
	if plc.count >= 8 {
		factor = 0.85
	} else if plc.count >= 4 {
		factor = 0.9
	}
	alpha = float32(alpha * factor)
	for i := 0; i < lc3_ne(dt, sr); i++ {
		seed = uint16((16831 + uint32(seed)*12821) & 0xffff)
		v := x[i]
		if seed&0x8000 != 0 {
			v = -v
		}
		y[i] = float32(alpha * v)
	}
	plc.seed, plc.alpha = seed, alpha
	plc.count++
}

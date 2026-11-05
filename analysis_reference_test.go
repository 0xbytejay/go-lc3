// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

package lc3

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

type analysisReference struct {
	DT         int    `json:"dt"`
	SR         int    `json:"sr"`
	Frame      int    `json:"frame"`
	EnergyHash uint32 `json:"energyHash"`
	NN         int    `json:"nn"`
	BW         int    `json:"bw"`
	Attack     int    `json:"attack"`
	EN1        int32  `json:"en1"`
	AN1        int32  `json:"an1"`
	PAtt       int    `json:"patt"`
	PLCHash    uint32 `json:"plcHash"`
	Seed       uint16 `json:"seed"`
	Count      int    `json:"count"`
	Alpha      uint32 `json:"alpha"`
}

func TestAnalysisReference(t *testing.T) {
	data, err := os.ReadFile("testdata/analysis_reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []analysisReference
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 312 {
		t.Fatalf("expected312 cases got%d", len(cases))
	}
	var att lc3_attdet_analysis_t
	var plc lc3_plc_state_t
	for _, v := range cases {
		if v.Frame == 0 {
			att = lc3_attdet_analysis_t{}
			lc3_plc_reset(&plc)
		}
		t.Run(fmt.Sprintf("dt%d_sr%d_frame%d", v.DT, v.SR, v.Frame), func(t *testing.T) {
			ns, nt, ne, nb := lc3_ns(v.DT, v.SR), lc3_nt(v.SR), lc3_ne(v.DT, v.SR), lc3_num_bands[v.DT][v.SR]
			x := make([]int16, ns+nt)
			spec := make([]float32, ne)
			energy := make([]float32, nb)
			concealed := make([]float32, ne)
			rng := uint32(0xdca12845) + uint32(v.DT*1000+v.SR*100+v.Frame)
			for i := range x {
				rng = rng*1664525 + 1013904223
				x[i] = int16(rng >> 16)
			}
			for i := range spec {
				rng = rng*1664525 + 1013904223
				spec[i] = float32(float32(int(rng>>16)-32768) * 0.00390625)
				if v.Frame%3 == 0 && i > ne*3/4 {
					spec[i] = float32(spec[i] * 0.0001)
				}
				if v.Frame%3 == 1 && i < ne*3/4 {
					spec[i] = float32(spec[i] * 0.0001)
				}
			}
			nn := lc3_energy_compute(v.DT, v.SR, spec, energy)
			bw := lc3_bwdet_run(v.DT, v.SR, energy)
			nbytes := [3]int{20, 100, 200}[v.Frame%3]
			attack := lc3_attdet_run(v.DT, v.SR, nbytes, &att, x)
			if spectrumFloatHash(energy) != v.EnergyHash || spec_bool(nn) != v.NN || bw != v.BW {
				t.Errorf("energy/bw got hash%08x nn%t bw%d want%08x %d %d", spectrumFloatHash(energy), nn, bw, v.EnergyHash, v.NN, v.BW)
			}
			if spec_bool(attack) != v.Attack || att.en1 != v.EN1 || att.an1 != v.AN1 || att.p_att != v.PAtt {
				t.Errorf("attack got%t %+v want%d %d %d %d", attack, att, v.Attack, v.EN1, v.AN1, v.PAtt)
			}
			if v.Frame == 2 {
				lc3_plc_suspend(&plc)
			}
			lc3_plc_synthesize(v.DT, v.SR, &plc, spec, concealed)
			if spectrumFloatHash(concealed) != v.PLCHash || plc.seed != v.Seed || plc.count != v.Count || math.Float32bits(plc.alpha) != v.Alpha {
				t.Errorf("PLC got%08x %+v want%08x seed%d count%d alpha%08x", spectrumFloatHash(concealed), plc, v.PLCHash, v.Seed, v.Count, v.Alpha)
			}
		})
	}
}

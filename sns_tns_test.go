// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0

package lc3

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math"
	"os"
	"reflect"
	"testing"
)

type snsTNSVector struct {
	DT          int       `json:"dt"`
	SR          int       `json:"sr"`
	Pattern     int       `json:"pattern"`
	NBytes      int       `json:"nbytes"`
	Att         bool      `json:"att"`
	SNS         [8]int    `json:"sns"`
	SNSForward  uint32    `json:"sns_forward"`
	SNSInverse  uint32    `json:"sns_inverse"`
	TNSWeight   bool      `json:"tns_weight"`
	TNSNFilters int       `json:"tns_nfilters"`
	TNSOrder    [2]int    `json:"tns_order"`
	TNSRC       [2][8]int `json:"tns_rc"`
	TNSForward  uint32    `json:"tns_forward"`
	TNSInverse  uint32    `json:"tns_inverse"`
	TNSNBits    int       `json:"tns_nbits"`
	Bitstream   uint32    `json:"bitstream"`
}

func snsTNSCRC(x []float32) uint32 {
	h := crc32.NewIEEE()
	var b [4]byte
	for _, v := range x {
		binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
		_, _ = h.Write(b[:])
	}
	return h.Sum32()
}
func snsTNSBool(v bool) int {
	if v {
		return 1
	}
	return 0
}
func TestSNSTNSScalarReference(t *testing.T) {
	data, err := os.ReadFile("testdata/sns_tns_scalar.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []snsTNSVector
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 78 {
		t.Fatalf("have %d cases, want 78", len(vectors))
	}
	for index, v := range vectors {
		t.Run(fmt.Sprintf("dt%d_sr%d_pattern%d", v.DT, v.SR, v.Pattern), func(t *testing.T) {
			rng := uint32(0x14912be1 + index)
			next := func() uint32 { rng ^= rng << 13; rng ^= rng >> 17; rng ^= rng << 5; return rng }
			nb, ne := lc3_num_bands[v.DT][v.SR], lc3_ne(v.DT, v.SR)
			eb := make([]float32, nb)
			x := make([]float32, ne)
			for i := range eb {
				eb[i] = float32((next()&65535)+1) / 16
			}
			walk := 10000
			for i := range x {
				r := int(next()&65535) - 32768
				if v.Pattern == 1 {
					walk += r % 257
					r = walk
				}
				if v.Pattern == 2 {
					r = (i%128)*128 - 8192
				}
				x[i] = float32(r) / 16
			}
			y, z := make([]float32, ne), make([]float32, ne)
			var sns lc3_sns_data_t
			lc3_sns_analyze(v.DT, v.SR, v.NBytes, eb, v.Att, &sns, x, y)
			got := [8]int{sns.lfcb, sns.hfcb, sns.shape, sns.gain, sns.idx_a, sns.idx_b, snsTNSBool(sns.ls_a), snsTNSBool(sns.ls_b)}
			if got != v.SNS {
				t.Errorf("SNS data: got %v want %v", got, v.SNS)
			}
			if crc := snsTNSCRC(y); crc != v.SNSForward {
				t.Errorf("SNS forward CRC got %08x want %08x", crc, v.SNSForward)
			}
			lc3_sns_synthesize(v.DT, v.SR, &sns, x, z)
			if crc := snsTNSCRC(z); crc != v.SNSInverse {
				t.Errorf("SNS inverse CRC got %08x want %08x", crc, v.SNSInverse)
			}
			var tns lc3_tns_data_t
			copy(y, x)
			copy(z, x)
			lc3_tns_analyze(v.DT, v.SR, false, v.NBytes, &tns, y)
			if tns.lpc_weighting != v.TNSWeight || tns.nfilters != v.TNSNFilters || tns.rc_order != v.TNSOrder || !reflect.DeepEqual(tns.rc, v.TNSRC) {
				t.Errorf("TNS data got %+v want weighting=%v filters=%d orders=%v rc=%v", tns, v.TNSWeight, v.TNSNFilters, v.TNSOrder, v.TNSRC)
			}
			encoded := make([]byte, v.NBytes)
			var bits lc3_bits_t
			lc3_setup_bits(&bits, lc3_BITS_MODE_WRITE, encoded, len(encoded))
			lc3_tns_put_data(&bits, &tns)
			lc3_sns_put_data(&bits, &sns)
			lc3_flush_bits(&bits)
			if crc := crc32.ChecksumIEEE(encoded); crc != v.Bitstream {
				t.Errorf("combined TNS/SNS bitstream CRC got %08x want %08x", crc, v.Bitstream)
			}
			lc3_setup_bits(&bits, lc3_BITS_MODE_READ, encoded, len(encoded))
			var st lc3_sns_data_t
			var tt lc3_tns_data_t
			if ret := lc3_tns_get_data(&bits, v.DT, v.SR, v.NBytes, &tt); ret != 0 {
				t.Errorf("TNS get_data returned %d", ret)
			}
			if ret := lc3_sns_get_data(&bits, &st); ret != 0 {
				t.Errorf("SNS get_data returned %d", ret)
			}
			if st != sns {
				t.Errorf("SNS bitstream roundtrip got %+v want %+v", st, sns)
			}
			if tt.nfilters != tns.nfilters || tt.lpc_weighting != tns.lpc_weighting || tt.rc_order != tns.rc_order {
				t.Errorf("TNS bitstream metadata roundtrip got %+v want %+v", tt, tns)
			}
			for f := 0; f < tns.nfilters; f++ {
				for k := 0; k < tns.rc_order[f]; k++ {
					if tt.rc[f][k] != tns.rc[f][k] {
						t.Errorf("TNS RC[%d][%d] roundtrip got %d want %d", f, k, tt.rc[f][k], tns.rc[f][k])
					}
				}
			}
			if n := lc3_tns_get_nbits(&tns); n != v.TNSNBits {
				t.Errorf("TNS bit count got %d want %d", n, v.TNSNBits)
			}
			if crc := snsTNSCRC(y); crc != v.TNSForward {
				t.Errorf("TNS forward CRC got %08x want %08x", crc, v.TNSForward)
			}
			lc3_tns_synthesize(v.DT, v.SR, &tns, z)
			if crc := snsTNSCRC(z); crc != v.TNSInverse {
				t.Errorf("TNS inverse CRC got %08x want %08x", crc, v.TNSInverse)
			}
		})
	}
}

func TestSNSPulseEnumeration(t *testing.T) {
	rng := uint32(0x7813ce91)
	for shape := 0; shape < 4; shape++ {
		for trial := 0; trial < 2000; trial++ {
			var c, want [16]int
			next := func() uint32 { rng ^= rng << 13; rng ^= rng >> 17; rng ^= rng << 5; return rng }
			n, k := 16, [...]int{10, 10, 8, 6}[shape]
			if shape < 2 {
				n = 10
			}
			for j := 0; j < k; j++ {
				want[int(next()%uint32(n))]++
			}
			if shape == 0 {
				want[10+int(next()%6)] = 1
			}
			for j := range want {
				if next()&1 != 0 {
					want[j] = -want[j]
				}
			}
			var ia, ib int
			var la, lb bool
			sns_enumerate(shape, want[:], &ia, &la, &ib, &lb)
			sns_deenumerate(shape, ia, la, ib, lb, c[:])
			if c != want {
				t.Fatalf("shape%d trial%d roundtrip %v -> %d,%v,%d,%v -> %v", shape, trial, want, ia, la, ib, lb, c)
			}
		}
	}
}

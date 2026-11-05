// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0

//go:build arm64 && go1.27 && goexperiment.simd && !purego

package lc3

import "testing"

func TestLTPFNEONResampleDotWrapAndRemainder(t *testing.T) {
	for n := 0; n <= 129; n++ {
		for patternA := 0; patternA < 4; patternA++ {
			for patternB := 0; patternB < 4; patternB++ {
				a, b := ltpfBackendSamples(n+3, patternA, 1)[3:], ltpfBackendSamples(n+1, patternB, 2)[1:]
				var want int32
				for i := 0; i < n; i++ {
					want += int32(a[i]) * int32(b[i])
				}
				if got := ltpfResampleDot(a, b, n); got != want {
					t.Fatalf("n=%d patterns=%d/%d got=%d want=%d", n, patternA, patternB, got, want)
				}
			}
		}
	}
}

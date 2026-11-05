// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

package lc3_test

import (
	"fmt"
	"time"

	lc3 "github.com/0xbytejay/go-lc3"
)

func Example() {
	config := lc3.Config{SampleRate: 16000, FrameDuration: 10 * time.Millisecond}
	encoder, err := lc3.NewEncoder(config)
	if err != nil {
		panic(err)
	}
	decoder, err := lc3.NewDecoder(config)
	if err != nil {
		panic(err)
	}
	size, err := config.FrameBytes(32000)
	if err != nil {
		panic(err)
	}
	input := make([]int16, encoder.FrameSamples())
	frame := make([]byte, size)
	output := make([]int16, decoder.FrameSamples())
	if err := encoder.Encode(frame, input); err != nil {
		panic(err)
	}
	concealed, err := decoder.Decode(output, frame)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(input), len(frame), len(output), concealed)
	// Output: 160 40 160 false
}

// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
package lc3

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

// FuzzEncode exercises the complete encoder, including stream history,
// resampling, all PCM representations, and every byte budget in each mode.
// Selector high bits also request invalid raw PCM or invalid API calls.
func FuzzEncode(f *testing.F) {
	for dt := 0; dt < 4; dt++ {
		for sr := 0; sr < 7; sr++ {
			if dt == 2 && sr >= 5 {
				continue
			}
			limit := 5
			if sr >= 5 {
				limit = 7
			}
			for pcm := sr; pcm < limit; pcm++ {
				for format := 0; format < 4; format++ {
					selector := uint8(dt + 4*(pcm-sr))
					tag := uint8(format + 4*(format%3))
					f.Add(selector, uint8(sr), tag, uint16(0), []byte{0, 127, 255, 128, 31, 64, 129, 254})
					f.Add(selector, uint8(sr), tag|64, uint16(65535), []byte{0, 0, 0, 0})
					if format == int(PCMFormatS24) || format == int(PCMFormatFloat32) {
						f.Add(selector, uint8(sr), tag|128, uint16(127), []byte{255, 255, 255, 127})
					}
				}
			}
		}
	}
	f.Fuzz(func(t *testing.T, duration, rate, tag uint8, budget uint16, data []byte) {
		if len(data) > 4096 {
			return
		}
		dt, sr := int(duration)%4, int(rate)%7
		if dt == 2 && sr >= 5 {
			return
		}
		limit := 5
		if sr >= 5 {
			limit = 7
		}
		pcmRate := sr + int(duration/4)%(limit-sr)
		cfg := codecConfig(dt, sr, pcmRate)
		format := PCMFormat(tag % 4)
		stride := 1 + int(tag/4)%3
		unit := format.width()
		samples := cfg.FrameSamples()
		minimum, maximum, err := cfg.FrameSizeRange()
		if err != nil {
			t.Fatal(err)
		}
		size := minimum + int(budget)%(maximum-minimum+1)
		input := make([]byte, samples*stride*unit)
		for i := range input {
			if len(data) > 0 {
				input[i] = data[i%len(data)]
			}
		}
		// Normalize the selected channel for the valid-input branch. Other channel
		// slots retain arbitrary bytes and must never be validated or consumed.
		if tag&128 == 0 {
			for i := 0; i < samples; i++ {
				p := input[i*stride*unit:]
				switch format {
				case PCMFormatS24:
					v := int32(binary.LittleEndian.Uint32(p)<<8) >> 8
					binary.LittleEndian.PutUint32(p, uint32(v))
				case PCMFormatFloat32:
					v := int32(binary.LittleEndian.Uint32(p)) >> 8
					binary.LittleEndian.PutUint32(p, math.Float32bits(float32(float32(v)*0x1p-23)))
				}
			}
		}
		encoder, err := NewEncoder(cfg)
		if err != nil {
			t.Fatal(err)
		}
		control, _ := NewEncoder(cfg)
		packet := bytes.Repeat([]byte{0xa5}, size)
		expected := make([]byte, size)
		if tag&64 != 0 {
			before := append([]byte(nil), packet...)
			switch budget % 3 {
			case 0:
				err = encoder.EncodePCM(packet[:minimum-1], input, format, stride)
			case 1:
				err = encoder.EncodePCM(packet, input[:(samples-1)*stride*unit], format, stride)
			default:
				err = encoder.EncodePCM(packet, input, format, 0)
			}
			if err == nil || (!errors.Is(err, ErrFrameSize) && !errors.Is(err, ErrPCM)) {
				t.Fatalf("invalid call returned %v", err)
			}
			if !bytes.Equal(packet, before) {
				t.Fatal("invalid call changed output")
			}
		}
		before := append([]byte(nil), packet...)
		err = encoder.EncodePCM(packet, input, format, stride)
		controlErr := control.EncodePCM(expected, input, format, stride)
		if err != nil {
			if tag&128 == 0 || !errors.Is(err, ErrPCM) || !errors.Is(controlErr, ErrPCM) {
				t.Fatalf("valid encode returned %v (control %v)", err, controlErr)
			}
			if !bytes.Equal(packet, before) {
				t.Fatal("invalid PCM changed output")
			}
			// Rejection must leave both history and output untouched, including when a
			// bad sample appears after many valid samples in a strided buffer.
			clear(input)
			if err = encoder.EncodePCM(packet, input, format, stride); err != nil {
				t.Fatal(err)
			}
			if err = control.EncodePCM(expected, input, format, stride); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(packet, expected) {
				t.Fatal("rejected PCM advanced encoder state")
			}
			return
		}
		if controlErr != nil || !bytes.Equal(packet, expected) {
			t.Fatalf("invalid call advanced state or output depends on destination initialization (control %v)", controlErr)
		}
		decoder, _ := NewDecoder(cfg)
		output := make([]float32, samples)
		for frame := 0; frame < 2; frame++ {
			concealed, err := decoder.DecodeFloat32(output, packet, 1)
			if err != nil || concealed {
				t.Fatalf("encoder produced invalid frame %d: concealed=%t err=%v", frame, concealed, err)
			}
			for _, v := range output {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < -1 || v > 1 {
					t.Fatalf("encoder roundtrip produced invalid PCM %v", v)
				}
			}
			if frame == 0 {
				next := minimum + (int(budget)*17+31)%(maximum-minimum+1)
				packet = make([]byte, next)
				if err = encoder.EncodePCM(packet, input, format, stride); err != nil {
					t.Fatal(err)
				}
			}
		}
	})
}

// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

package lc3

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

type codecReferenceFrame struct {
	Format    PCMFormat `json:"format"`
	Stride    int       `json:"stride"`
	Status    int       `json:"status"`
	Data      string    `json:"data"`
	PCMSHA256 string    `json:"pcmSHA256"`
}
type codecReferenceStream struct {
	DT     int                   `json:"dt"`
	SR     int                   `json:"sr"`
	PCM    int                   `json:"pcm"`
	Format PCMFormat             `json:"format"`
	Frames []codecReferenceFrame `json:"frames"`
}
type codecReference struct {
	Upstream string                 `json:"upstream"`
	Streams  []codecReferenceStream `json:"streams"`
}

var codecSine16 = [16]int{0, 12539, 23170, 30273, 32767, 30273, 23170, 12539, 0, -12539, -23170, -30273, -32767, -30273, -23170, -12539}
var codecRates = [7]int{8000, 16000, 24000, 32000, 48000, 48000, 96000}

func codecConfig(dt, sr, pcm int) Config {
	return Config{SampleRate: codecRates[sr], PCMSampleRate: codecRates[pcm], FrameDuration: time.Duration(2500*(1+dt)) * time.Microsecond, HighResolution: sr >= 5}
}
func codecPCM(dt, sr, pcm, frame int, format PCMFormat, stride int) []byte {
	n := lc3_ns(dt, pcm)
	width := format.width()
	out := bytes.Repeat([]byte{0xa5}, n*stride*width)
	rng := uint32(0x6a7f31e9) + uint32(frame*31+dt*1000+sr*100+pcm*10000)
	for i := 0; i < n; i++ {
		at := frame*n + i
		sample := int32(0)
		rng = rng*1664525 + 1013904223
		switch frame / 4 {
		case 1:
			sample = int32(codecSine16[((at*7+(sr+1)*11)&255)>>4] * 192)
		case 2:
			v := codecSine16[((at*3)&255)>>4] + codecSine16[((at*7)&255)>>4]/2
			env := 4 + (at/48)%12
			sample = int32(v * env * 8)
		case 3:
			sample = int32((int(rng>>16) - 32768) * 192)
		case 4:
			sample = int32(codecSine16[((at*11)&255)>>4] * 255)
		case 5:
			sample = int32((int(rng>>16) - 32768) / 64)
		case 6:
			if at%127 == 0 {
				sample = 8388607
			} else if at%127 == 1 {
				sample = -8388608
			}
		}
		p := out[i*stride*width:]
		u := uint32(sample)
		switch format {
		case PCMFormatS16:
			binary.LittleEndian.PutUint16(p, uint16(sample>>8))
		case PCMFormatFloat32:
			binary.LittleEndian.PutUint32(p, math.Float32bits(float32(float32(sample)*0x1p-23)))
		default:
			for j := 0; j < width; j++ {
				p[j] = byte(u >> (8 * j))
			}
		}
	}
	return out
}
func codecEncodeTyped(e *Encoder, dst, pcm []byte, format PCMFormat, stride int) error {
	switch format {
	case PCMFormatS16:
		s := make([]int16, len(pcm)/2)
		for i := range s {
			s[i] = int16(binary.LittleEndian.Uint16(pcm[2*i:]))
		}
		return e.EncodeS16(dst, s, stride)
	case PCMFormatS24:
		s := make([]int32, len(pcm)/4)
		for i := range s {
			s[i] = int32(binary.LittleEndian.Uint32(pcm[4*i:]))
		}
		return e.EncodeS24(dst, s, stride)
	case PCMFormatS24LE:
		return e.EncodeS24LE(dst, pcm, stride)
	default:
		s := make([]float32, len(pcm)/4)
		for i := range s {
			s[i] = math.Float32frombits(binary.LittleEndian.Uint32(pcm[4*i:]))
		}
		return e.EncodeFloat32(dst, s, stride)
	}
}
func codecDecodeTyped(d *Decoder, dst, frame []byte, format PCMFormat, stride int) (bool, error) {
	switch format {
	case PCMFormatS16:
		s := make([]int16, len(dst)/2)
		for i := range s {
			s[i] = int16(binary.LittleEndian.Uint16(dst[2*i:]))
		}
		plc, err := d.DecodeS16(s, frame, stride)
		for i, v := range s {
			binary.LittleEndian.PutUint16(dst[2*i:], uint16(v))
		}
		return plc, err
	case PCMFormatS24:
		s := make([]int32, len(dst)/4)
		for i := range s {
			s[i] = int32(binary.LittleEndian.Uint32(dst[4*i:]))
		}
		plc, err := d.DecodeS24(s, frame, stride)
		for i, v := range s {
			binary.LittleEndian.PutUint32(dst[4*i:], uint32(v))
		}
		return plc, err
	case PCMFormatS24LE:
		return d.DecodeS24LE(dst, frame, stride)
	default:
		s := make([]float32, len(dst)/4)
		for i := range s {
			s[i] = math.Float32frombits(binary.LittleEndian.Uint32(dst[4*i:]))
		}
		plc, err := d.DecodeFloat32(s, frame, stride)
		for i, v := range s {
			binary.LittleEndian.PutUint32(dst[4*i:], math.Float32bits(v))
		}
		return plc, err
	}
}
func TestCodecReference(t *testing.T) {
	f, err := os.Open("testdata/codec_reference.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var oracle codecReference
	if err = json.NewDecoder(reader).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Upstream != UpstreamVersion || len(oracle.Streams) != 276 {
		t.Fatal("unexpected oracle revision or stream count")
	}
	for _, stream := range oracle.Streams {
		t.Run(fmt.Sprintf("dt%d_sr%d_pcm%d_format%d", stream.DT, stream.SR, stream.PCM, stream.Format), func(t *testing.T) {
			cfg := codecConfig(stream.DT, stream.SR, stream.PCM)
			enc, err := NewEncoder(cfg)
			if err != nil {
				t.Fatal(err)
			}
			typedEnc, _ := NewEncoder(cfg)
			dec, _ := NewDecoder(cfg)
			typedDec, _ := NewDecoder(cfg)
			if len(stream.Frames) != 32 {
				t.Fatal("expected32 frames")
			}
			for index, frame := range stream.Frames {
				want, err := hex.DecodeString(frame.Data)
				if err != nil {
					t.Fatal(err)
				}
				input := codecPCM(stream.DT, stream.SR, stream.PCM, index, stream.Format, frame.Stride)
				got := make([]byte, len(want))
				typed := make([]byte, len(want))
				if err = enc.EncodePCM(got, input, stream.Format, frame.Stride); err != nil {
					t.Fatalf("frame%d encode:%v", index, err)
				}
				if err = codecEncodeTyped(typedEnc, typed, input, stream.Format, frame.Stride); err != nil {
					t.Fatalf("frame%d typed encode:%v", index, err)
				}
				if !bytes.Equal(got, want) || !bytes.Equal(typed, want) {
					i := 0
					for i < len(got) && got[i] == want[i] && typed[i] == want[i] {
						i++
					}
					t.Fatalf("frame%d encoded mismatch at byte%d: got%02x typed%02x want%02x", index, i, got[i], typed[i], want[i])
				}
				compressed := want
				if (index >= 8 && index <= 16) || index == 23 {
					compressed = nil
				}
				if index == 20 {
					compressed = bytes.Repeat([]byte{255}, len(want))
				}
				n := cfg.FrameSamples() * frame.Stride * frame.Format.width()
				output := bytes.Repeat([]byte{0xa5}, n)
				typedOutput := bytes.Repeat([]byte{0xa5}, n)
				plc, err := dec.DecodePCM(output, compressed, frame.Format, frame.Stride)
				if err != nil || spec_bool(plc) != frame.Status {
					t.Fatalf("frame%d decode PLC%t err%v wantStatus%d", index, plc, err, frame.Status)
				}
				typedPLC, err := codecDecodeTyped(typedDec, typedOutput, compressed, frame.Format, frame.Stride)
				if err != nil || typedPLC != plc {
					t.Fatalf("frame%d typed decode PLC%t err%v", index, typedPLC, err)
				}
				if h := sha256.Sum256(output); hex.EncodeToString(h[:]) != frame.PCMSHA256 {
					t.Fatalf("frame%d output format%d PCM SHA256 %x want%s", index, frame.Format, h, frame.PCMSHA256)
				}
				if !bytes.Equal(output, typedOutput) {
					t.Fatalf("frame%d typed output differs", index)
				}
			}
		})
	}
}

func TestInvalidCallsPreserveState(t *testing.T) {
	cfg := Config{SampleRate: 16000, FrameDuration: FrameDuration10M}
	enc, _ := NewEncoder(cfg)
	control, _ := NewEncoder(cfg)
	dec, _ := NewDecoder(cfg)
	decControl, _ := NewDecoder(cfg)
	pcm := make([]int16, cfg.FrameSamples())
	for i := range pcm {
		pcm[i] = int16(i*51 - 4000)
	}
	good := make([]byte, 40)
	if err := enc.Encode(good, pcm); err != nil {
		t.Fatal(err)
	}
	other := make([]byte, 40)
	control.Encode(other, pcm)
	out, otherOut := make([]int16, len(pcm)), make([]int16, len(pcm))
	dec.Decode(out, good)
	decControl.Decode(otherOut, good)
	badPCM := make([]float32, len(pcm))
	badPCM[len(badPCM)-1] = float32(math.NaN())
	s24 := make([]int32, len(pcm))
	s24[len(s24)-1] = 8388608
	for name, call := range map[string]func() error{
		"shortPCM": func() error { return enc.Encode(other, pcm[:len(pcm)-1]) }, "badSize": func() error { return enc.Encode(other[:19], pcm) }, "zeroStride": func() error { return enc.EncodeS16(other, pcm, 0) }, "hugeStride": func() error { return enc.EncodeS16(other, pcm, int(^uint(0)>>1)) }, "nonfinitePCM": func() error { return enc.EncodeFloat32(other, badPCM, 1) }, "invalidS24": func() error { return enc.EncodeS24(other, s24, 1) }, "invalidFormat": func() error { return enc.EncodePCM(other, make([]byte, len(pcm)*4), PCMFormat(255), 1) },
	} {
		t.Run(name, func(t *testing.T) {
			before := append([]byte(nil), other...)
			if err := call(); err == nil {
				t.Error("expected error")
			}
			if !bytes.Equal(other, before) {
				t.Error("invalid encode changed output")
			}
		})
	}
	for name, call := range map[string]func() (bool, error){"shortPCM": func() (bool, error) { return dec.Decode(out[:len(out)-1], good) }, "badSize": func() (bool, error) { return dec.Decode(out, good[:19]) }, "zeroStride": func() (bool, error) { return dec.DecodeS16(out, good, 0) }, "hugeStride": func() (bool, error) { return dec.DecodeS16(out, good, int(^uint(0)>>1)) }, "invalidFormat": func() (bool, error) { return dec.DecodePCM(make([]byte, len(pcm)*4), good, PCMFormat(255), 1) }} {
		t.Run(name, func(t *testing.T) {
			before := append([]int16(nil), out...)
			if _, err := call(); err == nil {
				t.Error("expected error")
			}
			for i := range out {
				if out[i] != before[i] {
					t.Fatal("invalid decode changed output")
				}
			}
		})
	}
	if err := enc.Encode(other, pcm); err != nil {
		t.Fatal(err)
	}
	if err := control.Encode(good, pcm); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(other, good) {
		t.Error("invalid encode advanced state")
	}
	dec.Decode(out, good)
	decControl.Decode(otherOut, good)
	for i := range out {
		if out[i] != otherOut[i] {
			t.Fatal("invalid decode advanced state")
		}
	}
	var zero Encoder
	if err := zero.Encode(other, pcm); !errors.Is(err, ErrConfig) {
		t.Error("zero encoder accepted")
	}
	var zeroDecoder Decoder
	if _, err := zeroDecoder.Decode(out, good); !errors.Is(err, ErrConfig) {
		t.Error("zero decoder accepted")
	}
}

func FuzzDecode(f *testing.F) {
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
				selector := uint8(dt + 4*(pcm-sr))
				f.Add(selector, uint8(sr), []byte{})
				f.Add(selector, uint8(sr), bytes.Repeat([]byte{255}, 40))
				f.Add(selector, uint8(sr), bytes.Repeat([]byte{0}, 400))
				cfg := codecConfig(dt, sr, pcm)
				enc, err := NewEncoder(cfg)
				if err != nil {
					f.Fatal(err)
				}
				minimum, maximum, _ := cfg.FrameSizeRange()
				compressed := make([]byte, min(minimum+80, maximum))
				input := make([]int16, cfg.FrameSamples())
				for i := range input {
					input[i] = int16(codecSine16[(i/7)%16] / 2)
				}
				for i := 0; i < 4; i++ {
					if err := enc.Encode(compressed, input); err != nil {
						f.Fatal(err)
					}
				}
				f.Add(selector, uint8(sr), compressed)
			}
		}
	}
	f.Fuzz(func(t *testing.T, duration, rate uint8, frame []byte) {
		dt, sr := int(duration)%4, int(rate)%7
		if dt == 2 && sr >= 5 {
			return
		}
		if len(frame) > 1024 {
			return
		}
		limit := 5
		if sr >= 5 {
			limit = 7
		}
		pcm := sr + int(duration/4)%(limit-sr)
		cfg := codecConfig(dt, sr, pcm)
		d, err := NewDecoder(cfg)
		if err != nil {
			t.Fatal(err)
		}
		dst := make([]float32, cfg.FrameSamples())
		for i := 0; i < 4; i++ {
			data := frame
			if i == 1 || i == 3 {
				data = nil
			}
			_, err = d.DecodeFloat32(dst, data, 1)
			if err != nil && !errors.Is(err, ErrFrameSize) {
				t.Fatal(err)
			}
			for _, v := range dst {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < -1 || v > 1 {
					t.Fatalf("nonfinite or invalid PCM%v", v)
				}
			}
		}
	})
}

func TestCodecResetResamplingMatrix(t *testing.T) {
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
				t.Run(fmt.Sprintf("dt%d_sr%d_pcm%d", dt, sr, pcm), func(t *testing.T) {
					cfg := codecConfig(dt, sr, pcm)
					e, _ := NewEncoder(cfg)
					d, _ := NewDecoder(cfg)
					freshE, _ := NewEncoder(cfg)
					freshD, _ := NewDecoder(cfg)
					lo, hi, _ := cfg.FrameSizeRange()
					packet := make([]byte, (lo+hi)/2)
					expected := make([]byte, len(packet))
					out := make([]int16, cfg.FrameSamples())
					want := make([]int16, len(out))
					input := make([]int16, len(out))
					for i := range input {
						input[i] = int16(codecSine16[(i/5)%16] / 2)
					}
					for frame := 0; frame < 12; frame++ {
						if err := e.Encode(packet, input); err != nil {
							t.Fatal(err)
						}
						var compressed []byte
						if frame%3 == 0 {
							compressed = packet
						}
						if _, err := d.Decode(out, compressed); err != nil {
							t.Fatal(err)
						}
					}
					e.Reset()
					d.Reset()
					if err := e.Encode(packet, input); err != nil {
						t.Fatal(err)
					}
					if err := freshE.Encode(expected, input); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(packet, expected) {
						t.Fatal("encoder Reset retained history")
					}
					plc, err := d.Decode(out, packet)
					otherPLC, otherErr := freshD.Decode(want, packet)
					if err != nil || otherErr != nil || plc != otherPLC {
						t.Fatal("decoder Reset status differs")
					}
					for i := range out {
						if out[i] != want[i] {
							t.Fatal("decoder Reset retained history")
						}
					}
				})
			}
		}
	}
}

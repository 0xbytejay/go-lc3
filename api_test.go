// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

package lc3

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
)

func apiPCM(n, phase int) []int16 {
	p := make([]int16, n)
	for i := range p {
		p[i] = int16(((i+phase)*137)%60001 - 30000)
	}
	return p
}
func apiEncoder(t testing.TB, c Config) *Encoder {
	t.Helper()
	e, err := NewEncoder(c)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func apiDecoder(t testing.TB, c Config) *Decoder {
	t.Helper()
	d, err := NewDecoder(c)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func apiFrame(t testing.TB, e *Encoder, pcm []int16, n int) []byte {
	t.Helper()
	f := make([]byte, n)
	if err := e.Encode(f, pcm); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestConfigQueries(t *testing.T) {
	for _, v := range []struct {
		c                        Config
		samples, delay, min, max int
	}{
		{Config{SampleRate: 8000, FrameDuration: FrameDuration2M5}, 20, 20, 20, 400},
		{Config{SampleRate: 16000, FrameDuration: FrameDuration5M}, 80, 40, 20, 400},
		{Config{SampleRate: 24000, FrameDuration: FrameDuration7M5}, 180, 96, 20, 400},
		{Config{SampleRate: 48000, FrameDuration: FrameDuration10M}, 480, 120, 20, 400},
		{Config{SampleRate: 8000, PCMSampleRate: 48000, FrameDuration: FrameDuration7M5}, 360, 192, 20, 400},
		{Config{SampleRate: 48000, HighResolution: true, FrameDuration: FrameDuration2M5}, 120, 120, 27, 210},
		{Config{SampleRate: 96000, HighResolution: true, FrameDuration: FrameDuration2M5}, 240, 240, 31, 210},
		{Config{SampleRate: 48000, HighResolution: true, FrameDuration: FrameDuration5M}, 240, 120, 46, 375},
		{Config{SampleRate: 96000, HighResolution: true, FrameDuration: FrameDuration5M}, 480, 240, 54, 375},
		{Config{SampleRate: 48000, HighResolution: true, FrameDuration: FrameDuration10M}, 480, 120, 78, 625},
		{Config{SampleRate: 96000, HighResolution: true, FrameDuration: FrameDuration10M}, 960, 240, 93, 625},
		{Config{SampleRate: 48000, PCMSampleRate: 96000, HighResolution: true, FrameDuration: FrameDuration10M}, 960, 240, 78, 625},
	} {
		t.Run(fmt.Sprintf("%d_%s_hr%v_pcm%d", v.c.SampleRate, v.c.FrameDuration, v.c.HighResolution, v.c.PCMSampleRate), func(t *testing.T) {
			if err := v.c.Validate(); err != nil {
				t.Fatal(err)
			}
			if n := v.c.FrameSamples(); n != v.samples {
				t.Errorf("FrameSamples=%d want%d", n, v.samples)
			}
			if n := v.c.DelaySamples(); n != v.delay {
				t.Errorf("DelaySamples=%d want%d", n, v.delay)
			}
			lo, hi, err := v.c.FrameSizeRange()
			if err != nil || lo != v.min || hi != v.max {
				t.Errorf("FrameSizeRange=(%d,%d,%v), want(%d,%d,nil)", lo, hi, err, v.min, v.max)
			}
			maxInt := int(^uint(0) >> 1)
			for _, channels := range []int{1, 2, 8} {
				n, err := v.c.FrameBlockBytes(channels, 0)
				if err != nil || n != channels*v.min {
					t.Errorf("zero budget=%d,%v", n, err)
				}
				n, err = v.c.FrameBlockBytes(channels, maxInt)
				maxBudget := min(channels*v.max, int(int64(8*672000)*int64(v.c.FrameDuration/time.Microsecond)/8000000))
				if err != nil || n != maxBudget {
					t.Errorf("maximum budget=%d,%v", n, err)
				}
			}
			e, d := apiEncoder(t, v.c), apiDecoder(t, v.c)
			if e.FrameSamples() != v.samples || d.FrameSamples() != v.samples || e.DelaySamples() != v.delay || d.DelaySamples() != v.delay {
				t.Fatal("instance queries disagree with configuration")
			}
			c := v.c
			if c.PCMSampleRate == 0 {
				c.PCMSampleRate = c.SampleRate
			}
			if e.Config() != c || d.Config() != c {
				t.Fatal("Config did not normalize PCM sample rate")
			}
		})
	}
	for _, duration := range []time.Duration{FrameDuration2M5, FrameDuration5M, FrameDuration7M5, FrameDuration10M} {
		c := Config{SampleRate: 16000, FrameDuration: duration}
		for _, n := range []int{0, 1, 20, 40, 400, 100000, 2684354, int(^uint(0) >> 1)} {
			got, err := c.ResolveBitrate(n)
			if err != nil {
				t.Fatal(err)
			}
			want := int64(2147483647)
			us := int64(duration / time.Microsecond)
			if int64(n) <= int64(2147483647)*us/8000000 {
				want = (int64(n)*8000000 + us - 1) / us
			}
			if int64(got) != want {
				t.Errorf("ResolveBitrate %s/%d=%d want%d", duration, n, got, want)
			}
		}
		if _, err := c.ResolveBitrate(-1); !errors.Is(err, ErrFrameSize) {
			t.Errorf("negative frame size: %v", err)
		}
		for _, v := range []struct{ channels, rate int }{{0, 0}, {9, 0}, {1, -1}} {
			if _, err := c.FrameBlockBytes(v.channels, v.rate); !errors.Is(err, ErrConfig) {
				t.Errorf("invalid block %v: %v", v, err)
			}
		}
	}
}

func TestInvalidConfigurations(t *testing.T) {
	for _, c := range []Config{
		{}, {SampleRate: 16000}, {SampleRate: 16000, FrameDuration: -1},
		{SampleRate: 16000, FrameDuration: time.Millisecond}, {SampleRate: 44100, FrameDuration: FrameDuration10M},
		{SampleRate: 96000, FrameDuration: FrameDuration10M}, {SampleRate: 16000, FrameDuration: FrameDuration10M, HighResolution: true},
		{SampleRate: 48000, FrameDuration: FrameDuration7M5, HighResolution: true},
		{SampleRate: 48000, PCMSampleRate: 16000, FrameDuration: FrameDuration10M},
		{SampleRate: 16000, PCMSampleRate: -1, FrameDuration: FrameDuration10M},
	} {
		if !errors.Is(c.Validate(), ErrConfig) {
			t.Errorf("Validate accepted %+v", c)
		}
		if e, err := NewEncoder(c); e != nil || !errors.Is(err, ErrConfig) {
			t.Errorf("NewEncoder(%+v)=%v,%v", c, e, err)
		}
		if d, err := NewDecoder(c); d != nil || !errors.Is(err, ErrConfig) {
			t.Errorf("NewDecoder(%+v)=%v,%v", c, d, err)
		}
		if c.FrameSamples() != 0 || c.DelaySamples() != 0 {
			t.Errorf("invalid queries nonzero: %+v", c)
		}
		if _, err := c.FrameBytes(32000); !errors.Is(err, ErrConfig) {
			t.Errorf("FrameBytes invalid config: %v", err)
		}
		if _, _, err := c.FrameSizeRange(); !errors.Is(err, ErrConfig) {
			t.Errorf("FrameSizeRange invalid config: %v", err)
		}
	}
	var e Encoder
	var d Decoder
	e.Reset()
	d.Reset()
	for _, enc := range []*Encoder{nil, &e} {
		if err := enc.Encode(make([]byte, 40), make([]int16, 160)); !errors.Is(err, ErrConfig) {
			t.Errorf("invalid encoder=%v", err)
		}
	}
	for _, dec := range []*Decoder{nil, &d} {
		if _, err := dec.Decode(make([]int16, 160), nil); !errors.Is(err, ErrConfig) {
			t.Errorf("invalid decoder=%v", err)
		}
	}
}

func TestEncoderInvalidInputDoesNotAdvanceState(t *testing.T) {
	c := Config{SampleRate: 16000, FrameDuration: FrameDuration10M}
	ns := c.FrameSamples()
	maxInt := int(^uint(0) >> 1)
	cases := []struct {
		name string
		want error
		call func(*Encoder, []byte) error
	}{
		{"short_frame", ErrFrameSize, func(e *Encoder, f []byte) error { return e.Encode(f[:19], apiPCM(ns, 2)) }},
		{"large_frame", ErrFrameSize, func(e *Encoder, f []byte) error { return e.Encode(make([]byte, 401), apiPCM(ns, 2)) }},
		{"short_pcm", ErrPCM, func(e *Encoder, f []byte) error { return e.Encode(f, apiPCM(ns-1, 2)) }},
		{"zero_stride", ErrPCM, func(e *Encoder, f []byte) error { return e.EncodeS16(f, apiPCM(ns, 2), 0) }},
		{"negative_stride", ErrPCM, func(e *Encoder, f []byte) error { return e.EncodeS16(f, apiPCM(ns, 2), -1) }},
		{"huge_stride", ErrPCM, func(e *Encoder, f []byte) error { return e.EncodeS16(f, apiPCM(ns, 2), maxInt) }},
		{"short_packed", ErrPCM, func(e *Encoder, f []byte) error { return e.EncodeS24LE(f, make([]byte, ns*3-1), 1) }},
		{"large_packed_stride", ErrPCM, func(e *Encoder, f []byte) error { return e.EncodeS24LE(f, make([]byte, ns*3), maxInt) }},
		{"unknown_format", ErrPCM, func(e *Encoder, f []byte) error { return e.EncodePCM(f, make([]byte, ns*4), PCMFormat(255), 1) }},
		{"bad_s24_last", ErrPCM, func(e *Encoder, f []byte) error {
			p := make([]int32, ns)
			p[ns-1] = 8388608
			return e.EncodeS24(f, p, 1)
		}},
		{"bad_s24_first", ErrPCM, func(e *Encoder, f []byte) error { p := make([]int32, ns); p[0] = -8388609; return e.EncodeS24(f, p, 1) }},
		{"nan_float_last", ErrPCM, func(e *Encoder, f []byte) error {
			p := make([]float32, ns)
			p[ns-1] = float32(math.NaN())
			return e.EncodeFloat32(f, p, 1)
		}},
		{"infinite_float", ErrPCM, func(e *Encoder, f []byte) error {
			p := make([]float32, ns)
			p[0] = float32(math.Inf(1))
			return e.EncodeFloat32(f, p, 1)
		}},
		{"out_of_range_float", ErrPCM, func(e *Encoder, f []byte) error {
			p := make([]float32, ns)
			p[ns-1] = math.Float32frombits(0x3f800001)
			return e.EncodeFloat32(f, p, 1)
		}},
		{"bad_s24_bytes", ErrPCM, func(e *Encoder, f []byte) error {
			p := make([]byte, ns*4)
			binary.LittleEndian.PutUint32(p[(ns-1)*4:], 8388608)
			return e.EncodePCM(f, p, PCMFormatS24, 1)
		}},
		{"nan_float_bytes", ErrPCM, func(e *Encoder, f []byte) error {
			p := make([]byte, ns*4)
			binary.LittleEndian.PutUint32(p[(ns-1)*4:], 0x7fc00000)
			return e.EncodePCM(f, p, PCMFormatFloat32, 1)
		}},
	}
	for _, v := range cases {
		t.Run(v.name, func(t *testing.T) {
			e, want := apiEncoder(t, c), apiEncoder(t, c)
			apiFrame(t, e, apiPCM(ns, 1), 40)
			apiFrame(t, want, apiPCM(ns, 1), 40)
			dst := bytes.Repeat([]byte{0xa5}, 40)
			if err := v.call(e, dst); !errors.Is(err, v.want) {
				t.Fatalf("error %v want %v", err, v.want)
			}
			if !bytes.Equal(dst, bytes.Repeat([]byte{0xa5}, 40)) {
				t.Fatal("invalid input modified output")
			}
			got, next := apiFrame(t, e, apiPCM(ns, 3), 40), apiFrame(t, want, apiPCM(ns, 3), 40)
			if !bytes.Equal(got, next) {
				t.Fatal("invalid call changed stream history")
			}
		})
	}
}

func TestDecoderInvalidInputDoesNotAdvanceState(t *testing.T) {
	c := Config{SampleRate: 16000, FrameDuration: FrameDuration10M}
	ns := c.FrameSamples()
	e := apiEncoder(t, c)
	f1, f2 := apiFrame(t, e, apiPCM(ns, 1), 40), apiFrame(t, e, apiPCM(ns, 2), 40)
	maxInt := int(^uint(0) >> 1)
	for _, v := range []struct {
		name string
		want error
		call func(*Decoder, []int16) (bool, error)
	}{
		{"empty_non_nil_frame", ErrFrameSize, func(d *Decoder, p []int16) (bool, error) { return d.Decode(p, []byte{}) }},
		{"short_frame", ErrFrameSize, func(d *Decoder, p []int16) (bool, error) { return d.Decode(p, f1[:19]) }},
		{"large_frame", ErrFrameSize, func(d *Decoder, p []int16) (bool, error) { return d.Decode(p, make([]byte, 401)) }},
		{"short_output", ErrPCM, func(d *Decoder, p []int16) (bool, error) { return d.Decode(p[:ns-1], f1) }},
		{"zero_stride", ErrPCM, func(d *Decoder, p []int16) (bool, error) { return d.DecodeS16(p, f1, 0) }},
		{"negative_stride", ErrPCM, func(d *Decoder, p []int16) (bool, error) { return d.DecodeS16(p, f1, -1) }},
		{"huge_stride", ErrPCM, func(d *Decoder, p []int16) (bool, error) { return d.DecodeS16(p, f1, maxInt) }},
		{"short_packed", ErrPCM, func(d *Decoder, p []int16) (bool, error) { return d.DecodeS24LE(make([]byte, ns*3-1), f1, 1) }},
		{"huge_packed_stride", ErrPCM, func(d *Decoder, p []int16) (bool, error) { return d.DecodeS24LE(make([]byte, ns*3), f1, maxInt) }},
		{"unknown_format", ErrPCM, func(d *Decoder, p []int16) (bool, error) {
			return d.DecodePCM(make([]byte, ns*4), f1, PCMFormat(255), 1)
		}},
	} {
		t.Run(v.name, func(t *testing.T) {
			d, want := apiDecoder(t, c), apiDecoder(t, c)
			scratch := make([]int16, ns)
			if _, err := d.Decode(scratch, f1); err != nil {
				t.Fatal(err)
			}
			if _, err := want.Decode(scratch, f1); err != nil {
				t.Fatal(err)
			}
			p := make([]int16, ns)
			for i := range p {
				p[i] = 12345
			}
			before := append([]int16(nil), p...)
			if concealed, err := v.call(d, p); !errors.Is(err, v.want) || concealed {
				t.Fatalf("got %v,%v want false,%v", concealed, err, v.want)
			}
			if !reflect.DeepEqual(before, p) {
				t.Fatal("invalid call modified PCM output")
			}
			a, b := make([]int16, ns), make([]int16, ns)
			ca, ea := d.Decode(a, f2)
			cb, eb := want.Decode(b, f2)
			if ca != cb || ea != nil || eb != nil || !reflect.DeepEqual(a, b) {
				t.Fatal("invalid call changed decoder state")
			}
		})
	}
	hr := Config{SampleRate: 96000, HighResolution: true, FrameDuration: FrameDuration10M}
	hd := apiDecoder(t, hr)
	pcm := make([]int16, hd.FrameSamples())
	if _, err := hd.Decode(pcm, make([]byte, 20)); err != nil {
		t.Fatalf("HR decoder must accept normal minimum length for fallback: %v", err)
	}
}

func TestCodecReset(t *testing.T) {
	for _, c := range []Config{{SampleRate: 16000, FrameDuration: FrameDuration10M}, {SampleRate: 48000, PCMSampleRate: 96000, HighResolution: true, FrameDuration: FrameDuration2M5}} {
		t.Run(fmt.Sprint(c), func(t *testing.T) {
			e, wantE := apiEncoder(t, c), apiEncoder(t, c)
			d, wantD := apiDecoder(t, c), apiDecoder(t, c)
			n, _ := c.FrameBytes(64000)
			p := apiPCM(c.FrameSamples(), 1)
			initial := apiFrame(t, wantE, p, n)
			for i := 0; i < 7; i++ {
				f := apiFrame(t, e, apiPCM(c.FrameSamples(), i), n)
				if _, err := d.Decode(make([]int16, c.FrameSamples()), f); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := d.Decode(make([]int16, c.FrameSamples()), nil); err != nil {
				t.Fatal(err)
			}
			e.Reset()
			d.Reset()
			if got := apiFrame(t, e, p, n); !bytes.Equal(got, initial) {
				t.Fatal("encoder Reset did not reproduce fresh state")
			}
			a, b := make([]int16, c.FrameSamples()), make([]int16, c.FrameSamples())
			ca, ea := d.Decode(a, initial)
			cb, eb := wantD.Decode(b, initial)
			if ca != cb || ea != nil || eb != nil || !reflect.DeepEqual(a, b) {
				t.Fatal("decoder Reset did not reproduce fresh state")
			}
			d.Reset()
			wantD.Reset()
			for i := 0; i < 5; i++ {
				ca, ea = d.Decode(a, nil)
				cb, eb = wantD.Decode(b, nil)
				if !ca || !cb || ea != nil || eb != nil || !reflect.DeepEqual(a, b) {
					t.Fatal("PLC state not reset")
				}
			}
		})
	}
}

func TestPCMFormatAndStride(t *testing.T) {
	c := Config{SampleRate: 16000, FrameDuration: FrameDuration10M}
	ns := c.FrameSamples()
	pcm := apiPCM(ns, 3)
	baseline := apiFrame(t, apiEncoder(t, c), pcm, 40)
	const stride = 3
	s16 := make([]int16, (ns-1)*stride+1)
	s24 := make([]int32, len(s16))
	floats := make([]float32, len(s16))
	for i := 0; i < ns; i++ {
		s16[i*stride] = pcm[i]
		s24[i*stride] = int32(pcm[i]) << 8
		floats[i*stride] = float32(pcm[i]) / 32768
	}
	for _, v := range []struct {
		name string
		f    func(*Encoder, []byte) error
	}{
		{"s16", func(e *Encoder, dst []byte) error { return e.EncodeS16(dst, s16, stride) }},
		{"s24", func(e *Encoder, dst []byte) error { return e.EncodeS24(dst, s24, stride) }},
		{"float32", func(e *Encoder, dst []byte) error { return e.EncodeFloat32(dst, floats, stride) }},
	} {
		t.Run(v.name, func(t *testing.T) {
			dst := make([]byte, 40)
			if err := v.f(apiEncoder(t, c), dst); err != nil || !bytes.Equal(dst, baseline) {
				t.Fatalf("stride output mismatch: %v", err)
			}
		})
	}
	for _, format := range []PCMFormat{PCMFormatS16, PCMFormatS24, PCMFormatS24LE, PCMFormatFloat32} {
		t.Run(fmt.Sprint(format), func(t *testing.T) {
			width := format.width()
			p := bytes.Repeat([]byte{0xa5}, width*((ns-1)*stride+1))
			for i, s := range pcm {
				q := p[i*stride*width:]
				switch format {
				case PCMFormatS16:
					binary.LittleEndian.PutUint16(q, uint16(s))
				case PCMFormatS24:
					binary.LittleEndian.PutUint32(q, uint32(int32(s)<<8))
				case PCMFormatS24LE:
					v := uint32(int32(s) << 8)
					q[0] = byte(v)
					q[1] = byte(v >> 8)
					q[2] = byte(v >> 16)
				case PCMFormatFloat32:
					binary.LittleEndian.PutUint32(q, math.Float32bits(float32(s)/32768))
				}
			}
			got := make([]byte, 40)
			if err := apiEncoder(t, c).EncodePCM(got, p, format, stride); err != nil || !bytes.Equal(got, baseline) {
				t.Fatalf("EncodePCM format %d mismatch: %v", format, err)
			}
			bytesOut := bytes.Repeat([]byte{0xa5}, len(p)+11)
			concealed, err := apiDecoder(t, c).DecodePCM(bytesOut, baseline, format, stride)
			if concealed || err != nil {
				t.Fatalf("DecodePCM: %v,%v", concealed, err)
			}
			selected := make(map[int]bool)
			for i := 0; i < ns; i++ {
				for j := 0; j < width; j++ {
					selected[i*stride*width+j] = true
				}
			}
			for i, b := range bytesOut {
				if !selected[i] && b != 0xa5 {
					t.Fatalf("DecodePCM wrote unused byte%d", i)
				}
			}
			d := apiDecoder(t, c)
			switch format {
			case PCMFormatS16:
				out := make([]int16, (ns-1)*stride+1)
				for i := range out {
					out[i] = 12345
				}
				_, err = d.DecodeS16(out, baseline, stride)
				if err != nil {
					t.Fatal(err)
				}
				for i := 0; i < ns; i++ {
					if uint16(out[i*stride]) != binary.LittleEndian.Uint16(bytesOut[i*stride*width:]) {
						t.Fatal("S16 typed/byte output differs")
					}
				}
				for i, v := range out {
					if i%stride != 0 && v != 12345 {
						t.Fatal("S16 wrote another channel")
					}
				}
			case PCMFormatS24:
				out := make([]int32, (ns-1)*stride+1)
				for i := range out {
					out[i] = 1234567
				}
				_, err = d.DecodeS24(out, baseline, stride)
				if err != nil {
					t.Fatal(err)
				}
				for i := 0; i < ns; i++ {
					if uint32(out[i*stride]) != binary.LittleEndian.Uint32(bytesOut[i*stride*width:]) {
						t.Fatal("S24 typed/byte output differs")
					}
				}
				for i, v := range out {
					if i%stride != 0 && v != 1234567 {
						t.Fatal("S24 wrote another channel")
					}
				}
			case PCMFormatS24LE:
				out := bytes.Repeat([]byte{0xa5}, len(bytesOut))
				_, err = d.DecodeS24LE(out, baseline, stride)
				if err != nil || !bytes.Equal(out, bytesOut) {
					t.Fatal("packed typed/byte output differs", err)
				}
			case PCMFormatFloat32:
				out := make([]float32, (ns-1)*stride+1)
				for i := range out {
					out[i] = 123.5
				}
				_, err = d.DecodeFloat32(out, baseline, stride)
				if err != nil {
					t.Fatal(err)
				}
				for i := 0; i < ns; i++ {
					v := out[i*stride]
					if math.Float32bits(v) != binary.LittleEndian.Uint32(bytesOut[i*stride*width:]) || v < -1 || v > 1 || math.IsNaN(float64(v)) {
						t.Fatal("float32 typed/byte output differs or invalid")
					}
				}
				for i, v := range out {
					if i%stride != 0 && v != 123.5 {
						t.Fatal("Float32 wrote another channel")
					}
				}
			}
		})
	}
}

func TestPCMRoundingAndSaturation(t *testing.T) {
	for _, v := range []struct {
		x   float32
		s16 int16
		s24 int32
	}{
		{0, 0, 0}, {0.5, 1, 128}, {-0.5, -1, -128}, {0.001953125, 0, 1}, {-0.001953125, 0, -1},
		{32767, 32767, 8388352}, {32768, 32767, 8388607}, {-32768, -32768, -8388608},
		{1e20, 32767, 8388607}, {-1e20, -32768, -8388608}, {math.MaxFloat32, 32767, 8388607}, {-math.MaxFloat32, -32768, -8388608},
		{float32(math.Inf(1)), 32767, 8388607}, {float32(math.Inf(-1)), -32768, -8388608}, {float32(math.NaN()), 0, 0},
	} {
		if got := roundS16(v.x); got != v.s16 {
			t.Errorf("roundS16(%v)=%v want%v", v.x, got, v.s16)
		}
		if got := roundS24(v.x); got != v.s24 {
			t.Errorf("roundS24(%v)=%v want%v", v.x, got, v.s24)
		}
	}
}

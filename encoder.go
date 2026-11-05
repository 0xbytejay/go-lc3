// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: idiomatic Go state and checked typed PCM interfaces.

package lc3

import (
	"encoding/binary"
	"math"
)

type sideData struct {
	bw    int
	pitch bool
	ltpf  lc3_ltpf_data_t
	sns   lc3_sns_data_t
	tns   lc3_tns_data_t
	spec  lc3_spec_side_t
}

// Encoder retains one channel's analysis history. Its zero value is invalid.
// Use separate instances for channels and independent streams.
// Do not copy an Encoder after construction; its buffers would be shared.
type Encoder struct {
	config        Config
	dt, sr, srPCM int
	ready         bool
	attdet        lc3_attdet_analysis_t
	ltpf          lc3_ltpf_analysis_t
	spec          lc3_spec_analysis_t
	xt            []int16
	xs, xd        []float32
}

// NewEncoder allocates state for the selected stream.
func NewEncoder(c Config) (*Encoder, error) {
	dt, sr, pcm, err := c.indices()
	if err != nil {
		return nil, err
	}
	if c.PCMSampleRate == 0 {
		c.PCMSampleRate = c.SampleRate
	}
	return &Encoder{config: c, dt: dt, sr: sr, srPCM: pcm, ready: true,
		xt: make([]int16, lc3_nt(pcm)+lc3_ns(dt, pcm)), xs: make([]float32, lc3_ns(dt, pcm)), xd: make([]float32, lc3_nd(dt, pcm))}, nil
}

// Config returns the normalized stream configuration.
func (e *Encoder) Config() Config { return e.config }

// FrameSamples returns the number of input PCM samples per channel.
func (e *Encoder) FrameSamples() int { return len(e.xs) }

// DelaySamples returns the algorithmic delay at the PCM sample rate.
func (e *Encoder) DelaySamples() int { return e.config.DelaySamples() }

// Reset clears stream history while retaining allocated buffers.
func (e *Encoder) Reset() {
	clear(e.xt)
	clear(e.xs)
	clear(e.xd)
	e.attdet = lc3_attdet_analysis_t{}
	e.ltpf = lc3_ltpf_analysis_t{}
	e.spec = lc3_spec_analysis_t{}
}
func (e *Encoder) check(dst []byte, length, stride, unit int) error {
	if e == nil || !e.ready {
		return ErrConfig
	}
	if len(dst) < lc3_min_frame_bytes(e.dt, e.sr) || len(dst) > lc3_max_frame_bytes(e.dt, e.sr) {
		return ErrFrameSize
	}
	if !pcmFits(length, len(e.xs), stride, unit) {
		return ErrPCM
	}
	return nil
}
func pcmFits(length, samples, stride, unit int) bool {
	if samples <= 0 || stride <= 0 || unit <= 0 || length < unit {
		return false
	}
	// Division avoids overflow when an untrusted stride is very large.
	return samples-1 <= (length-unit)/unit/stride
}

// Encode encodes one mono frame of signed 16-bit PCM into exactly len(dst) bytes.
func (e *Encoder) Encode(dst []byte, pcm []int16) error { return e.EncodeS16(dst, pcm, 1) }

// EncodeS16 accepts interleaved signed 16-bit PCM. Start pcm at the selected
// channel and set stride to the channel count. Extra samples are ignored.
func (e *Encoder) EncodeS16(dst []byte, pcm []int16, stride int) error {
	if err := e.check(dst, len(pcm), stride, 1); err != nil {
		return err
	}
	nt := lc3_nt(e.srPCM)
	for i := range e.xs {
		s := pcm[i*stride]
		e.xt[nt+i] = s
		e.xs[i] = float32(s)
	}
	e.encode(dst)
	return nil
}

// EncodeS24 accepts sign-extended signed 24-bit samples in int32 words.
func (e *Encoder) EncodeS24(dst []byte, pcm []int32, stride int) error {
	if err := e.check(dst, len(pcm), stride, 1); err != nil {
		return err
	}
	for i := range e.xs {
		s := pcm[i*stride]
		if s < -8388608 || s > 8388607 {
			return ErrPCM
		}
	}
	nt := lc3_nt(e.srPCM)
	for i := range e.xs {
		s := pcm[i*stride]
		e.xt[nt+i] = int16(s >> 8)
		e.xs[i] = lc3_ldexpf(float32(s), -8)
	}
	e.encode(dst)
	return nil
}

// EncodeS24LE accepts signed 24-bit little-endian PCM packed in three bytes.
// stride counts samples (three-byte words), not bytes.
func (e *Encoder) EncodeS24LE(dst, pcm []byte, stride int) error {
	if err := e.check(dst, len(pcm), stride, 3); err != nil {
		return err
	}
	nt := lc3_nt(e.srPCM)
	for i := range e.xs {
		p := pcm[3*i*stride:]
		s := int32(uint32(p[0])<<8 | uint32(p[1])<<16 | uint32(p[2])<<24)
		e.xt[nt+i] = int16(s >> 16)
		e.xs[i] = lc3_ldexpf(float32(s), -16)
	}
	e.encode(dst)
	return nil
}

// EncodeFloat32 accepts finite normalized PCM in [-1,1]. Values outside that
// range are rejected before changing state.
func (e *Encoder) EncodeFloat32(dst []byte, pcm []float32, stride int) error {
	if err := e.check(dst, len(pcm), stride, 1); err != nil {
		return err
	}
	for i := range e.xs {
		s := pcm[i*stride]
		if s < -1 || s > 1 || math.IsNaN(float64(s)) {
			return ErrPCM
		}
	}
	nt := lc3_nt(e.srPCM)
	for i := range e.xs {
		s := lc3_ldexpf(pcm[i*stride], 15)
		e.xs[i] = s
		e.xt[nt+i] = int16(min(max(int32(s), -32768), 32767))
	}
	e.encode(dst)
	return nil
}
func (e *Encoder) encode(dst []byte) {
	var side sideData
	dt, sr, pcm := e.dt, e.sr, e.srPCM
	nt, ns := lc3_nt(pcm), len(e.xs)
	att := lc3_attdet_run(dt, pcm, len(dst), &e.attdet, e.xt)
	side.pitch = lc3_ltpf_analyse(dt, pcm, &e.ltpf, e.xt, &side.ltpf)
	copy(e.xt[:nt], e.xt[ns:ns+nt])
	var energy [lc3_MAX_BANDS]float32
	lc3_mdct_forward(dt, pcm, sr, e.xs, e.xd, e.xs)
	nn := lc3_energy_compute(dt, sr, e.xs, energy[:])
	if nn {
		lc3_ltpf_disable(&side.ltpf)
	}
	side.bw = lc3_bwdet_run(dt, sr, energy[:])
	lc3_sns_analyze(dt, sr, len(dst), energy[:], att, &side.sns, e.xs, e.xs)
	lc3_tns_analyze(dt, side.bw, nn, len(dst), &side.tns, e.xs)
	lc3_spec_analyze(dt, sr, len(dst), side.pitch, &side.tns, &e.spec, e.xs, &side.spec)
	var bits lc3_bits_t
	lc3_setup_bits(&bits, lc3_BITS_MODE_WRITE, dst, len(dst))
	lc3_bwdet_put_bw(&bits, sr, side.bw)
	lc3_spec_put_side(&bits, dt, sr, &side.spec)
	lc3_tns_put_data(&bits, &side.tns)
	lc3_put_bit(&bits, boolInt(side.pitch))
	lc3_sns_put_data(&bits, &side.sns)
	if side.pitch {
		lc3_ltpf_put_data(&bits, &side.ltpf)
	}
	lc3_spec_encode(&bits, dt, sr, side.bw, len(dst), &side.spec, e.xs)
	lc3_flush_bits(&bits)
}

// PCMFormat identifies the byte representation accepted by EncodePCM and
// DecodePCM. Multi-byte words are little endian; Float32 uses IEEE 754.
type PCMFormat uint8

const (
	PCMFormatS16 PCMFormat = iota
	PCMFormatS24
	PCMFormatS24LE
	PCMFormatFloat32
)

func (f PCMFormat) width() int {
	switch f {
	case PCMFormatS16:
		return 2
	case PCMFormatS24, PCMFormatFloat32:
		return 4
	case PCMFormatS24LE:
		return 3
	}
	return 0
}

// EncodePCM is the byte-buffer counterpart of the typed PCM methods. stride
// counts sample words. No intermediate allocation is needed.
func (e *Encoder) EncodePCM(dst, pcm []byte, format PCMFormat, stride int) error {
	width := format.width()
	if err := e.check(dst, len(pcm), stride, width); err != nil {
		return err
	}
	if format == PCMFormatS24LE {
		return e.EncodeS24LE(dst, pcm, stride)
	}
	// Validate all input before mutating history.
	for i := range e.xs {
		p := pcm[i*stride*width:]
		if format == PCMFormatS24 {
			s := int32(binary.LittleEndian.Uint32(p))
			if s < -8388608 || s > 8388607 {
				return ErrPCM
			}
		}
		if format == PCMFormatFloat32 {
			s := math.Float32frombits(binary.LittleEndian.Uint32(p))
			if s < -1 || s > 1 || math.IsNaN(float64(s)) {
				return ErrPCM
			}
		}
	}
	nt := lc3_nt(e.srPCM)
	for i := range e.xs {
		p := pcm[i*stride*width:]
		switch format {
		case PCMFormatS16:
			s := int16(binary.LittleEndian.Uint16(p))
			e.xt[nt+i] = s
			e.xs[i] = float32(s)
		case PCMFormatS24:
			s := int32(binary.LittleEndian.Uint32(p))
			e.xt[nt+i] = int16(s >> 8)
			e.xs[i] = lc3_ldexpf(float32(s), -8)
		case PCMFormatFloat32:
			s := lc3_ldexpf(math.Float32frombits(binary.LittleEndian.Uint32(p)), 15)
			e.xs[i] = s
			e.xt[nt+i] = int16(min(max(int32(s), -32768), 32767))
		}
	}
	e.encode(dst)
	return nil
}

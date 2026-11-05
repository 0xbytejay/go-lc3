// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: idiomatic Go state and checked typed PCM interfaces.

package lc3

import (
	"encoding/binary"
	"math"
)

// Decoder retains one channel's synthesis and packet-loss history.
// Its zero value is invalid. Use one instance per channel and stream.
// Do not copy a Decoder after construction; its buffers would be shared.
type Decoder struct {
	config              Config
	dt, sr, srPCM       int
	ready               bool
	ltpf                lc3_ltpf_synthesis_t
	plc                 lc3_plc_state_t
	x                   []float32
	xsOff, xdOff, xgOff int
}

// NewDecoder allocates synthesis history for the selected stream.
func NewDecoder(c Config) (*Decoder, error) {
	dt, sr, pcm, err := c.indices()
	if err != nil {
		return nil, err
	}
	if c.PCMSampleRate == 0 {
		c.PCMSampleRate = c.SampleRate
	}
	nh, ns, nd := lc3_nh(dt, pcm), lc3_ns(dt, pcm), lc3_nd(dt, pcm)
	d := &Decoder{config: c, dt: dt, sr: sr, srPCM: pcm, ready: true, x: make([]float32, nh+ns+nd+ns), xsOff: nh, xdOff: nh + ns, xgOff: nh + ns + nd}
	lc3_plc_reset(&d.plc)
	return d, nil
}

// Config returns the normalized stream configuration.
func (d *Decoder) Config() Config { return d.config }

// FrameSamples returns the number of output PCM samples per channel.
func (d *Decoder) FrameSamples() int { return d.config.FrameSamples() }

// DelaySamples returns algorithmic delay at the PCM sample rate.
func (d *Decoder) DelaySamples() int { return d.config.DelaySamples() }

// Reset clears stream history while retaining allocated buffers.
func (d *Decoder) Reset() {
	clear(d.x)
	d.xsOff = lc3_nh(d.dt, d.srPCM)
	d.ltpf = lc3_ltpf_synthesis_t{}
	lc3_plc_reset(&d.plc)
}
func (d *Decoder) check(frame []byte, length, stride, unit int) error {
	if d == nil || !d.ready {
		return ErrConfig
	}
	// Match liblc3 decode: normal minimum is accepted even for HR PLC fallback.
	if frame != nil && (len(frame) < lc3_MIN_FRAME_BYTES || len(frame) > lc3_max_frame_bytes(d.dt, d.sr)) {
		return ErrFrameSize
	}
	if !pcmFits(length, d.FrameSamples(), stride, unit) {
		return ErrPCM
	}
	return nil
}

// Decode decodes a compressed frame to signed 16-bit mono PCM. A nil frame
// requests packet-loss concealment (PLC). A corrupt frame also produces PLC
// and returns concealed=true with no error, matching liblc3. Configuration or
// buffer errors leave state and output unchanged.
func (d *Decoder) Decode(dst []int16, frame []byte) (concealed bool, err error) {
	return d.DecodeS16(dst, frame, 1)
}

// DecodeS16 writes signed 16-bit PCM with the selected sample stride.
func (d *Decoder) DecodeS16(dst []int16, frame []byte, stride int) (bool, error) {
	if err := d.check(frame, len(dst), stride, 1); err != nil {
		return false, err
	}
	x, plc := d.decode(frame)
	for i, s := range x {
		dst[i*stride] = roundS16(s)
	}
	return plc, nil
}

// DecodeS24 writes sign-extended signed 24-bit samples in int32 words.
func (d *Decoder) DecodeS24(dst []int32, frame []byte, stride int) (bool, error) {
	if err := d.check(frame, len(dst), stride, 1); err != nil {
		return false, err
	}
	x, plc := d.decode(frame)
	for i, s := range x {
		dst[i*stride] = roundS24(s)
	}
	return plc, nil
}

// DecodeS24LE writes signed 24-bit little-endian PCM in three-byte words.
// stride counts sample words, not bytes.
func (d *Decoder) DecodeS24LE(dst, frame []byte, stride int) (bool, error) {
	if err := d.check(frame, len(dst), stride, 3); err != nil {
		return false, err
	}
	x, plc := d.decode(frame)
	for i, s := range x {
		v := uint32(roundS24(s))
		p := dst[3*i*stride:]
		p[0] = byte(v)
		p[1] = byte(v >> 8)
		p[2] = byte(v >> 16)
	}
	return plc, nil
}

// DecodeFloat32 writes normalized PCM clipped to [-1,1].
func (d *Decoder) DecodeFloat32(dst []float32, frame []byte, stride int) (bool, error) {
	if err := d.check(frame, len(dst), stride, 1); err != nil {
		return false, err
	}
	x, plc := d.decode(frame)
	for i, s := range x {
		dst[i*stride] = min(max(lc3_ldexpf(s, -15), -1), 1)
	}
	return plc, nil
}

// DecodePCM writes byte-encoded PCM. See PCMFormat and the typed methods.
func (d *Decoder) DecodePCM(dst, frame []byte, format PCMFormat, stride int) (bool, error) {
	width := format.width()
	if err := d.check(frame, len(dst), stride, width); err != nil {
		return false, err
	}
	if format == PCMFormatS24LE {
		return d.DecodeS24LE(dst, frame, stride)
	}
	x, plc := d.decode(frame)
	for i, s := range x {
		p := dst[i*stride*width:]
		switch format {
		case PCMFormatS16:
			binary.LittleEndian.PutUint16(p, uint16(roundS16(s)))
		case PCMFormatS24:
			binary.LittleEndian.PutUint32(p, uint32(roundS24(s)))
		case PCMFormatFloat32:
			binary.LittleEndian.PutUint32(p, math.Float32bits(min(max(lc3_ldexpf(s, -15), -1), 1)))
		}
	}
	return plc, nil
}
func roundS16(x float32) int16 {
	// Saturate before conversion: out-of-range float-to-int is implementation
	// dependent. Legal samples retain liblc3's half-away-from-zero rounding.
	if x >= 32767 {
		return 32767
	}
	if x <= -32768 {
		return -32768
	}
	if math.IsNaN(float64(x)) {
		return 0
	}
	if x >= 0 {
		return int16(int32(float32(x + 0.5)))
	}
	return int16(int32(float32(x - 0.5)))
}
func roundS24(x float32) int32 {
	// Clamp before exponent-bit scaling as well, so huge values cannot wrap.
	if x >= 32767.99609375 {
		return 8388607
	}
	if x <= -32768 {
		return -8388608
	}
	x = lc3_ldexpf(x, 8)
	if x >= 8388607 {
		return 8388607
	}
	if x <= -8388608 {
		return -8388608
	}
	if math.IsNaN(float64(x)) {
		return 0
	}
	if x >= 0 {
		return int32(float32(x + 0.5))
	}
	return int32(float32(x - 0.5))
}
func (d *Decoder) decode(frame []byte) ([]float32, bool) {
	var side sideData
	concealed := frame == nil || d.readFrame(frame, &side) < 0
	dt, sr, pcm := d.dt, d.sr, d.srPCM
	ns, ne := lc3_ns(dt, pcm), lc3_ne(dt, sr)
	xf := d.x[d.xsOff : d.xsOff+ns]
	xg := d.x[d.xgOff : d.xgOff+ns]
	xd := d.x[d.xdOff:]
	if !concealed {
		lc3_plc_suspend(&d.plc)
		lc3_tns_synthesize(dt, side.bw, &side.tns, xf)
		lc3_sns_synthesize(dt, sr, &side.sns, xf, xg)
		lc3_mdct_inverse(dt, pcm, sr, xg, xd, xf)
	} else {
		lc3_plc_synthesize(dt, sr, &d.plc, xg, xf)
		clear(xf[ne:])
		lc3_mdct_inverse(dt, pcm, sr, xf, xd, xf)
	}
	if !lc3_hr(sr) {
		var pitch *lc3_ltpf_data_t
		if !concealed && side.pitch {
			pitch = &side.ltpf
		}
		lc3_ltpf_synthesize(dt, pcm, len(frame), &d.ltpf, pitch, d.x[:d.xdOff], d.xsOff)
	}
	if d.xsOff < lc3_nh(dt, pcm) {
		d.xsOff += ns
	} else {
		d.xsOff = 0
	}
	return xf, concealed
}
func (d *Decoder) readFrame(frame []byte, side *sideData) int {
	var bits lc3_bits_t
	dt, sr := d.dt, d.sr
	xf := d.x[d.xsOff : d.xsOff+lc3_ns(dt, d.srPCM)]
	lc3_setup_bits(&bits, lc3_BITS_MODE_READ, frame, len(frame))
	if lc3_bwdet_get_bw(&bits, sr, &side.bw) < 0 {
		return -1
	}
	if lc3_spec_get_side(&bits, dt, sr, &side.spec) < 0 {
		return -1
	}
	if lc3_tns_get_data(&bits, dt, side.bw, len(frame), &side.tns) < 0 {
		return -1
	}
	side.pitch = lc3_get_bit(&bits) != 0
	if lc3_sns_get_data(&bits, &side.sns) < 0 {
		return -1
	}
	if side.pitch {
		lc3_ltpf_get_data(&bits, &side.ltpf)
	}
	if lc3_spec_decode(&bits, dt, sr, side.bw, len(frame), &side.spec, xf) < 0 {
		return -1
	}
	clear(xf[lc3_ne(dt, sr):lc3_ns(dt, sr)])
	return lc3_check_bits(&bits)
}

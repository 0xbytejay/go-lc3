// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type waveInfo struct {
	rate, channels, bits, width int
	samples                     uint32
}
type waveReader struct {
	info      waveInfo
	r         io.Reader
	remaining uint32
}

func readWave(r io.Reader) (*waveReader, error) {
	var hdr [12]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("read WAV header: %w", err)
	}
	if string(hdr[:4]) != "RIFF" || string(hdr[8:]) != "WAVE" {
		return nil, fmt.Errorf("expected a little-endian RIFF/WAVE file")
	}
	size := binary.LittleEndian.Uint32(hdr[4:])
	if size < 4 {
		return nil, fmt.Errorf("invalid RIFF size")
	}
	lr := &io.LimitedReader{R: r, N: int64(size) - 4}
	var info waveInfo
	formatSeen := false
	for {
		var ch [8]byte
		if _, err := io.ReadFull(lr, ch[:]); err != nil {
			return nil, fmt.Errorf("read WAV chunk: %w", err)
		}
		count := uint64(binary.LittleEndian.Uint32(ch[4:]))
		if int64(count) > lr.N {
			return nil, fmt.Errorf("WAV chunk exceeds RIFF size")
		}
		switch string(ch[:4]) {
		case "fmt ":
			if count < 16 {
				return nil, fmt.Errorf("short WAV format chunk")
			}
			var b [40]byte
			n := min(count, uint64(len(b)))
			if _, err := io.ReadFull(lr, b[:n]); err != nil {
				return nil, fmt.Errorf("read WAV format: %w", err)
			}
			if _, err := io.CopyN(io.Discard, lr, int64(count-n)); err != nil {
				return nil, err
			}
			tag := binary.LittleEndian.Uint16(b[:])
			channels := int(binary.LittleEndian.Uint16(b[2:]))
			rate := int(binary.LittleEndian.Uint32(b[4:]))
			align := int(binary.LittleEndian.Uint16(b[12:]))
			bits := binary.LittleEndian.Uint16(b[14:])
			if tag == 0xfffe {
				guid := [16]byte{1, 0, 0, 0, 0, 0, 16, 0, 128, 0, 0, 170, 0, 56, 155, 113}
				if count < 40 || binary.LittleEndian.Uint16(b[16:]) < 22 || binary.LittleEndian.Uint16(b[18:]) != bits || string(b[24:40]) != string(guid[:]) {
					return nil, fmt.Errorf("unsupported extensible WAV format")
				}
			} else if tag != 1 {
				return nil, fmt.Errorf("WAV input must use signed 16-bit or 24-bit PCM")
			}
			width := 0
			if channels > 0 {
				width = align / channels
			}
			if channels < 1 || channels > 8 || rate <= 0 || rate > 96000 || align != channels*width || !((bits == 16 && width == 2) || (bits == 24 && (width == 3 || width == 4))) || uint64(binary.LittleEndian.Uint32(b[8:])) != uint64(rate)*uint64(align) {
				return nil, fmt.Errorf("WAV input must use 1..8 channels of signed 16-bit or 24-bit PCM")
			}
			info = waveInfo{rate: rate, channels: channels, bits: int(bits), width: width}
			formatSeen = true
		case "data":
			if !formatSeen {
				return nil, fmt.Errorf("WAV data precedes its format")
			}
			if count%uint64(info.width*info.channels) != 0 {
				return nil, fmt.Errorf("WAV data contains a partial PCM sample")
			}
			info.samples = uint32(count / uint64(info.width*info.channels))
			return &waveReader{info: info, r: io.LimitReader(lr, int64(count)), remaining: info.samples}, nil
		default:
			if _, err := io.CopyN(io.Discard, lr, int64(count)); err != nil {
				return nil, fmt.Errorf("skip WAV chunk: %w", err)
			}
		}
		if count&1 != 0 {
			var pad [1]byte
			if _, err := io.ReadFull(lr, pad[:]); err != nil {
				return nil, fmt.Errorf("read WAV padding: %w", err)
			}
		}
	}
}
func (r *waveReader) readPCM(dst []byte) (int, error) {
	clear(dst)
	stride := r.info.channels * r.info.width
	frames := min(uint32(len(dst)/stride), r.remaining)
	n := int(frames) * stride
	if n == 0 {
		return 0, nil
	}
	if _, err := io.ReadFull(r.r, dst[:n]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return 0, fmt.Errorf("read WAV PCM: %w", err)
	}
	r.remaining -= frames
	return int(frames), nil
}
func writeWave(w io.Writer, rate, channels int, samples uint64) error {
	return writeWaveFormat(w, rate, channels, 16, 2, samples)
}
func writeWaveFormat(w io.Writer, rate, channels, bits, width int, samples uint64) error {
	n := samples * uint64(channels) * uint64(width)
	if n > math.MaxUint32-36 {
		return fmt.Errorf("output exceeds the RIFF/WAVE size limit")
	}
	var b [44]byte
	copy(b[:], "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+n))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], uint16(channels))
	binary.LittleEndian.PutUint32(b[24:], uint32(rate))
	binary.LittleEndian.PutUint32(b[28:], uint32(rate*channels*width))
	binary.LittleEndian.PutUint16(b[32:], uint16(channels*width))
	binary.LittleEndian.PutUint16(b[34:], uint16(bits))
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(n))
	return writeBytes(w, b[:])
}
func writeBytes(w io.Writer, b []byte) error {
	n, err := w.Write(b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}

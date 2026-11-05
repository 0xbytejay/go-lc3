// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0

// Package lc3file reads and writes the non-standard .lc3 container used by the
// liblc3 reference command-line tools. The container is distinct from raw LC3
// frames and from Bluetooth LE Audio transport framing.

package lc3file

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/0xbytejay/go-lc3"
)

const (
	// MaxChannels is the maximum number of independently coded channels.
	MaxChannels = 8
	// MaxBlockBytes is the largest supported frame block across all channels.
	MaxBlockBytes = MaxChannels * 625
)

// ErrInvalidFile indicates malformed or unsupported container data.
var ErrInvalidFile = errors.New("lc3file: invalid file")

// Header describes a reference-format LC3 file. Samples counts PCM samples per
// channel at SampleRate, excluding the algorithmic delay and padded tail.
// Bitrate is the total bitrate across all channels; it is advisory when reading.
type Header struct {
	SampleRate     int
	Bitrate        int
	Channels       int
	FrameDuration  time.Duration
	HighResolution bool
	Samples        uint32
}

// Config returns the codec configuration represented by the header.
func (h Header) Config() lc3.Config {
	return lc3.Config{SampleRate: h.SampleRate, FrameDuration: h.FrameDuration, HighResolution: h.HighResolution}
}

// Validate checks the header's duration, rate, channels, and metadata ranges.
func (h Header) Validate() error {
	if err := h.Config().Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}
	if h.Channels < 1 || h.Channels > MaxChannels {
		return fmt.Errorf("%w: channel count must be 1..%d", ErrInvalidFile, MaxChannels)
	}
	if h.Bitrate < 0 || h.Bitrate > math.MaxUint16*100 {
		return fmt.Errorf("%w: bitrate metadata is out of range", ErrInvalidFile)
	}
	return nil
}

// Reader reads frame blocks after parsing a container header. A Reader does not
// decode audio and does not own or close its underlying stream.
type Reader struct {
	r       io.Reader
	header  Header
	pending int
	failed  error
}

// NewReader reads and validates the header and skips any extension fields.
func NewReader(r io.Reader) (*Reader, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: nil reader", ErrInvalidFile)
	}
	var b [18]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return nil, fmt.Errorf("lc3file: read header: %w", err)
	}
	size := int(binary.LittleEndian.Uint16(b[2:]))
	if binary.LittleEndian.Uint16(b[:]) != 0xcc1c || size < 18 || size&1 != 0 {
		return nil, fmt.Errorf("%w: magic or header size", ErrInvalidFile)
	}
	if binary.LittleEndian.Uint16(b[12:]) != 0 {
		return nil, fmt.Errorf("%w: error protection is unsupported", ErrInvalidFile)
	}
	h := Header{
		SampleRate:    int(binary.LittleEndian.Uint16(b[4:])) * 100,
		Bitrate:       int(binary.LittleEndian.Uint16(b[6:])) * 100,
		Channels:      int(binary.LittleEndian.Uint16(b[8:])),
		FrameDuration: time.Duration(binary.LittleEndian.Uint16(b[10:])) * 10 * time.Microsecond,
		Samples:       binary.LittleEndian.Uint32(b[14:]),
	}
	if size >= 20 {
		var extension [2]byte
		if _, err := io.ReadFull(r, extension[:]); err != nil {
			return nil, fmt.Errorf("lc3file: read HR extension: %w", err)
		}
		h.HighResolution = binary.LittleEndian.Uint16(extension[:]) != 0
		size -= 2
	}
	if size > 18 {
		if _, err := io.CopyN(io.Discard, r, int64(size-18)); err != nil {
			return nil, fmt.Errorf("lc3file: read header extensions: %w", err)
		}
	}
	if err := h.Validate(); err != nil {
		return nil, err
	}
	return &Reader{r: r, header: h}, nil
}

// Header returns a copy of the parsed metadata.
func (r *Reader) Header() Header { return r.header }

// ReadBlock reads one frame block into dst and returns its length. Each channel
// receives n/channels bytes, with one extra byte for the first n%channels
// channels. A short destination returns io.ErrShortBuffer without consuming the
// payload; the caller may retry with a larger buffer. EOF before a new length
// prefix is io.EOF, while partial prefixes and payloads are io.ErrUnexpectedEOF.
func (r *Reader) ReadBlock(dst []byte) (int, error) {
	if r.failed != nil {
		return 0, r.failed
	}
	n := r.pending
	if n == 0 {
		var b [2]byte
		if _, err := io.ReadFull(r.r, b[:]); err != nil {
			r.failed = err
			return 0, err
		}
		n = int(binary.LittleEndian.Uint16(b[:]))
		if err := r.header.validateBlock(n); err != nil {
			r.failed = err
			return 0, err
		}
		r.pending = n
	}
	if len(dst) < n {
		return 0, io.ErrShortBuffer
	}
	if _, err := io.ReadFull(r.r, dst[:n]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		r.failed = err
		return 0, err
	}
	r.pending = 0
	return n, nil
}

// Writer writes a header and subsequent frame blocks. A Writer does not own or
// close its output. Callers are responsible for emitting enough blocks to
// represent Header.Samples plus the codec's algorithmic delay.
type Writer struct {
	w      io.Writer
	header Header
	failed error
}

// NewWriter validates and writes a reference-format header (18 bytes, or 20 in
// high-resolution mode). As in liblc3, bitrate metadata is rounded down to 100bps.
func NewWriter(w io.Writer, h Header) (*Writer, error) {
	if w == nil {
		return nil, fmt.Errorf("%w: nil writer", ErrInvalidFile)
	}
	if err := h.Validate(); err != nil {
		return nil, err
	}
	var b [20]byte
	size := 18
	if h.HighResolution {
		size = 20
		binary.LittleEndian.PutUint16(b[18:], 1)
	}
	binary.LittleEndian.PutUint16(b[:], 0xcc1c)
	binary.LittleEndian.PutUint16(b[2:], uint16(size))
	binary.LittleEndian.PutUint16(b[4:], uint16(h.SampleRate/100))
	binary.LittleEndian.PutUint16(b[6:], uint16(h.Bitrate/100))
	binary.LittleEndian.PutUint16(b[8:], uint16(h.Channels))
	binary.LittleEndian.PutUint16(b[10:], uint16(h.FrameDuration/(10*time.Microsecond)))
	binary.LittleEndian.PutUint32(b[14:], h.Samples)
	if err := writeAll(w, b[:size]); err != nil {
		return nil, err
	}
	return &Writer{w: w, header: h}, nil
}

// WriteBlock writes one complete multichannel frame block.
func (w *Writer) WriteBlock(block []byte) error {
	if w.failed != nil {
		return w.failed
	}
	if err := w.header.validateBlock(len(block)); err != nil {
		return err
	}
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], uint16(len(block)))
	if err := writeAll(w.w, b[:]); err != nil {
		w.failed = err
		return err
	}
	if err := writeAll(w.w, block); err != nil {
		w.failed = err
		return err
	}
	return nil
}

func (h Header) validateBlock(n int) error {
	maxFrame, err := h.Config().FrameBytes(math.MaxInt32)
	if err != nil {
		return err
	}
	// The reference decoder accepts 20-byte frames even in HR mode.
	if n < h.Channels*20 || n > h.Channels*maxFrame {
		return fmt.Errorf("%w: frame block has %d bytes", ErrInvalidFile, n)
	}
	return nil
}
func writeAll(w io.Writer, b []byte) error {
	n, err := w.Write(b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}

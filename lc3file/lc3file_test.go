// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0

package lc3file

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"
)

func validHeader() Header {
	return Header{SampleRate: 16000, Bitrate: 64000, Channels: 2, FrameDuration: 10 * time.Millisecond, Samples: 163}
}
func headerBytes(t *testing.T, h Header) []byte {
	t.Helper()
	var b bytes.Buffer
	if _, err := NewWriter(&b, h); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestHeadersAndBlocks(t *testing.T) {
	for _, hr := range []bool{false, true} {
		h := validHeader()
		if hr {
			h.HighResolution = true
			h.SampleRate = 96000
			h.Bitrate = 400000
		}
		var b bytes.Buffer
		w, err := NewWriter(&b, h)
		if err != nil {
			t.Fatal(err)
		}
		size := 18
		if hr {
			size = 20
		}
		if b.Len() != size {
			t.Fatalf("header length=%d", b.Len())
		}
		count := 83
		if hr {
			count = 400
		}
		block := bytes.Repeat([]byte{0x43}, count)
		if err = w.WriteBlock(block); err != nil {
			t.Fatal(err)
		}
		r, err := NewReader(&b)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.Header(); got != h {
			t.Fatalf("header=%+v want %+v", got, h)
		}
		if _, err = r.ReadBlock(make([]byte, count-1)); !errors.Is(err, io.ErrShortBuffer) {
			t.Fatalf("small buffer: %v", err)
		}
		got := make([]byte, count)
		n, err := r.ReadBlock(got)
		if err != nil || n != count || !bytes.Equal(got, block) {
			t.Fatalf("block n=%d err=%v", n, err)
		}
		if _, err = r.ReadBlock(got); !errors.Is(err, io.EOF) {
			t.Fatalf("end: %v", err)
		}
	}
}
func TestExtendedHeader(t *testing.T) {
	h := validHeader()
	b := headerBytes(t, h)
	binary.LittleEndian.PutUint16(b[2:], 24)
	b = append(b, 0, 0, 0xa5, 0x62, 0x31, 0x71, 40, 0)
	b = append(b, make([]byte, 40)...)
	r, err := NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if r.Header() != h {
		t.Fatal("extension changed metadata")
	}
	n, err := r.ReadBlock(make([]byte, 100))
	if err != nil || n != 40 {
		t.Fatalf("block %d %v", n, err)
	}
}
func TestInvalidHeaders(t *testing.T) {
	tests := map[string]func([]byte) []byte{
		"magic":         func(b []byte) []byte { b[0] = 0; return b },
		"short size":    func(b []byte) []byte { b[2] = 16; return b },
		"odd size":      func(b []byte) []byte { b[2] = 19; return b },
		"no channels":   func(b []byte) []byte { b[8] = 0; return b },
		"many channels": func(b []byte) []byte { b[8] = 9; return b },
		"bad rate":      func(b []byte) []byte { b[4] = 1; b[5] = 0; return b },
		"bad duration":  func(b []byte) []byte { b[10] = 0; b[11] = 0; return b },
		"protected":     func(b []byte) []byte { b[12] = 1; return b },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			b := change(headerBytes(t, validHeader()))
			if _, err := NewReader(bytes.NewReader(b)); !errors.Is(err, ErrInvalidFile) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	b := headerBytes(t, validHeader())
	for n := 0; n < len(b); n++ {
		if _, err := NewReader(bytes.NewReader(b[:n])); err == nil {
			t.Fatalf("accepted header truncated to %d", n)
		}
	}
	binary.LittleEndian.PutUint16(b[2:], 20)
	if _, err := NewReader(bytes.NewReader(b)); err == nil {
		t.Fatal("missing extension")
	}
	if _, err := NewReader(nil); err == nil {
		t.Fatal("nil reader")
	}
	if _, err := NewWriter(nil, validHeader()); err == nil {
		t.Fatal("nil writer")
	}
}
func TestTruncatedAndInvalidBlocks(t *testing.T) {
	for _, payload := range [][]byte{{1}, {40, 0}, {40, 0, 1, 2}, {0, 0}, {39, 0}, {0xff, 0xff}} {
		b := append(headerBytes(t, validHeader()), payload...)
		r, err := NewReader(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.ReadBlock(make([]byte, MaxBlockBytes)); err == nil {
			t.Fatalf("accepted payload %x", payload)
		}
	}
}

type brokenWriter struct {
	calls int
	short bool
}

func (w *brokenWriter) Write(b []byte) (int, error) {
	w.calls++
	if w.short {
		return len(b) - 1, nil
	}
	return 0, io.ErrClosedPipe
}
func TestWriteFailures(t *testing.T) {
	for _, short := range []bool{false, true} {
		w := &brokenWriter{short: short}
		if _, err := NewWriter(w, validHeader()); err == nil {
			t.Fatal("accepted failed header write")
		}
	}
	var b bytes.Buffer
	w, err := NewWriter(&b, validHeader())
	if err != nil {
		t.Fatal(err)
	}
	if err = w.WriteBlock(make([]byte, 39)); err == nil {
		t.Fatal("accepted short frame block")
	}
	failed := &brokenWriter{}
	w.w = failed
	if err = w.WriteBlock(make([]byte, 80)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err = w.WriteBlock(make([]byte, 80)); !errors.Is(err, io.ErrClosedPipe) || failed.calls != 1 {
		t.Fatal("writer continued after partial output")
	}
}
func FuzzReader(f *testing.F) {
	h := Header{SampleRate: 16000, Channels: 1, FrameDuration: 10 * time.Millisecond}
	var b bytes.Buffer
	w, _ := NewWriter(&b, h)
	w.WriteBlock(make([]byte, 40))
	f.Add(b.Bytes())
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		r, err := NewReader(bytes.NewReader(b))
		if err != nil {
			return
		}
		dst := make([]byte, MaxBlockBytes)
		for i := 0; i < 256; i++ {
			n, err := r.ReadBlock(dst)
			if err != nil {
				return
			}
			if n < 20 || n > MaxBlockBytes {
				t.Fatalf("invalid length %d", n)
			}
		}
	})
}

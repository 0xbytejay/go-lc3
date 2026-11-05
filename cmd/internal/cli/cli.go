// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0
// Package cli implements the reference-container command-line tools.

package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xbytejay/go-lc3"
	"github.com/0xbytejay/go-lc3/lc3file"
)

// RunEncoder encodes signed 16-bit or 24-bit PCM WAV input into reference-format LC3.
// A bitrate is required; positional file names default to stdin and stdout.
// Named outputs are replaced only after successful completion. stdout may
// contain partial output when a later error occurs.
func RunEncoder(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("elc3", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bitrate := fs.Int("b", 0, "total bitrate in bits per second (required)")
	duration := fs.Float64("m", 10, "frame duration in milliseconds: 2.5, 5, 7.5, or 10")
	rate := fs.Int("r", 0, "encoded sample rate (defaults to input rate)")
	hr := fs.Bool("H", false, "enable LC3plus high-resolution mode")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: elc3 -b bitrate [-m milliseconds] [-r sample-rate] [-H] [input.wav [output.lc3]]")
		fs.PrintDefaults()
	}
	flags, paths, err := splitArgs(args, map[string]bool{"b": true, "m": true, "r": true, "H": false})
	if err != nil {
		return err
	}
	if err = fs.Parse(flags); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *bitrate <= 0 {
		return fmt.Errorf("-b must specify a positive total bitrate")
	}
	if math.IsNaN(*duration) || math.IsInf(*duration, 0) || *duration < 0 || *duration > 10 {
		return fmt.Errorf("invalid frame duration")
	}
	frameDuration := time.Duration(*duration * float64(time.Millisecond))
	if float64(frameDuration) != *duration*float64(time.Millisecond) {
		return fmt.Errorf("invalid frame duration")
	}
	streams, err := openStreams(paths, stdin, stdout)
	if err != nil {
		return err
	}
	defer streams.abort()
	in, out := streams.in, streams.out
	wav, err := readWave(in)
	if err != nil {
		return err
	}
	info := wav.info
	encodedRate := *rate
	if encodedRate == 0 {
		encodedRate = info.rate
	}
	cfg := lc3.Config{SampleRate: encodedRate, PCMSampleRate: info.rate, FrameDuration: frameDuration, HighResolution: *hr}
	if err = cfg.Validate(); err != nil {
		return err
	}
	encoders := make([]*lc3.Encoder, info.channels)
	for ch := range encoders {
		encoders[ch], err = lc3.NewEncoder(cfg)
		if err != nil {
			return err
		}
	}
	// liblc3's reference encoder selects the block budget using the PCM rate.
	budgetConfig := cfg
	budgetConfig.SampleRate = info.rate
	blockBytes, err := budgetConfig.FrameBlockBytes(info.channels, *bitrate)
	if err != nil {
		return err
	}
	actualRate, err := cfg.ResolveBitrate(blockBytes)
	if err != nil {
		return err
	}
	if actualRate != *bitrate {
		fmt.Fprintf(stderr, "Bitrate adjusted to %d bps\n", actualRate)
	}
	samples := uint64(info.samples) * uint64(encodedRate) / uint64(info.rate)
	if samples > math.MaxUint32 {
		return fmt.Errorf("encoded sample count exceeds container limit")
	}
	writer, err := lc3file.NewWriter(out, lc3file.Header{SampleRate: encodedRate, Bitrate: actualRate, Channels: info.channels, FrameDuration: frameDuration, HighResolution: *hr, Samples: uint32(samples)})
	if err != nil {
		return err
	}
	ns := encoders[0].FrameSamples()
	total := uint64(info.samples) + uint64(encoders[0].DelaySamples())
	pcm := make([]byte, ns*info.channels*info.width)
	format := lc3.PCMFormatS16
	if info.width == 3 {
		format = lc3.PCMFormatS24LE
	} else if info.width == 4 {
		format = lc3.PCMFormatS24
	}
	block := make([]byte, blockBytes)
	for processed := uint64(0); processed < total; processed += uint64(ns) {
		if _, err = wav.readPCM(pcm); err != nil {
			return err
		}
		off := 0
		for ch, encoder := range encoders {
			n := blockBytes / info.channels
			if ch < blockBytes%info.channels {
				n++
			}
			if err = encoder.EncodePCM(block[off:off+n], pcm[ch*info.width:], format, info.channels); err != nil {
				return err
			}
			off += n
		}
		if err = writer.WriteBlock(block); err != nil {
			return err
		}
	}
	return streams.commit()
}

// RunDecoder decodes reference-format LC3 input into signed 16-bit or 24-bit PCM WAV.
// Invalid compressed frames and truncated files are reported as errors.
// Named outputs are replaced only after successful completion. stdout may
// contain partial output when a later error occurs.
func RunDecoder(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dlc3", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bits := fs.Int("b", 16, "output PCM bit depth: 16 or 24")
	rate := fs.Int("r", 0, "output sample rate (defaults to encoded rate)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: dlc3 [-r sample-rate] [input.lc3 [output.wav]]")
		fs.PrintDefaults()
	}
	flags, paths, err := splitArgs(args, map[string]bool{"b": true, "r": true})
	if err != nil {
		return err
	}
	if err = fs.Parse(flags); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *bits != 16 && *bits != 24 {
		return fmt.Errorf("output PCM bit depth must be 16 or 24")
	}
	width := *bits / 8
	format := lc3.PCMFormatS16
	if *bits == 24 {
		format = lc3.PCMFormatS24LE
	}
	streams, err := openStreams(paths, stdin, stdout)
	if err != nil {
		return err
	}
	defer streams.abort()
	in, out := streams.in, streams.out
	reader, err := lc3file.NewReader(in)
	if err != nil {
		return err
	}
	h := reader.Header()
	cfg := h.Config()
	if *rate != 0 {
		cfg.PCMSampleRate = *rate
	}
	if err = cfg.Validate(); err != nil {
		return err
	}
	outputRate := cfg.PCMSampleRate
	if outputRate == 0 {
		outputRate = cfg.SampleRate
	}
	samples := uint64(h.Samples) * uint64(outputRate) / uint64(h.SampleRate)
	decoders := make([]*lc3.Decoder, h.Channels)
	for ch := range decoders {
		decoders[ch], err = lc3.NewDecoder(cfg)
		if err != nil {
			return err
		}
	}
	if err = writeWaveFormat(out, outputRate, h.Channels, *bits, width, samples); err != nil {
		return err
	}
	ns := decoders[0].FrameSamples()
	delay := decoders[0].DelaySamples()
	remaining := samples
	pcm := make([]byte, ns*h.Channels*width)
	block := make([]byte, lc3file.MaxBlockBytes)
	for remaining > 0 {
		n, err := reader.ReadBlock(block)
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return fmt.Errorf("read LC3 frame block: %w", err)
		}
		off := 0
		for ch, decoder := range decoders {
			count := n / h.Channels
			if ch < n%h.Channels {
				count++
			}
			concealed, err := decoder.DecodePCM(pcm[ch*width:], block[off:off+count], format, h.Channels)
			if err != nil {
				return err
			}
			if concealed {
				return fmt.Errorf("corrupt LC3 frame in channel %d", ch)
			}
			off += count
		}
		skip := min(delay, ns)
		delay -= skip
		count := min(uint64(ns-skip), remaining)
		if err = writeBytes(out, pcm[skip*h.Channels*width:(skip+int(count))*h.Channels*width]); err != nil {
			return err
		}
		remaining -= count
	}
	return streams.commit()
}

func splitArgs(args []string, options map[string]bool) (flags, paths []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			paths = append(paths, arg)
			continue
		}
		flags = append(flags, arg)
		key := strings.TrimLeft(arg, "-")
		if strings.Contains(key, "=") {
			continue
		}
		needs, known := options[key]
		if key == "h" || key == "help" {
			known = true
		}
		if !known {
			return nil, nil, fmt.Errorf("unknown option %s", arg)
		}
		if needs {
			i++
			if i >= len(args) {
				return nil, nil, fmt.Errorf("missing value for %s", arg)
			}
			flags = append(flags, args[i])
		}
	}
	if len(paths) > 2 {
		return nil, nil, fmt.Errorf("expected at most input and output file names")
	}
	return
}

// openedStreams stages named outputs in the destination directory. Only a
// successful codec operation may commit the new file; every other path aborts
// and removes the temporary file. stdout remains an ordinary streaming writer.
type openedStreams struct {
	in            io.Reader
	out           io.Writer
	input         *os.File // Owned only when opened from a positional input name.
	output        *os.File
	inputInfo     os.FileInfo // Also obtained for an actual stdin *os.File.
	targetInfo    os.FileInfo
	targetPath    string
	temporaryPath string
	committed     bool
	aborted       bool
}

func openStreams(paths []string, stdin io.Reader, stdout io.Writer) (*openedStreams, error) {
	s := &openedStreams{in: stdin, out: stdout}
	fail := func(err error) (*openedStreams, error) { s.abort(); return nil, err }
	if len(paths) > 0 && paths[0] != "-" {
		var err error
		s.input, err = os.Open(paths[0])
		if err != nil {
			return fail(err)
		}
		s.in = s.input
	}
	if len(paths) < 2 || paths[1] == "-" {
		return s, nil
	}
	var err error
	s.targetPath, err = filepath.Abs(paths[1])
	if err != nil {
		return fail(err)
	}
	if len(paths) > 0 && paths[0] != "-" {
		inputPath, err := filepath.Abs(paths[0])
		if err != nil {
			return fail(err)
		}
		if inputPath == s.targetPath {
			return fail(fmt.Errorf("input and output must be different files"))
		}
	}
	if f, ok := s.in.(*os.File); ok {
		s.inputInfo, err = f.Stat()
		if err != nil {
			return fail(fmt.Errorf("inspect input: %w", err))
		}
	}
	s.targetInfo, err = os.Lstat(s.targetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fail(err)
	}
	if err == nil {
		if !s.targetInfo.Mode().IsRegular() {
			return fail(fmt.Errorf("output must be a regular file, not a symlink or special file"))
		}
		if s.inputInfo != nil && os.SameFile(s.inputInfo, s.targetInfo) {
			return fail(fmt.Errorf("input and output must be different files"))
		}
	}
	s.output, err = os.CreateTemp(filepath.Dir(s.targetPath), "."+filepath.Base(s.targetPath)+".tmp-*")
	if err != nil {
		return fail(err)
	}
	s.temporaryPath = s.output.Name()
	// Preserve an existing regular file's permissions. New outputs retain the
	// owner-only permissions chosen by CreateTemp.
	if s.targetInfo != nil {
		if err = s.output.Chmod(s.targetInfo.Mode().Perm()); err != nil {
			return fail(err)
		}
	}
	s.out = s.output
	return s, nil
}

func (s *openedStreams) abort() {
	if !s.committed {
		s.aborted = true
	}
	if s.input != nil {
		s.input.Close()
		s.input = nil
	}
	if s.output != nil {
		s.output.Close()
		s.output = nil
	}
	if s.temporaryPath != "" {
		os.Remove(s.temporaryPath)
		s.temporaryPath = ""
	}
}

func (s *openedStreams) commit() error {
	if s.committed {
		return nil
	}
	if s.aborted {
		return fmt.Errorf("output operation was aborted")
	}
	if s.input != nil {
		err := s.input.Close()
		s.input = nil
		if err != nil {
			return fmt.Errorf("close input: %w", err)
		}
	}
	if s.output == nil {
		if s.temporaryPath != "" {
			return fmt.Errorf("staged output is already closed")
		}
		s.committed = true
		return nil
	}
	if err := s.output.Sync(); err != nil {
		return fmt.Errorf("flush output: %w", err)
	}
	err := s.output.Close()
	s.output = nil
	if err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	// Refuse replacement if the destination changed during processing. This
	// also catches a newly created symlink, special file, or input hard link.
	current, err := os.Lstat(s.targetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if (s.targetInfo == nil) != (current == nil) {
		return fmt.Errorf("output changed during processing")
	}
	if current != nil {
		if !current.Mode().IsRegular() || !os.SameFile(s.targetInfo, current) ||
			s.targetInfo.Size() != current.Size() || !s.targetInfo.ModTime().Equal(current.ModTime()) ||
			s.targetInfo.Mode() != current.Mode() {
			return fmt.Errorf("output changed during processing")
		}
		if s.inputInfo != nil && os.SameFile(s.inputInfo, current) {
			return fmt.Errorf("input and output must be different files")
		}
	}
	if err = os.Rename(s.temporaryPath, s.targetPath); err != nil {
		return fmt.Errorf("commit output: %w", err)
	}
	s.temporaryPath = ""
	s.committed = true
	return nil
}

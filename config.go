// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

package lc3

import (
	"errors"
	"fmt"
	"time"
)

const (
	FrameDuration2M5 = 2500 * time.Microsecond
	FrameDuration5M  = 5 * time.Millisecond
	FrameDuration7M5 = 7500 * time.Microsecond
	FrameDuration10M = 10 * time.Millisecond
	// UpstreamVersion identifies the liblc3 source used for this port.
	UpstreamVersion = "48bbd3eacd36e99a57317a0a4867002e0b09e183"
)

var (
	ErrConfig    = errors.New("lc3: invalid configuration")
	ErrPCM       = errors.New("lc3: invalid PCM buffer or stride")
	ErrFrameSize = errors.New("lc3: invalid frame size")
)

// Config selects the compressed stream and optional PCM sample rate.
// SampleRate is 8000, 16000, 24000, 32000, or 48000 Hz in normal mode;
// high resolution permits 48000 or 96000 Hz. FrameDuration is 2.5, 5, 7.5,
// or 10 ms, except that high resolution does not support 7.5 ms.
// PCMSampleRate defaults to SampleRate and may be a supported higher rate:
// the encoder downsamples and the decoder upsamples internally.
// Bitrate is chosen per frame by the length of the encoded buffer.
type Config struct {
	SampleRate     int
	FrameDuration  time.Duration
	HighResolution bool
	PCMSampleRate  int
}

func rateIndex(hz int, hr bool) int {
	if hr {
		switch hz {
		case 48000:
			return lc3_SRATE_48K_HR
		case 96000:
			return lc3_SRATE_96K_HR
		}
		return -1
	}
	switch hz {
	case 8000:
		return lc3_SRATE_8K
	case 16000:
		return lc3_SRATE_16K
	case 24000:
		return lc3_SRATE_24K
	case 32000:
		return lc3_SRATE_32K
	case 48000:
		return lc3_SRATE_48K
	}
	return -1
}
func (c Config) indices() (dt, sr, pcm int, err error) {
	switch c.FrameDuration {
	case FrameDuration2M5:
		dt = 0
	case FrameDuration5M:
		dt = 1
	case FrameDuration7M5:
		dt = 2
	case FrameDuration10M:
		dt = 3
	default:
		return 0, 0, 0, fmt.Errorf("%w: frame duration %s", ErrConfig, c.FrameDuration)
	}
	if c.HighResolution && dt == lc3_DT_7M5 {
		return 0, 0, 0, fmt.Errorf("%w: 7.5 ms high resolution", ErrConfig)
	}
	sr = rateIndex(c.SampleRate, c.HighResolution)
	hz := c.PCMSampleRate
	if hz == 0 {
		hz = c.SampleRate
	}
	pcm = rateIndex(hz, c.HighResolution)
	if sr < 0 || pcm < 0 || sr > pcm {
		return 0, 0, 0, fmt.Errorf("%w: sample rates %d/%d", ErrConfig, c.SampleRate, hz)
	}
	return
}

// Validate reports whether the configuration is supported.
func (c Config) Validate() error { _, _, _, err := c.indices(); return err }

// FrameSamples returns PCM samples per channel, or zero for an invalid Config.
func (c Config) FrameSamples() int {
	dt, _, sr, err := c.indices()
	if err != nil {
		return 0
	}
	return lc3_ns(dt, sr)
}

// DelaySamples returns algorithmic delay in PCM samples per channel.
func (c Config) DelaySamples() int {
	dt, _, sr, err := c.indices()
	if err != nil {
		return 0
	}
	return 2*lc3_nd(dt, sr) - lc3_ns(dt, sr)
}

// FrameBytes returns the clamped byte budget at bitrate bits per second.
// Zero selects the smallest supported frame; a very high bitrate selects the
// largest. The actual bitrate may differ because frames contain whole bytes.
func (c Config) FrameBytes(bitrate int) (int, error) { return c.FrameBlockBytes(1, bitrate) }

// FrameBlockBytes returns the total byte budget for 1 through 8 channels.
// bitrate is the total bitrate of all channels, as in liblc3.
func (c Config) FrameBlockBytes(channels, bitrate int) (int, error) {
	dt, sr, _, err := c.indices()
	if err != nil {
		return 0, err
	}
	if channels < 1 || channels > 8 || bitrate < 0 {
		return 0, fmt.Errorf("%w: channels or bitrate", ErrConfig)
	}
	bitrate = min(bitrate, 8*672000)
	return min(max(bitrate*(1+dt)/3200, channels*lc3_min_frame_bytes(dt, sr)), channels*lc3_max_frame_bytes(dt, sr)), nil
}

// ResolveBitrate returns the ceiling bitrate for a frame or frame block.
func (c Config) ResolveBitrate(nbytes int) (int, error) {
	dt, _, _, err := c.indices()
	if err != nil {
		return 0, err
	}
	if nbytes < 0 {
		return 0, ErrFrameSize
	}
	const maxInt32 = 2147483647
	if nbytes > maxInt32/3200 { // avoid overflow even when int is 32 bits
		// Work in 64 bits; all values beyond this point still fit int64 on 32-bit.
		if int64(nbytes) > int64(maxInt32)*(int64(dt)+1)/3200 {
			return maxInt32, nil
		}
	}
	if int64(nbytes) > int64(maxInt32)*(int64(dt)+1)/3200 {
		return maxInt32, nil
	}
	return int((int64(nbytes)*3200 + int64(dt)) / (1 + int64(dt))), nil
}

// FrameSizeRange returns minimum and maximum compressed bytes per channel.
// High-resolution minima include liblc3's half-rate fallback sizes.
func (c Config) FrameSizeRange() (minimum, maximum int, err error) {
	dt, sr, _, err := c.indices()
	if err != nil {
		return 0, 0, err
	}
	return lc3_min_frame_bytes(dt, sr), lc3_max_frame_bytes(dt, sr), nil
}

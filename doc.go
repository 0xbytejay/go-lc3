// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0

// Package lc3 implements the LC3 and LC3plus audio codecs in pure Go.
// Each Encoder or Decoder represents one channel of a continuous stream.
// Instances retain history across frames and must not be copied or used
// concurrently. Compressed frames are independent of transport and containers;
// the lc3file subpackage implements the reference tools' file format.
package lc3

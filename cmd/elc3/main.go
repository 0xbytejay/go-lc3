// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"github.com/0xbytejay/go-lc3/cmd/internal/cli"
	"os"
)

func main() {
	if err := cli.RunEncoder(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "elc3:", err)
		os.Exit(1)
	}
}

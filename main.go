// Copyright 2026 soe1hom-arch
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		verifyCRC bool
		verbose   bool
		output    string
	)

	flag.BoolVar(&verifyCRC, "crc", false, "Verify CRC32 checksum")
	flag.BoolVar(&verbose, "v", false, "Verbose output (shorthand)")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.StringVar(&output, "o", "", "Output file path (shorthand)")
	flag.StringVar(&output, "output", "", "Output file path")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <input_sparse_image> [output_raw_image]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Convert Android sparse image to raw image\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s system.img\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s system.img system_raw.img\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -v -o system_raw.img system.img\n", os.Args[0])
	}

	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	input := flag.Arg(0)

	if _, err := os.Stat(input); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: input file does not exist: %s\n", input)
		os.Exit(1)
	}

	// Output dari: -o flag > arg ke-2 > default (input.raw)
	if output == "" {
		if flag.NArg() >= 2 {
			output = flag.Arg(1)
		} else {
			output = input + ".raw"
		}
	}

	converter := NewConverter(input, output)
	converter.VerifyCRC = verifyCRC
	converter.Verbose = verbose

	if err := converter.Convert(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !verbose {
		fmt.Println(output)
	}
}

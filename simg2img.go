package main

import (
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"
)

// Converter handles the sparse-to-raw image conversion
type Converter struct {
	Input     string
	Output    string
	VerifyCRC bool
	Verbose   bool
}

// NewConverter creates a new converter instance
func NewConverter(input, output string) *Converter {
	return &Converter{
		Input:  input,
		Output: output,
	}
}

// Convert performs the sparse to raw image conversion
func (c *Converter) Convert() error {
	sparse, err := OpenSparseImage(c.Input)
	if err != nil {
		return fmt.Errorf("failed to open sparse image: %w", err)
	}
	defer sparse.Close()

	if c.Verbose {
		fmt.Printf("Sparse Image: %s\n", c.Input)
		fmt.Printf("  Header: magic=0x%08X version=%d.%d\n", sparse.Header.Magic, sparse.Header.MajorVersion, sparse.Header.MinorVersion)
		fmt.Printf("  Block Size: %d\n", sparse.Header.BlockSz)
		fmt.Printf("  Total Blocks: %d\n", sparse.Header.TotalBlocks)
		fmt.Printf("  Total Chunks: %d\n", sparse.Header.TotalChunks)
		fmt.Printf("  Output Size: %d bytes (%s)\n", sparse.Header.GetOutputSize(), formatSize(sparse.Header.GetOutputSize()))
	}

	// Open output file
	outFile, err := os.OpenFile(c.Output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer outFile.Close()

	// Pre-allocate output file for performance
	if err := outFile.Truncate(sparse.Header.GetOutputSize()); err == nil && c.Verbose {
		fmt.Println("  Pre-allocated output file")
	}

	written := int64(0)
	crcHasher := crc32.NewIEEE()

	for i := uint32(0); i < sparse.Header.TotalChunks; i++ {
		ch, err := sparse.readChunkHeader()
		if err != nil {
			return fmt.Errorf("failed to read chunk header %d: %w", i+1, err)
		}

		if c.Verbose {
			fmt.Printf("  Chunk %d: type=%s blocks=%d\n", i+1, chunkTypeName(ch.ChunkType), ch.ChunkSz)
		}

		blockSize := int64(sparse.Header.BlockSz)
		chunkBytes := int64(ch.ChunkSz) * blockSize

		switch ch.ChunkType {
		case ChunkTypeRaw:
			err = c.processRaw(sparse, ch, outFile, &written, crcHasher)

		case ChunkTypeFill:
			err = c.processFill(sparse, ch, blockSize, chunkBytes, outFile, &written, crcHasher)

		case ChunkTypeDontCare:
			err = c.processDontCare(chunkBytes, outFile, &written, crcHasher)

		case ChunkTypeCRC32:
			err = c.processCRC32(sparse)

		default:
			return fmt.Errorf("unknown chunk type 0x%04X at chunk %d", ch.ChunkType, i+1)
		}

		if err != nil {
			return fmt.Errorf("error processing chunk %d: %w", i+1, err)
		}
	}

	expectedBytes := int64(sparse.Header.TotalBlocks) * int64(sparse.Header.BlockSz)
	if expectedBytes != written {
		return fmt.Errorf("written bytes mismatch: wrote %d bytes, expected %d bytes",
			written, expectedBytes)
	}

	if c.VerifyCRC && sparse.Header.ImageChecksum != 0 {
		actualCRC := crcHasher.Sum32()
		if actualCRC != sparse.Header.ImageChecksum {
			return fmt.Errorf("CRC32 mismatch: computed 0x%08X, expected 0x%08X",
				actualCRC, sparse.Header.ImageChecksum)
		}
		if c.Verbose {
			fmt.Printf("  CRC32 verified: 0x%08X\n", actualCRC)
		}
	}

	if c.Verbose {
		fmt.Printf("Output: %s (%s)\n", c.Output, formatSize(written))
	}

	return nil
}

func (c *Converter) processRaw(sparse *SparseImage, ch *ChunkHeader, outFile *os.File, written *int64, crcHasher hash.Hash32) error {
	dataSize := int64(ch.ChunkSz) * int64(sparse.Header.BlockSz)
	expectedDataSize := sparse.Header.chunkDataSize(ch)

	if expectedDataSize != dataSize {
		return fmt.Errorf("RAW chunk size mismatch: data=%d expected=%d", expectedDataSize, dataSize)
	}

	n, err := io.CopyN(io.MultiWriter(outFile, crcHasher), sparse.file, dataSize)
	if err != nil {
		return fmt.Errorf("failed to copy RAW chunk: %w", err)
	}
	*written += n
	return nil
}

func (c *Converter) processFill(sparse *SparseImage, ch *ChunkHeader, blockSize, chunkBytes int64, outFile *os.File, written *int64, crcHasher hash.Hash32) error {
	// Read the 4-byte fill value
	fillBuf := make([]byte, 4)
	if _, err := io.ReadFull(sparse.file, fillBuf); err != nil {
		return fmt.Errorf("failed to read FILL value: %w", err)
	}

	if sparse.Header.chunkDataSize(ch) != 4 {
		return fmt.Errorf("FILL chunk data size is %d, expected 4", sparse.Header.chunkDataSize(ch))
	}

	// CRC: for each block, the fill pattern is the data
	for i := int64(0); i < chunkBytes; i += 4 {
		crcHasher.Write(fillBuf)
	}

	// Write fill data to output
	if err := writeFillPattern(outFile, fillBuf, chunkBytes); err != nil {
		return fmt.Errorf("failed to write FILL chunk: %w", err)
	}
	*written += chunkBytes

	return nil
}

func (c *Converter) processDontCare(chunkBytes int64, outFile *os.File, written *int64, crcHasher hash.Hash32) error {
	// CRC: DONTCARE is counted as zeros
	zero := make([]byte, 4096)
	remaining := chunkBytes
	for remaining > 0 {
		w := int64(len(zero))
		if remaining < w {
			w = remaining
		}
		crcHasher.Write(zero[:w])
		remaining -= w
	}

	// Try to seek (create sparse output)
	var seeker io.Seeker = outFile
	if _, err := seeker.Seek(chunkBytes, io.SeekCurrent); err == nil {
		*written += chunkBytes
		return nil
	}

	// Fallback: write zeros
	zero = make([]byte, chunkBytes)
	if _, err := outFile.Write(zero); err != nil {
		return fmt.Errorf("failed to write DONTCARE chunk: %w", err)
	}
	*written += chunkBytes

	return nil
}

func (c *Converter) processCRC32(sparse *SparseImage) error {
	// Read and discard 4 bytes of CRC32 data
	crcBuf := make([]byte, 4)
	if _, err := io.ReadFull(sparse.file, crcBuf); err != nil {
		return fmt.Errorf("failed to read CRC32 data: %w", err)
	}
	return nil
}

// writeFillPattern writes a fill pattern for the specified total size
func writeFillPattern(w io.Writer, pattern []byte, totalSize int64) error {
	patternLen := int64(len(pattern))
	if patternLen == 0 {
		return nil
	}

	buf := make([]byte, totalSize)
	for i := int64(0); i < totalSize; i += patternLen {
		copy(buf[i:], pattern)
	}
	_, err := w.Write(buf)
	return err
}

// chunkTypeName returns a human-readable name for a chunk type
func chunkTypeName(chunkType uint16) string {
	switch chunkType {
	case ChunkTypeRaw:
		return "RAW"
	case ChunkTypeFill:
		return "FILL"
	case ChunkTypeDontCare:
		return "DONTCARE"
	case ChunkTypeCRC32:
		return "CRC32"
	default:
		return fmt.Sprintf("UNKNOWN(0x%04X)", chunkType)
	}
}

// formatSize formats bytes to human readable
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

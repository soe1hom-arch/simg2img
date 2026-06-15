// Copyright 2026 soe1hom-arch
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Sparse image constants
const (
	SparseHeaderMagic = 0xED26FF3A

	SparseHeaderSize = 28
	ChunkHeaderSize  = 12

	ChunkTypeRaw     = 0xCAC1
	ChunkTypeFill    = 0xCAC2
	ChunkTypeDontCare = 0xCAC3
	ChunkTypeCRC32   = 0xCAC4
)

// SparseHeader represents the Android sparse image header (28 bytes)
type SparseHeader struct {
	Magic         uint32 // 0xED26FF3A
	MajorVersion  uint16
	MinorVersion  uint16
	FileHeaderSz  uint16 // 28 bytes for first revision
	ChunkHeaderSz uint16 // 12 bytes for first revision
	BlockSz       uint32 // block size in bytes, must be a multiple of 4
	TotalBlocks   uint32 // total blocks in the non-sparse output image
	TotalChunks   uint32 // total chunks in the sparse input image
	ImageChecksum uint32 // CRC32 of original data (optional)
}

// ChunkHeader represents a chunk in the sparse image (12 bytes)
type ChunkHeader struct {
	ChunkType uint16 // 0xCAC1 RAW, 0xCAC2 FILL, 0xCAC3 DONTCARE, 0xCAC4 CRC32
	Reserved  uint16
	ChunkSz   uint32 // chunk size in blocks (in output image)
	TotalSz   uint32 // total size in bytes (header + data)
}

// SparseImage represents a parsed sparse image
type SparseImage struct {
	Header   SparseHeader
	Filename string
	file     *os.File
}

// OpenSparseImage opens and parses a sparse image file
func OpenSparseImage(filename string) (*SparseImage, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}

	sp := &SparseImage{
		Filename: filename,
		file:     file,
	}

	if err := sp.parseHeader(); err != nil {
		file.Close()
		return nil, err
	}

	return sp, nil
}

func (sp *SparseImage) parseHeader() error {
	// Read full header
	buf := make([]byte, SparseHeaderSize)
	if _, err := io.ReadFull(sp.file, buf); err != nil {
		return fmt.Errorf("cannot read sparse header: %w", err)
	}

	sp.Header.Magic = binary.LittleEndian.Uint32(buf[0:4])
	if sp.Header.Magic != SparseHeaderMagic {
		return fmt.Errorf("invalid sparse image magic: 0x%08X (expected 0x%08X)", sp.Header.Magic, SparseHeaderMagic)
	}

	sp.Header.MajorVersion = binary.LittleEndian.Uint16(buf[4:6])
	sp.Header.MinorVersion = binary.LittleEndian.Uint16(buf[6:8])

	if sp.Header.MajorVersion != 1 {
		return fmt.Errorf("unsupported sparse image major version: %d (expected 1)", sp.Header.MajorVersion)
	}

	sp.Header.FileHeaderSz = binary.LittleEndian.Uint16(buf[8:10])
	sp.Header.ChunkHeaderSz = binary.LittleEndian.Uint16(buf[10:12])
	sp.Header.BlockSz = binary.LittleEndian.Uint32(buf[12:16])
	sp.Header.TotalBlocks = binary.LittleEndian.Uint32(buf[16:20])
	sp.Header.TotalChunks = binary.LittleEndian.Uint32(buf[20:24])
	sp.Header.ImageChecksum = binary.LittleEndian.Uint32(buf[24:28])

	// Validate block size
	if sp.Header.BlockSz == 0 || sp.Header.BlockSz%4 != 0 {
		return fmt.Errorf("invalid block size: %d (must be >0 and multiple of 4)", sp.Header.BlockSz)
	}

	// If file header size is larger than standard, seek past the extra bytes
	if sp.Header.FileHeaderSz > SparseHeaderSize {
		extra := int64(sp.Header.FileHeaderSz - SparseHeaderSize)
		if _, err := sp.file.Seek(extra, io.SeekCurrent); err != nil {
			return fmt.Errorf("cannot skip extended header: %w", err)
		}
	}

	return nil
}

// readChunkHeader reads a single chunk header
func (sp *SparseImage) readChunkHeader() (*ChunkHeader, error) {
	buf := make([]byte, ChunkHeaderSize)
	if _, err := io.ReadFull(sp.file, buf); err != nil {
		return nil, err
	}

	ch := &ChunkHeader{
		ChunkType: binary.LittleEndian.Uint16(buf[0:2]),
		Reserved:  binary.LittleEndian.Uint16(buf[2:4]),
		ChunkSz:   binary.LittleEndian.Uint32(buf[4:8]),
		TotalSz:   binary.LittleEndian.Uint32(buf[8:12]),
	}

	return ch, nil
}

// Close closes the underlying file
func (sp *SparseImage) Close() error {
	return sp.file.Close()
}

// GetOutputSize calculates the expected output size in bytes
func (sh *SparseHeader) GetOutputSize() int64 {
	return int64(sh.TotalBlocks) * int64(sh.BlockSz)
}

// chunkDataSize returns the data size for a chunk (total_sz - chunk_header_sz)
func (sh *SparseHeader) chunkDataSize(ch *ChunkHeader) int64 {
	return int64(ch.TotalSz) - int64(sh.ChunkHeaderSz)
}

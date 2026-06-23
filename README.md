# simg2img

Convert Android sparse image to raw image — written in Go.

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.18+-00ADD8)](go.mod)

## Overview

`simg2img` converts Android sparse image files (`.simg` / `.sparse`) into raw
image files that can be mounted or flashed. It is a pure Go reimplementation
based on the official [AOSP sparse format specification](https://android.googlesource.com/platform/system/core/+/refs/heads/main/libsparse/sparse_format.h).

### Supported Chunk Types

| Type | Code | Description |
|------|------|-------------|
| RAW | `0xCAC1` | Copy raw data blocks |
| FILL | `0xCAC2` | Fill blocks with a 4‑byte pattern |
| DONTCARE | `0xCAC3` | Skip blocks (output as sparse holes or zeros) |
| CRC32 | `0xCAC4` | CRC32 checksum verification (optional) |

### Features

- ✅ Supports **all** chunk types including FILL
- ✅ CRC32 verification (`-crc` flag)
- ✅ Verbose mode with per‑chunk details (`-v`)
- ✅ Pre‑allocation for fast writing
- ✅ Output size validation
- ✅ Clean CLI with `-o`, `-v`, `-crc` flags

## Installation

### From source

```bash
# Prerequisites
sudo apt install golang-go  # Debian/Ubuntu
# or: brew install go       # macOS

# Build
git clone https://github.com/soe1hom-arch/simg2img.git
cd simg2img
go build -o simg2img .
sudo cp simg2img /usr/local/bin/
```

### Using Go install

```bash
go install github.com/soe1hom-arch/simg2img@latest
```

### Download binary

Download from [GitHub Releases](https://github.com/soe1hom-arch/simg2img/releases).

```bash
# Linux AMD64 example
wget https://github.com/soe1hom-arch/simg2img/releases/download/v1.0.0/simg2img-linux-amd64
chmod +x simg2img-linux-amd64
sudo mv simg2img-linux-amd64 /usr/local/bin/simg2img
```

## Usage

```bash
simg2img [options] <input_sparse_image>
```

### Options

| Flag | Description |
|------|-------------|
| `-o`, `-output` | Output file path (default: `<input>.raw`) |
| `-v`, `-verbose` | Show detailed chunk‑by‑chunk processing |
| `-crc` | Enable CRC32 checksum verification |

### Examples

```bash
# Basic conversion
simg2img system.img

# Custom output path
simg2img -o system_raw.img system.img

# Verbose mode
simg2img -v system.img

# With CRC32 verification
simg2img -crc system.img
```

## Credits

- **soe1hom-arch** — Go implementation and maintenance
- This project is a Go port of the `simg2img` utility from the
  [Android Open Source Project (AOSP)](https://android.googlesource.com/platform/system/core/+/refs/heads/main/libsparse/sparse_format.h).
- [AOSP sparse format](https://android.googlesource.com/platform/system/core/+/refs/heads/main/libsparse/sparse_format.h) — Sparse image format specification

## License

Apache License 2.0 — see [LICENSE](LICENSE).

// genfile/internal/adapters/gif/generator.go
package gif

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
	"github.com/hailam/genfile/internal/utils"
)

func init() {
	factory.RegisterGenerator(ports.FileTypeGIF, New())
}

type GifGenerator struct{}

func New() ports.FileGenerator {
	return &GifGenerator{}
}

// Generate creates a minimal, single-color GIF file. Padding to exact size is tricky
// and might rely on comment extensions or adjusting image dimensions slightly.
// This version focuses on creating a *valid* minimal GIF and pads simply.
func (g *GifGenerator) Generate(path string, targetSize int64) error {
	if targetSize < 0 {
		targetSize = 0
	}

	// --- Minimal GIF Structure ---
	var buf bytes.Buffer

	// 1. Header ("GIF89a") - 6 bytes
	buf.WriteString("GIF89a")

	// 2. Logical Screen Descriptor - 7 bytes
	width := uint16(1) // Minimal 1x1 pixel image
	height := uint16(1)
	packedFields := byte(0x80) // Use global color table (1), 1 bit color depth (000) -> 10000000
	bgColorIndex := byte(0)
	pixelAspectRatio := byte(0) // Standard
	binary.Write(&buf, binary.LittleEndian, width)
	binary.Write(&buf, binary.LittleEndian, height)
	buf.WriteByte(packedFields)
	buf.WriteByte(bgColorIndex)
	buf.WriteByte(pixelAspectRatio)

	// 3. Global Color Table (2 colors: black and white) - 6 bytes
	buf.Write([]byte{0x00, 0x00, 0x00}) // Color 0: Black
	buf.Write([]byte{0xFF, 0xFF, 0xFF}) // Color 1: White

	// 4. Image Descriptor - 10 bytes
	imageSeparator := byte(0x2C) // ','
	left := uint16(0)
	top := uint16(0)
	imgWidth := uint16(1)
	imgHeight := uint16(1)
	imgPackedFields := byte(0x00) // No local color table, not interlaced
	buf.WriteByte(imageSeparator)
	binary.Write(&buf, binary.LittleEndian, left)
	binary.Write(&buf, binary.LittleEndian, top)
	binary.Write(&buf, binary.LittleEndian, imgWidth)
	binary.Write(&buf, binary.LittleEndian, imgHeight)
	buf.WriteByte(imgPackedFields)

	// 5. Image Data (LZW minimum code size 2, standard minimal stream) - 7 bytes
	lzwMinCodeSize := byte(2)
	buf.WriteByte(lzwMinCodeSize) // LZW Minimum Code Size = 2

	// Data sub-block 1: Clear Code, Data Code
	buf.WriteByte(2)    // Block Size = 2 bytes follow
	buf.WriteByte(0x04) // Clear Code (for code size 2)
	buf.WriteByte(0x01) // Data: Use color index 1

	// Data sub-block 2: End Of Information Code
	buf.WriteByte(1)    // Block Size = 1 byte follows
	buf.WriteByte(0x05) // EOI Code (for code size 2)

	// Data sub-block 3: Terminator
	buf.WriteByte(0) // Block Size = 0 (Terminator)

	// 6. GIF Trailer - 1 byte
	trailer := byte(0x3B) // ';'
	buf.WriteByte(trailer)
	// --- End Minimal Structure ---

	minimalData := buf.Bytes()
	minimalSize := int64(len(minimalData))

	if targetSize < minimalSize {
		fmt.Printf("Warning: Target GIF size %d smaller than minimal %d. Writing minimal.\n", targetSize, minimalSize)
		return os.WriteFile(path, minimalData, 0666)
	}

	// --- Padding ---
	// Simple padding by appending random bytes. NOTE: This makes the GIF invalid
	// after the trailer. A better approach uses Comment Extension blocks, but is more complex.
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	_, err = bw.Write(minimalData)
	if err != nil {
		return fmt.Errorf("failed to write minimal GIF data: %w", err)
	}

	paddingNeeded := targetSize - minimalSize
	if paddingNeeded > 0 {
		fmt.Printf("GIF Generator: Padding with %d raw bytes (Note: may invalidate strict GIF readers)\n", paddingNeeded)
		err = utils.WriteRandomBytes(bw, paddingNeeded) // Use existing util
		if err != nil {
			return fmt.Errorf("failed to write padding bytes: %w", err)
		}
	}

	return bw.Flush()
}

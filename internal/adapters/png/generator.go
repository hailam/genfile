package png

import (
	"bytes"
	cryptoRand "crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"math"
	"math/rand/v2"
	"os"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
)

func init() {
	factory.RegisterGenerator(ports.FileTypePNG, New()) //
}

type PngGenerator struct{}

func New() ports.FileGenerator {
	return &PngGenerator{}
}

func (g *PngGenerator) Generate(path string, targetSize int64) error {
	// 1) Roughly estimate pixels needed. For random noise PNG, compressed size ≈ raw RGBA size,
	//    so ~4 bytes/pixel. Compute side length of a square image.
	pixelsNeeded := float64(targetSize) / 4.0
	side := int(math.Sqrt(pixelsNeeded))
	if side < 1 {
		side = 1
	}

	// 2) Create noise image
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for i := range img.Pix {
		img.Pix[i] = byte(rand.IntN(256))
	}

	// 3) Encode to PNG buffer
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return err
	}
	data := buf.Bytes()
	if int64(len(data)) > targetSize {
		// If overshot, shrink image by aspect √(target/actual) and re-try once
		factor := math.Sqrt(float64(targetSize) / float64(len(data)))
		newSide := int(float64(side) * factor)
		if newSide < 1 {
			return fmt.Errorf("target %d too small for any PNG image", targetSize)
		}
		return generatePNGWithSize(path, targetSize, newSide)
	}
	// 4) Pad with tEXt chunk
	return padPNGToSize(path, data, targetSize)
}

// Helper to regenerate PNG at a specific side length
func generatePNGWithSize(path string, targetSize int64, side int) error {
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for i := range img.Pix {
		img.Pix[i] = byte(rand.IntN(256))
	}
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return err
	}
	data := buf.Bytes()
	if int64(len(data)) > targetSize {
		return fmt.Errorf("even %dx%d PNG is %d bytes > target %d",
			side, side, len(data), targetSize)
	}
	return padPNGToSize(path, data, targetSize)
}

// Inject a single ancillary tEXt chunk to pad to exact size
func padPNGToSize(path string, pngData []byte, targetSize int64) error {
	needed := targetSize - int64(len(pngData))
	// Locate IEND (last 12 bytes)
	n := len(pngData)
	if n < 12 {
		return fmt.Errorf("PNG data too short")
	}
	iendStart := n - 12
	if string(pngData[iendStart+4:iendStart+8]) != "IEND" {
		return fmt.Errorf("invalid PNG: IEND not found")
	}
	body := pngData[:iendStart]
	iend := pngData[iendStart:]

	// Build tEXt chunk with keyword "Pad" + padding bytes
	keyword := "Pad"
	// Chunk data length = needed - 12 (chunk overhead) but ≥ len(keyword)+1
	dataLen := needed - 12
	minLen := int64(len(keyword) + 1)
	if dataLen < minLen {
		dataLen = minLen
	}
	padBytes := make([]byte, dataLen-int64(len(keyword))-1)
	cryptoRand.Read(padBytes)
	// Construct chunk
	chunkType := []byte("tEXt")
	chunkData := append([]byte(keyword+"\x00"), padBytes...)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(chunkData)))
	crc := crc32.NewIEEE()
	crc.Write(chunkType)
	crc.Write(chunkData)
	crcBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBytes, crc.Sum32())

	out := &bytes.Buffer{}
	out.Write(body)
	out.Write(length)
	out.Write(chunkType)
	out.Write(chunkData)
	out.Write(crcBytes)
	out.Write(iend)

	return os.WriteFile(path, out.Bytes(), 0666)
}

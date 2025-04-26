package jpeg

import (
	"bytes"
	cryptRand "crypto/rand"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"math"
	"math/rand/v2"
	"os"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
)

func init() {
	factory.RegisterGenerator(ports.FileTypeJPEG, New()) //
}

type JPEGGenerator struct{}

func New() ports.FileGenerator {
	return &JPEGGenerator{}
}

func (g *JPEGGenerator) Generate(path string, targetSize int64) error {
	// 1) Estimate pixels for random-noise JPEG. Empirically, noise JPEG ≈ 1.1 bytes/pixel at Q90
	estBPP := 1.1
	pixels := float64(targetSize) / estBPP
	side := int(math.Sqrt(pixels))
	if side < 1 {
		side = 1
	}
	return generateJPEGWithSide(path, targetSize, side)
}

func generateJPEGWithSide(path string, targetSize int64, side int) error {
	// Create noisy image
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	for i := range img.Pix {
		img.Pix[i] = byte(rand.IntN(256))
	}
	// Encode to JPEG
	buf := &bytes.Buffer{}
	opt := jpeg.Options{Quality: 90}
	if err := jpeg.Encode(buf, img, &opt); err != nil {
		log.Printf("JPEG encode error: %v", err)
		return err
	}
	data := buf.Bytes()
	if int64(len(data)) > targetSize {
		// Overshot → scale by √(target/actual)
		factor := math.Sqrt(float64(targetSize) / float64(len(data)))
		newSide := int(float64(side) * factor)
		if newSide < 1 {
			return fmt.Errorf("target %d too small for any JPEG", targetSize)
		}
		return generateJPEGWithSide(path, targetSize, newSide)
	}
	// Pad via COM segments
	return padJPEGToSize(path, data, targetSize)
}

func padJPEGToSize(path string, jpegData []byte, targetSize int64) error {
	needed := targetSize - int64(len(jpegData))
	// Split at SOS (0xFFDA)
	idx := bytes.Index(jpegData, []byte{0xFF, 0xDA})
	if idx < 0 {
		return fmt.Errorf("SOS marker not found")
	}
	pre := jpegData[:idx]
	post := jpegData[idx:]
	// Build COM segments
	var segments [][]byte
	rem := needed
	for rem > 0 {
		chunk := rem
		if chunk > 0xFFFD {
			chunk = 0xFFFD
		}
		// length = chunk + 2
		length := uint16(chunk + 2)
		hdr := []byte{0xFF, 0xFE, byte(length >> 8), byte(length & 0xFF)}
		data := make([]byte, int(chunk))
		cryptRand.Read(data)
		// escape 0xFF bytes
		for i := 0; i+1 < len(data); i++ {
			if data[i] == 0xFF && data[i+1] != 0x00 {
				data[i+1] = 0x00
			}
		}
		segments = append(segments, append(hdr, data...))
		rem -= int64(len(hdr) + len(data))
	}
	// Assemble
	out := &bytes.Buffer{}
	out.Write(pre)
	for _, seg := range segments {
		out.Write(seg)
	}
	out.Write(post)
	return os.WriteFile(path, out.Bytes(), 0666)
}

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
	currentSize := int64(len(jpegData))
	needed := targetSize - currentSize
	if needed < 0 {
		// Should be caught by generateAndPadJPEG, but defensive check
		return fmt.Errorf("internal error: data size %d > target %d before padding", currentSize, targetSize)
	}
	if needed == 0 {
		// Already correct size, just write it
		return os.WriteFile(path, jpegData, 0666)
	}

	// Split at SOS (0xFFDA)
	idx := bytes.Index(jpegData, []byte{0xFF, 0xDA})
	if idx < 0 {
		// If no SOS marker found, we can't reliably inject COM segments before it.
		// This might happen for extremely small/corrupt initial JPEGs.
		// Fallback: Write the data as is, size will be less than target.
		log.Printf("Warning: SOS marker not found in JPEG for padding. Final size may be less than target.")
		return os.WriteFile(path, jpegData, 0666)

	}
	pre := jpegData[:idx]
	post := jpegData[idx:]

	// Build COM segments
	var segments [][]byte
	rem := needed // Remaining bytes to pad

	for rem > 0 {
		// Calculate max data payload for this segment. Need 4 bytes for header (0xFFFE + length).
		maxDataPayload := rem - 4
		if maxDataPayload <= 0 {
			// Cannot fit even the 4-byte header. Break the loop.
			// This might leave 1, 2, or 3 bytes unpadded.
			if rem > 0 {
				log.Printf("Warning: Remaining %d bytes too small for JPEG COM segment header. Final size will be slightly less than target.", rem)
			}
			break
		}

		// Determine actual data payload size for this segment
		chunk := maxDataPayload
		if chunk > 0xFFFD { // Respect max COM data size (65533)
			chunk = 0xFFFD
		}

		// length field = data payload size + 2 bytes for length field itself
		length := uint16(chunk + 2)
		hdr := []byte{0xFF, 0xFE, byte(length >> 8), byte(length & 0xFF)} // 4 bytes: Marker + Length

		// Create random data payload
		data := make([]byte, int(chunk))
		_, err := cryptRand.Read(data)
		if err != nil {
			return fmt.Errorf("failed to read random bytes for padding: %w", err)
		}

		// Note: JPEG spec says 0xFF within COM data should be followed by 0x00.
		// This implementation doesn't currently escape 0xFF bytes in the random data.
		// While many decoders might ignore this, it's technically non-compliant.
		// For simplicity in this generator, we omit the escaping for now.

		// Append the constructed segment (header + data)
		segments = append(segments, append(hdr, data...))

		// Decrease remaining bytes needed by the *total size* of the segment added
		segmentSize := int64(len(hdr) + len(data)) // 4 + chunk
		rem -= segmentSize
	} // End padding loop

	// Assemble final file
	out := &bytes.Buffer{}
	out.Write(pre)
	for _, seg := range segments {
		out.Write(seg)
	}
	out.Write(post)

	finalBytes := out.Bytes()
	finalSize := int64(len(finalBytes))

	// Final size check and warning (optional but helpful)
	if finalSize != targetSize {
		// Only log warning if the difference is small (due to leftover 'rem' < 4)
		if targetSize-finalSize > 0 && targetSize-finalSize < 4 {
			log.Printf("Warning: Final JPEG size %d is %d bytes less than target %d due to padding constraints.", finalSize, targetSize-finalSize, targetSize)
		} else {
			// Log a more prominent warning for unexpected differences
			log.Printf("Warning: Final JPEG size %d differs unexpectedly from target %d.", finalSize, targetSize)
		}
	}

	return os.WriteFile(path, finalBytes, 0666)
}

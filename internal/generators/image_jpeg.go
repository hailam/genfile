package generators

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand"
	"os"
	"time"
)

func GenerateJPEG(path string, size int64) error {
	// Step 1: create random image (100x100)
	width, height := 100, 100
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	rand.Seed(time.Now().UnixNano())
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				uint8(rand.Intn(256)),
				uint8(rand.Intn(256)),
				uint8(rand.Intn(256)),
				255})
		}
	}
	// Step 2: encode to JPEG (use quality 90 for decent size)
	var buf bytes.Buffer
	opt := jpeg.Options{Quality: 90}
	if err := jpeg.Encode(&buf, img, &opt); err != nil {
		return err
	}
	jpegData := buf.Bytes()
	if int64(len(jpegData)) > size {
		return fmt.Errorf("cannot generate JPEG of %d bytes; minimum is %d", size, len(jpegData))
	}
	// Step 3: determine padding needed
	needed := size - int64(len(jpegData))
	if needed == 0 {
		return os.WriteFile(path, jpegData, 0666)
	}
	// JPEG structure: begins with 0xFFD8 (SOI) and ends with 0xFFD9 (EOI).
	// We'll insert COM segments before the final EOI.
	// Find the position of the SOS (Start of Scan) marker (0xFFDA) to know where image data begins.
	data := jpegData
	var sosIndex int = -1
	for i := 0; i < len(data)-1; i++ {
		if data[i] == 0xFF && data[i+1] == 0xDA {
			sosIndex = i
			break
		}
	}
	if sosIndex == -1 {
		return fmt.Errorf("SOS marker not found in JPEG data")
	}
	// We will insert comments just before SOS (i.e., after all frame headers).
	// Build the output JPEG in pieces: everything up to SOS, then comments, then the rest (from SOS to end).
	pre := data[:sosIndex]        // up to SOS marker (not including 0xFF)
	sosAndRest := data[sosIndex:] // from SOS to end (including EOI at end)
	// Prepare comment segments:
	// Each segment format: 0xFF 0xFE [2-byte length] [comment bytes]
	// Length includes the two length bytes, so max length field = 65535 (meaning 65533 bytes of actual comment data).
	neededComments := needed
	var commentSegments [][]byte
	for neededComments > 0 {
		// Determine comment data size for this segment
		var chunkSize int
		if neededComments <= 65535-2 {
			// if it fits in one segment
			chunkSize = int(neededComments)
		} else {
			chunkSize = 65535 - 2 // max data per segment
		}
		if chunkSize < 0 {
			break
		}
		// Ensure minimum segment length (including length bytes) is 4.
		if chunkSize < 2 {
			chunkSize = 2
		}
		neededComments -= int64(chunkSize)
		// chunkSize includes the length bytes? Actually, we want to set the length field = chunkSize + 2.
		// So actual comment text length = chunkSize (we include the length field in that).
		// Let's say we want chunkSize_total including length = chunkSize+2.
		lengthFieldVal := uint16(chunkSize + 2)
		if lengthFieldVal < 4 {
			lengthFieldVal = 4
		}
		// Generate random comment data of length = (lengthFieldVal - 2)
		commentLen := int(lengthFieldVal) - 2
		commentData := make([]byte, commentLen)
		rand.Read(commentData)
		// Ensure no 0xFF bytes in comment data followed by a non-zero (to avoid being mis-read as marker).
		for i := 0; i < len(commentData)-1; i++ {
			if commentData[i] == 0xFF && commentData[i+1] != 0x00 {
				commentData[i+1] = 0x00 // stuff a zero if 0xFF occurs
			}
		}
		// Construct the segment bytes
		seg := []byte{0xFF, 0xFE} // COM marker
		seg = append(seg, byte(lengthFieldVal>>8), byte(lengthFieldVal&0xFF))
		seg = append(seg, commentData...)
		commentSegments = append(commentSegments, seg)
	}
	// Assemble final JPEG data
	var out bytes.Buffer
	out.Write(pre)
	for _, seg := range commentSegments {
		out.Write(seg)
	}
	out.Write(sosAndRest)
	return os.WriteFile(path, out.Bytes(), 0666)
}

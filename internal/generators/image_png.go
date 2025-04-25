package generators

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"time"
)

/*
DOCX (Office Open XML for Word) is also a ZIP of XML files.

- create a docx file with a simple text
- pad using zip padding
*/

func GeneratePNG(path string, size int64) error {
	// Step 1: Create a random image.
	width, height := 100, 100
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	rand.Seed(time.Now().UnixNano())
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				uint8(rand.Intn(256)),
				uint8(rand.Intn(256)),
				uint8(rand.Intn(256)),
				255})
		}
	}
	// Step 2: Encode PNG to a buffer.
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	pngData := buf.Bytes()
	if int64(len(pngData)) > size {
		return fmt.Errorf("cannot generate PNG of %d bytes; minimum is %d", size, len(pngData))
	}
	// Step 3: Calculate how much padding is needed (excluding the 12-byte IEND chunk).
	needed := size - int64(len(pngData))
	if needed == 0 {
		// Already exactly the size
		return os.WriteFile(path, pngData, 0666)
	}
	// We will insert one tEXt chunk with 'Padding' keyword and random text of length (needed - 12 bytes for IEND and chunk header).
	if needed < 0 {
		return fmt.Errorf("size is smaller than PNG header (%d bytes)", len(pngData))
	}
	// Form the tEXt chunk: [length][type="tEXt"][keyword]\0[data][CRC]
	// Chunk data will be: "Padding\0<random_text>"
	keyword := "Padding"
	// We have 12 bytes overhead for chunk header & CRC (4 length, 4 "tEXt", 4 CRC) and 1 byte for the null separator.
	dataLen := needed - 12
	if dataLen < int64(len(keyword)+1) {
		// Ensure we have at least space for keyword and null terminator
		dataLen = int64(len(keyword) + 1)
	}
	padTextLen := dataLen - int64(len(keyword)) - 1
	if padTextLen < 0 {
		padTextLen = 0
	}
	padBytes := make([]byte, padTextLen)
	rand.Read(padBytes) // random bytes for padding content
	// Replace any 0x00 in padBytes to avoid prematurely ending the text (0x00 is used as terminator after keyword)
	for i, b := range padBytes {
		if b == 0 {
			padBytes[i] = 'A' // replace nulls with 'A'
		}
	}
	// Construct chunk bytes
	chunkType := []byte("tEXt")
	// Chunk data: keyword + \0 + pad bytes
	chunkData := append([]byte(keyword+"\x00"), padBytes...)
	lengthField := make([]byte, 4)
	// length of chunk data
	binary.BigEndian.PutUint32(lengthField, uint32(len(chunkData)))
	// Compute CRC over chunk type + data
	crc := crc32.NewIEEE()
	crc.Write(chunkType)
	crc.Write(chunkData)
	crcVal := crc.Sum32()
	crcField := make([]byte, 4)
	binary.BigEndian.PutUint32(crcField, crcVal)
	// Compose the final PNG: original data up to (but not including) IEND, then our chunk, then IEND.
	// Find IEND in the encoded data (should be last 12 bytes)
	if len(pngData) < 12 {
		return fmt.Errorf("encoded PNG data too short")
	}
	iendStart := len(pngData) - 12
	iendChunk := pngData[iendStart:] // 12-byte IEND chunk
	pngBody := pngData[:iendStart]
	// Validate that iendChunk indeed is IEND (for sanity)
	if string(iendChunk[4:8]) != "IEND" {
		return fmt.Errorf("PNG encoding error: IEND not found where expected")
	}
	// Assemble new PNG data
	var out bytes.Buffer
	out.Write(pngBody)
	out.Write(lengthField)
	out.Write(chunkType)
	out.Write(chunkData)
	out.Write(crcField)
	out.Write(iendChunk)
	// Write to file
	return os.WriteFile(path, out.Bytes(), 0666)
}

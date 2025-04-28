package pdf

import (
	"bytes"
	cryptRand "crypto/rand"
	_ "embed"
	"fmt"
	"io"
	"os"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
)

func init() {
	factory.RegisterGenerator(ports.FileTypePDF, New()) //
}

func New() ports.FileGenerator {
	return &PDFGenerator{}
}

// PDFGenerator implements FileGenerator to create minimal PDFs of a specific size.
type PDFGenerator struct{}

// Generate creates a minimal PDF file at outPath with exactly sizeBytes length.
// It embeds a stream of random (uncompressible) data to achieve the target size.
func (g *PDFGenerator) Generate(outPath string, sizeBytes int64) error {
	// --- Basic Size Check ---
	// Estimate minimum size needed for the PDF structure itself.
	// This is approximate, but avoids calculations for impossibly small files.
	// A safe lower bound is ~250-300 bytes.
	const minStructureSize = 300
	if sizeBytes < minStructureSize {
		return fmt.Errorf("requested size %d bytes is too small for a minimal PDF structure (minimum ~%d bytes)", sizeBytes, minStructureSize)
	}

	// --- Buffers for PDF Parts & Offset Tracking ---
	var headerBuf bytes.Buffer  // %PDF header
	var objectsBuf bytes.Buffer // Objects 1, 2, 3, and 4's dictionary part
	var trailerBuf bytes.Buffer // XRef table, trailer dict, startxref, %%EOF

	// Store the starting byte offset of each object (index matches object number)
	offsets := make([]int64, 5) // 0: unused, 1: Catalog, 2: Pages, 3: Page, 4: Stream Object

	// --- Build Header Part ---
	headerBuf.WriteString("%PDF-1.7\n")
	// Optional: Add binary comment often recommended for binary PDFs
	headerBuf.WriteString("%âãÏÓ\n") // Use \n for line endings per convention

	currentOffset := int64(headerBuf.Len())

	// --- Build Core Objects (Object 1: Catalog) ---
	offsets[1] = currentOffset
	obj1Str := fmt.Sprintf("%d 0 obj\n<< /Type /Catalog /Pages %d 0 R >>\nendobj\n", 1, 2)
	objectsBuf.WriteString(obj1Str)
	currentOffset += int64(len(obj1Str))

	// --- Build Core Objects (Object 2: Pages Root) ---
	offsets[2] = currentOffset
	obj2Str := fmt.Sprintf("%d 0 obj\n<< /Type /Pages /Kids [%d 0 R] /Count 1 >>\nendobj\n", 2, 3)
	objectsBuf.WriteString(obj2Str)
	currentOffset += int64(len(obj2Str))

	// --- Build Core Objects (Object 3: Minimal Page) ---
	offsets[3] = currentOffset
	// Using a tiny MediaBox; content is irrelevant for this generator.
	obj3Str := fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent %d 0 R /MediaBox [0 0 10 10] >>\nendobj\n", 3, 2)
	objectsBuf.WriteString(obj3Str)
	currentOffset += int64(len(obj3Str))

	// --- Calculate Stream Data Length (LLLL) ---
	// This requires knowing the size of ALL OTHER parts, including the trailer
	// which depends on offsets calculated LATER. This creates a dependency cycle.
	// Strategy: Calculate size based on TEMPLATES for later parts, then adjust.

	offsets[4] = currentOffset // Tentative offset for stream object start

	// Templates for dynamic parts (placeholders for stream length LLLL and offsets)
	streamDictTemplateFmt := "%d 0 obj\n<< /Length %d >>\nstream\n" // Object 4 dict
	streamEndMarker := "\nendstream\nendobj\n"                      // After stream data
	xrefHeader := "xref\n0 5\n"                                     // XRef table start
	xrefEntryFmt := "%010d 00000 n \n"                              // XRef entry format
	xrefEntry0 := "0000000000 65535 f \n"                           // XRef entry for object 0
	trailerTemplate := "trailer\n<< /Size 5 /Root 1 0 R >>\n"       // Trailer dictionary
	startxrefTemplateFmt := "startxref\n%d\n"                       // startxref line
	eofMarker := "%%EOF"                                            // End Of File marker

	// Function to calculate size of the trailer structure given offsets and LLLL
	calculateTrailerSize := func(o []int64, startXRefOffset int64) int64 {
		size := int64(len(xrefHeader))
		size += int64(len(xrefEntry0))
		size += int64(len(fmt.Sprintf(xrefEntryFmt, o[1])))
		size += int64(len(fmt.Sprintf(xrefEntryFmt, o[2])))
		size += int64(len(fmt.Sprintf(xrefEntryFmt, o[3])))
		size += int64(len(fmt.Sprintf(xrefEntryFmt, o[4]))) // Offset of stream obj
		size += int64(len(trailerTemplate))
		size += int64(len(fmt.Sprintf(startxrefTemplateFmt, startXRefOffset)))
		size += int64(len(eofMarker))
		return size
	}

	// Initial estimation loop (usually 1-2 iterations needed)
	var streamDataLen int64 = 0
	//var finalTrailerSize int64 = 0
	var finalStreamDictSize int64 = 0
	var startXRefOffset int64 = 0 // Offset of the 'xref' keyword

	for i := 0; i < 3; i++ { // Limit iterations to prevent infinite loops
		// Calculate size of stream dictionary based on current streamDataLen estimate
		streamDictStr := fmt.Sprintf(streamDictTemplateFmt, 4, streamDataLen)
		finalStreamDictSize = int64(len(streamDictStr))

		// Calculate size of fixed parts + stream dict + stream end marker
		nonTrailerSize := int64(headerBuf.Len()) + int64(objectsBuf.Len()) + finalStreamDictSize + int64(len(streamEndMarker))

		// Estimate startXRefOffset (position right after stream object's endobj)
		startXRefOffset = nonTrailerSize + streamDataLen // Offset where 'xref' will start

		// Estimate trailer size based on current offsets and estimated startXRefOffset
		estimatedTrailerSize := calculateTrailerSize(offsets, startXRefOffset)

		// Calculate required stream data length
		newStreamDataLen := sizeBytes - nonTrailerSize - estimatedTrailerSize

		if newStreamDataLen < 0 {
			// This can happen if the estimated trailer size grows due to larger offsets
			// in the second iteration, making the target size impossible.
			return fmt.Errorf("calculated negative stream data length (%d) for target size %d bytes after iteration %d. Target size is likely too small for structure overhead.", newStreamDataLen, sizeBytes, i+1)
		}

		// If length converges, break the loop
		if newStreamDataLen == streamDataLen && i > 0 {
			//finalTrailerSize = estimatedTrailerSize // Store the final calculated trailer size
			break
		}

		streamDataLen = newStreamDataLen
		//finalTrailerSize = estimatedTrailerSize // Update for next iteration or final use

		// Sanity check for the last iteration if size still not converged (unlikely)
		if i == 2 && newStreamDataLen != streamDataLen {
			return fmt.Errorf("failed to converge on stream data length calculation after %d iterations", i+1)
		}
	}

	// --- Final Assembly Calculation ---
	// Add the final stream dictionary to the objects buffer
	finalStreamDictStr := fmt.Sprintf(streamDictTemplateFmt, 4, streamDataLen)
	objectsBuf.WriteString(finalStreamDictStr)

	// Calculate final startXRefOffset precisely
	startXRefOffset = int64(headerBuf.Len()+objectsBuf.Len()) + streamDataLen + int64(len(streamEndMarker))

	// --- Build Trailer Structure ---
	trailerBuf.WriteString(xrefHeader)
	trailerBuf.WriteString(xrefEntry0)
	trailerBuf.WriteString(fmt.Sprintf(xrefEntryFmt, offsets[1]))
	trailerBuf.WriteString(fmt.Sprintf(xrefEntryFmt, offsets[2]))
	trailerBuf.WriteString(fmt.Sprintf(xrefEntryFmt, offsets[3]))
	trailerBuf.WriteString(fmt.Sprintf(xrefEntryFmt, offsets[4])) // Use final calculated offset
	trailerBuf.WriteString(trailerTemplate)
	trailerBuf.WriteString(fmt.Sprintf(startxrefTemplateFmt, startXRefOffset))
	trailerBuf.WriteString(eofMarker)

	// --- Size Verification (Crucial) ---
	calculatedTotalSize := int64(headerBuf.Len()) + int64(objectsBuf.Len()) + streamDataLen + int64(len(streamEndMarker)) + int64(trailerBuf.Len())

	if calculatedTotalSize != sizeBytes {
		// This *shouldn't* happen if the iterative calculation worked, but check anyway.
		return fmt.Errorf("internal calculation error: final calculated size %d does not match target size %d", calculatedTotalSize, sizeBytes)
	}

	// --- Generate Random Stream Data ---
	randomData := make([]byte, streamDataLen)
	if streamDataLen > 0 { // Only read if length > 0
		if _, err := io.ReadFull(cryptRand.Reader, randomData); err != nil {
			return fmt.Errorf("failed to generate %d bytes of random data: %w", streamDataLen, err)
		}
	}

	// --- Write to Output File ---
	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output file '%s': %w", outPath, err)
	}
	defer file.Close() // Ensure file is closed on exit

	// Write parts sequentially
	if _, err := headerBuf.WriteTo(file); err != nil {
		return fmt.Errorf("failed to write PDF header: %w", err)
	}
	if _, err := objectsBuf.WriteTo(file); err != nil {
		return fmt.Errorf("failed to write PDF objects: %w", err)
	}

	// Write the stream data directly
	if streamDataLen > 0 {
		if _, err := file.Write(randomData); err != nil {
			return fmt.Errorf("failed to write PDF stream data: %w", err)
		}
	}

	// Write the stream end marker
	if _, err := file.Write([]byte(streamEndMarker)); err != nil {
		return fmt.Errorf("failed to write PDF stream end marker: %w", err)
	}

	// Write the trailer structure
	if _, err := trailerBuf.WriteTo(file); err != nil {
		return fmt.Errorf("failed to write PDF trailer structure: %w", err)
	}

	// Explicitly check the file close error
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close output file '%s': %w", outPath, err)
	}

	// Final check of actual file size on disk (optional but recommended)
	info, err := os.Stat(outPath)
	if err != nil {
		// Don't return error here, generation might have succeeded but stat failed
		fmt.Fprintf(os.Stderr, "Warning: could not stat output file '%s': %v\n", outPath, err)
	} else if info.Size() != sizeBytes {
		// This indicates a flaw in calculation or writing
		return fmt.Errorf("internal error: final file size on disk (%d) does not match target size (%d)", info.Size(), sizeBytes)
	}

	return nil // Success
}

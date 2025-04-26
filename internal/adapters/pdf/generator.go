package pdf

import (
	"bytes"
	_ "embed"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
	"github.com/signintech/gopdf"
)

func init() {
	factory.RegisterGenerator(ports.FileTypePDF, New()) //
}

//go:embed fonts/DejaVuSans.ttf
var dejaVuSansFontData []byte

const (
	// Constants for font and layout
	fontName         = "DejaVuSans"
	fontSize         = 12
	lineHeight       = 15.0
	leftMargin       = 10.0
	topMargin        = 10.0
	contentChunkSize = 5000 // Add content in chunks

	// Constants for iterative approach
	initialBytesPerChar   = 1.0  // Initial estimate
	maxIterations         = 20   // Max attempts
	tolerance             = 5    // Target accuracy in bytes (very tight tolerance)
	minAdjustmentChars    = 1    // Minimum chars to add/remove
	dampingFactor         = 0.95 // Factor to reduce adjustment step
	directAdjustThreshold = 100  // When diff is smaller than this, adjust chars directly by diff amount

)

type PdfGenerator struct{}

func New() ports.FileGenerator {
	return &PdfGenerator{}
}

// setupPdfBase initializes a GoPdf object with standard settings, embedded font, and NO compression.
func setupPdfBase() (*gopdf.GoPdf, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.SetNoCompression() // Key change!
	err := pdf.AddTTFFontByReader(fontName, bytes.NewReader(dejaVuSansFontData))
	if err != nil {
		return nil, fmt.Errorf("failed to add embedded font %s: %w", fontName, err)
	}
	if err := pdf.SetFont(fontName, "", fontSize); err != nil {
		return nil, fmt.Errorf("failed to set embedded font %s: %w", fontName, err)
	}
	return pdf, nil
}

// addContentToPdf adds a specific number of random characters to the pdf object
// by processing the content in manageable chunks.
func addContentToPdf(pdf *gopdf.GoPdf, numChars int64) error {
	if numChars <= 0 {
		return nil
	}
	var currentCharsAdded int64 = 0
	var builder strings.Builder
	chunkCounter := 0
	rect := &gopdf.Rect{W: gopdf.PageSizeA4.W - (2 * leftMargin), H: lineHeight}

	for currentCharsAdded < numChars {
		chunkCounter++
		charsInChunk := int64(contentChunkSize)
		remainingNeeded := numChars - currentCharsAdded
		if charsInChunk > remainingNeeded {
			charsInChunk = remainingNeeded
		}
		if charsInChunk <= 0 {
			break
		}

		chunkContent := generateRandomPrintableString(int(charsInChunk))
		builder.WriteString(chunkContent)

		if err := pdf.SetFont(fontName, "", fontSize); err != nil {
			fmt.Printf("  Chunk %d: ERROR setting font: %v\n", chunkCounter, err)
			return fmt.Errorf("failed to set font before adding content chunk %d: %w", chunkCounter, err)
		}
		err := pdf.MultiCell(rect, builder.String())
		if err != nil {
			fmt.Printf("  Chunk %d: WARNING during MultiCell: %v.\n", chunkCounter, err)
		}
		builder.Reset()
		currentCharsAdded += charsInChunk
	}
	return nil
}

// generateAndGetSize generates an uncompressed PDF with numChars using chunked content addition,
// writes it to a temporary file, returns the size, and cleans up.
func generateAndGetSize(numChars int64) (size int64, baseSize int64, err error) {
	// Base PDF
	pdfBase, errBase := setupPdfBase()
	if errBase != nil {
		return -1, -1, fmt.Errorf("setupPdfBase failed for base size check: %w", errBase)
	}
	pdfBase.AddPage()
	pdfBase.SetX(leftMargin)
	pdfBase.SetY(topMargin)
	tmpFileBase, errTmpBase := os.CreateTemp("", "genfile-pdf-base-*.pdf")
	if errTmpBase != nil {
		return -1, -1, fmt.Errorf("failed to create temp file for base size: %w", errTmpBase)
	}
	tmpPathBase := tmpFileBase.Name()
	tmpFileBase.Close()
	defer os.Remove(tmpPathBase)
	if errWriteBase := pdfBase.WritePdf(tmpPathBase); errWriteBase != nil {
		return -1, -1, fmt.Errorf("failed to write temp base PDF %s: %w", tmpPathBase, errWriteBase)
	}
	infoBase, errStatBase := os.Stat(tmpPathBase)
	if errStatBase != nil {
		return -1, -1, fmt.Errorf("failed to stat temp base PDF %s: %w", tmpPathBase, errStatBase)
	}
	baseSize = infoBase.Size()

	// Content PDF
	pdfContent, errContentSetup := setupPdfBase()
	if errContentSetup != nil {
		return -1, baseSize, fmt.Errorf("setupPdfBase failed for content PDF: %w", errContentSetup)
	}
	pdfContent.AddPage()
	pdfContent.SetX(leftMargin)
	pdfContent.SetY(topMargin)
	if errAddContent := addContentToPdf(pdfContent, numChars); errAddContent != nil {
		fmt.Printf("Warning: addContentToPdf returned error during size calculation: %v\n", errAddContent)
	}
	tmpFileContent, errTmpContent := os.CreateTemp("", "genfile-pdf-content-*.pdf")
	if errTmpContent != nil {
		return -1, baseSize, fmt.Errorf("failed to create temp file for content size: %w", errTmpContent)
	}
	tmpPathContent := tmpFileContent.Name()
	tmpFileContent.Close()
	defer os.Remove(tmpPathContent)
	if errWriteContent := pdfContent.WritePdf(tmpPathContent); errWriteContent != nil {
		return -1, baseSize, fmt.Errorf("failed to write temp content PDF %s: %w", tmpPathContent, errWriteContent)
	}
	infoContent, errStatContent := os.Stat(tmpPathContent)
	if errStatContent != nil {
		return -1, baseSize, fmt.Errorf("failed to stat temp content PDF %s: %w", tmpPathContent, errStatContent)
	}

	return infoContent.Size(), baseSize, nil
}

// Generate creates an uncompressed PDF file by iteratively adjusting content size.
func (g *PdfGenerator) Generate(path string, targetSize int64) error {
	fmt.Printf("Target PDF size: %d bytes. Starting iterative generation (No Compression)...\n", targetSize)

	// Get initial base size
	_, initialBaseSize, errBase := generateAndGetSize(0)
	if errBase != nil {
		return fmt.Errorf("failed to get initial base size: %w", errBase)
	}
	fmt.Printf("  - Initial Base PDF Size (Uncompressed): %d bytes\n", initialBaseSize)

	if targetSize < initialBaseSize {
		return fmt.Errorf("target size %d is less than minimum possible size %d", targetSize, initialBaseSize)
	}
	if targetSize == initialBaseSize {
		fmt.Printf("Target size equals base size. Generating minimal PDF.\n")
		pdf, err := setupPdfBase()
		if err != nil {
			return err
		}
		pdf.AddPage()
		pdf.SetX(leftMargin)
		pdf.SetY(topMargin)
		err = pdf.WritePdf(path)
		if err == nil {
			info, _ := os.Stat(path)
			fmt.Printf("Generated minimal PDF: %s, Actual Size: %d\n", path, info.Size())
		}
		return err
	}

	// Initial character estimate
	contentBytesNeeded := targetSize - initialBaseSize
	currentChars := int64(float64(contentBytesNeeded) / initialBytesPerChar)
	if currentChars < 1 {
		currentChars = 1
	}

	// Iteration variables
	var lastActualSize int64 = -1
	var actualSize int64
	var currentBaseSize int64
	var err error
	dynamicBytesPerChar := initialBytesPerChar
	finalCharsFromIteration := currentChars

	for i := 0; i < maxIterations; i++ {
		fmt.Printf("\nIteration %d/%d: Attempting generation with %d characters...\n", i+1, maxIterations, currentChars)

		actualSize, currentBaseSize, err = generateAndGetSize(currentChars)
		if err != nil {
			return fmt.Errorf("iteration %d generation failed: %w", i+1, err)
		}
		if currentBaseSize == -1 {
			return fmt.Errorf("iteration %d failed to get base size", i+1)
		}

		diff := targetSize - actualSize
		fmt.Printf("  Base: %d, Target: %d, Actual: %d, Diff: %d bytes\n", currentBaseSize, targetSize, actualSize, diff)

		finalCharsFromIteration = currentChars // Store latest attempt

		// Check Convergence
		if diff == 0 || abs(diff) <= tolerance {
			if diff == 0 {
				fmt.Println("  Exact size match achieved. Final generation...")
			} else {
				fmt.Printf("  Size within tolerance (%d bytes). Final generation...\n", tolerance)
			}
			break
		}

		// Check for Oscillation or minimal change
		if lastActualSize != -1 {
			if ((diff > 0 && targetSize-lastActualSize < 0) || (diff < 0 && targetSize-lastActualSize > 0)) && abs(diff) < tolerance*5 { // Increased oscillation tolerance slightly
				fmt.Printf("  Size oscillating near target (%d vs %d), accepting current size and stopping.\n", actualSize, lastActualSize)
				break
			}
			if abs(actualSize-lastActualSize) < 10 && abs(diff) > tolerance { // Check for very small changes
				fmt.Printf("  Size change minimal (%d bytes) but difference still large (%d bytes). Stopping iteration.\n", actualSize-lastActualSize, diff)
				break
			}
		}

		// Calculate dynamic BPC
		contentBytesAdded := actualSize - currentBaseSize
		if contentBytesAdded > 0 && currentChars > 0 {
			dynamicBytesPerChar = float64(contentBytesAdded) / float64(currentChars)
			if dynamicBytesPerChar <= 0.1 {
				dynamicBytesPerChar = 0.1
			} // Keep a minimum sanity check
			fmt.Printf("  Dynamic BPC updated: %.4f (ContentBytes: %d, Chars: %d)\n", dynamicBytesPerChar, contentBytesAdded, currentChars)
		} else {
			fmt.Printf("  Warning: Content bytes (%d) or chars (%d) invalid for BPC calc. Using previous BPC: %.4f\n", contentBytesAdded, currentChars, dynamicBytesPerChar)
		}

		// Calculate Adjustment
		var adjustment int64
		// *** Refined Adjustment: Use direct diff when close ***
		if abs(diff) < directAdjustThreshold {
			// Assume ~1 char per byte difference when very close and uncompressed
			adjustment = diff // Add/remove roughly the number of bytes needed
			// Clamp the direct adjustment to avoid huge jumps if BPC was way off
			maxDirectAdjust := int64(directAdjustThreshold * 2) // Limit direct step
			adjustment = min(maxDirectAdjust, max(-maxDirectAdjust, adjustment))
			fmt.Printf("  Direct adjustment (diff %d < %d): %d chars\n", abs(diff), directAdjustThreshold, adjustment)
		} else {
			// If further away, use the damped BPC-based adjustment
			estimatedAdjustment := int64(float64(diff) / dynamicBytesPerChar * dampingFactor)
			if diff > 0 { // Too small
				adjustment = max(minAdjustmentChars, estimatedAdjustment)
			} else { // Too large
				adjustment = min(-minAdjustmentChars, estimatedAdjustment) // ensure negative
			}
			fmt.Printf("  Standard adjustment: %d chars\n", adjustment)
		}

		newChars := currentChars + adjustment
		if newChars < 0 {
			newChars = 0
		}

		// Prevent getting stuck if adjustment is zero
		if newChars == currentChars {
			if diff > 0 {
				newChars++
			} else if diff < 0 && newChars > 0 {
				newChars--
			}
			if newChars == currentChars {
				fmt.Printf("  Adjustment resulted in no change (%d -> %d) and cannot force step, stopping iteration.\n", currentChars, newChars)
				break
			} else {
				fmt.Printf("  Adjustment was zero, forcing minimal step to %d chars.\n", newChars)
			}
		}

		fmt.Printf("  Adjusting characters from %d to %d\n", currentChars, newChars)
		currentChars = newChars
		lastActualSize = actualSize

		if i == maxIterations-1 {
			finalCharsFromIteration = currentChars
			fmt.Printf("Warning: Reached max iterations (%d). Using %d chars for final generation.\n", maxIterations, finalCharsFromIteration)
		}
	} // End of iteration loop

	// --- Final Generation to Target Path ---
	fmt.Printf("\nGenerating final PDF with %d characters to %s...\n", finalCharsFromIteration, path)
	finalPdf, err := setupPdfBase()
	if err != nil {
		return fmt.Errorf("final setupPdfBase failed: %w", err)
	}
	finalPdf.AddPage()
	finalPdf.SetX(leftMargin)
	finalPdf.SetY(topMargin)
	err = addContentToPdf(finalPdf, finalCharsFromIteration)
	if err != nil {
		fmt.Printf("Warning: Error adding final content chunks: %v\n", err)
	}

	// *** Did try metadata padding here, but that was too unpredictable ***

	err = finalPdf.WritePdf(path)
	if err != nil {
		return fmt.Errorf("failed to write final PDF %s: %w", path, err)
	}

	// --- Final Report ---
	finalInfo, finalStatErr := os.Stat(path)
	finalSize := int64(-1)
	if finalStatErr == nil {
		finalSize = finalInfo.Size()
	} else {
		fmt.Printf("Warning: Could not stat final generated file at %s: %v\n", path, finalStatErr)
	}

	fmt.Printf("--------------------------------------------------\n")
	fmt.Printf("PDF Generation Complete for: %s\n", path)
	fmt.Printf("Target Size: %d bytes\n", targetSize)
	fmt.Printf("Final Size:  %d bytes\n", finalSize)
	if finalSize != -1 {
		fmt.Printf("Difference:  %d bytes (Tolerance: +/- %d bytes)\n", targetSize-finalSize, tolerance)
	}
	fmt.Printf("--------------------------------------------------\n")

	return nil
}

// generateRandomPrintableString generates a string of length n with printable ASCII chars.
func generateRandomPrintableString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 " // No newline
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}

// abs returns the absolute value of an int64.
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

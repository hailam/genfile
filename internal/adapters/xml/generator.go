// genfile/internal/adapters/xml/generator.go
package xml

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
)

func init() {
	factory.RegisterGenerator(ports.FileTypeXML, New())
}

const (
	xmlDeclaration  = `<?xml version="1.0" encoding="UTF-8"?>`
	rootTagOpen     = `<generatedRoot>`
	rootTagClose    = `</generatedRoot>`
	commentOpen     = "<!-- "
	commentClose    = ` -->`
	commentOverhead = int64(len(commentOpen) + len(commentClose))
)

type XmlGenerator struct{}

func New() ports.FileGenerator {
	return &XmlGenerator{}
}

// Generate creates an XML file with a root element and pads using comments.
func (g *XmlGenerator) Generate(path string, targetSize int64) error {
	baseContent := xmlDeclaration + "\n" + rootTagOpen + rootTagClose
	baseSize := int64(len(baseContent))

	if targetSize < 0 {
		targetSize = 0 // Ensure non-negative
	}

	if targetSize < baseSize {
		// Write truncated content if target is smaller than minimal structure
		fmt.Printf("Warning: Target size %d smaller than minimal XML %d. Truncating.\n", targetSize, baseSize)
		return os.WriteFile(path, []byte(baseContent[:targetSize]), 0666)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	// Write XML declaration and opening root tag
	_, err = f.WriteString(xmlDeclaration + "\n" + rootTagOpen)
	if err != nil {
		return fmt.Errorf("failed to write XML start: %w", err)
	}
	bytesWritten := int64(len(xmlDeclaration) + 1 + len(rootTagOpen))

	// Calculate padding needed inside the root tag
	paddingNeeded := targetSize - baseSize
	if paddingNeeded < 0 {
		paddingNeeded = 0 // Should be handled above, but safety check
	}

	// --- Padding Logic ---
	var bytesPadded int64 = 0
	var builder strings.Builder
	const bufSize = 4096 // Buffer size for padding content within comments

	for bytesPadded < paddingNeeded {
		builder.Reset()

		remainingTotalPadding := paddingNeeded - bytesPadded
		if remainingTotalPadding <= 0 {
			break
		}

		// Calculate max content for this comment block
		maxContentSize := remainingTotalPadding - commentOverhead
		if maxContentSize <= 0 {
			// Not enough space for a comment, pad with whitespace/newlines directly
			// (ensure valid placement if strict validation matters)
			paddingChars := generateXmlSafePaddingString(int(remainingTotalPadding))
			n, writeErr := f.WriteString(paddingChars)
			if writeErr != nil {
				return fmt.Errorf("failed to write final raw XML padding: %w", writeErr)
			}
			bytesPadded += int64(n)
			bytesWritten += int64(n)
			break // Exit padding loop
		}

		// Determine content size for this comment (up to buffer size)
		contentSize := int64(bufSize)
		if contentSize > maxContentSize {
			contentSize = maxContentSize
		}

		commentContent := generateXmlSafePaddingString(int(contentSize))

		// Build the comment string
		builder.WriteString(commentOpen)
		builder.WriteString(commentContent)
		builder.WriteString(commentClose)

		commentString := builder.String()
		n, writeErr := f.WriteString(commentString)
		if writeErr != nil {
			return fmt.Errorf("failed to write XML comment padding: %w", writeErr)
		}
		bytesPadded += int64(n)
		bytesWritten += int64(n)

		if int64(n) < int64(len(commentString)) {
			fmt.Printf("Warning: Partial write during XML comment padding (%d < %d)\n", n, len(commentString))
			break
		}
	}

	// Write the closing root tag
	_, err = f.WriteString(rootTagClose)
	if err != nil {
		return fmt.Errorf("failed to write XML end: %w", err)
	}
	bytesWritten += int64(len(rootTagClose))

	// Final Size Verification (optional but good practice)
	if syncErr := f.Sync(); syncErr != nil {
		fmt.Printf("Warning: Failed to sync file %s: %v\n", path, syncErr)
	}
	info, statErr := os.Stat(path)
	if statErr == nil {
		finalSize := info.Size()
		if finalSize != targetSize {
			fmt.Printf("Warning: Final XML size %d does not match target %d. Difference: %d\n", finalSize, targetSize, targetSize-finalSize)
			// Potential truncation if over, but risky:
			// if finalSize > targetSize {
			// 	if err := f.Truncate(targetSize); err != nil { ... }
			// }
		}
	} else {
		fmt.Printf("Warning: Could not stat final file %s: %v\n", path, statErr)
	}

	return nil
}

// generateXmlSafePaddingString generates a random string safe for XML comments or content.
// Avoids '<', '>', '&' and the sequence '--'.
func generateXmlSafePaddingString(n int) string {
	// Basic printable ASCII, excluding <, >, &, and '-' to avoid '--' conflicts easily
	const safeChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 \n\t.,;:!?()[]{}#@*+=/\\|~`^%$"
	var builder strings.Builder
	builder.Grow(n)
	for i := 0; i < n; i++ {
		char := safeChars[rand.IntN(len(safeChars))]
		builder.WriteByte(char)
	}
	return builder.String()
}

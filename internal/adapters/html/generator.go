package html

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
)

func init() {
	factory.RegisterGenerator(ports.FileTypeHTML, New()) //
}

const (
	// Basic HTML5 template structure
	htmlTemplateStart = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Generated HTML</title>
	<style>body { padding: 1rem; font-family: sans-serif; }</style>
</head>
<body>
	<h1>Generated HTML Document</h1>
	<p>This document was generated to meet a specific size requirement.</p>
	` // Removed the comment start/end from template to add it dynamically
	htmlTemplateEnd = `
</body>
</html>`

	// Overhead for HTML comments: "" = 7 bytes
	commentOverhead = 7
)

type HtmlGenerator struct{}

func New() ports.FileGenerator {
	return &HtmlGenerator{}
}

// Generate creates an HTML file at the specified path with the exact target size.
func (g *HtmlGenerator) Generate(path string, targetSize int64) error {
	baseSize := int64(len(htmlTemplateStart) + len(htmlTemplateEnd))

	if targetSize < baseSize {
		// Handle edge case: target is smaller than the minimal template.
		// Write a truncated start of the template.
		fmt.Printf("Warning: Target size %d is smaller than minimal HTML template %d. Truncating.\n", targetSize, baseSize)
		if targetSize < 0 {
			targetSize = 0
		} // Ensure non-negative size
		return os.WriteFile(path, []byte(htmlTemplateStart[:targetSize]), 0666)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	// Write the start of the template
	_, err = f.WriteString(htmlTemplateStart)
	if err != nil {
		return fmt.Errorf("failed to write HTML start: %w", err)
	}
	bytesWritten := int64(len(htmlTemplateStart))

	// Calculate bytes needed for padding (within comments)
	paddingBytesNeeded := targetSize - baseSize
	if paddingBytesNeeded < 0 {
		paddingBytesNeeded = 0
	} // Should be caught above, but safety check

	// --- Padding Logic using HTML Comments ---
	var bytesPadded int64 = 0
	var builder strings.Builder
	bufSize := 4096 // Buffer size for writing padding

	for bytesPadded < paddingBytesNeeded {
		builder.Reset()

		// Calculate remaining padding needed for this comment block
		remainingTotalPadding := paddingBytesNeeded - bytesPadded
		// Max content size for this comment block (leave space for )
		maxContentSize := remainingTotalPadding - commentOverhead
		if maxContentSize <= 0 {
			// Not enough space for a full comment, pad with raw bytes if possible
			// This raw padding will go *outside* any comment tags
			if remainingTotalPadding > 0 {
				paddingChars := generateHtmlSafePaddingString(int(remainingTotalPadding))
				// Write directly, not into builder as it's not part of a comment
				n, writeErr := f.WriteString(paddingChars)
				if writeErr != nil {
					return fmt.Errorf("failed to write final raw HTML padding: %w", writeErr)
				}
				bytesPadded += int64(n)
				bytesWritten += int64(n)
			}
			break // Exit padding loop
		}

		// Determine content size for this comment (up to buffer size)
		contentSize := int64(bufSize)
		if contentSize > maxContentSize {
			contentSize = maxContentSize
		}

		// Generate random content for the comment
		commentContent := generateHtmlSafePaddingString(int(contentSize)) // Generate content

		// Build the comment string
		builder.WriteString(commentContent)

		// Write the comment block
		commentString := builder.String()
		n, writeErr := f.WriteString(commentString)
		if writeErr != nil {
			return fmt.Errorf("failed to write HTML comment padding: %w", writeErr)
		}
		bytesPadded += int64(n) // Add actual bytes written
		bytesWritten += int64(n)

		// If somehow WriteString wrote less than expected (unlikely for strings)
		if int64(n) < int64(len(commentString)) {
			fmt.Printf("Warning: Partial write during comment padding (%d < %d)\n", n, len(commentString))
			break // Avoid potential infinite loops
		}
	}

	// Write the end of the template
	_, err = f.WriteString(htmlTemplateEnd)
	if err != nil {
		return fmt.Errorf("failed to write HTML end: %w", err)
	}
	bytesWritten += int64(len(htmlTemplateEnd))

	// --- Final Size Verification ---
	// Sync before statting
	if syncErr := f.Sync(); syncErr != nil {
		fmt.Printf("Warning: Failed to sync file %s: %v\n", path, syncErr)
	}
	info, statErr := os.Stat(path)
	if statErr == nil {
		finalSize := info.Size()
		if finalSize != targetSize {
			fmt.Printf("Warning: Final HTML size %d does not match target %d. Difference: %d\n", finalSize, targetSize, targetSize-finalSize)
			// If size is over, truncation might be needed, but risky for HTML structure
			// if finalSize > targetSize {
			// 	if err := f.Truncate(targetSize); err != nil {
			// 		return fmt.Errorf("failed to truncate file: %w", err)
			// 	}
			// }
		}
	} else {
		fmt.Printf("Warning: Could not stat final file %s: %v\n", path, statErr)
	}

	return nil // Return nil from Sync() if successful
}

// generateHtmlSafePaddingString generates a random string suitable for HTML content or comments.
// Avoids characters that could break HTML structure easily ('<', '>', '&').
// Also avoids comment end sequence '-->'.
func generateHtmlSafePaddingString(n int) string {
	const safeChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 \n\t.,;:!?()[]{}#@*+-=/\\|~`^%$" // Excludes < > &
	var builder strings.Builder
	builder.Grow(n)
	lastTwo := "--" // Track last two chars to avoid generating "-->"

	for i := 0; i < n; i++ {
		char := safeChars[rand.IntN(len(safeChars))]
		// Check if adding this char would create "-->"
		if lastTwo == "--" && char == '>' {
			// Replace '>' with a safe alternative, like space
			char = ' '
		}
		builder.WriteByte(char)
		// Update lastTwo sequence
		if len(lastTwo) < 2 {
			lastTwo += string(char)
		} else {
			lastTwo = string(lastTwo[1]) + string(char)
		}
	}
	return builder.String()
}

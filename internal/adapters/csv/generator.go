package csv

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/hailam/genfile/internal/ports"
)

const (
	// Define structure for CSV generation
	minColumns    = 3
	maxColumns    = 10
	minCellLength = 5
	maxCellLength = 25
	separator     = ","
	lineEnding    = "\n" // Use LF line endings for consistency
)

type CsvGenerator struct{}

func New() ports.FileGenerator {
	return &CsvGenerator{}
}

// Generate creates a CSV file at the specified path with the exact target size.
// It fills the file with rows of comma-separated random strings.
func (g *CsvGenerator) Generate(path string, targetSize int64) error {
	if targetSize <= 0 {
		// Create an empty file if size is zero or negative
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("failed to create empty file %s: %w", path, err)
		}
		f.Close()
		return nil
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	var bytesWritten int64 = 0
	var builder strings.Builder // Use a builder for efficient string concatenation per line

	// Buffer for writing to file to reduce syscalls
	bufSize := 8192
	fileBuffer := make([]byte, 0, bufSize)

	for bytesWritten < targetSize {
		builder.Reset() // Clear builder for the new line

		// Determine number of columns for this row
		numCols := rand.IntN(maxColumns-minColumns+1) + minColumns

		// Generate cells for the row
		for i := 0; i < numCols; i++ {
			cellLen := rand.IntN(maxCellLength-minCellLength+1) + minCellLength
			// Use utils.RandString or a similar function to generate random alphanumeric string
			// Ensure generated string doesn't contain the separator or line ending for simplicity
			cellContent := generateRandomCsvSafeString(cellLen)
			builder.WriteString(cellContent)
			if i < numCols-1 {
				builder.WriteString(separator)
			}
		}
		builder.WriteString(lineEnding) // Add line ending

		line := builder.String()
		lineBytes := []byte(line)
		lineLen := int64(len(lineBytes))

		// Check if adding the full line exceeds the target size
		if bytesWritten+lineLen > targetSize {
			// Calculate how many bytes we can actually write from this line
			bytesToWrite := targetSize - bytesWritten
			if bytesToWrite > 0 {
				fileBuffer = append(fileBuffer, lineBytes[:bytesToWrite]...)
				bytesWritten += bytesToWrite
			}
			// Flush remaining buffer content before breaking
			if len(fileBuffer) > 0 {
				if _, err := f.Write(fileBuffer); err != nil {
					return fmt.Errorf("failed to write final buffer content: %w", err)
				}
			}
			break // Target size reached
		}

		// Append line to buffer
		fileBuffer = append(fileBuffer, lineBytes...)
		bytesWritten += lineLen

		// Flush buffer if it's full
		if len(fileBuffer) >= bufSize {
			n, writeErr := f.Write(fileBuffer)
			if writeErr != nil {
				return fmt.Errorf("failed to write buffer to file: %w", writeErr)
			}
			// Reset buffer, keeping any remaining bytes if write was partial (unlikely with os.File)
			if n < len(fileBuffer) {
				fileBuffer = fileBuffer[n:] // Should not happen often
			} else {
				fileBuffer = fileBuffer[:0] // Reset buffer
			}
		}
	}

	// Flush any remaining content in the buffer after the loop finishes
	if len(fileBuffer) > 0 {
		if _, err := f.Write(fileBuffer); err != nil {
			return fmt.Errorf("failed to write final buffer content: %w", err)
		}
	}

	// Ensure file is synced to disk
	return f.Sync()
}

// generateRandomCsvSafeString generates a random string suitable for a CSV cell.
// Avoids commas, quotes, and newlines for simplicity.
func generateRandomCsvSafeString(n int) string {
	// Use a character set that excludes comma, double quote, CR, LF
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}

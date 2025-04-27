package csv

import (
	"bufio" // Import bufio
	"fmt"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
)

// init registers the CSV generator with the factory.
func init() {
	factory.RegisterGenerator(ports.FileTypeCSV, New()) //
}

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

// Generate creates a CSV file at the specified path with the exact target size using bufio.Writer.
func (g *CsvGenerator) Generate(path string, targetSize int64) (err error) { // Use named return for deferred flush error handling
	if targetSize < 0 { // Treat negative as zero
		targetSize = 0
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close() // Ensure file is closed eventually

	// Use bufio.Writer for efficient writing
	bw := bufio.NewWriter(f)
	defer func() { // Ensure final flush happens even on errors elsewhere
		flushErr := bw.Flush()
		// Report flush error only if no other error occurred during Generate
		// and assign it to the named return variable 'err'.
		if err == nil && flushErr != nil {
			err = fmt.Errorf("failed to flush writer: %w", flushErr)
		}
	}()

	var bytesWritten int64 = 0
	var builder strings.Builder // Still use builder for efficient line construction

	for bytesWritten < targetSize {
		builder.Reset()
		// --- Generate one line ---
		numCols := rand.IntN(maxColumns-minColumns+1) + minColumns
		for i := 0; i < numCols; i++ {
			cellLen := rand.IntN(maxCellLength-minCellLength+1) + minCellLength
			cellContent := generateRandomCsvSafeString(cellLen)
			builder.WriteString(cellContent)
			if i < numCols-1 {
				builder.WriteString(separator)
			}
		}
		builder.WriteString(lineEnding)
		// --- End generate line ---

		line := builder.String()
		lineBytes := []byte(line)
		lineLen := int64(len(lineBytes))

		// --- Check if this line fits ---
		if bytesWritten+lineLen <= targetSize {
			// Fits completely
			n, writeErr := bw.Write(lineBytes) // Write full line to buffer
			if writeErr != nil {
				return fmt.Errorf("failed to write full line: %w", writeErr)
			}
			bytesWritten += int64(n)
		} else {
			// Does not fit completely, write partial line and stop
			bytesToWrite := targetSize - bytesWritten
			if bytesToWrite > 0 {
				n, writeErr := bw.Write(lineBytes[:bytesToWrite]) // Write partial line to buffer
				if writeErr != nil {
					return fmt.Errorf("failed to write partial line: %w", writeErr)
				}
				bytesWritten += int64(n)
			}
			break // target size reached
		}
	} // End loop

	// Final flush is handled by defer.
	// The named return 'err' will be returned, potentially updated by the deferred flush.
	return err
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

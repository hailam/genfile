package json

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/hailam/genfile/internal/ports"
	// utils "github.com/hailam/genfile/internal/utils" // Can use utils.RandString if preferred
)

const (
	// Define structure for JSON generation
	keyLengthMin = 5
	keyLengthMax = 15
	valLengthMin = 5 // Allow smaller values for padding
	valLengthMax = 100
)

type JsonGenerator struct{}

func New() ports.FileGenerator {
	return &JsonGenerator{}
}

// Generate creates a JSON file at the specified path with the exact target size.
// It starts with an empty object {} and adds key-value pairs with random strings
// until the size is met, precisely padding the final value if needed.
func (g *JsonGenerator) Generate(path string, targetSize int64) error {
	if targetSize < 2 { // Minimum size for "{}"
		content := ""
		if targetSize == 1 {
			content = "{"
		}
		return os.WriteFile(path, []byte(content), 0666)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	var bytesWritten int64 = 0
	var loopBuilder strings.Builder  // Builder for pairs inside the loop
	var finalBuilder strings.Builder // Builder specifically for the final pair/padding

	// Buffer for writing to file
	bufSize := 8192
	fileBuffer := make([]byte, 0, bufSize)

	// Write initial brace and update count
	fileBuffer = append(fileBuffer, '{')
	bytesWritten = 1
	firstKey := true // Flag to handle comma placement

	// --- Main Loop to Add Full Pairs ---
	for bytesWritten < targetSize-1 { // Need 1 byte for '}'
		loopBuilder.Reset()

		// --- Calculate overhead for a potential next pair ---
		commaOverhead := 0
		if !firstKey {
			commaOverhead = 1
		} // Comma byte

		// Minimum overhead for the structure around the value: comma + "key":"" -> comma + 1+keyMin+1 + 1 + 1+0+1 = comma + 5 + keyMin
		minPairOverhead := int64(commaOverhead + 5 + keyLengthMin)
		// Maximum possible pair length (approximate, ignoring escapes): comma + "key": "value" -> comma + 5 + keyMax + valueMax
		maxPairLen := int64(commaOverhead + 5 + keyLengthMax + valLengthMax)

		// If even the smallest possible next pair might overflow, break to final padding
		if bytesWritten+minPairOverhead >= targetSize-1 {
			break
		}

		// If the largest possible next pair *definitely* overflows, break to final padding
		// This check helps avoid generating a pair we know won't fit.
		if bytesWritten+maxPairLen > targetSize-1 {
			// It's likely the next pair won't fit, move to final padding phase
			break
		}

		// --- Generate a Normal Key-Value Pair ---
		if !firstKey {
			loopBuilder.WriteString(",")
		} else {
			firstKey = false
		}

		keyLen := rand.IntN(keyLengthMax-keyLengthMin+1) + keyLengthMin
		key := generateJsonKeySafeString(keyLen)
		loopBuilder.WriteString(`"`)
		loopBuilder.WriteString(key)
		loopBuilder.WriteString(`":`)

		valLen := rand.IntN(valLengthMax-valLengthMin+1) + valLengthMin
		val := generateJsonStringSafeString(valLen)
		loopBuilder.WriteString(`"`)
		loopBuilder.WriteString(val)
		loopBuilder.WriteString(`"`)

		pairString := loopBuilder.String()
		pairBytes := []byte(pairString)
		pairLen := int64(len(pairBytes))

		// Double-check if this *specific* pair fits before adding
		if bytesWritten+pairLen > targetSize-1 {
			// This specific pair didn't fit, break to final padding
			firstKey = true // Reset flag as the last successful write didn't have a comma after it
			// Note: Removing the comma from fileBuffer if it was the last byte is complex
			// because the buffer might have been flushed. Simpler to potentially leave a trailing comma
			// before the final padding step if this break occurs, although slightly invalid JSON.
			// The final padding logic handles adding the comma correctly if needed.
			break
		}

		// --- Append Pair to Buffer ---
		fileBuffer = append(fileBuffer, pairBytes...)
		bytesWritten += pairLen

		// Flush buffer if needed
		if len(fileBuffer) >= bufSize {
			n, writeErr := f.Write(fileBuffer)
			if writeErr != nil {
				return fmt.Errorf("failed to write buffer to file: %w", writeErr)
			}
			if n < len(fileBuffer) {
				fileBuffer = fileBuffer[n:]
			} else {
				fileBuffer = fileBuffer[:0]
			}
		}
	} // End main loop

	// --- Final Padding Logic ---
	remainingSpace := targetSize - bytesWritten
	if remainingSpace > 1 { // Need space for at least '}' and potentially more
		finalBuilder.Reset()
		spaceForFinalPair := remainingSpace - 1 // Space before the final '}'

		// Add comma if needed (check if fileBuffer isn't just '{')
		commaOverhead := 0
		if bytesWritten > 1 {
			finalBuilder.WriteString(",")
			commaOverhead = 1
		}

		// Generate a final key
		// Try to make the key length fit within the remaining space budget
		maxFinalKeyLen := spaceForFinalPair - int64(commaOverhead+5) // Max length for key to allow empty value ""
		if maxFinalKeyLen < int64(keyLengthMin) {
			// Cannot even fit the smallest key + structure, proceed to closing brace
			fmt.Printf("Warning: Remaining space (%d bytes) too small for final key structure. Final size will be less than target.\n", spaceForFinalPair)
			// If we added a comma to the builder, clear it
			if commaOverhead > 0 {
				finalBuilder.Reset()
			}

		} else {
			// Adjust max key length if it exceeds the constant
			if maxFinalKeyLen > int64(keyLengthMax) {
				maxFinalKeyLen = int64(keyLengthMax)
			}

			finalKeyLen := rand.IntN(int(maxFinalKeyLen)-keyLengthMin+1) + keyLengthMin
			finalKey := generateJsonKeySafeString(finalKeyLen)
			finalBuilder.WriteString(`"`)
			finalBuilder.WriteString(finalKey)
			finalBuilder.WriteString(`":`)

			// Calculate overhead of the final pair structure (comma + "key" + : + "" )
			// comma + 1 + keyLen + 1 + 1 + 1 + 1 = comma + 5 + keyLen
			finalPairStructureOverhead := int64(commaOverhead + 5 + finalKeyLen)

			// Calculate exact bytes needed for the final value string content (inside the quotes)
			finalValueBytesNeeded := spaceForFinalPair - finalPairStructureOverhead

			finalValue := ""
			if finalValueBytesNeeded >= 0 {
				// Generate a value string exactly that long using spaces
				finalValue = strings.Repeat(" ", int(finalValueBytesNeeded))
				finalBuilder.WriteString(`"`)
				finalBuilder.WriteString(finalValue)
				finalBuilder.WriteString(`"`)

				finalPairString := finalBuilder.String()
				finalPairBytes := []byte(finalPairString)

				// Append final pair to buffer
				fileBuffer = append(fileBuffer, finalPairBytes...)
				bytesWritten += int64(len(finalPairBytes))

			} else {
				// This case should be caught by the maxFinalKeyLen check above, but handle defensively
				fmt.Printf("Warning: Negative bytes needed for final value (%d). Logic error likely. Final size will be less than target.\n", finalValueBytesNeeded)
				// If we added content to the builder (comma, key), clear it
				finalBuilder.Reset()
			}
		}
	}

	// Add the closing brace if space allows (should always be exactly 1 byte left if logic is correct)
	if bytesWritten < targetSize {
		fileBuffer = append(fileBuffer, '}')
		bytesWritten++
	}

	// Flush any remaining buffer content
	if len(fileBuffer) > 0 {
		if _, err := f.Write(fileBuffer); err != nil {
			return fmt.Errorf("failed to write final buffer content: %w", err)
		}
	}

	// --- Final Size Verification  ---
	info, statErr := os.Stat(path)
	if statErr == nil {
		finalSize := info.Size()
		if finalSize != targetSize {
			fmt.Printf("Warning: Final size %d does not match target %d. Difference: %d\n", finalSize, targetSize, targetSize-finalSize)
		}
	} else {
		fmt.Printf("Warning: Could not stat final file %s: %v\n", path, statErr)
	}

	return f.Sync()
}

// generateJsonKeySafeString generates a random alphanumeric string suitable for a JSON key.
func generateJsonKeySafeString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}

// generateJsonStringSafeString generates a random string, escaping necessary characters for JSON.
func generateJsonStringSafeString(n int) string {
	const letters = `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 !#$%&'()*+,-./:;<=>?@[\]^_{|}~`
	var builder strings.Builder
	builder.Grow(n + n/10) // Preallocate slightly more for potential escapes

	for i := 0; i < n; i++ {
		char := letters[rand.IntN(len(letters))]
		switch char {
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		default:
			builder.WriteByte(char)
		}
	}
	return builder.String()
}

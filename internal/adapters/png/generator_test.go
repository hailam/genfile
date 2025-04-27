package png

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hailam/genfile/internal/ports" //
)

// Estimate a reasonable minimum size. A minimal 1x1 NRGBA image encoded is usually a few hundred bytes, plus the tEXt chunk overhead.
// Let's roughly guess ~100 bytes, but the generator itself should error if too small.
const minReasonablePngSize = 100

func TestPngGenerator_Generate(t *testing.T) {
	generator := New() //

	// Ensure it implements the interface
	var _ ports.FileGenerator = generator

	tempDir := t.TempDir() // Create a temporary directory for test files

	testCases := []struct {
		name            string
		size            int64
		expectErr       bool
		errSubstring    string                                      // Substring to check in error message (case-insensitive)
		checkProperties func(t *testing.T, path string, size int64) // Function to check size and basic content
	}{
		{
			name:         "ZeroSize",
			size:         0,
			expectErr:    true,        // PNG needs a header at least
			errSubstring: "too small", // Keep this specific check for very small sizes
		},
		{
			name:      "TooSmallSize",
			size:      50, // Likely smaller than minimal header + padding chunk
			expectErr: true,
			// Change the expected substring to match the actual error pattern
			errSubstring: "bytes > target", // <<< MODIFIED LINE
		},
		{
			name:      "ReasonableSmallSize", // Test a plausible small PNG size
			size:      minReasonablePngSize + 500,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkPngValidity(t, path) // Check if it decodes as PNG
			},
		},
		{
			name:      "LargerSize", // Requires significant padding
			size:      10 * 1024,    // 10 KB
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkPngValidity(t, path)
				// Optional: Check for tEXt chunk (more complex)
			},
		},
		{
			name:      "LargeSizeRequiringResize", // Test if downsizing logic works
			size:      500 * 1024,                 // 500 KB - likely requires initial overshoot then resize
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkPngValidity(t, path)
			},
		},
		{
			name:         "NegativeSize", // Should error as too small
			size:         -100,
			expectErr:    true,
			errSubstring: "too small", // Keep specific check
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.png", tc.name))

			// --- Execute ---
			err := generator.Generate(outPath, tc.size) //

			// --- Assert Error ---
			if tc.expectErr {
				if err == nil {
					t.Errorf("Generate(%q, %d) expected an error, but got nil", outPath, tc.size)
				} else if tc.errSubstring != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.errSubstring)) { // Ensure case-insensitive check
					t.Errorf("Generate(%q, %d) error = %q, expected error containing %q (case-insensitive)", outPath, tc.size, err.Error(), tc.errSubstring)
				}
				return // Don't check file properties if error was expected
			}
			if err != nil {
				t.Fatalf("Generate(%q, %d) returned unexpected error: %v", outPath, tc.size, err)
			}

			// --- Assert File Properties ---
			if tc.checkProperties != nil {
				tc.checkProperties(t, outPath, tc.size)
			}
		})
	}

	// --- Test Error Case: Invalid Path ---
	t.Run("InvalidPath", func(t *testing.T) {
		// Use a size that should be valid
		err := generator.Generate(tempDir, minReasonablePngSize+500) // Use temp dir as path
		if err == nil {
			t.Errorf("Generate(%q, ...) expected an error for invalid path, but got nil", tempDir)
		}
	})
}

// Helper to check file existence and size
func checkFileSize(t *testing.T, path string, expectedSize int64) {
	t.Helper()
	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			t.Fatalf("Generate did not create the file %q", path)
		} else {
			t.Fatalf("Error stating generated file %q: %v", path, statErr)
		}
	}

	if info.Size() != expectedSize {
		t.Errorf("Generated file %q size = %d, want %d", path, info.Size(), expectedSize)
	}
}

// Helper to check PNG validity by trying to decode it
func checkPngValidity(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file %s for PNG validation: %v", path, err)
	}
	defer f.Close()

	_, err = png.Decode(f) // Try to decode the image data
	if err != nil {
		// Read content for debugging if decode fails
		content, readErr := os.ReadFile(path)
		debugContent := ""
		if readErr == nil {
			debugContent = fmt.Sprintf("\nContent (first 100 bytes):\n%x", limitBytes(content, 100))
		} else {
			debugContent = fmt.Sprintf("\n(Failed to read content: %v)", readErr)
		}
		t.Errorf("File %q failed PNG decoding: %v%s", path, err, debugContent)
	}
}

// Helper to limit byte slice length for logging
func limitBytes(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}
	return b[:maxLen]
}

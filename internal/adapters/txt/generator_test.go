package txt

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hailam/genfile/internal/ports" //
)

func TestTxtGenerator_Generate(t *testing.T) {
	generator := New() //

	// Ensure it implements the interface
	var _ ports.FileGenerator = generator

	tempDir := t.TempDir() // Create a temporary directory for test files

	testCases := []struct {
		name       string
		size       int64
		expectErr  bool // Whether an error is expected from Generate
		checkExist bool // Whether to check for file existence and size
	}{
		{"ZeroSize", 0, false, true},
		{"OneByte", 1, false, true},
		{"SmallSize", 100, false, true},
		{"BufferSize", 8192, false, true},         // Exactly one buffer write
		{"SlightlyOverBuffer", 8193, false, true}, // Multiple buffer writes
		{"LargeSize", 15000, false, true},         // Multiple buffer writes
		{"NegativeSize", -1, false, true},         // Should likely generate 0 bytes, based on loop condition
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.txt", tc.name))

			// --- Execute ---
			err := generator.Generate(outPath, tc.size) //

			// --- Assert Error ---
			if tc.expectErr {
				if err == nil {
					t.Errorf("Generate(%q, %d) expected an error, but got nil", outPath, tc.size)
				}
				return // Don't check file size if error was expected
			}
			if err != nil {
				t.Fatalf("Generate(%q, %d) returned unexpected error: %v", outPath, tc.size, err)
			}

			// --- Assert File Existence and Size ---
			if tc.checkExist {
				info, statErr := os.Stat(outPath)
				if statErr != nil {
					if os.IsNotExist(statErr) {
						t.Fatalf("Generate(%q, %d) did not create the file", outPath, tc.size)
					} else {
						t.Fatalf("Error stating generated file %q: %v", outPath, statErr)
					}
				}

				expectedSize := tc.size
				if expectedSize < 0 {
					// TxtGenerator loop condition `written < size` means negative size results in 0 bytes.
					expectedSize = 0
				}

				if info.Size() != expectedSize {
					t.Errorf("Generated file %q size = %d, want %d", outPath, info.Size(), expectedSize)
				}
			}
		})
	}

	// --- Test Error Case: Invalid Path ---
	t.Run("InvalidPath", func(t *testing.T) {
		// Use the temp directory itself as the output path, which should fail os.Create
		err := generator.Generate(tempDir, 100) //
		if err == nil {
			t.Errorf("Generate(%q, 100) expected an error for invalid path, but got nil", tempDir)
		}
		// We could check for specific error types (e.g., using errors.Is with os errors),
		// but just checking for non-nil error is often sufficient for this type of test.
	})
}

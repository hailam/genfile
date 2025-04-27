package csv

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hailam/genfile/internal/ports" //
)

func TestCsvGenerator_Generate(t *testing.T) {
	generator := New() //

	// Ensure it implements the interface
	var _ ports.FileGenerator = generator

	tempDir := t.TempDir() // Create a temporary directory for test files

	testCases := []struct {
		name            string
		size            int64
		expectErr       bool
		errSubstring    string
		checkProperties func(t *testing.T, path string, size int64) // Function to check size and basic content
	}{
		{
			name:      "ZeroSize",
			size:      0,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
			},
		},
		{
			name:      "OneByte",
			size:      1,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Failed to read file %s: %v", path, err)
				}
				if bytes.Contains(content, []byte("\n")) || bytes.Contains(content, []byte(separator)) {
					t.Errorf("Content %q should likely not contain newline or separator for 1 byte", content)
				}
			},
		},
		{
			name:      "SmallSize",
			size:      50,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Failed to read file %s: %v", path, err)
				}
				// Check if it contains at least one separator if size is large enough
				if size > 10 && !bytes.Contains(content, []byte(separator)) {
					t.Logf("Warning: Small file content %q doesn't contain separator '%s'", content, separator)
				}
			},
		},
		{
			name:      "LargerSizeExact", // Test a size likely ending exactly on a line ending
			size:      1024,              // Adjust if needed based on typical line lengths
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Failed to read file %s: %v", path, err)
				}
				// It's hard to guarantee the last char is newline without knowing exact content,
				// but we can check it's not truncated mid-character generally.
				if !strings.HasSuffix(string(content), lineEnding) {
					t.Logf("Warning: File content for size %d doesn't end with newline", size)
				}
			},
		},
		{
			name:      "LargerSizeTruncated", // Test a size likely requiring truncation
			size:      1030,                  // Should truncate the last line generated for 1024 test
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Failed to read file %s: %v", path, err)
				}
				// Check if the last character is NOT a newline, indicating truncation
				if strings.HasSuffix(string(content), lineEnding) {
					t.Errorf("File content for size %d unexpectedly ends with newline, indicating no truncation?", size)
				}
			},
		},
		{
			name:      "NegativeSize", // Should behave like ZeroSize
			size:      -10,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, 0) // Expect 0 size for negative input
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.csv", tc.name))

			// --- Execute ---
			err := generator.Generate(outPath, tc.size) //

			// --- Assert Error ---
			if tc.expectErr {
				if err == nil {
					t.Errorf("Generate(%q, %d) expected an error, but got nil", outPath, tc.size)
				} else if tc.errSubstring != "" && !strings.Contains(err.Error(), tc.errSubstring) {
					t.Errorf("Generate(%q, %d) error = %q, expected error containing %q", outPath, tc.size, err.Error(), tc.errSubstring)
				}
				return // Don't check file size/content if error was expected
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
		err := generator.Generate(tempDir, 100) // Use temp dir as path
		if err == nil {
			t.Errorf("Generate(%q, 100) expected an error for invalid path, but got nil", tempDir)
		}
	})
}

// Helper to check file existence and size
func checkFileSize(t *testing.T, path string, expectedSize int64) {
	t.Helper() // Mark as test helper
	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			t.Fatalf("Generate(%q, %d) did not create the file", path, expectedSize)
		} else {
			t.Fatalf("Error stating generated file %q: %v", path, statErr)
		}
	}

	if info.Size() != expectedSize {
		t.Errorf("Generated file %q size = %d, want %d", path, info.Size(), expectedSize)
	}
}

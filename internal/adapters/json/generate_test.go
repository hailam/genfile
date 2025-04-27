package json

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hailam/genfile/internal/ports" //
)

func TestJsonGenerator_Generate(t *testing.T) {
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
				checkFileSize(t, path, size) // Expect empty file
			},
		},
		{
			name:      "OneByte", // Invalid JSON, but testing edge case
			size:      1,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size) // Expect '{'
				content, _ := os.ReadFile(path)
				if string(content) != "{" {
					t.Errorf("Content for size 1: got %q, want %q", string(content), "{")
				}
			},
		},
		{
			name:      "TwoBytes", // Minimal valid JSON object
			size:      2,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size) // Expect '{}'
				checkJsonValidity(t, path, true)
				content, _ := os.ReadFile(path)
				if string(content) != "{}" {
					t.Errorf("Content for size 2: got %q, want %q", string(content), "{}")
				}
			},
		},
		{
			name:      "SmallSize", // Likely one key-value pair
			size:      50,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkJsonValidity(t, path, true) // Expect valid JSON
				content, _ := os.ReadFile(path)
				if !strings.HasPrefix(string(content), "{") || !strings.HasSuffix(string(content), "}") {
					t.Errorf("Content %q does not start/end with braces", content)
				}
				if !strings.Contains(string(content), ":") {
					t.Errorf("Content %q does not contain ':' separator", content)
				}
			},
		},
		{
			name:      "LargerSize", // Should contain multiple pairs
			size:      500,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkJsonValidity(t, path, true)
				content, _ := os.ReadFile(path)
				if !strings.HasPrefix(string(content), "{") || !strings.HasSuffix(string(content), "}") {
					t.Errorf("Content %q does not start/end with braces", content)
				}
				// Count occurrences of ':' as a rough check for multiple pairs
				if strings.Count(string(content), ":") < 2 {
					t.Logf("Warning: Larger file content %q might have fewer pairs than expected", content)
				}
			},
		},
		{
			name:      "SizeRequiringPadding", // Test precise padding
			size:      55,                     // Slightly larger than SmallSize, likely padding last value
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkJsonValidity(t, path, true)
			},
		},
		{
			name:      "NegativeSize", // Should behave like ZeroSize
			size:      -5,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				// The generator treats sizes < 2 special cases, -5 doesn't fit 0 or 1.
				// Let's trace the code: targetSize becomes -5. if targetSize < 2 is true.
				// if targetSize == 1 (false). Returns empty content. Size should be 0.
				checkFileSize(t, path, 0)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.json", tc.name))

			// --- Execute ---
			err := generator.Generate(outPath, tc.size) //

			// --- Assert Error ---
			if tc.expectErr {
				if err == nil {
					t.Errorf("Generate(%q, %d) expected an error, but got nil", outPath, tc.size)
				} else if tc.errSubstring != "" && !strings.Contains(err.Error(), tc.errSubstring) {
					t.Errorf("Generate(%q, %d) error = %q, expected error containing %q", outPath, tc.size, err.Error(), tc.errSubstring)
				}
				return // Don't check file properties if error was expected
			}
			if err != nil {
				// Check for the warning message about final size mismatch, don't fail the test for it
				// as the generator might sometimes be slightly off due to padding complexities.
				// However, the primary checkFileSize assertion below WILL fail if the size is wrong.
				if strings.Contains(err.Error(), "Final size") && strings.Contains(err.Error(), "does not match target") {
					t.Logf("Generate(%q, %d) returned a non-fatal warning: %v", outPath, tc.size, err)
				} else {
					t.Fatalf("Generate(%q, %d) returned unexpected error: %v", outPath, tc.size, err)
				}
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
		// Read content for debugging if size mismatches
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			content = []byte(fmt.Sprintf("failed to read file: %v", readErr))
		}
		t.Errorf("Generated file %q size = %d, want %d.\nContent (up to 500 bytes):\n%s",
			path, info.Size(), expectedSize, limitString(string(content), 500))
	}
}

// Helper to check JSON validity
func checkJsonValidity(t *testing.T, path string, expectValid bool) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s for JSON validation: %v", path, err)
	}
	if len(content) == 0 && expectValid {
		t.Errorf("Expected valid JSON but file %q is empty", path)
		return
	}
	if len(content) == 0 && !expectValid {
		return // Empty file is not valid JSON, matching expectation
	}

	isValid := json.Valid(content)
	if isValid != expectValid {
		errMsg := fmt.Sprintf("JSON validity for %q is %t, want %t.", path, isValid, expectValid)
		if !isValid && expectValid { // Add content snippet if it should have been valid
			errMsg += fmt.Sprintf("\nContent (up to 500 bytes):\n%s", limitString(string(content), 500))
		}
		t.Error(errMsg)
	}
}

// Helper to limit string length for logging
func limitString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

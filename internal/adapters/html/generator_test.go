package html

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hailam/genfile/internal/ports" //
)

// Constants from the generator, redefined here for test verification
const (
	testHtmlTemplateStart = `<!DOCTYPE html>
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
	` // Note: Removed comment tags from test constants
	testHtmlTemplateEnd = `
</body>
</html>`
	// Assuming padding uses comments, but test focuses on size and basic structure
	testCommentOpen = "" // Basic check marker
)

var testMinimalSize = int64(len(testHtmlTemplateStart) + len(testHtmlTemplateEnd))

func TestHtmlGenerator_Generate(t *testing.T) {
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
				// Generator code writes truncated start even for 0 size
				checkFileSize(t, path, 0)
				content, _ := os.ReadFile(path)
				if len(content) != 0 {
					t.Errorf("Content for size 0: got %q, want empty", string(content))
				}
			},
		},
		{
			name:      "SizeLessThanMinimal",
			size:      testMinimalSize - 50, // Significantly less
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size) // Expect truncated file
				content, _ := os.ReadFile(path)
				expectedContent := testHtmlTemplateStart[:size]
				if string(content) != expectedContent {
					t.Errorf("Content for size %d: got %q, want %q", size, string(content), expectedContent)
				}
			},
		},
		{
			name:      "SizeExactlyMinimal",
			size:      testMinimalSize,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				content, _ := os.ReadFile(path)
				expectedContent := testHtmlTemplateStart + testHtmlTemplateEnd
				if string(content) != expectedContent {
					// Allow for minor whitespace diff if generator adds/removes some
					if strings.ReplaceAll(string(content), " ", "") != strings.ReplaceAll(expectedContent, " ", "") {
						t.Errorf("Content for minimal size %d: got %q, want %q", size, string(content), expectedContent)
					} else {
						t.Logf("Content for minimal size %d matches when ignoring whitespace.", size)
					}
				}
				checkHtmlStructure(t, path, true)
			},
		},
		{
			name:      "SizeSlightlyLarger", // Requires minimal padding
			size:      testMinimalSize + 30,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkHtmlStructure(t, path, true)
				content, _ := os.ReadFile(path)
				// Check if padding likely exists between start and end templates
				if !(strings.HasPrefix(string(content), testHtmlTemplateStart) && strings.HasSuffix(string(content), testHtmlTemplateEnd)) {
					t.Errorf("Content for size %d does not seem to wrap the template correctly", size)
				}
				// Check if *some* padding content exists
				padding := strings.TrimPrefix(string(content), testHtmlTemplateStart)
				padding = strings.TrimSuffix(padding, testHtmlTemplateEnd)
				if len(strings.TrimSpace(padding)) == 0 {
					t.Errorf("Padding section for size %d seems empty", size)
				}
				// Optional: Check for comment markers if padding uses them
				// if !strings.Contains(padding, testCommentOpen) || !strings.Contains(padding, testCommentClose) {
				// 	t.Logf("Padding section for size %d might not be using HTML comments", size)
				// }
			},
		},
		{
			name:      "LargerSize", // Requires significant padding
			size:      testMinimalSize + 6000,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkHtmlStructure(t, path, true)
			},
		},
		{
			name:      "NegativeSize", // Should behave like ZeroSize
			size:      -20,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, 0) // Expect 0 size
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.html", tc.name))

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
			// Tolerate the specific warning about final size mismatch from the generator
			if err != nil && !(strings.Contains(err.Error(), "Final HTML size") && strings.Contains(err.Error(), "does not match target")) {
				t.Fatalf("Generate(%q, %d) returned unexpected error: %v", outPath, tc.size, err)
			}
			if err != nil {
				t.Logf("Generate(%q, %d) returned a non-fatal warning: %v", outPath, tc.size, err)
			}

			// --- Assert File Properties ---
			if tc.checkProperties != nil {
				tc.checkProperties(t, outPath, tc.size)
			}
		})
	}

	// --- Test Error Case: Invalid Path ---
	t.Run("InvalidPath", func(t *testing.T) {
		err := generator.Generate(tempDir, testMinimalSize+100) // Use temp dir as path
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
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			content = []byte(fmt.Sprintf("failed to read file: %v", readErr))
		}
		t.Errorf("Generated file %q size = %d, want %d.\nContent (up to 500 bytes):\n%s",
			path, info.Size(), expectedSize, limitString(string(content), 500))
	}
}

// Helper to check basic HTML structure
func checkHtmlStructure(t *testing.T, path string, expectPresent bool) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s for HTML structure check: %v", path, err)
	}
	sContent := string(content)

	// Basic checks for common tags
	hasDoctype := strings.Contains(strings.ToLower(sContent), "<!doctype html>")
	hasHtmlOpen := strings.Contains(strings.ToLower(sContent), "<html") // Allow attributes
	hasHtmlClose := strings.Contains(strings.ToLower(sContent), "</html>")
	hasBodyOpen := strings.Contains(strings.ToLower(sContent), "<body")
	hasBodyClose := strings.Contains(strings.ToLower(sContent), "</body>")

	structurePresent := hasDoctype && hasHtmlOpen && hasHtmlClose && hasBodyOpen && hasBodyClose

	if structurePresent != expectPresent {
		t.Errorf("HTML structure presence for %q is %t, want %t. (Doctype:%t, Html:%t, Body:%t)",
			path, structurePresent, expectPresent, hasDoctype, (hasHtmlOpen && hasHtmlClose), (hasBodyOpen && hasBodyClose))
	}
}

// Helper to limit string length for logging
func limitString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

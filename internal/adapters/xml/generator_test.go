package xml

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hailam/genfile/internal/ports" //
)

// Constants from the generator, redefined here for test verification
const (
	testXmlDeclaration = `<?xml version="1.0" encoding="UTF-8"?>`
	testRootTagOpen    = `<generatedRoot>`
	testRootTagClose   = `</generatedRoot>`
	testCommentOpen    = "<!-- "
	testCommentClose   = ` -->`
)

var testMinimalContent = testXmlDeclaration + "\n" + testRootTagOpen + testRootTagClose
var testMinimalSize = int64(len(testMinimalContent))

func TestXmlGenerator_Generate(t *testing.T) {
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
				// Check content is empty
				content, _ := os.ReadFile(path)
				if len(content) != 0 {
					t.Errorf("Content for size 0: got %q, want empty", string(content))
				}
			},
		},
		{
			name:      "SizeLessThanMinimal",
			size:      testMinimalSize - 10,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size) // Expect truncated file
				content, _ := os.ReadFile(path)
				expectedContent := testMinimalContent[:size]
				if string(content) != expectedContent {
					t.Errorf("Content for size %d: got %q, want %q", size, string(content), expectedContent)
				}
				// XML validity check would fail here, which is expected
			},
		},
		{
			name:      "SizeExactlyMinimal",
			size:      testMinimalSize,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkXmlStructure(t, path, true) // Should be valid XML
				content, _ := os.ReadFile(path)
				if string(content) != testMinimalContent {
					t.Errorf("Content for minimal size %d: got %q, want %q", size, string(content), testMinimalContent)
				}
			},
		},
		{
			name:      "SizeSlightlyLarger", // Requires minimal padding
			size:      testMinimalSize + 20,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkXmlStructure(t, path, true) // Should still be valid XML
				content, _ := os.ReadFile(path)
				if !strings.Contains(string(content), testCommentOpen) || !strings.Contains(string(content), testCommentClose) {
					t.Errorf("Content for size %d did not contain expected XML comment markers", size)
				}
			},
		},
		{
			name:      "LargerSize", // Requires significant padding
			size:      testMinimalSize + 5000,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkXmlStructure(t, path, true)
				content, _ := os.ReadFile(path)
				// Check that padding likely exists between root tags
				if !strings.Contains(string(content), testRootTagOpen+testCommentOpen) && !strings.Contains(string(content), testCommentClose+testRootTagClose) {
					// Allow for potential whitespace between tags and comments too
					if !strings.Contains(string(content), testCommentOpen) || !strings.Contains(string(content), testCommentClose) {
						t.Errorf("Content for size %d does not appear to have comments between root tags", size)
					} else {
						t.Logf("Padding comment likely exists for size %d, but not immediately adjacent to root tags.", size)
					}
				}
			},
		},
		{
			name:      "NegativeSize", // Should behave like ZeroSize
			size:      -10,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, 0) // Expect 0 size
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.xml", tc.name))

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
			if err != nil && !(strings.Contains(err.Error(), "Final XML size") && strings.Contains(err.Error(), "does not match target")) {
				t.Fatalf("Generate(%q, %d) returned unexpected error: %v", outPath, tc.size, err)
			}
			if err != nil {
				// Log the warning but continue test if it's just the size mismatch warning
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
		// Read content for debugging if size mismatches
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			content = []byte(fmt.Sprintf("failed to read file: %v", readErr))
		}
		t.Errorf("Generated file %q size = %d, want %d.\nContent (up to 500 bytes):\n%s",
			path, info.Size(), expectedSize, limitString(string(content), 500))
	}
}

// Helper to check basic XML structure and validity
func checkXmlStructure(t *testing.T, path string, expectValid bool) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s for XML validation: %v", path, err)
	}
	if len(content) == 0 && expectValid {
		t.Errorf("Expected valid XML but file %q is empty", path)
		return
	}
	if len(content) == 0 && !expectValid {
		return // Empty file is not valid XML, matching expectation
	}

	// Basic structure checks
	sContent := string(content)
	hasDecl := strings.HasPrefix(sContent, testXmlDeclaration)
	hasOpenTag := strings.Contains(sContent, testRootTagOpen)
	hasCloseTag := strings.Contains(sContent, testRootTagClose)

	if expectValid && (!hasDecl || !hasOpenTag || !hasCloseTag) {
		t.Errorf("XML basic structure check failed for %q: Declaration=%t, OpenTag=%t, CloseTag=%t",
			path, hasDecl, hasOpenTag, hasCloseTag)
	}

	// Use encoding/xml to check for well-formedness
	decoder := xml.NewDecoder(strings.NewReader(sContent))
	isValid := true
	for {
		_, err := decoder.Token()
		if err != nil {
			// io.EOF means end of file, successfully parsed
			if err.Error() == "EOF" { // Handle EOF specifically
				break
			}
			// Any other error means it's not well-formed
			isValid = false
			t.Logf("XML validation error for %q: %v", path, err) // Log the specific XML error
			break
		}
	}

	if isValid != expectValid {
		errMsg := fmt.Sprintf("XML validity for %q is %t, want %t.", path, isValid, expectValid)
		if !isValid && expectValid {
			errMsg += fmt.Sprintf("\nContent (up to 500 bytes):\n%s", limitString(sContent, 500))
		}
		t.Error(errMsg)
	}
}

// Helper to limit string length for logging (same as before)
func limitString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

package zip

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time" // Import time package

	"github.com/hailam/genfile/internal/ports" //
)

// Helper function directly within the test to calculate expected overhead
// Now includes setting the Modified time to match the generator.
func calculateTestOverhead(entryName string) int64 {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	// --- MATCHING THE GENERATOR'S HEADER CREATION ---
	hdr := &zip.FileHeader{
		Name:     entryName,
		Method:   zip.Store,
		Modified: time.Now(), // Add this line
	}
	// --- END MATCHING ---
	_, err := zw.CreateHeader(hdr)
	if err != nil {
		// This shouldn't fail in a simple test setup
		panic(fmt.Sprintf("test overhead calculation failed: %v", err))
	}
	err = zw.Close()
	if err != nil {
		panic(fmt.Sprintf("test overhead calculation failed on close: %v", err))
	}
	return int64(buf.Len())
}

func TestZipGenerator_Generate(t *testing.T) {
	generator := New() //

	// Ensure it implements the interface
	var _ ports.FileGenerator = generator

	tempDir := t.TempDir() // Create a temporary directory for test files

	// Calculate expected overhead for the standard dummy entry
	const entryName = "dummy.bin"
	expectedOverhead := calculateTestOverhead(entryName)
	if expectedOverhead <= 0 {
		t.Fatalf("Calculated test overhead is non-positive (%d), cannot proceed", expectedOverhead)
	}
	t.Logf("Calculated test overhead for '%s': %d bytes", entryName, expectedOverhead)

	testCases := []struct {
		name         string
		size         int64
		expectErr    bool
		errSubstring string // Substring to check in error message
		checkContent bool   // Whether to verify zip content
	}{
		{"ExactOverheadSize", expectedOverhead, false, "", true},
		{"SlightlyOverOverhead", expectedOverhead + 10, false, "", true},
		{"LargerSize", expectedOverhead + 15000, false, "", true},
		{"TooSmallSize", expectedOverhead - 1, true, "too small", false},
		{"ZeroSize", 0, true, "too small", false}, // Also too small
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.zip", tc.name))

			// --- Execute ---
			err := generator.Generate(outPath, tc.size) //

			// --- Assert Error ---
			if tc.expectErr {
				if err == nil {
					t.Errorf("Generate(%q, %d) expected an error, but got nil", outPath, tc.size)
				} else if tc.errSubstring != "" && !bytes.Contains([]byte(err.Error()), []byte(tc.errSubstring)) {
					t.Errorf("Generate(%q, %d) error = %q, expected error containing %q", outPath, tc.size, err.Error(), tc.errSubstring)
				}
				return // Don't check file size/content if error was expected
			}
			if err != nil {
				t.Fatalf("Generate(%q, %d) returned unexpected error: %v", outPath, tc.size, err)
			}

			// --- Assert File Existence and Size ---
			info, statErr := os.Stat(outPath)
			if statErr != nil {
				if os.IsNotExist(statErr) {
					t.Fatalf("Generate(%q, %d) did not create the file", outPath, tc.size)
				} else {
					t.Fatalf("Error stating generated file %q: %v", outPath, statErr)
				}
			}

			if info.Size() != tc.size {
				t.Errorf("Generated file %q size = %d, want %d", outPath, info.Size(), tc.size)
			}

			// --- Assert Zip Content (Optional but Recommended) ---
			if tc.checkContent && info.Size() > 0 { // Only check content if file has size
				zr, err := zip.OpenReader(outPath)
				if err != nil {
					t.Fatalf("Failed to open generated zip file %q: %v", outPath, err)
				}
				defer zr.Close()

				if len(zr.File) != 1 {
					t.Errorf("Generated zip %q contains %d files, want 1", outPath, len(zr.File))
				} else {
					fileEntry := zr.File[0]
					if fileEntry.Name != entryName {
						t.Errorf("Zip entry name = %q, want %q", fileEntry.Name, entryName)
					}
					if fileEntry.Method != zip.Store {
						t.Errorf("Zip entry method = %d, want %d (Store)", fileEntry.Method, zip.Store)
					}

					// Calculate expected data size within the entry
					expectedDataSize := uint64(tc.size - expectedOverhead)
					if fileEntry.UncompressedSize64 != expectedDataSize {
						t.Errorf("Zip entry UncompressedSize64 = %d, want %d", fileEntry.UncompressedSize64, expectedDataSize)
					}

					// Verify actual data size by reading the entry
					rc, err := fileEntry.Open()
					if err != nil {
						t.Fatalf("Failed to open zip entry %q: %v", fileEntry.Name, err)
					}
					defer rc.Close()
					// Count bytes read from the entry
					bytesRead, err := io.Copy(io.Discard, rc) // Read all data and discard
					if err != nil {
						t.Fatalf("Failed to read data from zip entry %q: %v", fileEntry.Name, err)
					}
					if uint64(bytesRead) != expectedDataSize {
						t.Errorf("Read %d bytes from zip entry %q, want %d", bytesRead, fileEntry.Name, expectedDataSize)
					}
				}
			}
		})
	}

	// --- Test Error Case: Invalid Path ---
	t.Run("InvalidPath", func(t *testing.T) {
		// Use the temp directory itself as the output path, which should fail os.Create
		err := generator.Generate(tempDir, expectedOverhead+100) // Use a valid size
		if err == nil {
			t.Errorf("Generate(%q, ...) expected an error for invalid path, but got nil", tempDir)
		}
	})
}

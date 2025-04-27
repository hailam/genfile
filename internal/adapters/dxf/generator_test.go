package dxf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hailam/genfile/internal/ports" //
	"github.com/yofu/dxf"                      // Import dxf lib used by generator
)

// Helper to estimate minimum size by generating a base DXF in the test
func estimateMinDxfSize(t *testing.T, tempDir string) int64 {
	t.Helper()
	tempPath := filepath.Join(tempDir, "min_dxf_calc.dxf")
	dwg := dxf.NewDrawing()
	dwg.Line(0.0, 0.0, 0.0, 100.0, 100.0, 0.0) // Match generator's minimal content
	err := dwg.SaveAs(tempPath)
	if err != nil {
		t.Fatalf("Failed to save minimal DXF for size calculation: %v", err)
	}
	info, err := os.Stat(tempPath)
	if err != nil {
		t.Fatalf("Failed to stat minimal DXF for size calculation: %v", err)
	}
	os.Remove(tempPath) // Clean up temp file
	return info.Size()
}

func TestDxfGenerator_Generate(t *testing.T) {
	generator := New() //

	// Ensure it implements the interface
	var _ ports.FileGenerator = generator

	tempDir := t.TempDir() // Create a temporary directory for test files

	// Estimate minimum size
	minSize := estimateMinDxfSize(t, tempDir)
	t.Logf("Estimated minimum DXF size: %d bytes", minSize)
	if minSize <= 0 {
		t.Fatalf("Estimated minimum DXF size (%d) is invalid", minSize)
	}

	testCases := []struct {
		name            string
		filename        string // Filename including extension (.dxf or .dwg)
		targetSize      int64
		expectErr       bool
		errSubstring    string
		checkProperties func(t *testing.T, path string, size int64)
	}{
		// --- .dxf extension tests ---
		{
			name:         "DXF_ZeroSize",
			filename:     "test_zerosize.dxf",
			targetSize:   0,
			expectErr:    true, // Cannot be zero size
			errSubstring: "minimum DXF is",
		},
		{
			name:         "DXF_TooSmallSize",
			filename:     "test_toosmall.dxf",
			targetSize:   minSize - 1,
			expectErr:    true,
			errSubstring: "minimum DXF is",
		},
		{
			name:       "DXF_ExactMinSize",
			filename:   "test_exactmin.dxf",
			targetSize: minSize,
			expectErr:  false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size, 0) // Expect exact size
				checkDxfStructure(t, path)
			},
		},
		{
			name:       "DXF_SlightlyLarger", // Requires padding
			filename:   "test_slightlylarger.dxf",
			targetSize: minSize + 100,
			expectErr:  false,
			checkProperties: func(t *testing.T, path string, size int64) {
				// Padding with "999\n...\n" might slightly overshoot if remainder < 5 bytes.
				// Allow tolerance up to 4 bytes over.
				checkFileSize(t, path, size, 4)
				checkDxfStructure(t, path)
				// Optionally check for "999" comments
			},
		},
		{
			name:       "DXF_LargerSize", // Requires more padding
			filename:   "test_larger.dxf",
			targetSize: minSize + 5000,
			expectErr:  false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size, 4) // Allow tolerance
				checkDxfStructure(t, path)
			},
		},
		// --- .dwg extension tests (should behave same way, just file named .dwg) ---
		{
			name:         "DWG_TooSmallSize",
			filename:     "test_toosmall.dwg", // Note extension
			targetSize:   minSize - 1,
			expectErr:    true,
			errSubstring: "minimum DXF is",
		},
		{
			name:       "DWG_ExactMinSize",
			filename:   "test_exactmin.dwg", // Note extension
			targetSize: minSize,
			expectErr:  false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size, 0) // Expect exact size
				checkDxfStructure(t, path)      // Content is still DXF
			},
		},
		{
			name:       "DWG_LargerSize",
			filename:   "test_larger.dwg", // Note extension
			targetSize: minSize + 6000,
			expectErr:  false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size, 4) // Allow tolerance
				checkDxfStructure(t, path)      // Content is still DXF
			},
		},
		{
			name:         "DXF_NegativeSize", // Should error as too small
			filename:     "test_negative.dxf",
			targetSize:   -100,
			expectErr:    true,             // Generator doesn't handle negative, SaveAs fails likely
			errSubstring: "minimum DXF is", // Error comes from size check relative to minSize
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(tempDir, tc.filename)

			// --- Execute ---
			err := generator.Generate(outPath, tc.targetSize) //

			// --- Assert Error ---
			if tc.expectErr {
				if err == nil {
					t.Errorf("Generate(%q, %d) expected an error, but got nil", outPath, tc.targetSize)
				} else if tc.errSubstring != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.errSubstring)) {
					t.Errorf("Generate(%q, %d) error = %q, expected error containing %q", outPath, tc.targetSize, err.Error(), tc.errSubstring)
				}
				// Clean up potentially created temp .dxf file if the final rename failed (for .dwg tests)
				if strings.HasSuffix(tc.filename, ".dwg") && err != nil {
					tempDxfPath := strings.TrimSuffix(outPath, ".dwg") + ".dxf"
					os.Remove(tempDxfPath)
				}
				return
			}
			if err != nil {
				t.Fatalf("Generate(%q, %d) returned unexpected error: %v", outPath, tc.targetSize, err)
			}

			// --- Assert File Properties ---
			if tc.checkProperties != nil {
				tc.checkProperties(t, outPath, tc.targetSize)
			}
		})
	}

	// --- Test Error Case: Invalid Path ---
	t.Run("InvalidPath", func(t *testing.T) {
		// Generate into the temp dir itself
		err := generator.Generate(tempDir, minSize+100)
		if err == nil {
			t.Errorf("Generate(%q, ...) expected an error for invalid path, but got nil", tempDir)
		}
		// Clean up potentially created temp file from SaveAs if it failed during append/rename
		os.Remove(tempDir + ".dxf") // Assuming SaveAs creates .dxf if path has no extension
	})
}

// Helper to check file existence and size, allowing for minor oversize due to padding
func checkFileSize(t *testing.T, path string, expectedSize int64, tolerance int64) {
	t.Helper()
	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			t.Fatalf("Generate did not create the file %q", path)
		} else {
			t.Fatalf("Error stating generated file %q: %v", path, statErr)
		}
	}

	actualSize := info.Size()
	diff := actualSize - expectedSize // Check difference (allow positive diff up to tolerance)

	if !(diff >= 0 && diff <= tolerance) {
		t.Errorf("Generated file %q size = %d, want %d (with +%d tolerance)", path, actualSize, expectedSize, tolerance)
	} else if diff > 0 {
		t.Logf("Note: Generated file %q size = %d, which is %d bytes larger than target %d (within DXF padding tolerance)", path, actualSize, diff, expectedSize)
	}
}

// Helper to check basic DXF structure (starts with common markers)
func checkDxfStructure(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s for DXF structure check: %v", path, err)
	}
	sContent := string(content)

	// Check for common DXF start sequence (e.g., "0\nSECTION") and end marker ("0\nEOF")
	// Use case-insensitive compare for section name if needed, although spec is usually uppercase.
	hasStart := strings.Contains(sContent, "0\nSECTION") || strings.Contains(sContent, "  0\nSECTION") // Allow for potential leading spaces
	hasEnd := strings.Contains(sContent, "\n0\nEOF")                                                   // Check includes preceding newline

	if !hasStart || !hasEnd {
		t.Errorf("DXF structure check failed for %q: StartSequence=%t, EndSequence=%t", path, hasStart, hasEnd)
	}
}

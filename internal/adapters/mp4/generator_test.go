package mp4

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4" // Import mp4ff for potential validation
	"github.com/hailam/genfile/internal/ports"
)

// Helper to estimate minimum size - this might need adjustment based on actual output
func estimateMinMp4Size() (int64, error) {
	// Generate the minimal H.264 elementary stream frame
	h264 := generateH264Elementary()
	hlen := int64(len(h264))

	// Create a basic init structure (ftyp + moov)
	init := mp4.CreateEmptyInit()
	tid := init.Moov.Mvhd.NextTrackID
	init.Moov.Mvhd.NextTrackID++
	trak := mp4.CreateEmptyTrak(tid, 90000, "video", "und")
	init.Moov.AddChild(trak)
	init.Moov.Mvex.AddChild(mp4.CreateTrex(tid))
	trak.SetAVCDescriptor("avc1", [][]byte{sps[4:]}, [][]byte{pps[4:]}, true)

	// Encode ftyp + moov to a buffer to get their size
	var initBuf bytes.Buffer
	ftyp := mp4.NewFtyp("isom", 0x200, []string{"isom", "iso2", "avc1", "mp41"})
	err := ftyp.Encode(&initBuf)
	if err != nil {
		return -1, fmt.Errorf("failed to encode ftyp for min size calc: %w", err)
	}
	err = init.Moov.Encode(&initBuf)
	if err != nil {
		return -1, fmt.Errorf("failed to encode moov for min size calc: %w", err)
	}
	initSize := int64(initBuf.Len())

	// Minimum mdat requires header (8 bytes) + one frame
	minMdatSize := int64(8) + hlen

	return initSize + minMdatSize, nil
}

func TestMp4Generator_Generate(t *testing.T) {
	generator := New() //

	// Ensure it implements the interface
	var _ ports.FileGenerator = generator

	tempDir := t.TempDir() // Create a temporary directory for test files

	// Estimate minimum size
	minSize, minSizeErr := estimateMinMp4Size()
	if minSizeErr != nil {
		t.Fatalf("Failed to estimate minimum MP4 size: %v", minSizeErr)
	}
	t.Logf("Estimated minimum MP4 size: %d bytes", minSize)
	if minSize <= 0 {
		t.Fatalf("Estimated minimum MP4 size (%d) is invalid", minSize)
	}

	testCases := []struct {
		name            string
		size            int64
		expectErr       bool
		errSubstring    string                                      // Substring to check in error message (case-insensitive)
		checkProperties func(t *testing.T, path string, size int64) // Function to check size and basic structure
	}{
		{
			name:         "ZeroSize",
			size:         0,
			expectErr:    true,
			errSubstring: "too small",
		},
		{
			name:         "TooSmallSize",
			size:         minSize - 1,
			expectErr:    true,
			errSubstring: "too small",
		},
		{
			name:      "ExactMinSize", // Contains minimal header + 1 frame
			size:      minSize,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkMp4Structure(t, path, size) // Check basic MP4 box structure
			},
		},
		{
			name:      "SlightlyOverMinSize", // Minimal header + 1 frame + padding
			size:      minSize + 50,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkMp4Structure(t, path, size)
			},
		},
		{
			name:      "SizeRequiringMultipleFrames", // Should contain > 1 frame repetition
			size:      minSize * 3,                   // Arbitrarily pick size likely needing >1 frame
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkMp4Structure(t, path, size)
				// Could add check for mdat size > 2 * hlen if hlen is accessible/recalculated
			},
		},
		{
			name:      "LargerSizeWithPadding", // Multiple frames + padding
			size:      minSize*3 + 100,
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkMp4Structure(t, path, size)
			},
		},
		{
			name:         "NegativeSize", // Should error as too small
			size:         -500,
			expectErr:    true,
			errSubstring: "too small",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.mp4", tc.name))

			// --- Execute ---
			err := generator.Generate(outPath, tc.size) //

			// --- Assert Error ---
			if tc.expectErr {
				if err == nil {
					t.Errorf("Generate(%q, %d) expected an error, but got nil", outPath, tc.size)
				} else if tc.errSubstring != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.errSubstring)) { // Case-insensitive check
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
		err := generator.Generate(tempDir, minSize+100) // Use temp dir as path
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

// Helper to check basic MP4 box structure (ftyp, moov, mdat)
func checkMp4Structure(t *testing.T, path string, totalSize int64) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file %s for MP4 structure check: %v", path, err)
	}
	defer f.Close()

	// Read first few bytes to find ftyp
	header := make([]byte, 8)
	_, err = io.ReadFull(f, header)
	if err != nil {
		t.Fatalf("Failed to read first 8 bytes of %s: %v", path, err)
	}
	ftypSize := int64(binary.BigEndian.Uint32(header[0:4]))
	ftypType := string(header[4:8])
	if ftypType != "ftyp" {
		t.Errorf("Expected 'ftyp' box at start, got %q", ftypType)
	}

	// Seek past ftyp and read next box header
	_, err = f.Seek(ftypSize, io.SeekStart)
	if err != nil {
		t.Fatalf("Failed to seek past ftyp box in %s: %v", path, err)
	}
	_, err = io.ReadFull(f, header)
	if err != nil {
		t.Fatalf("Failed to read second box header of %s: %v", path, err)
	}
	moovSize := int64(binary.BigEndian.Uint32(header[0:4]))
	moovType := string(header[4:8])
	if moovType != "moov" {
		t.Errorf("Expected 'moov' box after 'ftyp', got %q", moovType)
	}

	// Seek past moov and read next box header
	_, err = f.Seek(ftypSize+moovSize, io.SeekStart)
	if err != nil {
		t.Fatalf("Failed to seek past moov box in %s: %v", path, err)
	}
	_, err = io.ReadFull(f, header)
	if err != nil {
		// Allow EOF if file size is exactly ftyp+moov (unlikely for this generator)
		if err == io.ErrUnexpectedEOF && totalSize == ftypSize+moovSize {
			t.Logf("File ends after moov box as expected for size %d", totalSize)
			return
		}
		t.Fatalf("Failed to read third box header of %s: %v", path, err)
	}
	mdatSize := int64(binary.BigEndian.Uint32(header[0:4]))
	mdatType := string(header[4:8])
	if mdatType != "mdat" {
		t.Errorf("Expected 'mdat' box after 'moov', got %q", mdatType)
	}

	// Check if total size matches sum of box sizes found
	calculatedSize := ftypSize + moovSize + mdatSize
	if calculatedSize != totalSize {
		t.Errorf("Sum of box sizes (%d + %d + %d = %d) does not match total file size %d",
			ftypSize, moovSize, mdatSize, calculatedSize, totalSize)
	}
}

// generateH264Elementary needs to be copied here if not exported,
// or tests need access to the original package's internal functions.
// For simplicity, copy the implementation (ensure it's identical).
// --- Copied from mp4/generator.go ---
var sps_test = []byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
var pps_test = []byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xce, 0x38, 0x80}
var sliceHeader_test = []byte{0x00, 0x00, 0x00, 0x01, 0x05, 0x88, 0x84, 0x21, 0xa0}
var macroblockHeader_test = []byte{0x0d, 0x00}

const (
	widthMB_test  = 128 / 16
	heightMB_test = 96 / 16
)

// --- End Copied Section ---

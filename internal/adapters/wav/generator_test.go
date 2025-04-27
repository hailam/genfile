package wav

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hailam/genfile/internal/ports" //
)

const (
	wavHeaderSize      = 44
	expectedSampleRate = 44100
	expectedChannels   = 1
	expectedBitsSample = 8
	expectedAudioFmt   = 1 // PCM
)

func TestWavGenerator_Generate(t *testing.T) {
	generator := New() //

	// Ensure it implements the interface
	var _ ports.FileGenerator = generator

	tempDir := t.TempDir() // Create a temporary directory for test files

	testCases := []struct {
		name            string
		size            int64
		expectErr       bool
		errSubstring    string                                      // Substring to check in error message (case-insensitive)
		checkProperties func(t *testing.T, path string, size int64) // Function to check size and header
	}{
		{
			name:         "ZeroSize",
			size:         0,
			expectErr:    true,
			errSubstring: "must be at least 44 bytes",
		},
		{
			name:         "TooSmallSize",
			size:         wavHeaderSize - 1, // 43 bytes
			expectErr:    true,
			errSubstring: "must be at least 44 bytes",
		},
		{
			name:      "ExactHeaderSize",
			size:      wavHeaderSize, // 44 bytes
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkWavHeader(t, path, size) // Check header validity
			},
		},
		{
			name:      "SlightlyOverHeader",
			size:      wavHeaderSize + 10, // 54 bytes
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkWavHeader(t, path, size) // Check header validity
			},
		},
		{
			name:      "LargerSize",
			size:      10 * 1024, // 10 KB
			expectErr: false,
			checkProperties: func(t *testing.T, path string, size int64) {
				checkFileSize(t, path, size)
				checkWavHeader(t, path, size) // Check header validity
			},
		},
		{
			name:         "NegativeSize", // Should error as too small
			size:         -100,
			expectErr:    true,
			errSubstring: "must be at least 44 bytes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.wav", tc.name))

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
		err := generator.Generate(tempDir, wavHeaderSize+100) // Use temp dir as path
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

// Helper to check the WAV header fields
func checkWavHeader(t *testing.T, path string, totalSize int64) {
	t.Helper()
	if totalSize < wavHeaderSize {
		t.Logf("Skipping header check for file size %d < %d", totalSize, wavHeaderSize)
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s for WAV header validation: %v", path, err)
	}

	if len(content) < wavHeaderSize {
		t.Fatalf("File %s content length %d is less than expected header size %d", path, len(content), wavHeaderSize)
	}

	header := content[:wavHeaderSize]

	// Check RIFF Chunk
	if string(header[0:4]) != "RIFF" {
		t.Errorf("Header bytes 0-3: got %q, want %q", string(header[0:4]), "RIFF")
	}
	expectedRiffSize := uint32(totalSize - 8)
	actualRiffSize := binary.LittleEndian.Uint32(header[4:8])
	if actualRiffSize != expectedRiffSize {
		t.Errorf("Header bytes 4-7 (RIFF Size): got %d, want %d", actualRiffSize, expectedRiffSize)
	}
	if string(header[8:12]) != "WAVE" {
		t.Errorf("Header bytes 8-11: got %q, want %q", string(header[8:12]), "WAVE")
	}

	// Check "fmt " Sub-chunk
	if string(header[12:16]) != "fmt " {
		t.Errorf("Header bytes 12-15: got %q, want %q", string(header[12:16]), "fmt ")
	}
	expectedFmtSize := uint32(16) // For PCM
	actualFmtSize := binary.LittleEndian.Uint32(header[16:20])
	if actualFmtSize != expectedFmtSize {
		t.Errorf("Header bytes 16-19 (Fmt Size): got %d, want %d", actualFmtSize, expectedFmtSize)
	}
	actualAudioFmt := binary.LittleEndian.Uint16(header[20:22])
	if actualAudioFmt != expectedAudioFmt {
		t.Errorf("Header bytes 20-21 (Audio Format): got %d, want %d (PCM)", actualAudioFmt, expectedAudioFmt)
	}
	actualChannels := binary.LittleEndian.Uint16(header[22:24])
	if actualChannels != expectedChannels {
		t.Errorf("Header bytes 22-23 (Num Channels): got %d, want %d", actualChannels, expectedChannels)
	}
	actualSampleRate := binary.LittleEndian.Uint32(header[24:28])
	if actualSampleRate != expectedSampleRate {
		t.Errorf("Header bytes 24-27 (Sample Rate): got %d, want %d", actualSampleRate, expectedSampleRate)
	}
	// ByteRate = SampleRate * NumChannels * BitsPerSample/8
	expectedByteRate := uint32(expectedSampleRate * expectedChannels * (expectedBitsSample / 8))
	actualByteRate := binary.LittleEndian.Uint32(header[28:32])
	if actualByteRate != expectedByteRate {
		t.Errorf("Header bytes 28-31 (Byte Rate): got %d, want %d", actualByteRate, expectedByteRate)
	}
	// BlockAlign = NumChannels * BitsPerSample/8
	expectedBlockAlign := uint16(expectedChannels * (expectedBitsSample / 8))
	actualBlockAlign := binary.LittleEndian.Uint16(header[32:34])
	if actualBlockAlign != expectedBlockAlign {
		t.Errorf("Header bytes 32-33 (Block Align): got %d, want %d", actualBlockAlign, expectedBlockAlign)
	}
	actualBitsSample := binary.LittleEndian.Uint16(header[34:36])
	if actualBitsSample != expectedBitsSample {
		t.Errorf("Header bytes 34-35 (Bits Per Sample): got %d, want %d", actualBitsSample, expectedBitsSample)
	}

	// Check "data" Sub-chunk
	if string(header[36:40]) != "data" {
		t.Errorf("Header bytes 36-39: got %q, want %q", string(header[36:40]), "data")
	}
	expectedDataSize := uint32(totalSize - wavHeaderSize)
	actualDataSize := binary.LittleEndian.Uint32(header[40:44])
	if actualDataSize != expectedDataSize {
		t.Errorf("Header bytes 40-43 (Data Size): got %d, want %d", actualDataSize, expectedDataSize)
	}
}

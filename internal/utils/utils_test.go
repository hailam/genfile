package utils

import (
	"fmt"
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		// Valid cases
		{"500", 500, false},
		{"500B", 500, false},
		{"10k", 10 * 1024, false},
		{"10K", 10 * 1024, false},
		{"10kb", 10 * 1024, false},
		{"10KB", 10 * 1024, false},
		{"4m", 4 * 1024 * 1024, false},
		{"4M", 4 * 1024 * 1024, false},
		{"4MB", 4 * 1024 * 1024, false},
		{"1g", 1 * 1024 * 1024 * 1024, false},
		{"1G", 1 * 1024 * 1024 * 1024, false},
		{"1GB", 1 * 1024 * 1024 * 1024, false},
		{"1024", 1024, false},
		{"0", 0, false},
		{"0B", 0, false},
		{"0KB", 0, false},

		// Invalid cases
		{"", 0, true},        // Empty string
		{"-100", 0, true},    // Negative number (Sscanf might parse but logic could enforce non-negative) - current impl allows
		{"10P", 0, true},     // Unknown suffix
		{"KB", 0, true},      // No number
		{"10.5K", 0, true},   // Non-integer number part
		{"abc", 0, true},     // Non-numeric
		{"10 M B", 0, true},  // Space in suffix
		{"1 0 K B", 0, true}, // Space in number
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("Input_%s", tc.input), func(t *testing.T) {
			got, err := ParseSize(tc.input) //

			if (err != nil) != tc.wantErr {
				t.Errorf("ParseSize(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
				return
			}
			if !tc.wantErr && got != tc.expected {
				t.Errorf("ParseSize(%q) = %d, want %d", tc.input, got, tc.expected)
			}
		})
	}
}

// Add tests for WriteRandomBytes, PadZipExtend, ZipEntryOverhead, RandString if needed
func TestRandString(t *testing.T) {
	lengths := []int{0, 1, 10, 100}
	for _, length := range lengths {
		t.Run(fmt.Sprintf("Length_%d", length), func(t *testing.T) {
			s := RandString(length) //
			if len(s) != length {
				t.Errorf("RandString(%d) returned string of length %d, want %d", length, len(s), length)
			}
			// Optionally check character set if needed
			for _, r := range s {
				if r < 'A' || r > 'Z' {
					t.Errorf("RandString(%d) returned char '%c' outside expected A-Z range", length, r)
				}
			}
		})
	}
}

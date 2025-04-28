package pdf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPDFGenerator_Generate(t *testing.T) {
	// Create an instance of the generator
	// Assuming New() is exported from your pdf package correctly
	generator := New()

	// --- Test Case: Success ---
	t.Run("Success", func(t *testing.T) {
		// Test with a reasonable size (e.g., 10 KiB)
		targetSize := int64(10 * 1024)
		tempDir := t.TempDir()                           // Create a temporary directory for the test file
		outPath := filepath.Join(tempDir, "success.pdf") // Path for the output file

		err := generator.Generate(outPath, targetSize)
		require.NoError(t, err, "Generate should not return an error for valid size")

		// Verify file exists and has the correct size
		info, err := os.Stat(outPath)
		require.NoError(t, err, "os.Stat should not return an error for the generated file")
		require.NotNil(t, info, "File info should not be nil")
		require.Equal(t, targetSize, info.Size(), "Generated file size should match target size exactly")
	})

	// --- Test Case: Success Larger Size ---
	t.Run("SuccessLargerFile", func(t *testing.T) {
		// Test with a larger size (e.g., 1 MiB) to ensure calculations hold
		targetSize := int64(1 * 1024 * 1024) // 1 MiB
		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "success_large.pdf")

		err := generator.Generate(outPath, targetSize)
		require.NoError(t, err, "Generate should not return an error for valid large size")

		// Verify file exists and has the correct size
		info, err := os.Stat(outPath)
		require.NoError(t, err, "os.Stat should not return an error for the large generated file")
		require.NotNil(t, info, "File info should not be nil")
		require.Equal(t, targetSize, info.Size(), "Generated large file size should match target size exactly")
	})

	// --- Test Case: Error - Size Too Small ---
	t.Run("ErrorSizeTooSmall", func(t *testing.T) {
		// Use a size known to be below the minimum structure size constant in the implementation
		targetSize := int64(100) // Less than minStructureSize = 300
		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "too_small.pdf")

		err := generator.Generate(outPath, targetSize)
		require.Error(t, err, "Generate should return an error for size smaller than minimum")
		// Optional: Check for specific error content if desired
		require.ErrorContains(t, err, "too small for a minimal PDF structure", "Error message should indicate size is too small")

		// Verify file was likely not created or is empty
		_, err = os.Stat(outPath)
		require.ErrorIs(t, err, os.ErrNotExist, "File should not exist for failed generation due to small size")
	})

	// --- Test Case: Error - Zero Size ---
	t.Run("ErrorSizeZero", func(t *testing.T) {
		targetSize := int64(0)
		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "zero_size.pdf")

		err := generator.Generate(outPath, targetSize)
		require.Error(t, err, "Generate should return an error for zero size")
		require.ErrorContains(t, err, "too small", "Error message should indicate size is too small for zero size") // Should hit the min size check

		_, err = os.Stat(outPath)
		require.ErrorIs(t, err, os.ErrNotExist, "File should not exist for failed generation due to zero size")
	})

	// --- Test Case: Error - Negative Size ---
	t.Run("ErrorSizeNegative", func(t *testing.T) {
		targetSize := int64(-100)
		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "negative_size.pdf")

		err := generator.Generate(outPath, targetSize)
		require.Error(t, err, "Generate should return an error for negative size")
		require.ErrorContains(t, err, "too small", "Error message should indicate size is too small for negative size") // Should hit the min size check

		_, err = os.Stat(outPath)
		require.ErrorIs(t, err, os.ErrNotExist, "File should not exist for failed generation due to negative size")
	})

	// --- Test Case: Error - Invalid Output Path ---
	t.Run("ErrorInvalidPath", func(t *testing.T) {
		// Attempt to write to a path that likely cannot be created.
		// Note: This might depend on filesystem permissions and OS.
		targetSize := int64(1024)
		// Use a path within a non-existent directory
		invalidPath := filepath.Join(t.TempDir(), "nonexistent_dir", "invalid.pdf")
		// Or directly use a root path if permissions likely deny it (less reliable)
		// invalidPath := "/invalid.pdf"

		err := generator.Generate(invalidPath, targetSize)
		require.Error(t, err, "Generate should return an error for an invalid output path")
		// The error should come from os.Create failing
		require.ErrorIs(t, err, os.ErrNotExist, "Error should indicate path does not exist or cannot be created") // Or os.ErrPermission depending on path/OS
	})
}

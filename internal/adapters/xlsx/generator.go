package xlsx

import (
	"bytes"
	"fmt"
	"os"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
	"github.com/hailam/genfile/internal/utils"
	"github.com/xuri/excelize/v2"
)

func init() {
	factory.RegisterGenerator(ports.FileTypeXLSX, New()) //
}

type XlsxGenerator struct{}

func New() ports.FileGenerator {
	return &XlsxGenerator{}
}

// Generate creates an XLSX file, attempting to match the target size by adding cells
// and then padding. This version optimizes by checking size in memory.
func (g *XlsxGenerator) Generate(path string, targetSize int64) error {
	// 1) Compute overhead of pad.bin entry using the utility function
	padOH := utils.ZipEntryOverhead() //

	// --- Calculate Minimal Size (In Memory) ---
	bufMinimal := &bytes.Buffer{}
	f0 := excelize.NewFile()
	// Add minimal content to ensure basic structure exists
	f0.SetCellValue("Sheet1", "A1", "X")
	if err := f0.Write(bufMinimal); err != nil {
		return fmt.Errorf("failed to write minimal xlsx to buffer: %w", err)
	}
	minimal := int64(bufMinimal.Len())
	bufMinimal = nil // Release buffer memory
	f0 = nil         // Release excelize object memory

	// Check if target size is feasible
	if minimal+padOH > targetSize {
		// If even the base file + padding is too large, we can't generate it accurately.
		// Options: return error, or generate the minimal file anyway.
		// Current choice: return error as we can't meet the size requirement.
		return fmt.Errorf("target size %d too small for xlsx, minimum structure is %d + padding overhead %d", targetSize, minimal, padOH)
	}
	if targetSize == minimal+padOH {
		// If target size is exactly minimal + padding, generate minimal and pad
		fMin := excelize.NewFile()
		fMin.SetCellValue("Sheet1", "A1", "X")
		if err := fMin.SaveAs(path); err != nil {
			return fmt.Errorf("failed to save minimal xlsx file: %w", err)
		}
		return utils.PadZipExtend(path, targetSize) //
	}

	// --- Estimate Average Bytes Per Cell (In Memory) ---
	bufAvg := &bytes.Buffer{}
	fAvg := excelize.NewFile()
	const avgCellCount = 10
	const avgCellContent = "XXXXXXXXXXXXXXXXXXXX" // 20 chars
	for i := 1; i <= avgCellCount; i++ {
		cell, _ := excelize.CoordinatesToCellName(1, i)
		fAvg.SetCellValue("Sheet1", cell, avgCellContent)
	}
	if err := fAvg.Write(bufAvg); err != nil {
		// Non-fatal? Log warning and use a default avgCell value.
		fmt.Fprintf(os.Stderr, "Warning: failed to write avg xlsx to buffer: %v. Using default avgCell.\n", err)
		avgCell := int64(50) // Default fallback average cell size
		_ = avgCell          // Assign to avoid unused variable error if calculation below fails
	}

	avgSize := int64(bufAvg.Len())
	avgCell := (avgSize - minimal) / avgCellCount
	if avgCell < 1 {
		avgCell = 1 // Avoid division by zero or negative values
	}
	bufAvg = nil // Release buffer memory
	fAvg = nil   // Release excelize object memory

	// --- Estimate Cell Count and Find Optimal Count (In Memory) ---
	maxUsableContentSize := targetSize - padOH - minimal
	if maxUsableContentSize < 0 {
		maxUsableContentSize = 0 // Can't have negative content size
	}
	estCount := maxUsableContentSize / avgCell
	if estCount < 1 {
		// If calculation results in less than 1, but we know target > minimal+padOH,
		// we must need at least 1 cell beyond the minimal "A1".
		// However, our minimal already includes A1, so start checking from 1.
		estCount = 1
	}
	// Ensure estCount isn't excessively large if avgCell estimate was tiny
	// Add a reasonable upper bound if needed, e.g., max 1 million cells?

	var finalCount int = 0            // Use 0 to indicate not found yet
	var finalFileBuffer *bytes.Buffer // Buffer to hold the data of the best-fitting file

	fmt.Printf("XLSX: Target=%d, Minimal=%d, PadOH=%d, AvgCell=%d, EstCount=%d\n", targetSize, minimal, padOH, avgCell, estCount)

	// Iterate downwards from estimate to find the largest count that fits
	for cnt := estCount; cnt >= 1; cnt-- {
		currentBuf := &bytes.Buffer{} // Create in-memory buffer for this iteration
		f := excelize.NewFile()
		// Always add the base cell A1 included in 'minimal' calculation
		f.SetCellValue("Sheet1", "A1", "X")
		// Add additional cells up to cnt
		for r := 2; r <= int(cnt)+1; r++ { // Start from row 2, add 'cnt' more cells
			cell, _ := excelize.CoordinatesToCellName(1, r)
			// Use RandString or a fixed string for cell content
			f.SetCellValue("Sheet1", cell, utils.RandString(20)) //
		}

		// Write to buffer instead of disk
		if err := f.Write(currentBuf); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error writing xlsx (count %d) to buffer: %v\n", cnt, err)
			// Decide whether to continue or fail. Continuing might lead to wrong size.
			// Let's return error here, as failing to write means we can't judge size.
			return fmt.Errorf("error writing xlsx buffer for count %d: %w", cnt, err)
		}
		f = nil // Release excelize object memory for this iteration

		currentSize := int64(currentBuf.Len())

		// Check if this size fits when padding is added
		if currentSize+padOH <= targetSize {
			// This count fits. Store it and its buffer.
			finalCount = int(cnt)
			finalFileBuffer = currentBuf // Keep this buffer's content
			fmt.Printf("XLSX: Found fit with Count=%d, Size=%d (Total with PadOH: %d)\n", finalCount, currentSize, currentSize+padOH)
			break // Found the largest count that fits
		} else {
			// This count (cnt) is too large. Loop will try cnt-1.
			// Release memory for the buffer that was too large.
			currentBuf = nil
		}
	} // End of search loop

	// Handle cases where no fit was found
	if finalCount == 0 {
		// This means even cnt=1 was too large (or loop start estCount was < 1)
		// We already checked targetSize > minimal+padOH, so cnt=0 (minimal file) should fit.
		fmt.Println("XLSX: No count >= 1 fits. Generating minimal file.")
		// Generate the minimal file content again into finalFileBuffer
		finalFileBuffer = &bytes.Buffer{}
		fMinFinal := excelize.NewFile()
		fMinFinal.SetCellValue("Sheet1", "A1", "X")
		if err := fMinFinal.Write(finalFileBuffer); err != nil {
			return fmt.Errorf("failed to write final minimal xlsx to buffer: %w", err)
		}
		// finalCount remains 0, indicating minimal file content was used.
	}

	// --- Single Disk Write ---
	fmt.Printf("XLSX: Writing final file content (derived from count %d) to %s\n", finalCount, path)
	outFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create final output file %s: %w", path, err)
	}
	defer outFile.Close() // Ensure file is closed eventually

	_, err = outFile.Write(finalFileBuffer.Bytes())
	if err != nil {
		// Close is deferred, but return the write error
		return fmt.Errorf("failed to write final buffer to file %s: %w", path, err)
	}
	// Explicitly close before padding to ensure all data is flushed
	if err := outFile.Close(); err != nil {
		return fmt.Errorf("failed to close final file before padding %s: %w", path, err)
	}

	// --- Padding ---
	fmt.Printf("XLSX: Padding file %s to target size %d\n", path, targetSize)
	return utils.PadZipExtend(path, targetSize) //
}

package generators

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// generateXLSX creates an Excel file with enough random cells to approximate
// the targetSize, then pads exactly the remainder via ZIP comment.
func GenerateXLSX(path string, targetSize int64) error {
	// 1) Measure overhead with a single cell
	buf1 := &bytes.Buffer{}
	f1 := excelize.NewFile()
	sheet := f1.GetSheetName(0)
	f1.SetCellValue(sheet, "A1", "X")
	if err := f1.Write(buf1); err != nil {
		return err
	}
	overhead := int64(buf1.Len())

	if overhead >= targetSize {
		return fmt.Errorf("target %d < minimal XLSX %d", targetSize, overhead)
	}

	// 2) Measure avg per cell with 10 test cells
	buf2 := &bytes.Buffer{}
	f2 := excelize.NewFile()
	for i := 1; i <= 10; i++ {
		cell, _ := excelize.CoordinatesToCellName(1, i)
		f2.SetCellValue(sheet, cell, strings.Repeat("X", 20))
	}
	if err := f2.Write(buf2); err != nil {
		return err
	}
	size2 := int64(buf2.Len())
	avgCell := (size2 - overhead) / 10
	if avgCell < 1 {
		avgCell = 1
	}

	// 3) Compute pad entry overhead
	padOH := zipEntryOverhead()

	// 4) Determine how many cells we can fill
	usable := targetSize - overhead - padOH
	cellCount := usable / avgCell
	if cellCount < 1 {
		cellCount = 1
	}

	// 5) Build real workbook
	f := excelize.NewFile()
	rows := int(cellCount)
	for r := 1; r <= rows; r++ {
		cell, _ := excelize.CoordinatesToCellName(1, r)
		// random text ~20 chars
		txt := randString(20)
		f.SetCellValue(sheet, cell, txt)
	}
	if err := f.SaveAs(path); err != nil {
		return err
	}

	// 6) Pad the ZIP to exact size
	return padZipExtend(path, targetSize)
}

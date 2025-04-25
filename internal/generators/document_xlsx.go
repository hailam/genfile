package generators

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hailam/genfile/internal/utils"
	"github.com/xuri/excelize/v2"
)

// generateXLSX creates an Excel file with enough random cells to approximate
// the targetSize, then pads exactly the remainder via ZIP comment.
func GenerateXLSX(path string, targetSize int64) error {
	// 1) Compute overhead of pad.bin entry
	padOH := utils.ZipEntryOverhead()

	// 2) Measure minimal XLSX overhead
	buf := &bytes.Buffer{}
	f0 := excelize.NewFile()
	f0.SetCellValue("Sheet1", "A1", "X")
	if err := f0.Write(buf); err != nil {
		return err
	}
	minimal := int64(buf.Len())
	if minimal+padOH > targetSize {
		return fmt.Errorf("target %d too small (min %d + padOH %d)", targetSize, minimal, padOH)
	}

	// 3) Estimate average bytes per cell
	buf2 := &bytes.Buffer{}
	f2 := excelize.NewFile()
	for i := 1; i <= 10; i++ {
		cell, _ := excelize.CoordinatesToCellName(1, i)
		f2.SetCellValue("Sheet1", cell, strings.Repeat("X", 20))
	}
	if err := f2.Write(buf2); err != nil {
		return err
	}
	avgCell := (int64(buf2.Len()) - minimal) / 10
	if avgCell < 1 {
		avgCell = 1
	}

	// 4) Find maximal cellCount so that fileSize + padOH ≤ target
	maxUsable := targetSize - padOH
	// initial guess
	estCount := (maxUsable - minimal) / avgCell
	if estCount < 1 {
		estCount = 1
	}

	var finalCount int
	for cnt := estCount; cnt >= 1; cnt-- {
		// write cnt cells
		f := excelize.NewFile()
		for r := 1; r <= int(cnt); r++ {
			cell, _ := excelize.CoordinatesToCellName(1, r)
			f.SetCellValue("Sheet1", cell, utils.RandString(20))
		}
		if err := f.SaveAs(path); err != nil {
			return err
		}
		info, _ := os.Stat(path)
		if info.Size()+padOH <= targetSize {
			finalCount = int(cnt)
			break
		}
		// overshot → try one fewer
	}
	if finalCount == 0 {
		return errors.New("could not fit even one cell")
	}

	// 5) Re-write finalCount cells (to get the correct file)
	f := excelize.NewFile()
	for r := 1; r <= finalCount; r++ {
		cell, _ := excelize.CoordinatesToCellName(1, r)
		f.SetCellValue("Sheet1", cell, utils.RandString(20))
	}
	if err := f.SaveAs(path); err != nil {
		return err
	}

	// 6) Pad via zip-extend
	return utils.PadZipExtend(path, targetSize)
}

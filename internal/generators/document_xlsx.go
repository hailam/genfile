package generators

import (
	"fmt"
	"os"

	"github.com/hailam/genfile/internal/utils"
	"github.com/xuri/excelize/v2"
)

/*
XLSX (Office Open XML for Excel) files are actually ZIP archives containing XML files. To generate a minimal .xlsx:

    Use an Excel library like Excelize or tealeg/xlsx to create a simple workbook (maybe with one sheet and one cell of data). These libraries handle the internal structure (Content_Types, [Content_Types].xml, xl/workbook.xml, etc.) automatically​
    awesome-go.com
    .

    Save the workbook to a file.

    Check the size. If additional padding is needed, we can use a similar approach as with ZIP files (since XLSX is a zip archive). Specifically, we can open the output file with archive/zip and insert a dummy large entry that Excel will ignore. For instance, adding an extra file in the ZIP that isn't referenced by the workbook. Excel will generally ignore unrecognized files in the package (it may not even notice them).

    Alternatively, we can add a large innocuous XML part. For example, create a custom worksheet with lots of random data or a custom XML part with dummy content, and include it via the library if possible.

    The simplest reliable method: after creating the xlsx, append a ZIP comment exactly as we did for ZIP. XLSX files often tolerate a ZIP comment without issues (the comment is outside the actual XML structure).
*/

func GenerateXLSX(path string, size int64) error {
	// Create a new workbook with one sheet
	f := excelize.NewFile()
	sheetName := "Sheet1"
	// Write something in cell A1 just to have content
	f.SetCellValue(sheetName, "A1", "Dummy Data")
	// Save to file
	if err := f.SaveAs(path); err != nil {
		return err
	}
	// Check current size
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	baseSize := info.Size()
	if baseSize > size {
		return fmt.Errorf("cannot generate XLSX of %d bytes, minimum is %d", size, baseSize)
	}
	if baseSize == size {
		return nil
	}
	// If padding is needed, use ZIP techniques:
	return utils.PadZipFile(path, size)
}

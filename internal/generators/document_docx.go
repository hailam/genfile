package generators

import (
	"fmt"
	"os"

	"github.com/hailam/genfile/internal/utils"
	"github.com/unidoc/unioffice/document"
)

func GenerateDOCX(path string, size int64) error {
	// Create a new Word document
	doc := document.New()
	defer doc.Close()
	// Add a paragraph with some text
	para := doc.AddParagraph()
	para.AddRun().AddText("Lorem ipsum dolor sit amet.")
	// Save the document
	if err := doc.SaveToFile(path); err != nil {
		return err
	}
	// Get size and pad if necessary
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	baseSize := info.Size()
	if baseSize > size {
		return fmt.Errorf("cannot generate DOCX of %d bytes, minimum is %d", size, baseSize)
	}
	if baseSize == size {
		return nil
	}
	return utils.PadZipFile(path, size)
}

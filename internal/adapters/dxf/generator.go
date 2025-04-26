package dxf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
	"github.com/yofu/dxf"
)

func init() {
	factory.RegisterGenerator(ports.FileTypeDXF, New()) //
	factory.RegisterGenerator(ports.FileTypeDWG, New()) //
}

type DxfGenerator struct{}

func New() ports.FileGenerator {
	return &DxfGenerator{}
}

// Generate creates a DXF file at the specified path with the given size.
func (g *DxfGenerator) Generate(path string, size int64) error {
	// Create a simple DXF drawing
	dwg := dxf.NewDrawing()
	// Add a line (for example) so the drawing isn't empty
	dwg.Line(0.0, 0.0, 0.0, 100.0, 100.0, 0.0)
	// Save to a DXF file (ASCII DXF format)
	tempPath := path
	if strings.ToLower(filepath.Ext(path)) == ".dwg" {
		// Ensure it has .dxf extension for output, we'll rename later
		tempPath = strings.TrimSuffix(path, filepath.Ext(path)) + ".dxf"
	}
	if err := dwg.SaveAs(tempPath); err != nil {
		return err
	}
	// Now tempPath is a DXF file. Let's get its size.
	info, err := os.Stat(tempPath)
	if err != nil {
		return err
	}
	baseSize := info.Size()
	if baseSize > size {
		return fmt.Errorf("cannot generate drawing of %d bytes, minimum DXF is %d bytes", size, baseSize)
	}
	if baseSize == size {
		// If extension was originally .dwg, rename the .dxf to .dwg
		if tempPath != path {
			return os.Rename(tempPath, path)
		}
		return nil
	}
	// Open the file for append (text mode)
	f, err := os.OpenFile(tempPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	// Pad with DXF comment lines (999 code).
	// Each comment line in DXF takes the form:
	// 999\nYour comment text\n
	// That's 4 bytes for "999\n" + N bytes of text + 1 byte for newline. Max text length ~255.
	padNeeded := size - baseSize
	// We'll add as many full-length (255 char) comments as needed.
	commentLine := strings.Repeat("X", 255) // 255 'X' characters as comment text
	lineBytes := []byte("999\n" + commentLine + "\n")
	bytesPerComment := int64(len(lineBytes)) // which should be 4 + 255 + 1 = 260 bytes
	fullLines := padNeeded / bytesPerComment
	remainder := padNeeded % bytesPerComment
	// Write full-length comment lines
	for i := int64(0); i < fullLines; i++ {
		if _, err := f.Write(lineBytes); err != nil {
			return err
		}
	}
	if remainder > 0 {
		// If remainder is less than 5, we cannot create a smaller-than-minimum comment (which is "999\n\n" = 5 bytes).
		// We'll just create one more comment line slightly longer than needed (overshoot a bit) if necessary.
		if remainder < 5 {
			remainder = 5
		}
		// remainder includes both code+newline and text+newline.
		// If remainder = 5, that implies 0-length text comment.
		textLen := remainder - 5 // subtract "999\n" (4) and trailing newline (1)
		if textLen < 0 {
			textLen = 0
		}
		comment := strings.Repeat("X", int(textLen))
		lastLine := "999\n" + comment + "\n"
		if _, err := f.Write([]byte(lastLine)); err != nil {
			return err
		}
	}
	f.Close()
	// If original request was .dwg extension, rename .dxf to .dwg.
	if tempPath != path {
		return os.Rename(tempPath, path)
	}
	return nil
}

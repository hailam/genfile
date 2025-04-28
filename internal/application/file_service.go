package application

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hailam/genfile/internal/ports"
)

// FileService orchestrates file generation by parsing sizes, selecting
// the correct generator, and invoking it.
type FileService struct {
	factory ports.GeneratorFactory
	parser  ports.SizeParser
}

// NewFileService constructs a FileService with the given factory and parser.
func NewFileService(factory ports.GeneratorFactory, parser ports.SizeParser) *FileService {
	return &FileService{factory: factory, parser: parser}
}

// CreateFile generates a file at outPath of size sizeSpec (e.g., "10MB").
// It parses the size, infers the file type from the extension, looks up the
// appropriate generator, and runs it.
func (s *FileService) CreateFile(outPath, sizeSpec string) error {
	// 1. Parse human-readable size into bytes
	sizeBytes, err := s.parser.Parse(sizeSpec)
	if err != nil {
		return fmt.Errorf("invalid size '%s': %w", sizeSpec, err)
	}

	// 2. Determine file type from extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(outPath), "."))
	fileType, err := mapExtensionToFileType(ext)
	if err != nil {
		return err
	}

	// 3. Retrieve the generator for this type
	generator, err := s.factory.For(fileType)
	if err != nil {
		return fmt.Errorf("no generator for type '%s': %w", fileType, err)
	}

	// 4. Invoke the generator
	if err := generator.Generate(outPath, sizeBytes); err != nil {
		return fmt.Errorf("failed to generate %s: %w", outPath, err)
	}
	return nil
}

// mapExtensionToFileType maps file extensions to FileType constants.
func mapExtensionToFileType(ext string) (ports.FileType, error) {
	switch ext {
	case "txt", "text":
		return ports.FileTypeTXT, nil
	case "png":
		return ports.FileTypePNG, nil
	case "jpg", "jpeg":
		return ports.FileTypeJPEG, nil
	case "mp4":
		return ports.FileTypeMP4, nil
	case "m4v":
		return ports.FileTypeM4V, nil
	case "wav":
		return ports.FileTypeWAV, nil
	case "dwg":
		return ports.FileTypeDWG, nil
	case "dxf":
		return ports.FileTypeDXF, nil
	case "zip":
		return ports.FileTypeZIP, nil
	case "xlsx":
		return ports.FileTypeXLSX, nil
	case "docx":
		return ports.FileTypeDOCX, nil
	case "pdf":
		return ports.FileTypePDF, nil
	case "csv":
		return ports.FileTypeCSV, nil
	case "json":
		return ports.FileTypeJSON, nil
	case "html":
		return ports.FileTypeHTML, nil
	case "md":
		return ports.FileTypeMD, nil
	case "log":
		return ports.FileTypeLog, nil
	case "xml":
		return ports.FileTypeXML, nil
	case "gif":
		return ports.FileTypeGIF, nil
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

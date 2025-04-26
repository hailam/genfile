package factory

import (
	"fmt"

	"github.com/hailam/genfile/internal/adapters/docx"
	"github.com/hailam/genfile/internal/adapters/dxf"
	"github.com/hailam/genfile/internal/adapters/jpeg"
	"github.com/hailam/genfile/internal/adapters/mp4"
	"github.com/hailam/genfile/internal/adapters/pdf"
	"github.com/hailam/genfile/internal/adapters/png"
	"github.com/hailam/genfile/internal/adapters/txt"
	"github.com/hailam/genfile/internal/adapters/wav"
	"github.com/hailam/genfile/internal/adapters/xlsx"
	"github.com/hailam/genfile/internal/adapters/zip"
	"github.com/hailam/genfile/internal/ports"
)

// StaticGeneratorFactory provides concrete implementations for FileGenerators.
type StaticGeneratorFactory struct {
	generators map[ports.FileType]ports.FileGenerator
}

// NewStaticGeneratorFactory creates a new factory with pre-initialized generators.
func NewStaticGeneratorFactory() ports.GeneratorFactory {
	return &StaticGeneratorFactory{
		generators: map[ports.FileType]ports.FileGenerator{
			ports.FileTypeTXT:  txt.New(),
			ports.FileTypePNG:  png.New(),
			ports.FileTypeJPEG: jpeg.New(),
			ports.FileTypeMP4:  mp4.New(),
			ports.FileTypeM4V:  mp4.New(), // M4V uses the MP4 generator
			ports.FileTypeWAV:  wav.New(),
			ports.FileTypeDWG:  dxf.New(), // We actually Don't have a dedicated DWG generator
			ports.FileTypeZIP:  zip.New(),
			ports.FileTypeXLSX: xlsx.New(),
			ports.FileTypeDOCX: docx.New(),
			ports.FileTypePDF:  pdf.New(),
		},
	}
}

// For returns the appropriate FileGenerator for the given FileType.
func (f *StaticGeneratorFactory) For(t ports.FileType) (ports.FileGenerator, error) {
	gen, ok := f.generators[t]
	if !ok {
		return nil, fmt.Errorf("unsupported file type: %s", t)
	}
	return gen, nil
}

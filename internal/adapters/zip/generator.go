package zip

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/hailam/genfile/internal/ports"
	"github.com/hailam/genfile/internal/utils"
)

type ZipGenerator struct{}

func New() ports.FileGenerator {
	return &ZipGenerator{}
}

func (g *ZipGenerator) Generate(path string, size int64) error {
	const entryName = "dummy.bin"

	// 1. Compute overhead: size of a ZIP with dummy.bin but zero payload.
	overhead := zipEntryOverhead(entryName)
	if overhead > size {
		return fmt.Errorf("requested size %d too small, minimum is %d", size, overhead)
	}

	// 2. Payload bytes to write
	dataBytes := size - overhead

	// 3. Open file and ZIP writer
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)

	// 4. Create uncompressed entry
	hdr := &zip.FileHeader{
		Name:     entryName,
		Method:   zip.Store,
		Modified: time.Now(),
	}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		zw.Close()
		f.Close()
		return err
	}

	// 5. Fill with random data
	if err := utils.WriteRandomBytes(w, dataBytes); err != nil {
		zw.Close()
		f.Close()
		return err
	}

	// 6. Close ZIP (writes central directory + EOCD)
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// zipEntryOverhead returns the byte-length of a ZIP containing
// exactly one STORE-method entry named `name` with zero payload.
func zipEntryOverhead(name string) int64 {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	// Create a STORE entry with no data
	hdr := &zip.FileHeader{Name: name, Method: zip.Store}
	zw.CreateHeader(hdr)
	zw.Close()
	return int64(buf.Len())
}

package zip

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"time" // Ensure time is imported

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
	"github.com/hailam/genfile/internal/utils"
)

func init() {
	factory.RegisterGenerator(ports.FileTypeZIP, New()) //
}

type ZipGenerator struct{}

func New() ports.FileGenerator {
	return &ZipGenerator{}
}

func (g *ZipGenerator) Generate(path string, size int64) error {
	const entryName = "dummy.bin"

	// 1. Compute overhead: size of a ZIP with dummy.bin but zero payload.
	//    Use the internal helper which MUST match the header creation below.
	overhead := zipEntryOverhead(entryName) // Use the updated helper below
	if overhead <= 0 {
		// Basic sanity check
		return fmt.Errorf("internal error: calculated zip overhead is %d", overhead)
	}
	if size < overhead { // Check if size is less than the *correct* overhead
		return fmt.Errorf("requested size %d too small, minimum is %d", size, overhead)
	}

	// 2. Payload bytes to write
	dataBytes := size - overhead

	// 3. Open file and ZIP writer
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() // Ensure file is closed eventually
	zw := zip.NewWriter(f)
	defer zw.Close() // Ensure zip writer is closed eventually

	// 4. Create uncompressed entry - THIS MUST MATCH THE OVERHEAD CALCULATION
	hdr := &zip.FileHeader{
		Name:     entryName,
		Method:   zip.Store,
		Modified: time.Now(), // Include modification time here
	}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		// No need to close zw/f explicitly due to defer
		return fmt.Errorf("failed to create zip header: %w", err)
	}

	// 5. Fill with random data
	if dataBytes > 0 { // Only write if there's data to write
		if err := utils.WriteRandomBytes(w, dataBytes); err != nil {
			// No need to close zw/f explicitly due to defer
			return fmt.Errorf("failed to write zip data: %w", err)
		}
	}

	// 6. Close ZIP (writes central directory + EOCD) - Handled by defer zw.Close()
	// 7. Close File - Handled by defer f.Close()

	// Explicitly check errors from deferred Close calls
	if err := zw.Close(); err != nil {
		// f.Close() will still run due to its defer
		return fmt.Errorf("failed to close zip writer: %w", err)
	}
	// f.Close() error is implicitly handled by returning from the function

	return nil // Success
}

// zipEntryOverhead returns the byte-length of a ZIP containing
// exactly one STORE-method entry named `name` with zero payload.
// THIS MUST MATCH THE HEADER FIELDS USED IN Generate!
func zipEntryOverhead(name string) int64 {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	// Create a STORE entry with no data, matching Generate's header settings
	hdr := &zip.FileHeader{
		Name:     name,
		Method:   zip.Store,
		Modified: time.Now(), // Include modification time here too!
	}
	// We don't actually need the writer, just create the header effects
	_, err := zw.CreateHeader(hdr)
	if err != nil {
		// Should not happen in this controlled scenario
		fmt.Fprintf(os.Stderr, "Warning: zipEntryOverhead internal CreateHeader failed: %v\n", err)
		return -1 // Indicate error
	}
	// Close the writer to finalize the structure (central directory etc.)
	err = zw.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: zipEntryOverhead internal Close failed: %v\n", err)
		return -1 // Indicate error
	}
	return int64(buf.Len())
}

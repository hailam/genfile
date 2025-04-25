package generators

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hailam/genfile/internal/utils"
)

func GenerateZIP(path string, size int64) error {
	if size < 22 {
		return fmt.Errorf("minimum ZIP size is 22 bytes (ZIP header overhead)")
	}
	// Create the zip file
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	// We'll create one entry "dummy.bin"
	header := &zip.FileHeader{Name: "dummy.bin"}
	header.Method = zip.Store // no compression
	// Set an arbitrary modification time
	header.Modified = time.Now()
	// Add the file to the ZIP
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	// Calculate how many bytes to put in this entry.
	// We have to account for central directory and end-of-CD overhead. Simplistically, assume:
	// Local header ~ (30 + len(Name)) bytes, Central directory entry ~ (46 + len(Name)) bytes, EOCD 22 bytes.
	nameLen := len(header.Name)
	overhead := int64(30 + nameLen + 46 + nameLen + 22)
	if overhead > size {
		return fmt.Errorf("requested size too small to form a valid zip")
	}
	dataBytes := size - overhead
	// Write random content for the entry
	if err := utils.WriteRandomBytes(w, dataBytes); err != nil {
		return err
	}
	// (The zip writer will compute CRC and sizes for central directory because we didn't specify them.)
	// Close the zip to write central directory
	if err := zw.Close(); err != nil {
		return err
	}
	// Now, check if the size matches exactly. If not, we can adjust using a ZIP comment.
	info, err := f.Stat()
	if err != nil {
		return err
	}
	finalSize := info.Size()
	if finalSize == size {
		return nil
	}
	// If there's a mismatch, we can add a comment. We need to rewrite the zip file with a comment because
	// archive/zip doesn't let us set a comment after closing. We'll reopen and append a comment field manually.
	diff := size - finalSize
	if diff < 0 {
		return fmt.Errorf("zip overshot size by %d bytes, cannot reduce", -diff)
	}
	if diff > 65535 {
		return fmt.Errorf("need to add %d bytes, which exceeds ZIP comment limit", diff)
	}
	// Open file in append mode and add a comment to EOCD:
	// EOCD structure (22 bytes without comment):
	// [4] signature, [2] disk no., [2] start disk, [2] disk entries, [2] total entries, [4] central dir size, [4] central dir offset, [2] comment length, [comment bytes]...
	// The archive/zip writer would have written comment length = 0 originally.
	// We will overwrite the last 2 bytes with the new comment length and append comment bytes.
	if diff > 0 {
		// diff bytes of comment (we'll just use 'X')
		comment := bytes.Repeat([]byte("X"), int(diff))
		// Reopen file
		f2, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		// Seek to end to append comment bytes
		if _, err := f2.Seek(0, io.SeekEnd); err != nil {
			f2.Close()
			return err
		}
		if _, err := f2.Write(comment); err != nil {
			f2.Close()
			return err
		}
		// Now write the comment length into the appropriate place in EOCD.
		// The EOCD starts at finalSize-22 (since no comment). After adding comment, it's size-22 still points to where length was.
		//newSize := finalSize + diff
		// Comment length field offset from end = newSize - (diff + 2) (the original end plus where the 2-byte length field was).
		// Actually easier: since original had 0 length, we just write diff as uint16 at position finalSize (which was original EOF).
		if diff > 0xFFFF {
			f2.Close()
			return fmt.Errorf("comment too large")
		}
		commentLenField := make([]byte, 2)
		binary.LittleEndian.PutUint16(commentLenField, uint16(diff))
		// Seek to original end-of-file (which is finalSize)
		if _, err := f2.Seek(finalSize-2, io.SeekStart); err != nil {
			f2.Close()
			return err
		}
		// Overwrite the 2 bytes of comment length at the end of original EOCD.
		if _, err := f2.Write(commentLenField); err != nil {
			f2.Close()
			return err
		}
		f2.Close()
	}
	return nil
}

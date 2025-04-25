package utils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"
)

// parseSize parses strings like "500", "10K", "4MB", "1G" into a number of bytes.
func ParseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, errors.New("size string is empty")
	}
	// Suffix multipliers
	suffixes := map[string]int64{
		"B": 1,
		"K": 1024, "KB": 1024,
		"M": 1024 * 1024, "MB": 1024 * 1024,
		"G": 1024 * 1024 * 1024, "GB": 1024 * 1024 * 1024,
	}
	sizeStr = strings.TrimSpace(sizeStr)
	sizeStr = strings.ToUpper(sizeStr)
	// Find numeric part and suffix part
	var numPart string
	var suffix string
	for i, r := range sizeStr {
		if r < '0' || r > '9' {
			numPart = sizeStr[:i]
			suffix = strings.TrimSpace(sizeStr[i:])
			break
		}
	}
	if numPart == "" {
		// No explicit suffix, entire string is number or with suffix at end
		// e.g., "1024" or "1024MB"
		// Try to separate digits and letters
		numPart = sizeStr
		suffix = ""
		for len(suffix) < len(sizeStr) && suffixes[numPart] == 0 {
			// if the whole string isn't a known suffix key (unlikely), just break
			break
		}
	}
	// Parse numeric part
	var baseVal int64
	_, err := fmt.Sscanf(numPart, "%d", &baseVal)
	if err != nil {
		return 0, fmt.Errorf("invalid size number: %v", err)
	}
	if suffix == "" || suffix == "B" {
		return baseVal, nil
	}
	mult, ok := suffixes[suffix]
	if !ok {
		return 0, fmt.Errorf("unknown size suffix '%s'", suffix)
	}
	return baseVal * mult, nil
}

// writeRandomBytes writes n random bytes to w. It uses a fixed seed for reproducibility (optional).
func WriteRandomBytes(w io.Writer, n int64) error {
	bufSize := 64 * 1024
	buf := make([]byte, bufSize)
	// Use math/rand for speed (cryptographic quality not needed for noise)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var written int64 = 0
	for written < n {
		toWrite := bufSize
		if n-written < int64(bufSize) {
			toWrite = int(n - written)
		}
		// Fill buffer with random bytes
		for i := 0; i < toWrite; i++ {
			buf[i] = byte(r.Intn(256))
		}
		_, err := w.Write(buf[:toWrite])
		if err != nil {
			return err
		}
		written += int64(toWrite)
	}
	return nil
}

// padZipFile adds a zip comment or dummy entry to reach the exact size.
func PadZipFile(zipPath string, targetSize int64) error {
	// Similar to what we did in generateZIP: open the zip, add comment
	file, err := os.OpenFile(zipPath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	currSize := stat.Size()
	if currSize == targetSize {
		return nil
	}
	if currSize > targetSize {
		return fmt.Errorf("file already larger than target size")
	}
	diff := targetSize - currSize
	if diff <= 65535 {
		// We can use the comment field
		// Seek to end and write diff bytes of comment
		if _, err := file.Seek(0, io.SeekEnd); err != nil {
			return err
		}
		comment := bytes.Repeat([]byte("X"), int(diff))
		if _, err := file.Write(comment); err != nil {
			return err
		}
		// Write the comment length into EOCD (last 2 bytes of original file)
		if diff > 0xFFFF {
			return fmt.Errorf("diff too large for comment")
		}
		// Overwrite original EOCD comment length (which is 0) with diff
		if _, err := file.Seek(currSize-2, io.SeekStart); err != nil {
			return err
		}
		var lenBuf [2]byte
		binary.LittleEndian.PutUint16(lenBuf[:], uint16(diff))
		if _, err := file.Write(lenBuf[:]); err != nil {
			return err
		}
		return nil
	}
	// If diff > 65535, we need another strategy: perhaps add a dummy entry in the zip.
	// For brevity, not fully implemented here. We could copy zip entries to a new file and insert a large file entry.
	return fmt.Errorf("padding >65KB not implemented")
}

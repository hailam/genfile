package utils

import (
	"archive/zip"
	"bytes"
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
func PadZipExtend(inPath string, targetSize int64) error {
	info, err := os.Stat(inPath)
	if err != nil {
		return err
	}
	orig := info.Size()
	if orig > targetSize {
		return fmt.Errorf("file is %d > target %d", orig, targetSize)
	}
	// compute overhead of empty pad.bin entry
	padOH := ZipEntryOverhead()
	needed := targetSize - orig - padOH

	// open original
	zr, err := zip.OpenReader(inPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	tmp := inPath + ".tmp"
	outF, _ := os.Create(tmp)
	zw := zip.NewWriter(outF)

	// copy entries
	for _, f := range zr.File {
		hdr := f.FileHeader
		w, _ := zw.CreateHeader(&hdr)
		r, _ := f.Open()
		io.Copy(w, r)
		r.Close()
	}
	// create pad.bin uncompressed
	padHdr := &zip.FileHeader{Name: "pad.bin", Method: zip.Store}
	w, _ := zw.CreateHeader(padHdr)
	zero := make([]byte, 64*1024)
	for needed > 0 {
		chunk := int64(len(zero))
		if chunk > needed {
			chunk = needed
		}
		w.Write(zero[:chunk])
		needed -= chunk
	}
	zw.Close()
	outF.Close()
	os.Rename(tmp, inPath)
	return nil
}

// zipEntryOverhead returns the byte-length of an empty 'pad.bin' entry in a new ZIP.
func ZipEntryOverhead() int64 {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	hdr := &zip.FileHeader{Name: "pad.bin", Method: zip.Store}
	zw.CreateHeader(hdr)
	zw.Close()
	return int64(buf.Len())
}

// randString returns a random A–Z string of length n.
func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('A' + rand.Intn(26))
	}
	return string(b)
}

package generators

import (
	"math/rand"
	"os"
	"time"
)

func GenerateTXT(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// We will generate random printable ASCII characters (space 0x20 to '~' 0x7E).
	const printableStart, printableEnd = 0x20, 0x7E
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	bufSize := 8192
	buf := make([]byte, bufSize)
	var written int64
	for written < size {
		toWrite := bufSize
		if size-written < int64(bufSize) {
			toWrite = int(size - written)
		}
		for i := 0; i < toWrite; i++ {
			buf[i] = byte(printableStart + r.Intn(printableEnd-printableStart+1))
		}
		if _, err := f.Write(buf[:toWrite]); err != nil {
			return err
		}
		written += int64(toWrite)
	}
	return f.Sync()
}

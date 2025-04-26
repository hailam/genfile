package txt

import (
	"math/rand/v2"
	"os"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
)

func init() {
	gen := New()
	factory.RegisterGenerator(ports.FileTypeTXT, gen)
	factory.RegisterGenerator(ports.FileTypeLog, gen) // Register for LOG
	factory.RegisterGenerator(ports.FileTypeMD, gen)
}

type TxtGenerator struct{}

func New() ports.FileGenerator {
	return &TxtGenerator{}
}

func (g *TxtGenerator) Generate(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// We will generate random printable ASCII characters (space 0x20 to '~' 0x7E).
	const printableStart, printableEnd = 0x20, 0x7E
	bufSize := 8192
	buf := make([]byte, bufSize)
	var written int64
	for written < size {
		toWrite := bufSize
		if size-written < int64(bufSize) {
			toWrite = int(size - written)
		}
		for i := 0; i < toWrite; i++ {
			buf[i] = byte(printableStart + rand.IntN(printableEnd-printableStart+1))
		}
		if _, err := f.Write(buf[:toWrite]); err != nil {
			return err
		}
		written += int64(toWrite)
	}
	return f.Sync()
}

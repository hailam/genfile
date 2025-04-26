package wav

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/hailam/genfile/internal/ports"
	"github.com/hailam/genfile/internal/utils"
)

type WavGenerator struct{}

func New() ports.FileGenerator {
	return &WavGenerator{}
}

func (g *WavGenerator) Generate(path string, size int64) error {
	// WAV header is 44 bytes for PCM 8-bit mono.
	if size < 44 {
		return fmt.Errorf("WAV size must be at least 44 bytes for header")
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dataBytes := size - 44
	var buf [4]byte

	// RIFF header
	// ChunkID "RIFF"
	if _, err := f.Write([]byte("RIFF")); err != nil {
		return err
	}
	// ChunkSize (4 bytes) = 36 + dataBytes (size-8 overall).
	riffSize := uint32(size - 8)
	binary.LittleEndian.PutUint32(buf[:4], riffSize)
	if _, err := f.Write(buf[:4]); err != nil {
		return err
	}
	// Format "WAVE"
	if _, err := f.Write([]byte("WAVE")); err != nil {
		return err
	}
	// Subchunk1 ID "fmt "
	if _, err := f.Write([]byte("fmt ")); err != nil {
		return err
	}
	// Subchunk1 size (PCM) = 16
	binary.LittleEndian.PutUint32(buf[:4], 16)
	if _, err := f.Write(buf[:4]); err != nil {
		return err
	}
	// Audio format (PCM=1), 2 bytes
	binary.LittleEndian.PutUint16(buf[:2], 1)
	if _, err := f.Write(buf[:2]); err != nil {
		return err
	}
	// NumChannels = 1 (mono), 2 bytes
	binary.LittleEndian.PutUint16(buf[:2], 1)
	if _, err := f.Write(buf[:2]); err != nil {
		return err
	}
	// SampleRate = 44100, 4 bytes
	binary.LittleEndian.PutUint32(buf[:4], 44100)
	if _, err := f.Write(buf[:4]); err != nil {
		return err
	}
	// ByteRate = SampleRate * NumChannels * BitsPerSample/8. For 8-bit mono: 44100 * 1 * 1 = 44100.
	binary.LittleEndian.PutUint32(buf[:4], 44100)
	if _, err := f.Write(buf[:4]); err != nil {
		return err
	}
	// BlockAlign = NumChannels * BitsPerSample/8 = 1*1 = 1 (1 byte per sample frame)
	binary.LittleEndian.PutUint16(buf[:2], 1)
	if _, err := f.Write(buf[:2]); err != nil {
		return err
	}
	// BitsPerSample = 8
	binary.LittleEndian.PutUint16(buf[:2], 8)
	if _, err := f.Write(buf[:2]); err != nil {
		return err
	}
	// Subchunk2 ID "data"
	if _, err := f.Write([]byte("data")); err != nil {
		return err
	}
	// Subchunk2 size = dataBytes
	binary.LittleEndian.PutUint32(buf[:4], uint32(dataBytes))
	if _, err := f.Write(buf[:4]); err != nil {
		return err
	}
	// Now write dataBytes of random audio samples (8-bit each)
	if err := utils.WriteRandomBytes(f, dataBytes); err != nil {
		return err
	}
	return f.Sync()
}

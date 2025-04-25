package generators

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
)

func GenerateMP4(path string, size int64) error {
	// Use ffmpeg to create a small MP4 (32x32, 1-second, silent video).
	// This command generates 30 frames of static noise and encodes to MP4 (H.264 codec).
	ffmpegCmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "noise=c=gray:s=32x32:d=1",
		"-vcodec", "libx264", "-pix_fmt", "yuv420p",
		"-t", "1", "-movflags", "faststart",
		path)
	ffmpegCmd.Stderr = os.Stderr
	ffmpegCmd.Stdout = os.Stdout
	if err := ffmpegCmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg execution failed: %v", err)
	}
	// Check file size
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	currentSize := info.Size()
	if currentSize > size {
		return fmt.Errorf("cannot generate MP4 of %d bytes, minimum is %d bytes", size, currentSize)
	}
	if currentSize == size {
		return nil // done
	}
	// Pad the MP4 by appending a 'free' box (an MP4 padding atom)&#8203;:contentReference[oaicite:9]{index=9}.
	// We'll append a free box of (size - currentSize) bytes.
	padBytes := size - currentSize
	if padBytes < 8 {
		// MP4 boxes have at least 8 bytes (4 bytes size, 4 bytes type).
		// If padBytes is less than 8, we'll make one 8-byte 'free' box and accept slight overshoot.
		padBytes = 8
	}
	// Construct 'free' box: 4-byte size and 4-byte type.
	// The size includes these 4 bytes, so if padBytes is total size, we write padBytes and "free".
	freeBoxHeader := make([]byte, 8)
	binary.BigEndian.PutUint32(freeBoxHeader[:4], uint32(padBytes))
	copy(freeBoxHeader[4:], []byte("free"))
	extra := padBytes - 8
	padData := make([]byte, extra) // the rest of the box content
	// Write to file by appending
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(freeBoxHeader); err != nil {
		return err
	}
	if extra > 0 {
		if _, err := f.Write(padData); err != nil {
			return err
		}
	}
	return nil
}

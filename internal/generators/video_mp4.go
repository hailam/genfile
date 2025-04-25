package generators

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/Eyevinn/mp4ff/mp4"
)

// Hard-coded Annex B NAL units from Ben Mesander’s “World’s Smallest H.264 Encoder”
// SPS: start-code + baseline profile, level 1.0 parameters :contentReference[oaicite:0]{index=0}
var sps = []byte{
	0x00, 0x00, 0x00, 0x01,
	0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2,
}

// PPS: start-code + simple PPS :contentReference[oaicite:1]{index=1}
var pps = []byte{
	0x00, 0x00, 0x00, 0x01,
	0x68, 0xce, 0x38, 0x80,
}

// SQCIF resolution: 128×96 → 8×6 macroblocks
const (
	widthMB  = 128 / 16
	heightMB = 96 / 16
)

// Slice header & macroblock header from Hello264.c :contentReference[oaicite:2]{index=2}
var sliceHeader = []byte{0x00, 0x00, 0x00, 0x01, 0x05, 0x88, 0x84, 0x21, 0xa0}
var macroblockHeader = []byte{0x0d, 0x00}

func generateH264Elementary() []byte {
	buf := &bytes.Buffer{}
	buf.Write(sps)
	buf.Write(pps)
	buf.Write(sliceHeader)
	for y := 0; y < heightMB; y++ {
		for x := 0; x < widthMB; x++ {
			buf.Write(macroblockHeader)
			// Y plane: 16×16 zeros
			buf.Write(make([]byte, 16*16))
			// Cb/Cr: 8×8 zeros each
			buf.Write(make([]byte, 8*8))
			buf.Write(make([]byte, 8*8))
		}
	}
	buf.WriteByte(0x80) // slice stop
	return buf.Bytes()
}

// generateMP4 writes a “progressive” MP4 with a single H.264 frame,
// padding the ‘mdat’ to exactly targetSize bytes, using mp4ff only :contentReference[oaicite:3]{index=3}.
func GenerateMP4(path string, targetSize int64) error {
	// 1) Build raw H.264 ES
	h264 := generateH264Elementary()

	// 2) Create init segment (ftyp + moov)
	init := mp4.CreateEmptyInit()
	// Add one video track
	tid := init.Moov.Mvhd.NextTrackID
	init.Moov.Mvhd.NextTrackID++
	trak := mp4.CreateEmptyTrak(tid, 90000, "video", "und")
	init.Moov.AddChild(trak)
	init.Moov.Mvex.AddChild(mp4.CreateTrex(tid))
	// Set AVC config: remove the 4-byte start codes for avcC :contentReference[oaicite:4]{index=4}
	trak.SetAVCDescriptor("avc1",
		[][]byte{sps[4:]},
		[][]byte{pps[4:]},
		true)

	// 3) Open file and write init
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	ftyp := mp4.NewFtyp("isom", 0x200, []string{"isom", "iso2", "avc1", "mp41"})
	if err := ftyp.Encode(w); err != nil {
		return err
	}
	if err := init.Moov.Encode(w); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}

	// 4) Determine how much goes into mdat
	fi, _ := f.Stat()
	initSize := fi.Size()
	mdatSize := targetSize - initSize
	// must at least hold header+frame
	if mdatSize < int64(len(h264))+8 {
		return fmt.Errorf("size too small: need ≥ %d", initSize+int64(len(h264))+8)
	}

	// 5) Write mdat header
	hdr := make([]byte, 8)
	binary.BigEndian.PutUint32(hdr[0:4], uint32(mdatSize))
	copy(hdr[4:8], []byte("mdat"))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	// 6) Write frame
	if _, err := w.Write(h264); err != nil {
		return err
	}
	// 7) Pad with zeros
	pad := mdatSize - int64(len(h264)) - 8
	zero := make([]byte, 4096)
	for pad > 0 {
		n := int64(len(zero))
		if n > pad {
			n = pad
		}
		if _, err := w.Write(zero[:n]); err != nil {
			return err
		}
		pad -= n
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}

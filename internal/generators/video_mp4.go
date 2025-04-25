package generators

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/Eyevinn/mp4ff/mp4"
)

// NAL units from “World’s Smallest H.264 Encoder”
var sps = []byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
var pps = []byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xce, 0x38, 0x80}
var sliceHeader = []byte{0x00, 0x00, 0x00, 0x01, 0x05, 0x88, 0x84, 0x21, 0xa0}
var macroblockHeader = []byte{0x0d, 0x00}

// SQCIF: 128×96 → 8×6 macroblocks
const (
	widthMB  = 128 / 16
	heightMB = 96 / 16
)

// generateH264Elementary builds one blank I‐frame
func generateH264Elementary() []byte {
	buf := make([]byte, 0, 1024*10)
	buf = append(buf, sps...)
	buf = append(buf, pps...)
	buf = append(buf, sliceHeader...)
	// zeroed macroblocks
	for y := 0; y < heightMB; y++ {
		for x := 0; x < widthMB; x++ {
			buf = append(buf, macroblockHeader...)
			buf = append(buf, make([]byte, 16*16+8*8+8*8)...)
		}
	}
	buf = append(buf, 0x80) // slice stop
	return buf
}

// generateMP4 writes an MP4 of exactly targetSize bytes,
// with `repeats` blank frames and correct duration metadata.
func GenerateMP4(path string, targetSize int64) error {
	// 1) H.264 ES
	h264 := generateH264Elementary()
	hlen := int64(len(h264))

	// 2) Build init (ftyp+moov)
	init := mp4.CreateEmptyInit()
	tid := init.Moov.Mvhd.NextTrackID
	init.Moov.Mvhd.NextTrackID++
	trak := mp4.CreateEmptyTrak(tid, 90000, "video", "und")
	init.Moov.AddChild(trak)
	init.Moov.Mvex.AddChild(mp4.CreateTrex(tid))
	// give it our SPS/PPS in avcC
	trak.SetAVCDescriptor("avc1", [][]byte{sps[4:]}, [][]byte{pps[4:]}, true)

	// 3) Write init to file
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	// ftyp
	mp4.NewFtyp("isom", 0x200, []string{"isom", "iso2", "avc1", "mp41"}).Encode(w)
	// moov
	if err := init.Moov.Encode(w); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}

	// 4) Compute how many bytes for mdat
	fi, _ := f.Stat()
	initSize := fi.Size()
	mdatTotal := targetSize - initSize
	if mdatTotal < hlen+8 {
		return fmt.Errorf("target %d too small; need at least %d", targetSize, initSize+hlen+8)
	}
	payload := mdatTotal - 8

	// 5) Estimate repeats and leftover
	repeats := payload / hlen
	if repeats < 1 {
		repeats = 1
	}
	// Choose 25 fps → 90000/25 = 3600 time‐units/frame
	const fps = 25
	sampleDur := uint32(90000 / fps)
	totalDur := uint64(sampleDur) * uint64(repeats)

	// 6) Patch durations & STTS
	init.Moov.Mvhd.Duration = totalDur
	for _, tr := range init.Moov.Traks {
		tr.Tkhd.Duration = totalDur
		tr.Mdia.Mdhd.Duration = totalDur
		tr.Mdia.Minf.Stbl.Stts = &mp4.SttsBox{
			Version: 0, Flags: 0,
			SampleCount:     []uint32{uint32(repeats)},
			SampleTimeDelta: []uint32{sampleDur},
		}
	}

	// 7) Rewrite moov with new durations
	// Seek back and overwrite moov (simple approach: reopen file after ftyp, or
	// if streaming isn’t needed, rebuild init and re-encode both ftyp+moov).
	// For brevity, assume we re-encode both:
	f.Truncate(0)
	f.Seek(0, 0)
	w.Reset(f)
	mp4.NewFtyp("isom", 0x200, []string{"isom", "iso2", "avc1", "mp41"}).Encode(w)
	init.Moov.Encode(w)
	w.Flush()

	// 8) Write mdat header
	hdr := make([]byte, 8)
	binary.BigEndian.PutUint32(hdr[0:4], uint32(mdatTotal))
	copy(hdr[4:8], []byte("mdat"))
	if _, err := f.Write(hdr); err != nil {
		return err
	}

	// 9) Write frames
	for i := int64(0); i < repeats; i++ {
		if _, err := f.Write(h264); err != nil {
			return err
		}
	}

	// 10) Pad remainder
	rem := payload - (repeats * hlen)
	zero := make([]byte, 4096)
	for rem > 0 {
		n := int64(len(zero))
		if n > rem {
			n = rem
		}
		if _, err := f.Write(zero[:n]); err != nil {
			return err
		}
		rem -= n
	}
	return f.Close()
}

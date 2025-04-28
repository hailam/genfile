package dwg

import (
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/hailam/genfile/internal/adapters/factory"
	"github.com/hailam/genfile/internal/ports"
)

func init() {
	factory.RegisterGenerator(ports.FileTypeDWG, New()) //
}

func New() ports.FileGenerator {
	return &DWGGenerator{}
}

// DWGGenerator implements FileGenerator for DWG files
type DWGGenerator struct{}

// Generate creates a DWG file at outPath with sizeBytes length
func (g *DWGGenerator) Generate(outPath string, sizeBytes int64) error {
	// Open the output file for writing
	file, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// DWG version and sentinel constants
	var versionBytes = []byte("AC1032") // DWG version string for R2018&#8203;:contentReference[oaicite:1]{index=1}
	startSentinel := []byte{0x30, 0x84, 0xE0, 0xDC, 0x02, 0x21, 0xC7, 0x56, 0xA0, 0x83, 0x97, 0x47, 0xB1, 0x92, 0xCC, 0xA0}
	endSentinel := []byte{0x2B, 0x84, 0xDE, 0x31, 0xD7, 0x6C, 0x60, 0x40, 0xAC, 0xDB, 0xBF, 0xF6, 0xED, 0xC3, 0x55, 0xFE}
	// 108-byte magic XOR key for header directory encryption
	xorKey := [...]byte{
		0x29, 0x23, 0xBE, 0x84, 0xE1, 0x6C, 0xD6, 0xAE, 0x52, 0x90, 0x49, 0xF1, 0xF1, 0xBB, 0xE9, 0xEB,
		0xB3, 0xA6, 0xDB, 0x3C, 0x87, 0x0C, 0x3E, 0x99, 0x24, 0x5E, 0x0D, 0x1C, 0x06, 0xB7, 0x47, 0xDE,
		0xB3, 0x12, 0x4D, 0xC8, 0x43, 0xBB, 0x8B, 0xA6, 0x1F, 0x03, 0x5A, 0x7D, 0x09, 0x38, 0x25, 0x1F,
		0x5D, 0xD4, 0xCB, 0xFC, 0x96, 0xF5, 0x45, 0x3B, 0x13, 0x0D, 0x89, 0x0A, 0x1C, 0xDB, 0xAE, 0x32,
		0x20, 0x9A, 0x50, 0xEE, 0x40, 0x78, 0x36, 0xFD, 0x12, 0x49, 0x32, 0xF6, 0x9E, 0x7D, 0x49, 0xDC,
		0xAD, 0x4F, 0x14, 0xF2, 0x44, 0x40, 0x66, 0xD0, 0x6B, 0xC4, 0x30, 0xB7, 0x32, 0x3B, 0xA1, 0x22,
		0xF6, 0x22, 0x91, 0x9D, 0xE1, 0x8B, 0x1F, 0xDA, 0xB0, 0xCA, 0x99, 0x02}

	// Prepare buffers for each main section
	headerDir := make([]byte, 108) // 108-byte section directory (to encrypt)
	var summarySec, previewSec, headerSec, classesSec, objectsSec, handlesSec, freeSec []byte

	// --- Summary Info Section ---
	summarySec = append(summarySec, startSentinel...)
	// Minimal summary info: just pad to default 0x80 (128) bytes page size
	for len(summarySec) < 128-16 { // leave 16 bytes for end sentinel
		summarySec = append(summarySec, 0x00)
	}
	summarySec = append(summarySec, endSentinel...)
	summarySize := len(summarySec) // should be 128 bytes total

	// --- Preview Section ---
	previewSec = append(previewSec, startSentinel...)
	// Fill to 0x400 (1024) bytes with zeros (no preview image data)
	for len(previewSec) < 0x400-16 {
		previewSec = append(previewSec, 0x00)
	}
	previewSec = append(previewSec, endSentinel...)
	previewSize := len(previewSec) // 0x400 bytes

	// --- Classes Section ---
	classesSec = append(classesSec, startSentinel...)
	// No custom classes (empty section)
	classesSec = append(classesSec, endSentinel...)
	classesSize := len(classesSec) // just 32 bytes of sentinels

	// --- Header Variables Section ---
	headerSec = append(headerSec, startSentinel...)
	// (In a full implementation, header variables like $HANDSEED, $INSBASE, etc. would be written here.
	//  We omit detailed variables and only include the required sentinels.)
	headerSec = append(headerSec, endSentinel...)
	headerVarsSize := len(headerSec)

	// We will now build the Objects section (entities and table objects)

	// Helper type for handle index entries (for the Handles section)
	type objEntry struct {
		handle uint32
		offset uint32
	}
	var handleIndex []objEntry

	// --- Bit-level writer for object data ---
	type bitWriter struct {
		buf    []byte
		bitPos uint8 // bits used in current byte (0-7)
	}
	bw := &bitWriter{}
	flushByteAlign := func() {
		// Align to next byte boundary by padding with zero bits
		if bw.bitPos != 0 {
			bw.buf = append(bw.buf, 0x00) // any partial byte, pad remaining bits with 0
			bw.bitPos = 0
		}
	}
	writeBits := func(value uint64, bits uint8) {
		// Write exactly 'bits' bits (LSB of value) into the bit stream
		for bits > 0 {
			if bw.bitPos == 0 {
				// start a new byte in buffer
				bw.buf = append(bw.buf, 0x00)
			}
			space := uint8(8 - bw.bitPos) // free bit space in current byte
			if bits <= space {
				// all bits can fit into current byte
				//shift := bits
				shiftPos := space - bits // align value bits to MSB of available space
				mask := uint64((1 << bits) - 1)
				part := (value & mask) << shiftPos
				bw.buf[len(bw.buf)-1] |= byte(part)
				bw.bitPos += bits
				if bw.bitPos == 8 {
					bw.bitPos = 0 // byte filled up
				}
				bits = 0
			} else {
				// fill current byte and continue with remaining bits
				mask := uint64((1 << space) - 1)
				part := value & mask
				bw.buf[len(bw.buf)-1] |= byte(part) // fill remaining bits in this byte
				value >>= space
				bits -= space
				bw.bitPos = 0
			}
		}
	}
	// Helpers for bit-coded data types (BitShort, BitDouble)
	writeBitShort := func(val int) {
		// Write an integer in BITSHORT compressed form&#8203;:contentReference[oaicite:2]{index=2}
		if val == 0 {
			writeBits(0b10, 2) // '10' denotes 0
		} else if val == 256 {
			writeBits(0b11, 2) // '11' denotes 256
		} else if val >= 0 && val < 256 {
			writeBits(0b01, 2) // '01' prefix, one-byte follows
			writeBits(uint64(val), 8)
		} else {
			writeBits(0b00, 2) // '00' prefix, 16-bit follows
			// Write 16-bit little-endian representation
			writeBits(uint64(val&0xFF), 8)
			writeBits(uint64((val>>8)&0xFF), 8)
		}
	}
	writeBitDouble := func(val float64) {
		// Write a float64 in BITDOUBLE compressed form (special-case 0.0 and 1.0)&#8203;:contentReference[oaicite:3]{index=3}
		if val == 0.0 {
			writeBits(0b10, 2) // '10' for 0.0
		} else if val == 1.0 {
			writeBits(0b11, 2) // '11' for 1.0
		} else {
			writeBits(0b00, 2) // '00' prefix, full 64-bit follows
			bits := math.Float64bits(val)
			// Write 8 bytes (little-endian) for the double
			for i := 0; i < 8; i++ {
				writeBits(uint64((bits>>(8*i))&0xFF), 8)
			}
		}
	}
	writeHandleRef := func(refHandle uint32) {
		// Write a handle reference in variable-length format (1-byte length + handle bytes)
		if refHandle == 0 {
			// NULL handle reference
			bw.buf = append(bw.buf, 0x00)
			bw.bitPos = 0
			return
		}
		// Collect bytes of handle value (little-endian)
		handleValue := refHandle
		var bytes []byte
		for handleValue > 0 {
			bytes = append(bytes, byte(handleValue&0xFF))
			handleValue >>= 8
		}
		if len(bytes) == 0 {
			bytes = []byte{0x00}
		}
		bw.buf = append(bw.buf, byte(len(bytes)))
		bw.buf = append(bw.buf, bytes...)
		bw.bitPos = 0 // handle always written at byte boundary
	}

	// Pre-assign handles for essential table objects
	blockCtrlHandle := uint32(1)
	modelSpaceBlockHdl := uint32(2)
	layerCtrlHandle := uint32(3)
	layer0Handle := uint32(4)
	ltypeCtrlHandle := uint32(5)
	contLtypeHandle := uint32(6)
	// Start assigning entity handles from a higher number (e.g., 0x100) to clearly separate
	nextEntityHandle := uint32(0x100)

	objectsSec = append(objectsSec, startSentinel...)

	// --- Write BLOCK CONTROL object (type 0x30) ---
	bw.buf = nil
	bw.bitPos = 0
	bw.buf = append(bw.buf, 0x00, 0x00) // reserve 2 bytes for length
	writeBitShort(0x30)                 // type = BLOCK CONTROL (0x30)
	writeBitShort(1)                    // number of block records owned = 1 (ModelSpace only)
	flushByteAlign()
	writeHandleRef(0) // no owner (hard owner is NULL for top-level)
	// (No reactors, no XDictionary for table control objects in this minimal example)
	// Append CRC16 placeholder (2 bytes 0x0000)
	bw.buf = append(bw.buf, 0x00, 0x00)
	// Patch length field (exclude its 2 bytes and CRC2 bytes from count)
	objLen := len(bw.buf) - 4
	binary.LittleEndian.PutUint16(bw.buf[0:2], uint16(objLen))
	blockCtrlOffset := uint32(len(objectsSec))
	objectsSec = append(objectsSec, bw.buf...)
	handleIndex = append(handleIndex, objEntry{blockCtrlHandle, blockCtrlOffset})

	// --- Write BLOCK RECORD (Model Space) object (type 0x31) ---
	bw.buf = nil
	bw.bitPos = 0
	bw.buf = append(bw.buf, 0x00, 0x00)
	writeBitShort(0x31) // type = BLOCK RECORD (0x31)
	flushByteAlign()
	name := "*Model_Space"
	bw.buf = append(bw.buf, byte(len(name))) // Block name (text) length
	bw.buf = append(bw.buf, name...)         // "*Model_Space" name bytes
	writeBitShort(0)                         // flags (0 = model space, not anonymous, etc.)
	writeBitShort(0)                         // owned object count (placeholder, update later)
	flushByteAlign()
	writeHandleRef(blockCtrlHandle)     // hard owner: Block Control
	writeHandleRef(0)                   // no reactors
	writeHandleRef(0)                   // no XDictionary
	bw.buf = append(bw.buf, 0x00, 0x00) // CRC placeholder
	objLen = len(bw.buf) - 4
	binary.LittleEndian.PutUint16(bw.buf[0:2], uint16(objLen))
	modelSpaceBlockOffset := uint32(len(objectsSec))
	objectsSec = append(objectsSec, bw.buf...)
	handleIndex = append(handleIndex, objEntry{modelSpaceBlockHdl, modelSpaceBlockOffset})
	// Save index to update the owned object count later (after adding entities)
	//modelSpaceObjIndex := len(handleIndex) - 1

	// --- Write LAYER CONTROL object (0x32) ---
	bw.buf = nil
	bw.bitPos = 0
	bw.buf = append(bw.buf, 0x00, 0x00)
	writeBitShort(0x32) // type = LAYER CONTROL (0x32)
	writeBitShort(1)    // number of layers = 1
	flushByteAlign()
	writeHandleRef(0)                   // no owner (owned by NamedObjects in full DWG, but we omit NOD)
	bw.buf = append(bw.buf, 0x00, 0x00) // CRC placeholder
	objLen = len(bw.buf) - 4
	binary.LittleEndian.PutUint16(bw.buf[0:2], uint16(objLen))
	layerCtrlOffset := uint32(len(objectsSec))
	objectsSec = append(objectsSec, bw.buf...)
	handleIndex = append(handleIndex, objEntry{layerCtrlHandle, layerCtrlOffset})

	// --- Write LAYER "0" object (0x33) ---
	bw.buf = nil
	bw.bitPos = 0
	bw.buf = append(bw.buf, 0x00, 0x00)
	writeBitShort(0x33) // type = LAYER (0x33)
	flushByteAlign()
	name = "0"
	bw.buf = append(bw.buf, byte(len(name)))
	bw.buf = append(bw.buf, name...) // Layer name "0"
	writeBitShort(0)                 // flags (default = 0, layer is on/unlocked)
	writeBitShort(7)                 // color index = 7 (white)&#8203;:contentReference[oaicite:4]{index=4}
	flushByteAlign()
	writeHandleRef(contLtypeHandle) // linetype handle = Continuous
	// Plot style flags & lineweight: we assume defaults (ByLayer), which typically are stored as default codes.
	writeBitShort(0) // plot flags (0 = plot enabled)
	writeBitShort(0) // lineweight (0 = default weight)
	flushByteAlign()
	writeHandleRef(layerCtrlHandle)     // hard owner: Layer Control
	writeHandleRef(0)                   // no reactors
	writeHandleRef(0)                   // no XDictionary
	bw.buf = append(bw.buf, 0x00, 0x00) // CRC
	objLen = len(bw.buf) - 4
	binary.LittleEndian.PutUint16(bw.buf[0:2], uint16(objLen))
	layer0Offset := uint32(len(objectsSec))
	objectsSec = append(objectsSec, bw.buf...)
	handleIndex = append(handleIndex, objEntry{layer0Handle, layer0Offset})

	// --- Write LINETYPE CONTROL object (0x38) ---
	bw.buf = nil
	bw.bitPos = 0
	bw.buf = append(bw.buf, 0x00, 0x00)
	writeBitShort(0x38) // type = LTYPE CONTROL (0x38)
	writeBitShort(1)    // number of linetypes = 1
	flushByteAlign()
	writeHandleRef(0)                   // no owner (top-level or in NOD, omitted)
	bw.buf = append(bw.buf, 0x00, 0x00) // CRC
	objLen = len(bw.buf) - 4
	binary.LittleEndian.PutUint16(bw.buf[0:2], uint16(objLen))
	ltypeCtrlOffset := uint32(len(objectsSec))
	objectsSec = append(objectsSec, bw.buf...)
	handleIndex = append(handleIndex, objEntry{ltypeCtrlHandle, ltypeCtrlOffset})

	// --- Write LINETYPE "Continuous" object (0x39) ---
	bw.buf = nil
	bw.bitPos = 0
	bw.buf = append(bw.buf, 0x00, 0x00)
	writeBitShort(0x39) // type = LTYPE (0x39)
	flushByteAlign()
	name = "Continuous"
	bw.buf = append(bw.buf, byte(len(name)))
	bw.buf = append(bw.buf, name...) // Linetype name "Continuous"
	// Pattern data:
	writeBitDouble(0.0) // pattern length = 0.0 (continuous line)
	writeBitShort(0)    // number of dash segments = 0
	flushByteAlign()
	desc := "Solid"
	bw.buf = append(bw.buf, byte(len(desc)))
	bw.buf = append(bw.buf, desc...) // descriptive text "Solid"
	flushByteAlign()
	writeHandleRef(ltypeCtrlHandle)     // hard owner: Linetype Control
	writeHandleRef(0)                   // no reactors
	writeHandleRef(0)                   // no XDictionary
	bw.buf = append(bw.buf, 0x00, 0x00) // CRC
	objLen = len(bw.buf) - 4
	binary.LittleEndian.PutUint16(bw.buf[0:2], uint16(objLen))
	contLtypeOffset := uint32(len(objectsSec))
	objectsSec = append(objectsSec, bw.buf...)
	handleIndex = append(handleIndex, objEntry{contLtypeHandle, contLtypeOffset})

	// --- Generate random LINE/CIRCLE entities until file size is nearly reached ---
	rand.Seed(time.Now().UnixNano())
	entityCount := 0
	for {
		// Estimate current file length if we closed now (for break condition)
		currentLen := 128 + 108 + summarySize + previewSize + headerVarsSize + classesSize +
			len(objectsSec) + 16 /*objects end sentinel*/ +
			/*handles + free will be added later*/
			16 + 16 /*min sentinel overhead for handles & free*/
		if currentLen >= int(sizeBytes) {
			break
		}

		// Create either a LINE or CIRCLE entity
		isLine := rand.Intn(2) == 0
		bw.buf = nil
		bw.bitPos = 0
		bw.buf = append(bw.buf, 0x00, 0x00)
		if isLine {
			writeBitShort(0x13) // LINE entity type (0x13)&#8203;:contentReference[oaicite:5]{index=5}
		} else {
			writeBitShort(0x12) // CIRCLE entity type (0x12)
		}
		// Common entity data:
		flushByteAlign()
		writeHandleRef(layer0Handle) // layer = layer "0"
		// (Linetype omitted -> defaults to ByLayer, color omitted -> ByLayer)
		// Coordinates:
		if isLine {
			// LINE: start point (10), end point (11)
			x1 := float64(rand.Intn(2000) - 1000)
			y1 := float64(rand.Intn(2000) - 1000)
			z1 := float64(0)
			x2 := float64(rand.Intn(2000) - 1000)
			y2 := float64(rand.Intn(2000) - 1000)
			z2 := float64(0)
			writeBitDouble(x1)
			writeBitDouble(y1)
			writeBitDouble(z1)
			writeBitDouble(x2)
			writeBitDouble(y2)
			writeBitDouble(z2)
			writeBitDouble(0.0) // thickness (39) = 0
			writeBitDouble(0.0)
			writeBitDouble(0.0)
			writeBitDouble(1.0) // extrusion (210) = default (0,0,1)
		} else {
			// CIRCLE: center (10), radius (40)
			cx := float64(rand.Intn(2000) - 1000)
			cy := float64(rand.Intn(2000) - 1000)
			cz := float64(0)
			radius := float64(rand.Intn(500) + 1)
			writeBitDouble(cx)
			writeBitDouble(cy)
			writeBitDouble(cz)
			writeBitDouble(radius)
			writeBitDouble(0.0) // thickness = 0
			writeBitDouble(0.0)
			writeBitDouble(0.0)
			writeBitDouble(1.0) // extrusion = default
		}
		flushByteAlign()
		writeHandleRef(modelSpaceBlockHdl)  // hard owner: Model Space block record
		writeHandleRef(0)                   // no reactors
		writeHandleRef(0)                   // no XDictionary
		bw.buf = append(bw.buf, 0x00, 0x00) // CRC placeholder
		objLen = len(bw.buf) - 4
		binary.LittleEndian.PutUint16(bw.buf[0:2], uint16(objLen))

		// Check if adding this entity would exceed requested file size
		futureLen := len(objectsSec) + len(bw.buf) + 16 /*objects end sentinel*/ + 32 /*approx handles+free*/
		if 128+108+summarySize+previewSize+headerVarsSize+classesSize+futureLen > int(sizeBytes) {
			break // adding this entity would overshoot sizeBytes
		}
		// Append the entity to objects section
		entOffset := uint32(len(objectsSec))
		objectsSec = append(objectsSec, bw.buf...)
		handleIndex = append(handleIndex, objEntry{nextEntityHandle, entOffset})
		nextEntityHandle++
		entityCount++
	}

	// Update the Model Space block record's entity count now that we know entityCount
	// Easiest is to regenerate that object with the correct count and replace its bytes
	for _, entry := range handleIndex {
		if entry.handle == modelSpaceBlockHdl {
			// Rebuild the block record object with updated count
			bw.buf = nil
			bw.bitPos = 0
			bw.buf = append(bw.buf, 0x00, 0x00)
			writeBitShort(0x31)
			flushByteAlign()
			name := "*Model_Space"
			bw.buf = append(bw.buf, byte(len(name)))
			bw.buf = append(bw.buf, name...)
			writeBitShort(0)
			writeBitShort(entityCount) // updated owned object count
			flushByteAlign()
			writeHandleRef(blockCtrlHandle)
			writeHandleRef(0)
			writeHandleRef(0)
			bw.buf = append(bw.buf, 0x00, 0x00) // CRC
			objLen = len(bw.buf) - 4
			binary.LittleEndian.PutUint16(bw.buf[0:2], uint16(objLen))
			// Replace the bytes in objectsSec at the recorded offset
			objPos := entry.offset
			copy(objectsSec[objPos:objPos+uint32(len(bw.buf))], bw.buf)
			break
		}
	}

	// Close the Objects section
	objectsSec = append(objectsSec, endSentinel...)
	objectsSize := len(objectsSec)

	// --- Build HANDLES (object map) Section ---
	handlesSec = append(handlesSec, startSentinel...)
	// Sort handleIndex by handle value to list in ascending order
	sort.Slice(handleIndex, func(i, j int) bool {
		return handleIndex[i].handle < handleIndex[j].handle
	})
	for _, entry := range handleIndex {
		// Write handle (with length and bytes)
		hVal := entry.handle
		var hBytes []byte
		for {
			hBytes = append(hBytes, byte(hVal&0xFF))
			hVal >>= 8
			if hVal == 0 {
				break
			}
		}
		handlesSec = append(handlesSec, byte(len(hBytes)))
		handlesSec = append(handlesSec, hBytes...) // LSB-first
		// Write 4-byte offset into AcDbObjects section
		off := entry.offset
		offBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(offBytes, off)
		handlesSec = append(handlesSec, offBytes...)
	}
	handlesSec = append(handlesSec, endSentinel...)
	handlesSize := len(handlesSec)

	// --- Build FREE SPACE Section ---
	freeSec = append(freeSec, startSentinel...)
	// Determine how many padding bytes are needed to reach exactly sizeBytes
	currentLength := 128 + 108 + summarySize + previewSize + headerVarsSize + classesSize + objectsSize + handlesSize + len(freeSec) + len(endSentinel)
	if currentLength < int(sizeBytes) {
		padBytes := int(sizeBytes) - currentLength
		for i := 0; i < padBytes; i++ {
			freeSec = append(freeSec, 0x00)
		}
	}
	freeSec = append(freeSec, endSentinel...)
	freeSize := len(freeSec)

	// --- Build Header Section Directory (108 bytes, to be XOR-encrypted) ---
	pos := 0
	writeDirEntry := func(hash uint32, offset uint32, size uint32, encryption uint8, encoding uint8) {
		binary.LittleEndian.PutUint32(headerDir[pos:pos+4], hash)
		binary.LittleEndian.PutUint32(headerDir[pos+4:pos+8], offset)
		binary.LittleEndian.PutUint32(headerDir[pos+8:pos+12], size)
		headerDir[pos+12] = encryption
		headerDir[pos+13] = encoding
		pos += 14
	}
	// Compute starting file offsets for each section after header+directory (236 bytes):
	baseOffset := uint32(128 + 108) // offset in file after initial header+dir
	// We use known section hash codes from ODA spec and mark encoding=1 (uncompressed), encryption=0 for simplicity
	writeDirEntry(0x32B803D9, uint32(baseOffset)+uint32(previewSize)+uint32(summarySize), uint32(headerVarsSize), 0, 1)                                                                              // AcDb:Header (header variables)&#8203;:contentReference[oaicite:6]{index=6}
	writeDirEntry(0x3F54045F, uint32(baseOffset)+uint32(previewSize)+uint32(summarySize)+uint32(headerVarsSize), uint32(classesSize), 0, 1)                                                          // AcDb:Classes
	writeDirEntry(0x674C05A9, uint32(baseOffset)+uint32(previewSize)+uint32(summarySize)+uint32(headerVarsSize)+uint32(classesSize), uint32(objectsSize), 0, 1)                                      // AcDb:AcDbObjects
	writeDirEntry(0x3F6E0450, uint32(baseOffset)+uint32(previewSize)+uint32(summarySize)+uint32(headerVarsSize)+uint32(classesSize)+uint32(objectsSize), uint32(handlesSize), 0, 1)                  // AcDb:Handles
	writeDirEntry(0x77E2061F, uint32(baseOffset)+uint32(previewSize)+uint32(summarySize)+uint32(headerVarsSize)+uint32(classesSize)+uint32(objectsSize)+uint32(handlesSize), uint32(freeSize), 0, 1) // AcDb:ObjFreeSpace
	// (Preview & Summary are referenced via header pointers rather than directory in this version, but we include them for completeness)
	writeDirEntry(0x40AA0473, baseOffset, uint32(previewSize), 0, 1)                     // AcDb:Preview
	writeDirEntry(0x717A060F, baseOffset+uint32(previewSize), uint32(summarySize), 0, 1) // AcDb:SummaryInfo
	// Pad any remaining bytes (if any) with 0
	for pos < len(headerDir) {
		headerDir[pos] = 0x00
		pos++
	}
	// XOR-encrypt the 108-byte directory using the magic key
	for i := 0; i < len(headerDir); i++ {
		headerDir[i] ^= xorKey[i]
	}

	// --- Write 128-byte File Header ---
	header := make([]byte, 128)
	// 0x00-0x05: version string "AC1032"
	copy(header[0:6], versionBytes)
	// 0x06-0x0B: six 0x00 bytes (R2018: these include the maintenance version at 0x0C)&#8203;:contentReference[oaicite:7]{index=7}
	header[0x0C] = 0x01 // ACAD maintenance version (set 0x01)&#8203;:contentReference[oaicite:8]{index=8}
	// 0x0D-0x10: pointer to preview image data (absolute address of preview section begin)&#8203;:contentReference[oaicite:9]{index=9}
	binary.LittleEndian.PutUint32(header[0x0D:0x11], uint32(128+108))
	// 0x11: application version (e.g., 0x1C for ACAD 2018; use 0x1C as an example)
	header[0x11] = 0x1C
	// 0x12: application maintenance version
	header[0x12] = 0x00
	// 0x13-0x14: codepage (e.g., 0x04E4 for ANSI_1252)&#8203;:contentReference[oaicite:10]{index=10}
	header[0x13] = 0xE4
	header[0x14] = 0x04
	// 0x15-0x17: 3 reserved 0x00 bytes (section locator count not used in new format)&#8203;:contentReference[oaicite:11]{index=11}
	// 0x18-0x1B: security flags = 0 (no encryption)&#8203;:contentReference[oaicite:12]{index=12}
	// 0x1C-0x1F: unknown (0)&#8203;:contentReference[oaicite:13]{index=13}
	// 0x20-0x23: pointer to SummaryInfo section&#8203;:contentReference[oaicite:14]{index=14}
	binary.LittleEndian.PutUint32(header[0x20:0x24], uint32(128+108+previewSize))
	// 0x24-0x27: pointer to VBA section (0 = none)
	// 0x28-0x2B: constant 0x00000080 (as observed in spec)&#8203;:contentReference[oaicite:15]{index=15}
	binary.LittleEndian.PutUint32(header[0x28:0x2C], 0x00000080)
	// 0x2C-0x7F: 84 bytes of 0x00 padding
	// Write header to file
	if _, err = file.Write(header); err != nil {
		return err
	}
	// Write the encrypted 108-byte directory
	if _, err = file.Write(headerDir); err != nil {
		return err
	}

	// --- Write all sections to file in sequence ---
	if _, err = file.Write(previewSec); err != nil {
		return err
	}
	if _, err = file.Write(summarySec); err != nil {
		return err
	}
	if _, err = file.Write(headerSec); err != nil {
		return err
	}
	if _, err = file.Write(classesSec); err != nil {
		return err
	}
	if _, err = file.Write(objectsSec); err != nil {
		return err
	}
	if _, err = file.Write(handlesSec); err != nil {
		return err
	}
	if _, err = file.Write(freeSec); err != nil {
		return err
	}

	return nil
}
